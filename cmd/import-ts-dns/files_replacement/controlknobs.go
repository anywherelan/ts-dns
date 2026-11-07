// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package controlknobs contains client options configurable from control which can be turned on
// or off. The ability to turn options on and off is for incrementally adding features in.
package controlknobs

import (
	"sync/atomic"
)

// Knobs is the set of knobs that the control plane's coordination server can
// adjust at runtime.
type Knobs struct {
	// DisableLocalDNSOverrideViaNRPT indicates that the node's DNS manager should not
	// create a default (catch-all) Windows NRPT rule when "Override local DNS" is enabled.
	// Without this rule, Windows 8.1 and newer devices issue parallel DNS requests to DNS servers
	// associated with all network adapters, even when "Override local DNS" is enabled and/or
	// a Mullvad exit node is being used, resulting in DNS leaks.
	// We began creating this rule on 2024-06-14, and this knob
	// allows us to disable the new behavior remotely if needed.
	DisableLocalDNSOverrideViaNRPT atomic.Bool

	// DisableHostsFileUpdates indicates that the node's DNS manager should not create
	// hosts file entries when it normally would, such as when we're not the primary
	// resolver on Windows or when the host is domain-joined and its primary domain
	// takes precedence over MagicDNS. As of 2026-02-13, it is only used on Windows.
	DisableHostsFileUpdates atomic.Bool
}
