// Copyright (c) 2021 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"errors"

	"github.com/anywherelan/ts-dns/util/dnsname"
	"inet.af/netaddr"
)

// An OSConfigurator applies DNS settings to the operating system.
type OSConfigurator interface {
	// SetDNS updates the OS's DNS configuration to match cfg.
	// If cfg is the zero value, all Tailscale-related DNS
	// configuration is removed.
	// SetDNS must not be called after Close.
	SetDNS(cfg OSConfig) error
	// SupportsSplitDNS reports whether the configurator is capable of
	// installing a resolver only for specific DNS suffixes. If false,
	// the configurator can only set a global resolver.
	SupportsSplitDNS() bool
	// GetBaseConfig returns the OS's "base" configuration, i.e. the
	// resolver settings the OS would use without Tailscale
	// contributing any configuration.
	// GetBaseConfig must return the tailscale-free base config even
	// after SetDNS has been called to set a Tailscale configuration.
	// Only works when SupportsSplitDNS=false.

	// Implementations that don't support getting the base config must
	// return ErrGetBaseConfigNotSupported.
	GetBaseConfig() (OSConfig, error)
	// Close removes Tailscale-related DNS configuration from the OS.
	Close() error
}

// OSConfig is an OS DNS configuration.
type OSConfig struct {
	// Nameservers are the IP addresses of the nameservers to use.
	Nameservers []netaddr.IP
	// SearchDomains are the domain suffixes to use when expanding
	// single-label name queries. SearchDomains is additive to
	// whatever non-Tailscale search domains the OS has.
	SearchDomains []dnsname.FQDN
	// MatchDomains are the DNS suffixes for which Nameservers should
	// be used. If empty, Nameservers is installed as the "primary" resolver.
	// A non-empty MatchDomains requests a "split DNS" configuration
	// from the OS, which will only work with OSConfigurators that
	// report SupportsSplitDNS()=true.
	MatchDomains []dnsname.FQDN
}

func (o OSConfig) IsZero() bool {
	return len(o.Nameservers) == 0 && len(o.SearchDomains) == 0 && len(o.MatchDomains) == 0
}

func (a OSConfig) Equal(b OSConfig) bool {
	if len(a.Nameservers) != len(b.Nameservers) {
		return false
	}
	if len(a.SearchDomains) != len(b.SearchDomains) {
		return false
	}
	if len(a.MatchDomains) != len(b.MatchDomains) {
		return false
	}

	for i := range a.Nameservers {
		if a.Nameservers[i] != b.Nameservers[i] {
			return false
		}
	}
	for i := range a.SearchDomains {
		if a.SearchDomains[i] != b.SearchDomains[i] {
			return false
		}
	}
	for i := range a.MatchDomains {
		if a.MatchDomains[i] != b.MatchDomains[i] {
			return false
		}
	}

	return true
}

// ErrGetBaseConfigNotSupported is the error
// OSConfigurator.GetBaseConfig returns when the OSConfigurator
// doesn't support reading the underlying configuration out of the OS.
var ErrGetBaseConfigNotSupported = errors.New("getting OS base config is not supported")
