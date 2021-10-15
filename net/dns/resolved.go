// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux
// +build linux

package dns

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/anywherelan/ts-dns/types/logger"
	"github.com/anywherelan/ts-dns/util/dnsname"
	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"
)

// resolvedListenAddr is the listen address of the resolved stub resolver.
//
// We only consider resolved to be the system resolver if the stub resolver is;
// that is, if this address is the sole nameserver in /etc/resolved.conf.
// In other cases, resolved may be managing the system DNS configuration directly.
// Then the nameserver list will be a concatenation of those for all
// the interfaces that register their interest in being a default resolver with
//   SetLinkDomains([]{{"~.", true}, ...})
// which includes at least the interface with the default route, i.e. not us.
// This does not work for us: there is a possibility of getting NXDOMAIN
// from the other nameservers before we are asked or get a chance to respond.
// We consider this case as lacking resolved support and fall through to dnsDirect.
//
// While it may seem that we need to read a config option to get at this,
// this address is, in fact, hard-coded into resolved.
var resolvedListenAddr = netaddr.IPv4(127, 0, 0, 53)

var errNotReady = errors.New("interface not ready")

type resolvedLinkNameserver struct {
	Family  int32
	Address []byte
}

type resolvedLinkDomain struct {
	Domain      string
	RoutingOnly bool
}

// isResolvedActive determines if resolved is currently managing system DNS settings.
func isResolvedActive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), reconfigTimeout)
	defer cancel()

	conn, err := dbus.SystemBus()
	if err != nil {
		// Probably no DBus on the system, or we're not allowed to use
		// it. Cannot control resolved.
		return false
	}

	rd := conn.Object("org.freedesktop.resolve1", dbus.ObjectPath("/org/freedesktop/resolve1"))
	call := rd.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0)
	if call.Err != nil {
		// Can't talk to resolved.
		return false
	}

	config, err := newDirectManager().readResolvConf()
	if err != nil {
		return false
	}

	// The sole nameserver must be the systemd-resolved stub.
	if len(config.Nameservers) == 1 && config.Nameservers[0] == resolvedListenAddr {
		return true
	}

	return false
}

// resolvedManager is an OSConfigurator which uses the systemd-resolved DBus API.
type resolvedManager struct {
	logf     logger.Logf
	ifidx    int
	resolved dbus.BusObject
}

func newResolvedManager(logf logger.Logf, interfaceName string) (*resolvedManager, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, err
	}

	return &resolvedManager{
		logf:     logf,
		ifidx:    iface.Index,
		resolved: conn.Object("org.freedesktop.resolve1", dbus.ObjectPath("/org/freedesktop/resolve1")),
	}, nil
}

func (m *resolvedManager) SetDNS(config OSConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), reconfigTimeout)
	defer cancel()

	var linkNameservers = make([]resolvedLinkNameserver, len(config.Nameservers))
	for i, server := range config.Nameservers {
		ip := server.As16()
		if server.Is4() {
			linkNameservers[i] = resolvedLinkNameserver{
				Family:  unix.AF_INET,
				Address: ip[12:],
			}
		} else {
			linkNameservers[i] = resolvedLinkNameserver{
				Family:  unix.AF_INET6,
				Address: ip[:],
			}
		}
	}

	err := m.resolved.CallWithContext(
		ctx, "org.freedesktop.resolve1.Manager.SetLinkDNS", 0,
		m.ifidx, linkNameservers,
	).Store()
	if err != nil {
		return fmt.Errorf("setLinkDNS: %w", err)
	}

	linkDomains := make([]resolvedLinkDomain, 0, len(config.SearchDomains)+len(config.MatchDomains))
	seenDomains := map[dnsname.FQDN]bool{}
	for _, domain := range config.SearchDomains {
		if seenDomains[domain] {
			continue
		}
		seenDomains[domain] = true
		linkDomains = append(linkDomains, resolvedLinkDomain{
			Domain:      domain.WithTrailingDot(),
			RoutingOnly: false,
		})
	}
	for _, domain := range config.MatchDomains {
		if seenDomains[domain] {
			// Search domains act as both search and match in
			// resolved, so it's correct to skip.
			continue
		}
		seenDomains[domain] = true
		linkDomains = append(linkDomains, resolvedLinkDomain{
			Domain:      domain.WithTrailingDot(),
			RoutingOnly: true,
		})
	}
	if len(config.MatchDomains) == 0 && len(config.Nameservers) > 0 {
		// Caller requested full DNS interception, install a
		// routing-only root domain.
		linkDomains = append(linkDomains, resolvedLinkDomain{
			Domain:      ".",
			RoutingOnly: true,
		})
	}

	err = m.resolved.CallWithContext(
		ctx, "org.freedesktop.resolve1.Manager.SetLinkDomains", 0,
		m.ifidx, linkDomains,
	).Store()
	if err != nil {
		return fmt.Errorf("setLinkDomains: %w", err)
	}

	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.SetLinkDefaultRoute", 0, m.ifidx, len(config.MatchDomains) == 0); call.Err != nil {
		return fmt.Errorf("setLinkDefaultRoute: %w", call.Err)
	}

	// Some best-effort setting of things, but resolved should do the
	// right thing if these fail (e.g. a really old resolved version
	// or something).

	// Disable LLMNR, we don't do multicast.
	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.SetLinkLLMNR", 0, m.ifidx, "no"); call.Err != nil {
		m.logf("[v1] failed to disable LLMNR: %v", call.Err)
	}

	// Disable mdns.
	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.SetLinkMulticastDNS", 0, m.ifidx, "no"); call.Err != nil {
		m.logf("[v1] failed to disable mdns: %v", call.Err)
	}

	// We don't support dnssec consistently right now, force it off to
	// avoid partial failures when we split DNS internally.
	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.SetLinkDNSSEC", 0, m.ifidx, "no"); call.Err != nil {
		m.logf("[v1] failed to disable DNSSEC: %v", call.Err)
	}

	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.SetLinkDNSOverTLS", 0, m.ifidx, "no"); call.Err != nil {
		m.logf("[v1] failed to disable DoT: %v", call.Err)
	}

	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.FlushCaches", 0); call.Err != nil {
		m.logf("failed to flush resolved DNS cache: %v", call.Err)
	}

	return nil
}

func (m *resolvedManager) SupportsSplitDNS() bool {
	return true
}

func (m *resolvedManager) GetBaseConfig() (OSConfig, error) {
	return OSConfig{}, ErrGetBaseConfigNotSupported
}

func (m *resolvedManager) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), reconfigTimeout)
	defer cancel()

	if call := m.resolved.CallWithContext(ctx, "org.freedesktop.resolve1.Manager.RevertLink", 0, m.ifidx); call.Err != nil {
		return fmt.Errorf("RevertLink: %w", call.Err)
	}

	return nil
}
