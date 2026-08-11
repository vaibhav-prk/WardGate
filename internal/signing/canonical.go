// Package signing provides HMAC-SHA256 request signing verification for the
// gateway. It detects parameter tampering by re-deriving and comparing the
// signature over a canonical request string on every inbound request.
package signing

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// canonicalRequest builds a deterministic string from the request that both
// the client and gateway agree to sign.
func canonicalRequest(r *http.Request) (string, error) {
	method := r.Method
	path := r.URL.Path
	query := canonicalQueryString(r)
	headers := canonicalHeaders(r)
	bodyHash, err := hashBody(r)
	if err != nil {
		return "", fmt.Errorf("signing: failed to hash body: %w", err)
	}

	return strings.Join([]string{
		method,
		path,
		query,
		headers,
		"",
		bodyHash,
	}, "\n"), nil
}

// canonicalQueryString sorts query parameters by key and encodes them.
// Sorting ensures that param order differences don't break verification.
func canonicalQueryString(r *http.Request) string {
	params := r.URL.Query()

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		// sort multiple values for the same key too
		vals := params[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, k+"="+v)
		}
	}

	return strings.Join(parts, "&")
}

// canonicalHeaders builds a sorted, lowercased header string from the
// required signed headers. Only headers the client declared in
// X-Signed-Headers are included — anything else is ignored.
func canonicalHeaders(r *http.Request) string {
	signed := r.Header.Get("X-Signed-Headers")
	if signed == "" {
		return ""
	}

	names := strings.Split(signed, ";")
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(strings.ToLower(name))
		val := strings.TrimSpace(r.Header.Get(name))
		parts = append(parts, name+":"+val)
	}

	return strings.Join(parts, "\n")
}

// hashBody reads the request body, computes its SHA-256 hash, and restores
// the body so downstream handlers can still read it.
// An empty body produces the SHA-256 of an empty string.
func hashBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return emptySHA256(), nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	// restore the body so downstream middleware and the proxy can still read it
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum), nil
}

// emptySHA256 returns the hex-encoded SHA-256 of an empty byte slice.
func emptySHA256() string {
	sum := sha256.Sum256([]byte{})
	return fmt.Sprintf("%x", sum)
}
