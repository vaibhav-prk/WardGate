// Package limiter defines the rate-limiter interface and the shared types
// used across the zero-trust middleware chain.
package limiter

import "context"

// ─── Enumerations ─────────────────────────────────────────────────────────────

// LimiterMode identifies which enforcement strategy is active.
type LimiterMode string

const (
	ModeStatic   LimiterMode = "static"
	ModeAdaptive LimiterMode = "adaptive"
)

// Decision is the raw outcome returned by the limiter's token-bucket check.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Action is the enforcement action the PolicyEngine turns a Decision into.
type Action string

const (
	ActionAllow     Action = "allow"
	ActionThrottle  Action = "throttle"
	ActionChallenge Action = "challenge"
	ActionBlock     Action = "block"
)

// RequestSignal carries per-request abuse indicators produced by the
// zero-trust middleware chain (authn → signing → replay).  It is the
// primary input to both the risk scorer and the adaptive limiter.
type RequestSignal struct {
	ClientID       string
	Path           string
	StatusCode     int
	TamperDetected bool
	ReplayDetected bool
}

// RateLimiter is the interface both StaticLimiter and AdaptiveLimiter satisfy.
type RateLimiter interface {
	// Allow checks whether the request from clientID is within the effective rate limit.
	Allow(ctx context.Context, clientID string, riskScore float64) (Decision, error)

	// Type returns the enforcement mode this limiter was constructed with.
	Type() LimiterMode
}

// RiskScorer computes and updates the per-client risk score from behavioral signals.
type RiskScorer interface {
	Score(ctx context.Context, clientID string, signal RequestSignal) (float64, error)
}

// PolicyEngine converts a raw limiter Decision and the client's risk score
// into a concrete enforcement Action.
type PolicyEngine interface {
	Decide(ctx context.Context, decision Decision, riskScore float64) Action
}
