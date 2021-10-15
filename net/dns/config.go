// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"sort"

	"github.com/anywherelan/ts-dns/types/dnstype"
	"github.com/anywherelan/ts-dns/util/dnsname"
	"inet.af/netaddr"
)

// Config is a DNS configuration.
type Config struct {
	// DefaultResolvers are the DNS resolvers to use for DNS names
	// which aren't covered by more specific per-domain routes below.
	// If empty, the OS's default resolvers (the ones that predate
	// Tailscale altering the configuration) are used.
	DefaultResolvers []dnstype.Resolver
	// Routes maps a DNS suffix to the resolvers that should be used
	// for queries that fall within that suffix.
	// If a query doesn't match any entry in Routes, the
	// DefaultResolvers are used.
	// A Routes entry with no resolvers means the route should be
	// authoritatively answered using the contents of Hosts.
	Routes map[dnsname.FQDN][]dnstype.Resolver
	// SearchDomains are DNS suffixes to try when expanding
	// single-label queries.
	SearchDomains []dnsname.FQDN
	// Hosts maps DNS FQDNs to their IPs, which can be a mix of IPv4
	// and IPv6.
	// Queries matching entries in Hosts are resolved locally by
	// 100.100.100.100 without leaving the machine.
	// Adding an entry to Hosts merely creates the record. If you want
	// it to resolve, you also need to add appropriate routes to
	// Routes.
	Hosts map[dnsname.FQDN][]netaddr.IP
}

// needsAnyResolvers reports whether c requires a resolver to be set
// at the OS level.
func (c Config) needsOSResolver() bool {
	return c.hasDefaultResolvers() || c.hasRoutes()
}

func (c Config) hasRoutes() bool {
	return len(c.Routes) > 0
}

// hasDefaultIPResolversOnly reports whether the only resolvers in c are
// DefaultResolvers, and that those resolvers are simple IP addresses.
func (c Config) hasDefaultIPResolversOnly() bool {
	if !c.hasDefaultResolvers() || c.hasRoutes() {
		return false
	}
	for _, r := range c.DefaultResolvers {
		if ipp, err := netaddr.ParseIPPort(r.Addr); err == nil && ipp.Port() == 53 {
			continue
		}
		if _, err := netaddr.ParseIP(r.Addr); err != nil {
			return false
		}
	}
	return true
}

func (c Config) hasDefaultResolvers() bool {
	return len(c.DefaultResolvers) > 0
}

// singleResolverSet returns the resolvers used by c.Routes if all
// routes use the same resolvers, or nil if multiple sets of resolvers
// are specified.
func (c Config) singleResolverSet() []dnstype.Resolver {
	var (
		prev            []dnstype.Resolver
		prevInitialized bool
	)
	for _, resolvers := range c.Routes {
		if !prevInitialized {
			prev = resolvers
			prevInitialized = true
			continue
		}
		if !sameResolverNames(prev, resolvers) {
			return nil
		}
	}
	return prev
}

// matchDomains returns the list of match suffixes needed by Routes.
func (c Config) matchDomains() []dnsname.FQDN {
	ret := make([]dnsname.FQDN, 0, len(c.Routes))
	for suffix := range c.Routes {
		ret = append(ret, suffix)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].WithTrailingDot() < ret[j].WithTrailingDot()
	})
	return ret
}

func sameResolverNames(a, b []dnstype.Resolver) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Addr != b[i].Addr {
			return false
		}
		if !sameIPs(a[i].BootstrapResolution, b[i].BootstrapResolution) {
			return false
		}
	}
	return true
}

func sameIPs(a, b []netaddr.IP) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
