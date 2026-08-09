// Package authn provides zero-trust request authentication for the gateway.
// Every request must carry a valid JWT — no network-origin trust is granted.
package authn

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vaibhav-prk/Wardgate/internal/config"
)

// claims defines the required fields every valid gateway JWT must contain.
type claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// Authenticator verifies JWTs on every inbound request.
// Construct one via New and register Handle as chi middleware.
type Authenticator struct {
	secret []byte
}

func New(cfg *config.Config) *Authenticator {
	return &Authenticator{secret: cfg.JWTSecret}
}

func (a *Authenticator) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := a.extractAndVerify(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		c, ok := token.Claims.(*claims)
		if !ok || c.ClientID == "" {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, SetClientID(r, c.ClientID))
	})
}

func (a *Authenticator) extractAndVerify(r *http.Request) (*jwt.Token, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, errors.New("auth: missing Bearer token")
	}

	raw := strings.TrimPrefix(header, "Bearer ")

	token, err := jwt.ParseWithClaims(raw, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("auth: unexpected signing method")
		}

		return a.secret, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("auth: invalid token")
	}

	return token, nil
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
