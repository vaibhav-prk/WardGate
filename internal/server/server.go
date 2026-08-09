// Package server wires the gateway's dependencies — router, Redis, and
// middleware chain — into a single Server type ready to serve requests.
package server

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/vaibhav-prk/Wardgate/internal/authn"
	"github.com/vaibhav-prk/Wardgate/internal/config"
)

// Server holds every dependency the gateway needs.
type Server struct {
	cfg    *config.Config
	router *chi.Mux
	rdb    *redis.Client
	proxy  *httputil.ReverseProxy
	authn  *authn.Authenticator
}

// NewServer constructs a Server, wires all middleware and routes.
func NewServer(cfg *config.Config, rdb *redis.Client, target *url.URL) *Server {
	s := &Server{
		cfg:    cfg,
		router: chi.NewRouter(),
		rdb:    rdb,
		proxy:  httputil.NewSingleHostReverseProxy(target),
		authn:  authn.New(cfg),
	}

	s.routes()

	return s
}

// Run starts the HTTP server on the given address.
func (s *Server) Run() error {
	srv := http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}
