// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package health is a registry for other packages to report & check
// overall health status of the node.
package health

import (
	"time"
)

// Tracker tracks the health of various Tailscale subsystems,
// comparing each subsystems' state with each other to make sure
// they're consistent based on the user's intended state.
type Tracker struct {
}

// SetDNSOSHealth sets the state of the net/dns.OSConfigurator
//
// Deprecated: Warnables should be preferred over Subsystem errors.
func (t *Tracker) SetDNSOSHealth(err error) {}

// SetDNSManagerHealth sets the state of the Linux net/dns manager's
// discovery of the /etc/resolv.conf situation.
//
// Deprecated: Warnables should be preferred over Subsystem errors.
func (t *Tracker) SetDNSManagerHealth(err error) {}

// SetHealthy removes any warningState for the given Warnable.
func (t *Tracker) SetHealthy(w *Warnable) {
}

// SetUnhealthy sets a warningState for the given Warnable with the provided Args, and should be
// called when a Warnable becomes unhealthy, or its unhealthy status needs to be updated.
// SetUnhealthy takes ownership of args. The args can be nil if no additional information is
// needed for the unhealthy state.
func (t *Tracker) SetUnhealthy(w *Warnable, args Args) {
}

// Subsystem is the name of a subsystem whose health can be monitored.
type Subsystem string

const (
	// SysDNSOS is the name of the net/dns OSConfigurator subsystem.
	SysDNSOS = Subsystem("dns-os")

	// SysDNSManager is the name of the net/dns manager subsystem.
	SysDNSManager = Subsystem("dns-manager")
)

// WarnableCode is a string that distinguishes each Warnable from others. It is globally unique within
// the program.
type WarnableCode string

// A Warnable is something that we might want to warn the user about, or not. A Warnable is either
// in an healthy or unhealth state. A Warnable is unhealthy if the Tracker knows about a WarningState
// affecting the Warnable.
// In most cases, Warnables are components of the backend (for instance, "DNS" or "Magicsock").
// Warnables are similar to the Subsystem type previously used in this package, but they provide
// a unique identifying code for each Warnable, along with more metadata that makes it easier for
// a GUI to display the Warnable in a user-friendly way.
type Warnable struct {
	// Code is a string that uniquely identifies this Warnable across the entire Tailscale backend,
	// and can be mapped to a user-displayable localized string.
	Code WarnableCode
	// Title is a string that the GUI uses as title for any message involving this Warnable. The title
	// should be short and fit in a single line.
	Title string
	// Text is a function that generates an extended string that the GUI will display to the user when
	// this Warnable is in an unhealthy state. The function can use the Args map to provide dynamic
	// information to the user.
	Text func(args Args) string
	// Severity is the severity of the Warnable, which the GUI can use to determine how to display it.
	// For instance, a Warnable with SeverityHigh could trigger a modal view, while a Warnable with
	// SeverityLow could be displayed in a less intrusive way.
	// TODO(angott): turn this into a SeverityFunc, which allows the Warnable to change its severity based on
	// the Args of the unhappy state, just like we do in the Text function.
	Severity Severity
	// DependsOn is a set of Warnables that this Warnable depends, on and need to be healthy
	// before this Warnable can also be healthy again. The GUI can use this information to ignore
	// this Warnable if one of its dependencies is unhealthy.
	DependsOn []*Warnable

	// MapDebugFlag is a MapRequest.DebugFlag that is sent to control when this Warnable is unhealthy
	//
	// Deprecated: this is only used in one case, and will be removed in a future PR
	MapDebugFlag string

	// ImpactsConnectivity is whether this Warnable in an unhealthy state will impact the user's
	// ability to connect to the Internet or other nodes on the tailnet. On platforms where
	// the client GUI supports a tray icon, the client will display an exclamation mark
	// on the tray icon when ImpactsConnectivity is set to true and the Warnable is unhealthy.
	ImpactsConnectivity bool

	// TimeToVisible is the Duration that the Warnable has to be in an unhealthy state before it
	// should be surfaced as unhealthy to the user. This is used to prevent transient errors from being
	// displayed to the user.
	TimeToVisible time.Duration
}

// Arg is a type for the key to be used in the Args of a Warnable.
type Arg string

const (
	// ArgAvailableVersion provides an update notification Warnable with the available version of the Tailscale client.
	ArgAvailableVersion Arg = "available-version"

	// ArgCurrentVersion provides an update notification Warnable with the current version of the Tailscale client.
	ArgCurrentVersion Arg = "current-version"

	// ArgDuration provides a Warnable with how long the Warnable has been in an unhealthy state.
	ArgDuration Arg = "duration"

	// ArgError provides a Warnable with the underlying error behind an unhealthy state.
	ArgError Arg = "error"

	// ArgMagicsockFunctionName provides a Warnable with the name of the Magicsock function that caused the unhealthy state.
	ArgMagicsockFunctionName Arg = "magicsock-function-name"

	// ArgDERPRegionID provides a Warnable with the ID of a DERP server involved in the unhealthy state.
	ArgDERPRegionID Arg = "derp-region-id"

	// ArgDERPRegionName provides a Warnable with the name of a DERP server involved in the unhealthy state.
	// It is used to show a more friendly message like "the Seattle relay server failed to connect" versus
	// "relay server 10 failed to connect".
	ArgDERPRegionName Arg = "derp-region-name"

	// ArgServerName provides a Warnable with the hostname of a server involved in the unhealthy state.
	ArgServerName Arg = "server-name"
)

// StaticMessage returns a function that always returns the input string, to be used in
// simple Warnables that do not use the Args map to generate their Text.
func StaticMessage(s string) func(Args) string {
	return func(Args) string { return s }
}

// Severity represents how serious an error is. Each GUI interprets this severity value in different ways,
// to surface the error in a more or less visible way. For instance, the macOS GUI could change its menubar
// icon to display an exclamation mark and present a modal notification for SeverityHigh warnings, but not
// for SeverityLow messages, which would only appear in the Settings window.
type Severity string

const (
	// SeverityHigh is the highest severity level, used for critical errors that need immediate attention.
	// On platforms where the client GUI can deliver notifications, a SeverityHigh Warnable will trigger
	// a modal notification.
	SeverityHigh Severity = "high"
	// SeverityMedium is used for errors that are important but not critical. This won't trigger a modal
	// notification, however it will be displayed in a more visible way than a SeverityLow Warnable.
	SeverityMedium Severity = "medium"
	// SeverityLow is used for less important notices that don't need immediate attention. The user will
	// have to go to a Settings window, or another "hidden" GUI location to see these messages.
	SeverityLow Severity = "low"
)

// Args is a map of Args to string values that can be used to provide parameters regarding
// the unhealthy state of a Warnable.
// For instance, if you have a Warnable to track the health of DNS lookups, here you can include
// the hostname that failed to resolve, or the IP address of the DNS server that has been failing
// to respond. You can then use these parameters in the Text function of the Warnable to provide a detailed
// error message to the user.
type Args map[Arg]string

func Register(w *Warnable) *Warnable {
	return w
}
