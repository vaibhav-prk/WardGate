package risk

import (
	"math"
	"sync"
	"time"
)

// ThreatState represents the current classification of the client
type ThreatState string

const (
	StateLowRisk    ThreatState = "LOW"
	StateSuspicious ThreatState = "SUSPICIOUS"
	StateHighRisk   ThreatState = "HIGH_RISK"
	StateBanned     ThreatState = "BANNED"
)

type ScorerConfig struct {
	Lambda              float64 // Decay constant
	SuspiciousThreshold float64 // Threshold to transition to SUSPICIOUS
	HighRiskThreshold   float64 // Threshold to transition to HIGH_RISK
	RecoveryThreshold   float64 // Lower threshold required to recover (Hysteresis)
}

func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{
		Lambda:              0.005, // Score halves approx every ~138 seconds
		SuspiciousThreshold: 30.0,
		HighRiskThreshold:   70.0,
		RecoveryThreshold:   20.0,
	}
}

type EntityRisk struct {
	Score     float64
	LastEvent time.Time
	State     ThreatState
}

type Scorer struct {
	mu     sync.RWMutex
	config ScorerConfig
	state  map[string]*EntityRisk
}

func NewScorer(config ScorerConfig) *Scorer {
	return &Scorer{
		config: config,
		state:  make(map[string]*EntityRisk),
	}
}

// ApplyDecay calculates score(t) = score(t0) * exp(-lambda * delta_t)
func (s *Scorer) ApplyDecay(currentScore float64, lastTime, now time.Time) float64 {
	dt := now.Sub(lastTime).Seconds()
	if dt <= 0 {
		return currentScore
	}
	return currentScore * math.Exp(-s.config.Lambda*dt)
}

// Evaluate updates and evaluates the risk score for an entity given new stats and penalties
func (s *Scorer) Evaluate(entityID string, penalty float64, now time.Time) (float64, ThreatState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entity, exists := s.state[entityID]
	if !exists {
		entity = &EntityRisk{
			Score:     0.0,
			LastEvent: now,
			State:     StateLowRisk,
		}
		s.state[entityID] = entity
	}

	// 1. Apply Exponential Decay
	decayedScore := s.ApplyDecay(entity.Score, entity.LastEvent, now)

	// 2. Add new penalty
	newScore := decayedScore + penalty
	entity.Score = newScore
	entity.LastEvent = now

	// 3. State machine with Hysteresis
	switch {
	case newScore >= s.config.HighRiskThreshold:
		entity.State = StateHighRisk
	case newScore >= s.config.SuspiciousThreshold:
		if entity.State != StateHighRisk || newScore < s.config.HighRiskThreshold {
			entity.State = StateSuspicious
		}
	case newScore < s.config.RecoveryThreshold:
		entity.State = StateLowRisk
	}

	return entity.Score, entity.State
}