// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

// Package pkey defines the keys used to store system policies in the registry.
// Only the keys used by net/dns are kept.
package pkey

// Key is a string that uniquely identifies a policy and must remain unchanged
// once established and documented for a given policy setting.
type Key string

const (
	// EnableDNSRegistration is a string value that can be set to "always", "never"
	// or "user-decides". It controls whether DNS registration and dynamic DNS
	// updates are enabled for the network interface. The default is "never".
	// It is only used on Windows.
	EnableDNSRegistration Key = "EnableDNSRegistration"
)
