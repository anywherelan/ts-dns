// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package feature tracks which features are linked into the binary.
package feature

import (
	"reflect"
)

var in = map[string]bool{}

// Registered reports the set of registered features.
//
// The returned map should not be modified by the caller,
// not accessed concurrently with calls to Register.
func Registered() map[string]bool { return in }

// IsRegistered reports whether the named feature is linked into the binary.
func IsRegistered(name string) bool { return in[name] }

// Register notes that the named feature is linked into the binary and
// reports whether it should initialize itself.
//
// Register panics if the feature is already registered.
func Register(name string) bool {
	if _, ok := in[name]; ok {
		panic("duplicate feature registration for " + name)
	}
	in[name] = true
	return true
}

// Hook is a func that can only be set once.
//
// It is not safe for concurrent use.
type Hook[Func any] struct {
	f  Func
	ok bool
}

// IsSet reports whether the hook has been set.
func (h *Hook[Func]) IsSet() bool {
	return h.ok
}

// Set sets the hook function, panicking if it's already been set
// or f is the zero value.
//
// It's meant to be called in init.
func (h *Hook[Func]) Set(f Func) {
	if h.ok {
		panic("Set on already-set feature hook")
	}
	if reflect.ValueOf(f).IsZero() {
		panic("Set with zero value")
	}
	h.f = f
	h.ok = true
}

// SetForTest sets the hook function for tests, blowing
// away any previous value.
//
// It returns a restore function that resets the hook
// to its previous value.
func (h *Hook[Func]) SetForTest(f Func) (restore func()) {
	old := *h
	h.f, h.ok = f, true
	return func() { *h = old }
}

// Get returns the hook function, or panics if it hasn't been set.
func (h *Hook[Func]) Get() Func {
	if !h.ok {
		panic("Get on unset feature hook, without IsSet")
	}
	return h.f
}

// GetOk returns the hook function and true if it has been set,
// otherwise its zero value and false.
func (h *Hook[Func]) GetOk() (f Func, ok bool) {
	return h.f, h.ok
}

// GetOrNil returns the hook function or nil if it hasn't been set.
func (h *Hook[Func]) GetOrNil() Func {
	return h.f
}
