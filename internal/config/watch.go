package config

import (
	"errors"
	"sync"
)

// Snapshot is an immutable deep-copy of a Config. Subscribers receive
// Snapshots so they can read configuration without holding any lock.
type Snapshot struct {
	// Version is a monotonically increasing revision number assigned by the
	// Watcher when the snapshot is published. It is useful for idempotency
	// checks and for logging which revision a pipeline run used.
	Version uint64
	// Config is the effective configuration. Callers MUST treat it as
	// read-only; the Watcher never mutates a published snapshot.
	Config Config
}

// Watcher provides thread-safe access to the current Config and publishes
// updates on subscribe channels.
//
// Hot-reload applies to non-listener fields only (per docs/agents/08-config.md
// §4). Changes to `listen` are accepted into the snapshot but the HTTP server
// ignores them until a restart; the Watcher does not enforce this — it is a
// consumer contract.
type Watcher struct {
	mu       sync.RWMutex
	current  Snapshot
	version  uint64
	subs     map[int]chan Snapshot
	nextSub  int
	buffered int // per-subscriber channel buffer size
	closed   bool
}

// NewWatcher constructs a Watcher seeded with the given initial Config.
// The initial Snapshot is NOT published to any subscriber (there are none
// yet); new subscribers call Current() to bootstrap their state.
//
// buffered controls the size of each subscriber's delivery channel. A value
// <= 0 yields an unbuffered channel (strict back-pressure); the recommended
// default is 1 so that a slow subscriber sees the *latest* update rather
// than blocking the publisher.
func NewWatcher(initial Config, buffered int) *Watcher {
	return &Watcher{
		current:  Snapshot{Version: 1, Config: initial.Clone()},
		version:  1,
		subs:     make(map[int]chan Snapshot),
		buffered: buffered,
	}
}

// Current returns the most recently published Snapshot. Safe to call from any
// goroutine. Returned value is a deep-copy — callers may freely read or share.
func (w *Watcher) Current() Snapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return Snapshot{
		Version: w.current.Version,
		Config:  w.current.Config.Clone(),
	}
}

// Publish replaces the current Config with next (after validating it) and
// fan-outs the new Snapshot to every subscriber. Subscribers with full buffers
// receive a coalesced update: the oldest pending snapshot is dropped and
// replaced with the newest. This keeps slow consumers eventually-consistent
// without blocking the publisher.
//
// Returns an error if the Watcher is closed or if next fails Validate().
func (w *Watcher) Publish(next Config) error {
	if errs := next.Validate(); len(errs) > 0 {
		return errors.Join(errs...)
	}

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("config: watcher closed")
	}
	w.version++
	snap := Snapshot{Version: w.version, Config: next.Clone()}
	w.current = snap
	// Copy subs under lock; deliver without the lock held so a blocked
	// subscriber can't stall Publish for other subscribers.
	channels := make([]chan Snapshot, 0, len(w.subs))
	for _, ch := range w.subs {
		channels = append(channels, ch)
	}
	w.mu.Unlock()

	for _, ch := range channels {
		deliver(ch, snap)
	}
	return nil
}

// Subscribe returns a channel that receives every published Snapshot AFTER
// the subscription is registered. The returned cancel func deregisters the
// subscription and closes the channel. The caller MUST call cancel() exactly
// once (e.g. via defer) to avoid leaking a goroutine-visible channel.
//
// The channel is buffered per the Watcher's `buffered` setting; see Publish
// for delivery semantics.
func (w *Watcher) Subscribe() (<-chan Snapshot, func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	id := w.nextSub
	w.nextSub++
	buf := w.buffered
	if buf < 0 {
		buf = 0
	}
	ch := make(chan Snapshot, buf)
	w.subs[id] = ch
	cancel := func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if sub, ok := w.subs[id]; ok {
			delete(w.subs, id)
			close(sub)
		}
	}
	return ch, cancel
}

// Close stops the Watcher, deregisters all subscribers, and closes their
// channels. After Close, Publish returns an error and Subscribe still works
// but will never deliver anything (for simpler shutdown sequencing we keep
// it a no-op rather than panicking).
func (w *Watcher) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	for id, ch := range w.subs {
		delete(w.subs, id)
		close(ch)
	}
}

// deliver attempts a non-blocking send. When the channel buffer is full it
// drops the oldest pending value and enqueues the new one — subscribers
// should always consume the *most recent* Snapshot, never a stale one.
// For unbuffered channels it falls back to a best-effort non-blocking send
// (dropping if the receiver isn't ready).
func deliver(ch chan Snapshot, snap Snapshot) {
	for {
		select {
		case ch <- snap:
			return
		default:
			// Drain one old value, then retry.
			select {
			case <-ch:
				// drained; loop to enqueue fresh snap
			default:
				// Unbuffered channel with no ready receiver: drop.
				return
			}
		}
	}
}
