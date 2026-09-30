package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/vaibhav-prk/Wardgate/internal/authn"
	"github.com/vaibhav-prk/Wardgate/internal/limiter"
	"github.com/vaibhav-prk/Wardgate/internal/risk"
)

// routes registers the gateway routes and middleware pipeline.
func (s *Server) routes() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	// --- Health check (no domain middleware) ---
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "OK",
		})
	})

	// --- API routes (zero-trust pipeline) ---
	s.router.Route("/api", func(r chi.Router) {
		r.Use(s.authn.Handle)
		r.Use(s.signer.Handle)
		r.Use(s.replay.Handle)
		r.Use(s.rateLimitMiddleware)

		r.Handle("/*", s.proxy)
	})
}

// rateLimitMiddleware records the request in the risk tracker, computes
// a behavioral penalty, and passes the resulting risk score to the limiter
// and policy engine.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := authn.GetClientID(r.Context())
		now := time.Now()

		// Record this request and get the updated risk score.
		riskScore := s.tracker.RecordAndScore(clientID, risk.RequestEvent{
			Timestamp:  now,
			Endpoint:   r.URL.Path,
			StatusCode: 0, // not known yet (pre-proxy)
			FailedAuth: false,
		})

		decision, err := s.limiter.Allow(r.Context(), clientID, riskScore)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		action := s.policy.Decide(r.Context(), decision, riskScore)

		switch action {
		case limiter.ActionAllow:
			next.ServeHTTP(w, r)
		case limiter.ActionThrottle:
			writeJSON(w, http.StatusTooManyRequests, "rate limit exceeded")
		case limiter.ActionChallenge:
			writeJSON(w, http.StatusUnauthorized, "step-up authentication required")
		case limiter.ActionBlock:
			writeJSON(w, http.StatusForbidden, "access denied")
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
