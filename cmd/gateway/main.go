// Command gateway is the entry point for the Wardgate API gateway.
// It connects dependencies and delegates all logic to internal packages.
package main

import (
	"log"

	"github.com/vaibhav-prk/Wardgate/internal/config"
	"github.com/vaibhav-prk/Wardgate/internal/server"
)

func main() {
	// Load env variables
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Initialize the Redis client.
	rdb, err := server.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("failed to connect to redis %v", err)
	}
	log.Println("connected to redis")

	// Resolve the backend service URL.
	target, err := server.ParseBackendURL(cfg)
	if err != nil {
		log.Fatalf("invalid backend URL: %v", err)
	}

	// Construct and run the API gateway server.
	srv := server.NewServer(cfg, rdb, target)
	log.Printf("Gateway listening on %s", cfg.Addr)

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
	log.Println("Server shut down gracefully")
}
