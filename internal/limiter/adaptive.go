package limiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// riskAndLimitLua is the core atomic script — it decays the risk score,
// adds new penalty, computes the adaptive ceiling, and does the sliding
// window check all in one EVAL so no request sees a stale score.
//
// KEYS[1] = "ratelimit:adaptive:{clientID}"  (sorted set)
// KEYS[2] = "risk:{clientID}"                (hash: score, last_ts)
//
// ARGV[1] = base_limit       ARGV[2] = window_ms
// ARGV[3] = now_ms           ARGV[4] = min_fraction (e.g. 0.1)
// ARGV[5] = penalty          ARGV[6] = lambda (decay constant)
// ARGV[7] = member_id
//
// Returns: {allowed(0|1), risk_score*1000, effective_limit, current_count}
// (score is multiplied by 1000 because Redis Lua only returns integers)
const riskAndLimitLua = `
local rl_key    = KEYS[1]
local risk_key  = KEYS[2]
local base      = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms    = tonumber(ARGV[3])
local min_frac  = tonumber(ARGV[4])
local penalty   = tonumber(ARGV[5])
local lambda    = tonumber(ARGV[6])
local member_id = ARGV[7]

-- 1. Read current risk state
local old_score = tonumber(redis.call("HGET", risk_key, "score") or "0") or 0
local old_ts    = tonumber(redis.call("HGET", risk_key, "last_ts") or "0") or 0

-- 2. Decay: score(t) = score(t0) * exp(-lambda * dt_seconds)
local score = old_score
if old_ts > 0 and now_ms > old_ts then
    local dt = (now_ms - old_ts) / 1000.0
    score = score * math.exp(-lambda * dt)
end

-- 3. Add penalty
score = score + penalty
if score > 100 then score = 100 end
if score < 0   then score = 0   end

-- 4. Store updated risk state
redis.call("HSET", risk_key, "score", tostring(score), "last_ts", tostring(now_ms))
redis.call("PEXPIRE", risk_key, window_ms * 10)

-- 5. Compute effective limit: base * max(min_frac, 1 - score/100)
local factor = 1 - score / 100
if factor < min_frac then factor = min_frac end
local eff_limit = math.floor(base * factor)
if eff_limit < 1 then eff_limit = 1 end

-- 6. Sliding window check
local cutoff = now_ms - window_ms
redis.call("ZREMRANGEBYSCORE", rl_key, "-inf", cutoff)
local current = redis.call("ZCARD", rl_key)

if current >= eff_limit then
    return {1, math.floor(score * 1000), eff_limit, current}
end

redis.call("ZADD", rl_key, now_ms, member_id)
redis.call("PEXPIRE", rl_key, window_ms)

return {0, math.floor(score * 1000), eff_limit, current}
`

// AdaptiveLimiter tightens the rate limit as risk score rises.
// effective_limit = base_limit × max(min_fraction, 1 − score/100)
type AdaptiveLimiter struct {
	rdb        *redis.Client
	script     *redis.Script
	baseLimit  int
	windowMS   int64
	minFrac    float64 // floor ratio, e.g. 0.1 = never below 10% of base
	lambda     float64 // decay constant for risk score
}

func NewAdaptiveLimiter(rdb *redis.Client, baseLimit int, window time.Duration, minFraction, lambda float64) *AdaptiveLimiter {
	return &AdaptiveLimiter{
		rdb:       rdb,
		script:    redis.NewScript(riskAndLimitLua),
		baseLimit: baseLimit,
		windowMS:  window.Milliseconds(),
		minFrac:   minFraction,
		lambda:    lambda,
	}
}

// Allow decays the risk score, applies penalty (via riskScore param),
// computes the adaptive ceiling, and checks the sliding window — all atomic.
func (al *AdaptiveLimiter) Allow(ctx context.Context, clientID string, riskScore float64) (Decision, error) {
	rlKey   := fmt.Sprintf("ratelimit:adaptive:%s", clientID)
	riskKey := fmt.Sprintf("risk:%s", clientID)
	nowMS   := time.Now().UnixMilli()
	memberID := fmt.Sprintf("%d:%d", nowMS, time.Now().UnixNano())

	result, err := al.script.Run(ctx, al.rdb,
		[]string{rlKey, riskKey},
		strconv.Itoa(al.baseLimit),
		strconv.FormatInt(al.windowMS, 10),
		strconv.FormatInt(nowMS, 10),
		strconv.FormatFloat(al.minFrac, 'f', -1, 64),
		strconv.FormatFloat(riskScore, 'f', -1, 64),
		strconv.FormatFloat(al.lambda, 'f', -1, 64),
		memberID,
	).Int64Slice()
	if err != nil {
		return DecisionDeny, fmt.Errorf("adaptive limiter: lua error: %w", err)
	}

	if result[0] == 1 {
		return DecisionDeny, nil
	}
	return DecisionAllow, nil
}

func (al *AdaptiveLimiter) Type() LimiterMode {
	return ModeAdaptive
}
