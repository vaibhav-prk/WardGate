package server

import (
	"errors"
	"net/url"

	"github.com/vaibhav-prk/Wardgate/internal/config"
)

// ParseBackendURL parses and returns the backend service URL.
func ParseBackendURL(cfg *config.Config) (*url.URL, error) {
	target, err := url.Parse(cfg.BackendURL)
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
