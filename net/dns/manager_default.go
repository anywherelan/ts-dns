// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

//go:build !linux && !freebsd && !openbsd && !windows && !darwin

package dns

import (
	"github.com/anywherelan/ts-dns/control/controlknobs"
	"github.com/anywherelan/ts-dns/health"
	"github.com/anywherelan/ts-dns/types/logger"
)

// NewOSConfigurator creates a new OS configurator.
//
// The health tracker and the knobs may be nil and are ignored on this platform.
func NewOSConfigurator(logger.Logf, *health.Tracker, *controlknobs.Knobs, string) (OSConfigurator, error) {
	return NewNoopManager()
}
