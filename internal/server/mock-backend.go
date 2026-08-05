package server

import (
	"net/url"
	"os"
)

// ParseBackendURL parses and returns the backend service URL.
func ParseBackendURL() (*url.URL, error) {
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8081"
	}

	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}

	return target, nil
}
