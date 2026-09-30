package risk

import (
	"sync"
	"time"
)

// RequestEvent represents a single recorded request event within the tracking window.
type RequestEvent struct {
	Timestamp  time.Time
	Endpoint   string
	StatusCode int
	FailedAuth bool
}

// RollingTracker maintains sliding window events for risk feature extraction.
type RollingTracker struct {
	mu     sync.RWMutex
	window time.Duration
	events []RequestEvent
}

// NewRollingTracker creates a tracker with a specified sliding window duration.
func NewRollingTracker(window time.Duration) *RollingTracker {
	return &RollingTracker{
		window: window,
		events: make([]RequestEvent, 0),
	}
}

// Record appends a new event and prunes expired entries outside the rolling window.
func (rt *RollingTracker) Record(event RequestEvent) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.events = append(rt.events, event)
	cutoff := event.Timestamp.Add(-rt.window)
	rt.prune(cutoff)
}

// prune removes events older than the cutoff timestamp cleanly.
func (rt *RollingTracker) prune(cutoff time.Time) {
	firstValid := -1
	for i, e := range rt.events {
		if e.Timestamp.After(cutoff) {
			firstValid = i
			break
		}
	}

	if firstValid == -1 {
		// All events expired
		rt.events = rt.events[:0]
	} else if firstValid > 0 {
		// Slice from the first unexpired event onwards
		copy(rt.events, rt.events[firstValid:])
		rt.events = rt.events[:len(rt.events)-firstValid]
	}
}

// Count returns the number of active events currently inside the window.
func (rt *RollingTracker) Count(now time.Time) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	cutoff := now.Add(-rt.window)
	rt.prune(cutoff)
	return len(rt.events)
}

// CountFailures returns the count of failed authentication events in the window.
func (rt *RollingTracker) CountFailures(now time.Time) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	cutoff := now.Add(-rt.window)
	rt.prune(cutoff)

	failures := 0
	for _, e := range rt.events {
		if e.FailedAuth || e.StatusCode == 401 || e.StatusCode == 403 {
			failures++
		}
	}
	return failures
}

// UniqueEndpoints returns the count of distinct endpoints accessed in the window.
func (rt *RollingTracker) UniqueEndpoints(now time.Time) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	cutoff := now.Add(-rt.window)
	rt.prune(cutoff)

	seen := make(map[string]struct{})
	for _, e := range rt.events {
		seen[e.Endpoint] = struct{}{}
	}
	return len(seen)
}