package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

type AttackClient struct {
	BaseURL    string
	ClientID   string
	SecretKey  string
	HTTPClient *http.Client
}

func NewAttackClient(baseURL, clientID, secretKey string) *AttackClient {
	return &AttackClient{
		BaseURL:   baseURL,
		ClientID:  clientID,
		SecretKey: secretKey,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *AttackClient) generateToken(valid bool) string {
	if !valid {
		return "invalid.bearer.token"
	}
	claims := CustomClaims{
		ClientID: c.ClientID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(c.SecretKey))
	return tokenStr
}

func (c *AttackClient) signRequest(method, path string, query url.Values, body []byte) string {
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	var keys []string
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var queryParts []string
	for _, k := range keys {
		vals := query[k]
		sort.Strings(vals)
		for _, v := range vals {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	canonicalQuery := strings.Join(queryParts, "&")

	canonical := strings.Join([]string{
		method,
		path,
		canonicalQuery,
		"",
		"",
		bodyHashHex,
	}, "\n")

	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *AttackClient) SendRequest(ctx context.Context, method, path string, query url.Values, body []byte, validAuth bool, tamperSig bool, nonce string) (int, string, error) {
	sig := c.signRequest(method, path, query, body)
	if tamperSig {
		sig = sig + "tampered"
	}

	reqURL := fmt.Sprintf("%s%s", c.BaseURL, path)
	if len(query) > 0 {
		reqURL = fmt.Sprintf("%s?%s", reqURL, query.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}

	token := c.generateToken(validAuth)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Client-ID", c.ClientID)
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func main() {
	scenario := flag.String("scenario", "s3", "Attack scenario: s3, s4, s5, s6, s7")
	target := flag.String("target", "http://localhost:8080", "Target Gateway Base URL")
	secretKey := flag.String("secret", "super-secret-jwt-key-for-wardgate-capstone-test-32bytes", "Shared Secret Key (JWT & HMAC)")
	clientID := flag.String("client", "attacker-tenant-1", "Simulated Client ID")
	requests := flag.Int("count", 10, "Number of requests to execute")
	flag.Parse()

	ctx := context.Background()
	client := NewAttackClient(*target, *clientID, *secretKey)
	fmt.Printf("[WardGate AttackGen] Running Scenario %s against %s (Client: %s)...\n", *scenario, *target, *clientID)

	switch *scenario {
	case "s3":
		fmt.Println("[S3: Credential Stuffing] Injecting rapid invalid authentication attempts...")
		for i := 1; i <= *requests; i++ {
			nonce := fmt.Sprintf("nonce-s3-%d-%d", time.Now().UnixNano(), i)
			payload := []byte(fmt.Sprintf(`{"user":"victim_%d","pass":"wrong_pass"}`, i))
			status, _, _ := client.SendRequest(ctx, "POST", "/api/login", nil, payload, false, false, nonce)
			fmt.Printf("  -> Request #%d: Status %d (Expected: 401)\n", i, status)
			time.Sleep(50 * time.Millisecond)
		}

	case "s4":
		fmt.Println("[S4: Endpoint Scraping / Fan-Out] Scanning endpoints with valid Auth & Signatures...")
		endpoints := []string{"/api/users", "/api/accounts", "/api/orders", "/api/products", "/api/users"}
		for i := 1; i <= *requests; i++ {
			ep := endpoints[i%len(endpoints)]
			nonce := fmt.Sprintf("nonce-s4-%d-%d", time.Now().UnixNano(), i)
			status, _, _ := client.SendRequest(ctx, "GET", ep, nil, nil, true, false, nonce)
			fmt.Printf("  -> Hit %s: Status %d\n", ep, status)
			time.Sleep(80 * time.Millisecond)
		}

	case "s5":
		fmt.Println("[S5: Replay Attack] Capturing valid signed request and replaying duplicate nonce...")
		fixedNonce := fmt.Sprintf("fixed-replayed-nonce-%d", time.Now().Unix())
		status1, _, _ := client.SendRequest(ctx, "GET", "/api/users", nil, nil, true, false, fixedNonce)
		fmt.Printf("  -> Legitimate Request (Nonce: %s): Status %d\n", fixedNonce, status1)
		time.Sleep(100 * time.Millisecond)
		status2, _, _ := client.SendRequest(ctx, "GET", "/api/users", nil, nil, true, false, fixedNonce)
		fmt.Printf("  -> Replay Attempt (Nonce: %s): Status %d (Expected: 429/403)\n", fixedNonce, status2)

	case "s6":
		fmt.Println("[S6: Parameter Tampering] Mutating signed payload post-signature...")
		nonce := fmt.Sprintf("nonce-s6-%d", time.Now().UnixNano())
		status, _, _ := client.SendRequest(ctx, "POST", "/api/users", nil, []byte(`{"role":"admin"}`), true, true, nonce)
		fmt.Printf("  -> Tampered Signature Request: Status %d (Expected: 403)\n", status)

	case "s7":
		fmt.Println("[S7: Slow-Drip Escalation] Measuring Time-To-Detect with escalating anomalies...")
		startTime := time.Now()
		detected := false
		for i := 1; i <= *requests; i++ {
			nonce := fmt.Sprintf("nonce-s7-%d-%d", time.Now().UnixNano(), i)
			tamper := i > (*requests / 2)
			delay := time.Duration(300-(i*20)) * time.Millisecond
			if delay < 50*time.Millisecond {
				delay = 50 * time.Millisecond
			}

			status, _, _ := client.SendRequest(ctx, "GET", "/api/users", nil, nil, true, tamper, nonce)
			fmt.Printf("  -> Request #%d (Tampered=%v): Status %d\n", i, tamper, status)

			if (status == 429 || status == 403) && !detected {
				fmt.Printf("  [!] ATTACK DETECTED at request #%d! Time-to-detect: %v\n", i, time.Since(startTime))
				detected = true
			}
			time.Sleep(delay)
		}

	default:
		fmt.Printf("Unknown scenario: %s\n", *scenario)
	}
}
