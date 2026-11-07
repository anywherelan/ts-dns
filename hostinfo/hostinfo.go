// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package hostinfo answers questions about the host environment.
package hostinfo

import (
	"bytes"
	"os/exec"
	"runtime"
)

// IsSELinuxEnforcing reports whether SELinux is in "Enforcing" mode.
func IsSELinuxEnforcing() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	out, _ := exec.Command("getenforce").Output()
	return string(bytes.TrimSpace(out)) == "Enforcing"
}
