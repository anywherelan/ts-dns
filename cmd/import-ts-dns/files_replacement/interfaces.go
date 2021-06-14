// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package interfaces contains helpers for looking up system network interfaces.
package interfaces

import (
	"net"

	"inet.af/netaddr"
)

// Tailscale returns the current machine's Tailscale interface, if any.
// If none is found, all zero values are returned.
// A non-nil error is only returned on a problem listing the system interfaces.
func Tailscale() ([]netaddr.IP, *net.Interface, error) {
    return nil, nil, nil
}
