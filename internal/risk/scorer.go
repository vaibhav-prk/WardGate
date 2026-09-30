// Package risk provides behavioral risk scoring, sliding-window feature extraction,
// and state transition management for zero-trust API rate limiting.
package risk

import (
	"math"
	"sync"
	"time"
)

type ClientState string

const (
	StateLowRisk    ClientState = "LOW_RISK"
	StateNormal     ClientState = "LOW_RISK"
	StateSuspicious ClientState = "SUSPICIOUS"
	StateHighRisk   ClientState = "HIGH_RISK"
	StateBanned     ClientState = "BANNED"
)

type ScorerConfig struct {
	DecayHalfLife       time.Duration
	SuspiciousThreshold float64
	HighRiskThreshold   float64
	RecoveryThreshold   float64
	StaleClientTTL      time.Duration
}

func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{
		DecayHalfLife:       30 * time.Second,
		SuspiciousThreshold: 40.0,
		HighRiskThreshold:   70.0,
		RecoveryThreshold:   25.0,
		StaleClientTTL:      10 * time.Minute,
	}
}

type EntityRecord struct {
	Score      float64
	State      ClientState
	LastUpdate time.Time
}

type Scorer struct {
	mu      sync.RWMutex
	config  ScorerConfig
	records map[string]*EntityRecord
}

func NewScorer(config ScorerConfig) *Scorer {
	return &Scorer{
		config:  config,
		records: make(map[string]*EntityRecord),
	}
}

// applyDecay computes exponential decay: score(t) = score(t0) * exp(-lambda * delta_t)
func (s *Scorer) applyDecay(score float64, lastUpdate, now time.Time) float64 {
	elapsed := now.Sub(lastUpdate)
	if elapsed <= 0 {
		return score
	}
	lambda := math.Ln2 / s.config.DecayHalfLife.Seconds()
	decayed := score * math.Exp(-lambda*elapsed.Seconds())
	if decayed < 0.01 {
		return 0
	}
	return decayed
}

// Evaluate updates client risk atomically with score capping, hysteresis, and state machine transitions
func (s *Scorer) Evaluate(clientID string, penalty float64, now time.Time) (float64, ClientState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entity, exists := s.records[clientID]
	if !exists {
		entity = &EntityRecord{
			Score:      0,
			State:      StateLowRisk,
			LastUpdate: now,
		}
		s.records[clientID] = entity
	}

	decayedScore := s.applyDecay(entity.Score, entity.LastUpdate, now)

	// Issue 1 Fix: Cap score strictly between 0 and 100
	newScore := math.Min(100.0, math.Max(0.0, decayedScore+penalty))
	entity.Score = newScore
	entity.LastUpdate = now

	// Issue 2 Fix: Correct hysteresis transitions between LOW_RISK, SUSPICIOUS, and HIGH_RISK
	switch {
	case newScore >= s.config.HighRiskThreshold:
		entity.State = StateHighRisk
	case newScore >= s.config.SuspiciousThreshold:
		// Hysteresis: only switch to Suspicious if not already HighRisk,
		// or if the score has genuinely decayed below HighRiskThreshold
		if entity.State != StateHighRisk || newScore < s.config.HighRiskThreshold {
			entity.State = StateSuspicious
		}
	case newScore <= s.config.RecoveryThreshold:
		// Full recovery down to LowRisk
		entity.State = StateLowRisk
	}

	// Issue 4 Fix: Memory cleanup for stale clients whose score decayed to 0
	if entity.Score == 0 && now.Sub(entity.LastUpdate) > s.config.StaleClientTTL {
		delete(s.records, clientID)
	}

	return entity.Score, entity.State
}

// CleanupStale removes clients inactive beyond the configured TTL
func (s *Scorer) CleanupStale(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entity := range s.records {
		if now.Sub(entity.LastUpdate) > s.config.StaleClientTTL {
			delete(s.records, id)
		}
	}
}

func (s *Scorer) GetEntity(clientID string) (EntityRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, exists := s.records[clientID]
	if !exists {
		return EntityRecord{}, false
	}
	return *rec, true
}
