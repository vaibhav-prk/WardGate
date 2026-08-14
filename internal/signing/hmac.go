package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/vaibhav-prk/Wardgate/internal/config"
)

// Signer verifies the HMAC-SHA256 signature on every inbound request.
// A signature mismatch means the request was tampered with after signing.
// Construct one via NewSigner and register Handle as chi middleware.
type Signer struct {
	secret []byte
}

// NewSigner returns a Signer that verifies request signatures using secret.
func NewSigner(cfg *config.Config) *Signer {
	return &Signer{secret: cfg.JWTSecret}
}

// Handle is a chi-compatible middleware. It builds the canonical request
// string, recomputes the HMAC-SHA256 signature, and compares it against
// the X-Signature header provided by the client.
func (s *Signer) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.verify(r); err != nil {
			http.Error(w, "forbidden: invalid request signature", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// verify recomputes the canonical request HMAC and compares it with the
// X-Signature header.
func (s *Signer) verify(r *http.Request) error {
	clientSig := r.Header.Get("X-Signature")
	if clientSig == "" {
		return errors.New("signing: missing X-Signature header")
	}

	canonical, err := canonicalRequest(r)
	if err != nil {
		return err
	}

	expected := s.sign(canonical)

	clientSigBytes, err := hex.DecodeString(clientSig)
	if err != nil {
		return errors.New("signing: X-Signature is not valid hex")
	}

	if !hmac.Equal(expected, clientSigBytes) {
		return errors.New("signing: signature mismatch")
	}

	return nil
}

// sign computes HMAC-SHA256 over the canonical string using the configured
// secret and returns the raw bytes.
func (s *Signer) sign(canonical string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(canonical))
	return mac.Sum(nil)
}
