package risk

import (
	"math"
	"sync"
	"time"
)

// RequestEvent logs an individual request's metadata in the sliding window
type RequestEvent struct {
	Timestamp  time.Time
	Path       string
	StatusCode int
}

// WindowStats holds calculated statistical signals over the sliding window
type WindowStats struct {
	RequestCount int
	CV           float64 // Coefficient of variation of inter-arrival times
	BurstRatio   float64 // Short-term burst vs rolling baseline
	ErrorRate    float64 // Ratio of 4xx/5xx responses
	FanOut       int     // Unique endpoints visited in window
}

// RollingTracker tracks sliding window traffic for a single entity (IP or User)
type RollingTracker struct {
	mu          sync.RWMutex
	window      time.Duration
	shortWindow time.Duration
	events      []RequestEvent
}

// NewRollingTracker creates a tracker with a default sliding window (e.g., 60s) and burst window (e.g., 5s)
func NewRollingTracker(window, shortWindow time.Duration) *RollingTracker {
	return &RollingTracker{
		window:      window,
		shortWindow: shortWindow,
		events:      make([]RequestEvent, 0),
	}
}

// Record adds a new request event and trims expired events outside the window
func (rt *RollingTracker) Record(path string, statusCode int, now time.Time) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	cutoff := now.Add(-rt.window)

	// Prune expired events
	validIdx := 0
	for i, e := range rt.events {
		if e.Timestamp.After(cutoff) {
			validIdx = i
			break
		}
	}
	if validIdx > 0 {
		rt.events = rt.events[validIdx:]
	} else if len(rt.events) > 0 && rt.events[0].Timestamp.Before(cutoff) {
		rt.events = nil
	}

	rt.events = append(rt.events, RequestEvent{
		Timestamp:  now,
		Path:       path,
		StatusCode: statusCode,
	})
}

// ComputeStats calculates CV, Burst Ratio, Error Rate, and Fan-Out
func (rt *RollingTracker) ComputeStats(now time.Time) WindowStats {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	n := len(rt.events)
	if n < 2 {
		return WindowStats{
			RequestCount: n,
			CV:           0.0,
			BurstRatio:   1.0,
			ErrorRate:    0.0,
			FanOut:       n,
		}
	}

	// 1. Inter-arrival times and CV
	intervals := make([]float64, n-1)
	var sumIntervals float64
	for i := 1; i < n; i++ {
		dt := rt.events[i].Timestamp.Sub(rt.events[i-1].Timestamp).Seconds()
		intervals[i-1] = dt
		sumIntervals += dt
	}

	meanInterval := sumIntervals / float64(len(intervals))
	var varianceSum float64
	for _, dt := range intervals {
		varianceSum += math.Pow(dt-meanInterval, 2)
	}
	stdDev := math.Sqrt(varianceSum / float64(len(intervals)))

	var cv float64
	if meanInterval > 0 {
		cv = stdDev / meanInterval
	}

	// 2. Error rate & Endpoint Fan-Out
	uniquePaths := make(map[string]struct{})
	var errorCount int
	var shortWindowCount int
	shortCutoff := now.Add(-rt.shortWindow)

	for _, e := range rt.events {
		uniquePaths[e.Path] = struct{}{}
		if e.StatusCode >= 400 {
			errorCount++
		}
		if e.Timestamp.After(shortCutoff) {
			shortWindowCount++
		}
	}

	errorRate := float64(errorCount) / float64(n)

	// 3. Burst Ratio = (rate in shortWindow) / (rate in full window)
	shortRate := float64(shortWindowCount) / rt.shortWindow.Seconds()
	fullRate := float64(n) / rt.window.Seconds()
	burstRatio := 1.0
	if fullRate > 0 {
		burstRatio = shortRate / fullRate
	}

	return WindowStats{
		RequestCount: n,
		CV:           cv,
		BurstRatio:   burstRatio,
		ErrorRate:    errorRate,
		FanOut:       len(uniquePaths),
	}
}