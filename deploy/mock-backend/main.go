package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", handler)

	srv := http.Server{
		Addr:              ":8081",
		Handler:           nil,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Println("Mock backend listening on :8081")

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("mock backend error: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"path":    r.URL.Path,
		"method":  r.Method,
		"time":    time.Now().UTC().Format(time.RFC3339),
		"headers": r.Header,
	}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
