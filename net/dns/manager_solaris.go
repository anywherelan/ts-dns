// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package dns

import (
	"github.com/anywherelan/ts-dns/control/controlknobs"
	"github.com/anywherelan/ts-dns/health"
	"github.com/anywherelan/ts-dns/types/logger"
	"github.com/anywherelan/ts-dns/util/eventbus"
	"github.com/anywherelan/ts-dns/util/syspolicy/policyclient"
)

func NewOSConfigurator(logf logger.Logf, health *health.Tracker, bus *eventbus.Bus, _ policyclient.Client, _ *controlknobs.Knobs, iface string) (OSConfigurator, error) {
	return newDirectManager(logf, health, bus), nil
}
