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
)

// Server holds every dependency the gateway needs.
type Server struct {
	router *chi.Mux
	rdb    *redis.Client
	proxy  *httputil.ReverseProxy
}

// NewServer constructs a Server, wires all middleware and routes
func NewServer(rdb *redis.Client, target *url.URL) *Server {
	s := &Server{
		router: chi.NewRouter(),
		rdb:    rdb,
		proxy:  httputil.NewSingleHostReverseProxy(target),
	}

	s.routes()

	return s
}

// Run starts the HTTP server on the given address.
func (s *Server) Run() error {
	srv := http.Server{
		Addr:    ":8080",
		Handler: s.router,
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
