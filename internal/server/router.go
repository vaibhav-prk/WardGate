package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
		// r.Use(s.signer.Handle)
		// r.Use(s.replay.Handle)
		// r.Use(s.rateLimit)
		// r.Use(s.policy.Handle)

		r.Handle("/*", s.proxy)
	})
}
