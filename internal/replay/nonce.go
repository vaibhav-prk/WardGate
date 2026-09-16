// Package replay implements replay-attack prevention for the gateway.
package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vaibhav-prk/Wardgate/internal/authn"
	"github.com/vaibhav-prk/Wardgate/internal/config"
)

const replayLuaScript = `
local nonce_key    = KEYS[1]
local req_ts       = tonumber(ARGV[1])
local server_ts    = tonumber(ARGV[2])
local window_secs  = tonumber(ARGV[3])
local nonce_ttl    = tonumber(ARGV[4])

local drift = math.abs(server_ts - req_ts)
if drift > window_secs then
    return 2
end

local stored = redis.call("SET", nonce_key, "1", "EX", nonce_ttl, "NX")
if stored == false then
    return 1
end

return 0
`

// replayResult maps the integer return values from the Lua script.
type replayResult int

const (
	resultAllowed  replayResult = 0
	resultReplayed replayResult = 1
	resultStale    replayResult = 2
)

// NonceChecker is chi-compatible middleware that prevents request replay.
type NonceChecker struct {
	rdb    *redis.Client
	script *redis.Script
	window int // seconds
}

// New returns a NonceChecker that uses rdb for nonce storage.
func New(cfg *config.Config, rdb *redis.Client) *NonceChecker {
	return &NonceChecker{
		rdb:    rdb,
		script: redis.NewScript(replayLuaScript),
		window: cfg.NonceWindowSec,
	}
}

// Handle is the chi middleware entry point.
func (nc *NonceChecker) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := r.Header.Get("X-Nonce")
		tsHeader := r.Header.Get("X-Timestamp")

		if nonce == "" || tsHeader == "" {
			writeJSONError(w, http.StatusBadRequest, "replay: missing X-Nonce or X-Timestamp header")
			return
		}

		reqTS, err := strconv.ParseInt(tsHeader, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "replay: X-Timestamp must be a Unix epoch integer")
			return
		}

		clientID := authn.GetClientID(r.Context())

		result, err := nc.check(r.Context(), clientID, nonce, reqTS)
		if err != nil {
			// Redis unavailability: fail open with a log rather than blocking
			// all traffic.  The risk scorer will not receive a replay signal.
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		switch result {
		case resultAllowed:
			next.ServeHTTP(w, r)
		case resultReplayed:
			writeJSONError(w, http.StatusTooManyRequests, "replay: duplicate nonce — request already processed")
		case resultStale:
			writeJSONError(w, http.StatusTooManyRequests,
				fmt.Sprintf("replay: timestamp outside ±%ds window", nc.window))
		default:
			// Defensive: unknown Lua return code.
			writeJSONError(w, http.StatusForbidden, "replay: unrecognised check result")
		}
	})
}

// check executes the Lua script atomically in Redis.
// It returns a replayResult and any Redis communication error.
func (nc *NonceChecker) check(ctx context.Context, clientID, nonce string, reqTS int64) (replayResult, error) {
	nonceKey := fmt.Sprintf("replay:%s:%s", clientID, nonce)
	serverTS := time.Now().Unix()

	raw, err := nc.script.Run(ctx, nc.rdb,
		[]string{nonceKey},
		strconv.FormatInt(reqTS, 10),
		strconv.FormatInt(serverTS, 10),
		strconv.Itoa(nc.window),
		strconv.Itoa(nc.window),
	).Int()
	if err != nil {
		return 0, fmt.Errorf("replay: lua script error: %w", err)
	}

	return replayResult(raw), nil
}

// writeJSONError writes a JSON error body consistent with the rest of the
// gateway's error format.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
