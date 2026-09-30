package risk

import (
	"sync"
	"time"
)

// Penalty weights for signal-to-penalty conversion.
const (
	WeightFailedAuth float64 = 8.0 // per failed auth in window
	WeightFanOut     float64 = 3.0 // per unique endpoint beyond threshold
	WeightHighRate   float64 = 5.0 // when request count exceeds rate threshold
	FanOutThreshold          = 4   // unique endpoints before penalty kicks in
	RateThreshold            = 20  // requests/window before penalty kicks in
)

// ClientTracker manages per-client RollingTrackers and Scorer state.
type ClientTracker struct {
	mu       sync.Mutex
	trackers map[string]*RollingTracker
	scorer   *Scorer
	window   time.Duration
}

func NewClientTracker(window time.Duration, scorerCfg ScorerConfig) *ClientTracker {
	return &ClientTracker{
		trackers: make(map[string]*RollingTracker),
		scorer:   NewScorer(scorerCfg),
		window:   window,
	}
}

func (ct *ClientTracker) getTracker(clientID string) *RollingTracker {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	t, ok := ct.trackers[clientID]
	if !ok {
		t = NewRollingTracker(ct.window)
		ct.trackers[clientID] = t
	}
	return t
}

// RecordAndScore records a request event, computes behavioral penalty,
// feeds it to the scorer, and returns the updated risk score.
func (ct *ClientTracker) RecordAndScore(clientID string, event RequestEvent) float64 {
	now := event.Timestamp
	tracker := ct.getTracker(clientID)
	tracker.Record(event)

	penalty := ct.computePenalty(tracker, now)
	score, _ := ct.scorer.Evaluate(clientID, penalty, now)
	return score
}

func (ct *ClientTracker) computePenalty(t *RollingTracker, now time.Time) float64 {
	var penalty float64

	failures := t.CountFailures(now)
	penalty += float64(failures) * WeightFailedAuth

	fanOut := t.UniqueEndpoints(now)
	if fanOut > FanOutThreshold {
		penalty += float64(fanOut-FanOutThreshold) * WeightFanOut
	}

	count := t.Count(now)
	if count > RateThreshold {
		penalty += WeightHighRate
	}

	return penalty
}
