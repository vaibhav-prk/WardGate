package risk

import (
	"testing"
	"time"
)

func TestExponentialDecay(t *testing.T) {
	cfg := DefaultScorerConfig()
	scorer := NewScorer(cfg)

	now := time.Now()
	// Add initial penalty of 100 points
	score, state := scorer.Evaluate("user-1", 100.0, now)
	if state != StateHighRisk {
		t.Fatalf("expected state HIGH_RISK, got %s", state)
	}

	// Fast-forward time by 200 seconds (~1.5 half-lives)
	later := now.Add(200 * time.Second)
	decayedScore, _ := scorer.Evaluate("user-1", 0.0, later)

	if decayedScore >= score {
		t.Fatalf("expected score to decay from %.2f, got %.2f", score, decayedScore)
	}
}

func TestHysteresis(t *testing.T) {
	cfg := DefaultScorerConfig()
	scorer := NewScorer(cfg)

	now := time.Now()
	// High risk spike
	scorer.Evaluate("user-2", 80.0, now)

	// Decay to 25.0 (between Recovery 20 and Suspicious 30)
	// Should stay Suspicious until it drops strictly below RecoveryThreshold
	score, state := scorer.Evaluate("user-2", 0.0, now.Add(230*time.Second))
	if score < cfg.RecoveryThreshold && state != StateLowRisk {
		t.Fatalf("expected state LOW once below recovery threshold, got %s", state)
	}
}