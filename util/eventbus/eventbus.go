// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package eventbus is a minimal replacement of tailscale.com/util/eventbus with the same API subset.
//
// Unlike upstream, subscriber funcs are called asynchronously in a new goroutine for each event,
// so events may be delivered out of order and after Close.
package eventbus

import (
	"reflect"
	"slices"
	"sync"
)

// Bus is an event bus that distributes published events to interested subscribers.
type Bus struct {
	mu   sync.Mutex
	subs map[reflect.Type][]*subscriber
}

type subscriber struct {
	client *Client
	call   func(any)
}

// New returns a new bus.
func New() *Bus {
	return &Bus{}
}

// Client returns a new client with no subscriptions.
func (b *Bus) Client(name string) *Client {
	return &Client{bus: b}
}

// Close removes all subscriptions.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = nil
}

func (b *Bus) subscribe(t reflect.Type, s *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = make(map[reflect.Type][]*subscriber)
	}
	b.subs[t] = append(b.subs[t], s)
}

func (b *Bus) unsubscribe(match func(*subscriber) bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for t, subs := range b.subs {
		b.subs[t] = slices.DeleteFunc(subs, match)
	}
}

func (b *Bus) subscribers(t reflect.Type) []*subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.subs[t])
}

// A Client can publish and subscribe to events on its attached bus.
type Client struct {
	bus *Bus
}

// Close removes all subscriptions of the client.
func (c *Client) Close() {
	c.bus.unsubscribe(func(s *subscriber) bool { return s.client == c })
}

// Publisher publishes typed events on a bus.
type Publisher[T any] struct {
	client *Client
	mu     sync.Mutex
	closed bool
}

// Publish returns a publisher for event type T using the given client.
func Publish[T any](c *Client) *Publisher[T] {
	return &Publisher[T]{client: c}
}

// Publish publishes event v on the bus.
func (p *Publisher[T]) Publish(v T) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return
	}
	for _, s := range p.client.bus.subscribers(reflect.TypeFor[T]()) {
		go s.call(v)
	}
}

// Close closes the publisher, later events are dropped.
func (p *Publisher[T]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
}

// ShouldPublish reports whether anyone is subscribed to the events.
func (p *Publisher[T]) ShouldPublish() bool {
	return len(p.client.bus.subscribers(reflect.TypeFor[T]())) > 0
}

// SubscriberFunc calls a function for each event of type T.
type SubscriberFunc[T any] struct {
	client *Client
	sub    *subscriber
}

// SubscribeFunc calls f for each event of type T published on the bus.
func SubscribeFunc[T any](c *Client, f func(T)) *SubscriberFunc[T] {
	s := &subscriber{client: c, call: func(v any) { f(v.(T)) }}
	c.bus.subscribe(reflect.TypeFor[T](), s)
	return &SubscriberFunc[T]{client: c, sub: s}
}

// Close removes the subscription.
func (s *SubscriberFunc[T]) Close() {
	s.client.bus.unsubscribe(func(sub *subscriber) bool { return sub == s.sub })
}
