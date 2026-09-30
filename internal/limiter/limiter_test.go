package limiter_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/vaibhav-prk/Wardgate/internal/limiter"
)

func setupRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

// TestStaticLimiter_BasicAllow verifies basic allow/deny at the threshold.
func TestStaticLimiter_BasicAllow(t *testing.T) {
	_, rdb := setupRedis(t)
	sl := limiter.NewStaticLimiter(rdb, 5, 1*time.Second)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		d, err := sl.Allow(ctx, "client-1", 0)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if d != limiter.DecisionAllow {
			t.Fatalf("request %d: expected allow, got deny", i)
		}
	}

	// 6th request should be denied
	d, err := sl.Allow(ctx, "client-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != limiter.DecisionDeny {
		t.Fatal("expected deny after exceeding limit")
	}
}

// TestStaticLimiter_Concurrent fires 50 goroutines against a limit of 20.
// Exactly 20 should be allowed, 30 denied — no races.
func TestStaticLimiter_Concurrent(t *testing.T) {
	_, rdb := setupRedis(t)
	const limit = 20
	const goroutines = 50

	sl := limiter.NewStaticLimiter(rdb, limit, 5*time.Second)
	ctx := context.Background()

	var allowed atomic.Int64
	var denied atomic.Int64
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			d, err := sl.Allow(ctx, "concurrent-client", 0)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if d == limiter.DecisionAllow {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	wg.Wait()

	if allowed.Load() != limit {
		t.Errorf("expected exactly %d allowed, got %d", limit, allowed.Load())
	}
	if denied.Load() != goroutines-limit {
		t.Errorf("expected exactly %d denied, got %d", goroutines-limit, denied.Load())
	}
}

// TestStaticLimiter_IsolatesClients verifies different clients have separate windows.
func TestStaticLimiter_IsolatesClients(t *testing.T) {
	_, rdb := setupRedis(t)
	sl := limiter.NewStaticLimiter(rdb, 3, 1*time.Second)
	ctx := context.Background()

	// Exhaust client-a
	for i := 0; i < 3; i++ {
		sl.Allow(ctx, "client-a", 0)
	}

	// client-b should still be allowed
	d, _ := sl.Allow(ctx, "client-b", 0)
	if d != limiter.DecisionAllow {
		t.Fatal("client-b should not be affected by client-a's limit")
	}
}

// TestAdaptiveLimiter_TightensWithRisk verifies that higher risk scores
// result in a tighter effective limit.
func TestAdaptiveLimiter_TightensWithRisk(t *testing.T) {
	_, rdb := setupRedis(t)
	// base=100, window=5s, min_fraction=0.1, lambda=0.005
	al := limiter.NewAdaptiveLimiter(rdb, 100, 5*time.Second, 0.1, 0.005)
	ctx := context.Background()

	// With risk=0, effective_limit=100. Send 50 — all should pass.
	for i := 0; i < 50; i++ {
		d, err := al.Allow(ctx, "low-risk", 0)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if d != limiter.DecisionAllow {
			t.Fatalf("request %d: low-risk client denied too early", i)
		}
	}

	// With risk=80 (penalty), effective_limit ≈ 100 * max(0.1, 1-80/100) = 100*0.2 = 20.
	// Send 25 requests — first ~20 should pass, rest denied.
	var highRiskAllowed int
	for i := 0; i < 25; i++ {
		d, err := al.Allow(ctx, "high-risk", 80)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if d == limiter.DecisionAllow {
			highRiskAllowed++
		}
	}

	// The effective limit should be around 20 (with some tolerance for decay)
	if highRiskAllowed > 22 {
		t.Errorf("high-risk client allowed %d requests, expected ≤22 (effective_limit ~20)", highRiskAllowed)
	}
	if highRiskAllowed < 10 {
		t.Errorf("high-risk client only allowed %d, expected at least 10", highRiskAllowed)
	}
}

// TestAdaptiveLimiter_Concurrent fires 50 goroutines with a high penalty.
// Verifies no race condition in the atomic Lua script.
func TestAdaptiveLimiter_Concurrent(t *testing.T) {
	_, rdb := setupRedis(t)
	// base=10, small window, min_frac=0.1
	al := limiter.NewAdaptiveLimiter(rdb, 10, 5*time.Second, 0.1, 0.001)
	ctx := context.Background()

	const goroutines = 50
	var allowed atomic.Int64
	var denied atomic.Int64
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			// Each goroutine adds a small penalty
			penalty := float64(idx % 5)
			d, err := al.Allow(ctx, "adaptive-concurrent", penalty)
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
				return
			}
			if d == limiter.DecisionAllow {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}(i)
	}
	wg.Wait()

	total := allowed.Load() + denied.Load()
	if total != goroutines {
		t.Errorf("expected %d total decisions, got %d", goroutines, total)
	}

	// With accumulating penalties and base=10, allowed must be ≤ base limit
	if allowed.Load() > 10 {
		t.Errorf("allowed %d requests with base_limit=10 — race condition!", allowed.Load())
	}

	t.Logf("adaptive concurrent: %d allowed, %d denied (base=10, 50 goroutines)",
		allowed.Load(), denied.Load())
}

// TestStaticLimiter_WindowExpiry verifies requests are allowed again after window expires.
func TestStaticLimiter_WindowExpiry(t *testing.T) {
	mr, rdb := setupRedis(t)
	sl := limiter.NewStaticLimiter(rdb, 2, 1*time.Second)
	ctx := context.Background()

	// Exhaust the limit
	sl.Allow(ctx, "expiry-client", 0)
	sl.Allow(ctx, "expiry-client", 0)
	d, _ := sl.Allow(ctx, "expiry-client", 0)
	if d != limiter.DecisionDeny {
		t.Fatal("expected deny after limit exhausted")
	}

	// Fast-forward time past the window
	mr.FastForward(2 * time.Second)

	d, _ = sl.Allow(ctx, "expiry-client", 0)
	if d != limiter.DecisionAllow {
		t.Fatal("expected allow after window expired")
	}
}

// TestLimiterType verifies Type() returns the correct mode.
func TestLimiterType(t *testing.T) {
	_, rdb := setupRedis(t)

	sl := limiter.NewStaticLimiter(rdb, 10, time.Second)
	if sl.Type() != limiter.ModeStatic {
		t.Errorf("expected static, got %s", sl.Type())
	}

	al := limiter.NewAdaptiveLimiter(rdb, 10, time.Second, 0.1, 0.005)
	if al.Type() != limiter.ModeAdaptive {
		t.Errorf("expected adaptive, got %s", al.Type())
	}
}

// BenchmarkStaticLimiter_Allow measures per-request overhead.
func BenchmarkStaticLimiter_Allow(b *testing.B) {
	mr := miniredis.RunT(b)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sl := limiter.NewStaticLimiter(rdb, 1000000, 10*time.Second)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Allow(ctx, fmt.Sprintf("bench-%d", i%100), 0)
	}
}
