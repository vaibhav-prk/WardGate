package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/vaibhav-prk/Wardgate/internal/authn"
	"github.com/vaibhav-prk/Wardgate/internal/limiter"
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

// rateLimitMiddleware combines the limiter check and policy decision.
// It passes 0 as penalty for static mode (ignored) and could pass
// computed penalties for adaptive mode from request signals.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := authn.GetClientID(r.Context())

		// For now penalty is 0 for normal requests.
		// TODO: compute penalty from RequestSignal (tamper/replay flags)
		// once the signal-propagation context keys are added.
		var penalty float64

		decision, err := s.limiter.Allow(r.Context(), clientID, penalty)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Let the policy engine escalate denials based on risk score.
		// For static mode riskScore is 0 so it always returns ActionThrottle.
		action := s.policy.Decide(r.Context(), decision, penalty)

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
