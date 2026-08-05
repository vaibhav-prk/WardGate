package server

import (
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
		w.WriteHeader(http.StatusOK)
	})

	// --- API routes (zero-trust pipeline) ---
	s.router.Route("/api", func(r chi.Router) {
		// Middleware slots — replace stubs with real implementations
		// r.Use(s.authn.Handle)
		// r.Use(s.signer.Handle)
		// r.Use(s.replay.Handle)
		// r.Use(s.rateLimit)
		// r.Use(s.policy.Handle)

		r.Handle("/*", s.proxy)
	})
}
