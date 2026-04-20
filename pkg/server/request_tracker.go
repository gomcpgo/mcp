package server

import (
	"context"
	"fmt"
	"sync"
)

// requestTracker tracks in-flight requests so that inbound
// notifications/cancelled messages can cancel the matching handler's context
// and so that handleRequest can suppress responses for cancelled requests.
//
// Keys are the request IDs normalized via fmt.Sprintf("%v", id). JSON-RPC
// allows string or number IDs; this matches the client-side normalization.
type requestTracker struct {
	mu      sync.Mutex
	entries map[string]*trackerEntry
}

type trackerEntry struct {
	cancel    context.CancelFunc
	cancelled bool
}

func newRequestTracker() *requestTracker {
	return &requestTracker{
		entries: make(map[string]*trackerEntry),
	}
}

// register derives a cancellable context from parent and stores the cancel
// func against id so a later cancel(id) can fire it. Returns the ctx the
// handler should use and the cancel func the caller must invoke on return
// (via defer) so the tracker entry is cleaned up.
func (t *requestTracker) register(parent context.Context, id interface{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[fmt.Sprintf("%v", id)] = &trackerEntry{cancel: cancel}
	return ctx, cancel
}

// cancel fires the cancel func for id if one is tracked and flags the entry
// as cancelled so handleRequest can suppress the response when the handler
// eventually returns. Returns true if a tracked entry was cancelled.
func (t *requestTracker) cancel(id interface{}) bool {
	t.mu.Lock()
	entry, ok := t.entries[fmt.Sprintf("%v", id)]
	if !ok {
		t.mu.Unlock()
		return false
	}
	entry.cancelled = true
	cancelFn := entry.cancel
	t.mu.Unlock()
	if cancelFn != nil {
		cancelFn()
	}
	return true
}

// wasCancelled reports whether id was marked cancelled. Intended to be called
// after the handler returns to decide if the response should be suppressed.
func (t *requestTracker) wasCancelled(id interface{}) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.entries[fmt.Sprintf("%v", id)]
	if !ok {
		return false
	}
	return entry.cancelled
}

// unregister removes id from the tracker. Called after the handler returns
// so the map does not grow unbounded.
func (t *requestTracker) unregister(id interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, fmt.Sprintf("%v", id))
}
