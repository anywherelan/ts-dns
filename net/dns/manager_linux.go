// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/anywherelan/ts-dns/types/logger"
	"github.com/anywherelan/ts-dns/util/cmpver"
	"github.com/godbus/dbus/v5"
	"inet.af/netaddr"
)

type kv struct {
	k, v string
}

func (kv kv) String() string {
	return fmt.Sprintf("%s=%s", kv.k, kv.v)
}

func NewOSConfigurator(logf logger.Logf, interfaceName string) (ret OSConfigurator, err error) {
	env := newOSConfigEnv{
		fs:                directFS{},
		dbusPing:          dbusPing,
		nmIsUsingResolved: nmIsUsingResolved,
		nmVersionBetween:  nmVersionBetween,
		resolvconfStyle:   resolvconfStyle,
	}
	mode, err := dnsMode(logf, env)
	if err != nil {
		return nil, err
	}
	switch mode {
	case "direct":
		return newDirectManagerOnFS(env.fs), nil
	case "systemd-resolved":
		return newResolvedManager(logf, interfaceName)
	case "network-manager":
		return newNMManager(interfaceName)
	case "debian-resolvconf":
		return newDebianResolvconfManager(logf)
	case "openresolv":
		return newOpenresolvManager()
	default:
		logf("[unexpected] detected unknown DNS mode %q, using direct manager as last resort", mode)
		return newDirectManagerOnFS(env.fs), nil
	}
}

// newOSConfigEnv are the funcs newOSConfigurator needs, pulled out for testing.
type newOSConfigEnv struct {
	fs                        wholeFileFS
	dbusPing                  func(string, string) error
	nmIsUsingResolved         func() error
	nmVersionBetween          func(v1, v2 string) (safe bool, err error)
	resolvconfStyle           func() string
	isResolvconfDebianVersion func() bool
}

func dnsMode(logf logger.Logf, env newOSConfigEnv) (ret string, err error) {
	var debug []kv
	dbg := func(k, v string) {
		debug = append(debug, kv{k, v})
	}
	defer func() {
		if ret != "" {
			dbg("ret", ret)
		}
		logf("dns: %v", debug)
	}()

	bs, err := env.fs.ReadFile(resolvConf)
	if os.IsNotExist(err) {
		dbg("rc", "missing")
		return "direct", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading /etc/resolv.conf: %w", err)
	}

	switch resolvOwner(bs) {
	case "systemd-resolved":
		dbg("rc", "resolved")
		// Some systems, for reasons known only to them, have a
		// resolv.conf that has the word "systemd-resolved" in its
		// header, but doesn't actually point to resolved. We mustn't
		// try to program resolved in that case.
		// https://github.com/tailscale/tailscale/issues/2136
		if err := resolvedIsActuallyResolver(bs); err != nil {
			dbg("resolved", "not-in-use")
			return "direct", nil
		}
		if err := env.dbusPing("org.freedesktop.resolve1", "/org/freedesktop/resolve1"); err != nil {
			dbg("resolved", "no")
			return "direct", nil
		}
		if err := env.dbusPing("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/DnsManager"); err != nil {
			dbg("nm", "no")
			return "systemd-resolved", nil
		}
		dbg("nm", "yes")
		if err := env.nmIsUsingResolved(); err != nil {
			dbg("nm-resolved", "no")
			return "systemd-resolved", nil
		}
		dbg("nm-resolved", "yes")

		// Version of NetworkManager before 1.26.6 programmed resolved
		// incorrectly, such that NM's settings would always take
		// precedence over other settings set by other resolved
		// clients.
		//
		// If we're dealing with such a version, we have to set our
		// DNS settings through NM to have them take.
		//
		// However, versions 1.26.6 later both fixed the resolved
		// programming issue _and_ started ignoring DNS settings for
		// "unmanaged" interfaces - meaning NM 1.26.6 and later
		// actively ignore DNS configuration we give it. So, for those
		// NM versions, we can and must use resolved directly.
		//
		// Even more fun, even-older versions of NM won't let us set
		// DNS settings if the interface isn't managed by NM, with a
		// hard failure on DBus requests. Empirically, NM 1.22 does
		// this. Based on the versions popular distros shipped, we
		// conservatively decree that only 1.26.0 through 1.26.5 are
		// "safe" to use for our purposes. This roughly matches
		// distros released in the latter half of 2020.
		//
		// In a perfect world, we'd avoid this by replacing
		// configuration out from under NM entirely (e.g. using
		// directManager to overwrite resolv.conf), but in a world
		// where resolved runs, we need to get correct configuration
		// into resolved regardless of what's in resolv.conf (because
		// resolved can also be queried over dbus, or via an NSS
		// module that bypasses /etc/resolv.conf). Given that we must
		// get correct configuration into resolved, we have no choice
		// but to use NM, and accept the loss of IPv6 configuration
		// that comes with it (see
		// https://github.com/tailscale/tailscale/issues/1699,
		// https://github.com/tailscale/tailscale/pull/1945)
		safe, err := env.nmVersionBetween("1.26.0", "1.26.5")
		if err != nil {
			// Failed to figure out NM's version, can't make a correct
			// decision.
			return "", fmt.Errorf("checking NetworkManager version: %v", err)
		}
		if safe {
			dbg("nm-safe", "yes")
			return "network-manager", nil
		}
		dbg("nm-safe", "no")
		return "systemd-resolved", nil
	case "resolvconf":
		dbg("rc", "resolvconf")
		style := env.resolvconfStyle()
		switch style {
		case "":
			dbg("resolvconf", "no")
			return "direct", nil
		case "debian":
			dbg("resolvconf", "debian")
			return "debian-resolvconf", nil
		case "openresolv":
			dbg("resolvconf", "openresolv")
			return "openresolv", nil
		default:
			// Shouldn't happen, that means we updated flavors of
			// resolvconf without updating here.
			dbg("resolvconf", style)
			logf("[unexpected] got unknown flavor of resolvconf %q, falling back to direct manager", env.resolvconfStyle())
			return "direct", nil
		}
	case "NetworkManager":
		// You'd think we would use newNMManager somewhere in
		// here. However, as explained in
		// https://github.com/tailscale/tailscale/issues/1699 , using
		// NetworkManager for DNS configuration carries with it the
		// cost of losing IPv6 configuration on the Tailscale network
		// interface. So, when we can avoid it, we bypass
		// NetworkManager by replacing resolv.conf directly.
		//
		// If you ever try to put NMManager back here, keep in mind
		// that versions >=1.26.6 will ignore DNS configuration
		// anyway, so you still need a fallback path that uses
		// directManager.
		dbg("rc", "nm")
		return "direct", nil
	default:
		dbg("rc", "unknown")
		return "direct", nil
	}
}

func nmVersionBetween(first, last string) (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		// DBus probably not running.
		return false, err
	}

	nm := conn.Object("org.freedesktop.NetworkManager", dbus.ObjectPath("/org/freedesktop/NetworkManager"))
	v, err := nm.GetProperty("org.freedesktop.NetworkManager.Version")
	if err != nil {
		return false, err
	}

	version, ok := v.Value().(string)
	if !ok {
		return false, fmt.Errorf("unexpected type %T for NM version", v.Value())
	}

	outside := cmpver.Compare(version, first) < 0 || cmpver.Compare(version, last) > 0
	return !outside, nil
}

func nmIsUsingResolved() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		// DBus probably not running.
		return err
	}

	nm := conn.Object("org.freedesktop.NetworkManager", dbus.ObjectPath("/org/freedesktop/NetworkManager/DnsManager"))
	v, err := nm.GetProperty("org.freedesktop.NetworkManager.DnsManager.Mode")
	if err != nil {
		return fmt.Errorf("getting NM mode: %w", err)
	}
	mode, ok := v.Value().(string)
	if !ok {
		return fmt.Errorf("unexpected type %T for NM DNS mode", v.Value())
	}
	if mode != "systemd-resolved" {
		return errors.New("NetworkManager is not using systemd-resolved for DNS")
	}
	return nil
}

func resolvedIsActuallyResolver(bs []byte) error {
	cfg, err := readResolv(bytes.NewBuffer(bs))
	if err != nil {
		return err
	}
	// We've encountered at least one system where the line
	// "nameserver 127.0.0.53" appears twice, so we look exhaustively
	// through all of them and allow any number of repeated mentions
	// of the systemd-resolved stub IP.
	if len(cfg.Nameservers) == 0 {
		return errors.New("resolv.conf has no nameservers")
	}
	for _, ns := range cfg.Nameservers {
		if ns != netaddr.IPv4(127, 0, 0, 53) {
			return errors.New("resolv.conf doesn't point to systemd-resolved")
		}
	}
	return nil
}

func dbusPing(name, objectPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := dbus.SystemBus()
	if err != nil {
		// DBus probably not running.
		return err
	}

	obj := conn.Object(name, dbus.ObjectPath(objectPath))
	call := obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0)
	return call.Err
}
