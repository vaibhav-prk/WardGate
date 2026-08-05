// Command gateway is the entry point for the Wardgate API gateway.
// It connects dependencies and delegates all logic to internal packages.
package main

import (
	"log"

	"github.com/vaibhav-prk/Wardgate/internal/server"
)

func main() {
	// Initialize the Redis client.
	rdb, err := server.NewRedisClient()
	if err != nil {
		log.Fatalf("failed to connect to redis %v", err)
	}
	log.Println("connected to redis")

	// Resolve the backend service URL.
	target, err := server.NewBackend()
	if err != nil {
		log.Fatalf("invalid backend URL: %v", err)
	}

	// Construct and run the API gateway server.
	srv := server.NewServer(rdb, target)
	log.Println("Gateway listening on :8080")

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
	log.Println("Server shut down gracefully")
}
