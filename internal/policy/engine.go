package policy

import (
	"context"

	"github.com/vaibhav-prk/Wardgate/internal/limiter"
)

// Thresholds for escalating beyond a simple 429.
const (
	challengeScoreThreshold = 60.0 // risk score above this → re-auth challenge
	blockScoreThreshold     = 85.0 // risk score above this → hard block
)

// Engine decides the enforcement action from a limiter Decision + risk score.
type Engine struct{}

func New() *Engine { return &Engine{} }

// Decide maps (Decision, riskScore) → Action.
//
// Allow + any score       → ActionAllow
// Deny  + score < 60      → ActionThrottle  (429)
// Deny  + 60 ≤ score < 85 → ActionChallenge (401, re-auth)
// Deny  + score ≥ 85      → ActionBlock     (403, hard deny)
func (e *Engine) Decide(_ context.Context, d limiter.Decision, riskScore float64) limiter.Action {
	if d == limiter.DecisionAllow {
		return limiter.ActionAllow
	}

	switch {
	case riskScore >= blockScoreThreshold:
		return limiter.ActionBlock
	case riskScore >= challengeScoreThreshold:
		return limiter.ActionChallenge
	default:
		return limiter.ActionThrottle
	}
}
