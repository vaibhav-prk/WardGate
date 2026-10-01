import crypto from "k6/crypto";
import encoding from "k6/encoding";

const SECRET_KEY =
  __ENV.JWT_SECRET || "super-secret-jwt-key-for-wardgate-capstone-test-32bytes";
const CLIENT_ID = __ENV.CLIENT_ID || "k6-tenant-test";

// Base64URL encoder helper using k6/encoding
function toBase64Url(str) {
  return encoding
    .b64encode(str)
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

// Generate valid HS256 JWT
export function generateJWT(clientId = CLIENT_ID, secret = SECRET_KEY) {
  const header = JSON.stringify({ alg: "HS256", typ: "JWT" });
  const now = Math.floor(Date.now() / 1000);
  const payload = JSON.stringify({
    client_id: clientId,
    iat: now,
    exp: now + 3600,
  });

  const encodedHeader = toBase64Url(header);
  const encodedPayload = toBase64Url(payload);
  const signingInput = `${encodedHeader}.${encodedPayload}`;

  // Generate base64 signature and convert to base64url
  const sigBase64 = crypto.hmac("sha256", secret, signingInput, "base64");
  const sigBase64Url = sigBase64
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");

  return `${signingInput}.${sigBase64Url}`;
}

// Canonical HMAC-SHA256 signing matching internal/signing/canonical.go
export function generateHeaders(
  method,
  path,
  body = "",
  clientId = CLIENT_ID,
  secret = SECRET_KEY,
) {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = `${Date.now()}-${Math.floor(Math.random() * 1000000)}`;

  // SHA-256 hash of body (empty string produces hash of empty string)
  const bodyHashHex = crypto.sha256(body || "", "hex");

  // Format: method \n path \n query \n headers \n \n bodyHash
  const canonicalRequest = `${method}\n${path}\n\n\n\n${bodyHashHex}`;

  // Gateway expects lowercase hex for X-Signature
  const signatureHex = crypto.hmac("sha256", secret, canonicalRequest, "hex");

  const token = generateJWT(clientId, secret);

  return {
    Authorization: `Bearer ${token}`,
    "X-Client-ID": clientId,
    "X-Signature": signatureHex,
    "X-Timestamp": timestamp,
    "X-Nonce": nonce,
    "Content-Type": "application/json",
  };
}
