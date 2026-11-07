// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package buildfeatures reports which features are compiled into the binary.
// Upstream generates a pair of files per feature, here only the used ones are kept.
package buildfeatures

// HasDNS is whether the binary was built with support for modular feature "DNS configuration and resolution".
const HasDNS = true
