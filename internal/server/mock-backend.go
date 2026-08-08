package server

import (
	"errors"
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

	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("URL must use http or https")
	}

	if target.Host == "" {
		return nil, errors.New("URL must contain a host")
	}

	return target, nil
}
