// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package policyclient

import "github.com/anywherelan/ts-dns/util/syspolicy/pkey"

// PolicySnapshot is a stub when syspolicy is omitted from the build.
type policySnapshot struct{}

func GetStringArrayFromSnapshot(snapshot *PolicySnapshot, key pkey.Key) []string {
	return nil
}
