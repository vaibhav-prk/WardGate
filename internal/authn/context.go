package authn

import (
	"context"
	"net/http"
)

// ctxKey is an unexported type for context keys in this package.
// Using a named type prevents collisions with keys from other packages.
type ctxKey string

const clientIDKey ctxKey = "client_id"

// SetClientID returns a shallow copy of r with clientID stored in its context.
func SetClientID(r *http.Request, clientID string) *http.Request {
	ctx := context.WithValue(r.Context(), clientIDKey, clientID)
	return r.WithContext(ctx)
}

// GetClientID retrieves the clientID stored by SetClientID.
// Returns an empty string if not set
func GetClientID(ctx context.Context) string {
	v, _ := ctx.Value(clientIDKey).(string)
	return v
}
