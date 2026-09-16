package limiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// staticLimitLua is a sliding-window rate limiter using a Redis sorted set.
//
// Each allowed request is added as a member scored by its millisecond
// timestamp.  Expired entries are pruned on every call, giving smooth
// per-ms precision without fixed-window boundary spikes.
//
// KEYS[1]  = sorted-set key, e.g. "ratelimit:static:{clientID}"
// ARGV[1]  = limit      (max requests in window)
// ARGV[2]  = window_ms  (window size in milliseconds)
// ARGV[3]  = now_ms     (current timestamp in milliseconds)
// ARGV[4]  = member_id  (unique request ID to avoid dedup of concurrent reqs)
//
// Returns [3]int: {0|1 = allowed|denied, current_count, limit}
const staticLimitLua = `
local key       = KEYS[1]
local limit     = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms    = tonumber(ARGV[3])
local member_id = ARGV[4]

local cutoff = now_ms - window_ms
redis.call("ZREMRANGEBYSCORE", key, "-inf", cutoff)

local current = redis.call("ZCARD", key)

if current >= limit then
    return {1, current, limit}
end

redis.call("ZADD", key, now_ms, member_id)
redis.call("PEXPIRE", key, window_ms)

return {0, current, limit}
`

// StaticLimiter enforces a fixed rate limit (base_limit per window) for
// every client, regardless of risk score.  It satisfies the RateLimiter
// interface and is selected when LIMITER_MODE=static.
type StaticLimiter struct {
	rdb       *redis.Client
	script    *redis.Script
	baseLimit int
	windowMS  int64
}

// NewStaticLimiter constructs a StaticLimiter.
// baseLimit is the max requests per window; window is the sliding window duration.
func NewStaticLimiter(rdb *redis.Client, baseLimit int, window time.Duration) *StaticLimiter {
	return &StaticLimiter{
		rdb:       rdb,
		script:    redis.NewScript(staticLimitLua),
		baseLimit: baseLimit,
		windowMS:  window.Milliseconds(),
	}
}

// Allow checks the sliding window for clientID.  riskScore is ignored in
// static mode — the limit is always baseLimit.
func (sl *StaticLimiter) Allow(ctx context.Context, clientID string, _ float64) (Decision, error) {
	key := fmt.Sprintf("ratelimit:static:%s", clientID)
	nowMS := time.Now().UnixMilli()

	// member_id must be unique per request to prevent sorted-set dedup.
	// Combining timestamp with a counter-style suffix is sufficient since
	// the Lua script runs atomically — no two calls share the same nowMS
	// inside a single EVAL.  For extra safety under clock coarseness, we
	// append the nanosecond component.
	memberID := fmt.Sprintf("%d:%d", nowMS, time.Now().UnixNano())

	result, err := sl.script.Run(ctx, sl.rdb,
		[]string{key},
		strconv.Itoa(sl.baseLimit),
		strconv.FormatInt(sl.windowMS, 10),
		strconv.FormatInt(nowMS, 10),
		memberID,
	).Int64Slice()
	if err != nil {
		return DecisionDeny, fmt.Errorf("static limiter: lua script error: %w", err)
	}

	if result[0] == 1 {
		return DecisionDeny, nil
	}
	return DecisionAllow, nil
}

// Type returns ModeStatic.
func (sl *StaticLimiter) Type() LimiterMode {
	return ModeStatic
}
