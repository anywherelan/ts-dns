// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package dns

import (
	"errors"
	"fmt"
	"time"

	"tailscale.com/health"
)

var osConfigurationSetWarnable = health.Register(&health.Warnable{
	Code:  "dns-set-os-config-failed",
	Title: "Failed to set system DNS configuration",
	Text: func(args health.Args) string {
		return fmt.Sprintf("Failed to set the DNS configuration of your device: %v", args[health.ArgError])
	},
	Severity: health.SeverityMedium,
})

// errEmptyBaseConfig is returned when the OS has no upstream resolvers
// to forward the default route to. See
// https://github.com/tailscale/tailscale/issues/20341
var errEmptyBaseConfig = errors.New("no upstream resolvers in OS base config")

// TrampleDNS is an event indicating we detected that DNS config was
// overwritten by another process.
type TrampleDNS struct {
	LastTrample       time.Time
	TramplesInTimeout int64
}
