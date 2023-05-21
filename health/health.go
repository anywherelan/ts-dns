// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package health is a registry for other packages to report & check
// overall health status of the node.
package health

import (
	"sync"
	"sync/atomic"
)

var (
	// mu guards everything in this var block.
	mu sync.Mutex

	sysErr    = map[Subsystem]error{}    // error key => err (or nil for no error)
	warnables = map[*Warnable]struct{}{} // set of warnables
)

// Subsystem is the name of a subsystem whose health can be monitored.
type Subsystem string

const (
	// SysDNSOS is the name of the net/dns OSConfigurator subsystem.
	SysDNSOS = Subsystem("dns-os")

	// SysDNSManager is the name of the net/dns manager subsystem.
	SysDNSManager = Subsystem("dns-manager")
)

// NewWarnable returns a new warnable item that the caller can mark
// as health or in warning state.
func NewWarnable(opts ...WarnableOpt) *Warnable {
	w := new(Warnable)
	for _, o := range opts {
		o.mod(w)
	}
	mu.Lock()
	defer mu.Unlock()
	warnables[w] = struct{}{}
	return w
}

// WarnableOpt is an option passed to NewWarnable.
type WarnableOpt interface {
	mod(*Warnable)
}

// WithMapDebugFlag returns a WarnableOpt for NewWarnable that makes the returned
// Warnable report itself to the coordination server as broken with this
// string in MapRequest.DebugFlag when Set to a non-nil value.
func WithMapDebugFlag(name string) WarnableOpt {
	return warnOptFunc(func(w *Warnable) {
		w.debugFlag = name
	})
}

type warnOptFunc func(*Warnable)

func (f warnOptFunc) mod(w *Warnable) { f(w) }

// Warnable is a health check item that may or may not be in a bad warning state.
// The caller of NewWarnable is responsible for calling Set to update the state.
type Warnable struct {
	debugFlag string // optional MapRequest.DebugFlag to send when unhealthy

	isSet atomic.Bool
	mu    sync.Mutex
	err   error
}

// Set updates the Warnable's state.
// If non-nil, it's considered unhealthy.
func (w *Warnable) Set(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.err = err
	w.isSet.Store(err != nil)
}

func (w *Warnable) get() error {
	if !w.isSet.Load() {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// SetDNSOSHealth sets the state of the net/dns.OSConfigurator
func SetDNSOSHealth(err error) { setErr(SysDNSOS, err) }

// SetDNSManagerHealth sets the state of the Linux net/dns manager's
// discovery of the /etc/resolv.conf situation.
func SetDNSManagerHealth(err error) { setErr(SysDNSManager, err) }

// DNSOSHealth returns the net/dns.OSConfigurator error state.
func DNSOSHealth() error { return get(SysDNSOS) }

func get(key Subsystem) error {
	mu.Lock()
	defer mu.Unlock()
	return sysErr[key]
}

func setErr(key Subsystem, err error) {
	mu.Lock()
	defer mu.Unlock()
	setLocked(key, err)
}

func setLocked(key Subsystem, err error) {
	old, ok := sysErr[key]
	if !ok && err == nil {
		// Initial happy path.
		sysErr[key] = nil
		return
	}
	if ok && (old == nil) == (err == nil) {
		// No change in overall error status (nil-vs-not), so
		// don't run callbacks, but exact error might've
		// changed, so note it.
		if err != nil {
			sysErr[key] = err
		}
		return
	}
	sysErr[key] = err
}
