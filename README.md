# WardGate

WardGate is a Go API gateway prototype that sits in front of a backend service. It authenticates requests with JWTs, verifies HMAC-SHA256 request signatures, rejects replayed requests, and applies per-client sliding-window rate limits. Its adaptive limiter adjusts a client's effective limit using a behavioral risk score.

## Features

- JWT bearer-token authentication using a required `client_id` claim.
- HMAC-SHA256 verification over a canonical representation of each request.
- Replay protection using a timestamp and one-time nonce stored in Redis.
- Redis-backed static or adaptive sliding-window rate limiting.
- Risk-based policy responses when a request exceeds its limit.
- A `/health` endpoint and a small mock backend for local development.
- Benchmark scenarios comparing static and adaptive limiter modes.

## Request flow

Requests to `/api/*` pass through these checks before being proxied to the configured backend:

1. Verify the JWT and get the client ID from its `client_id` claim.
2. Verify the `X-Signature` HMAC.
3. Validate the timestamp and ensure the `X-Nonce` has not been used for that client.
4. Record the request, calculate its behavioral risk score, and apply the selected rate limiter and policy.
5. Proxy allowed requests to the backend.

`/health` is public and does not pass through the `/api` middleware.

## Requirements

- Go **1.26.5** (see `go.mod`).
- Redis, either installed locally or started with Docker Compose.
- Docker with the Compose plugin for the containerized setup.

## Run with Docker Compose

Create a `.env` file in the repository root:

```dotenv
JWT_SECRET=replace-with-a-long-random-secret
LIMITER_MODE=static
```

Use a securely generated secret in real deployments. The same `JWT_SECRET` is used to verify JWTs and request signatures.

Start the gateway, Redis, and mock backend:

```sh
docker compose --env-file .env -f deploy/docker-compose.yml up --build
```

The gateway is available at `http://localhost:8080`; the mock backend is available at `http://localhost:8081`. Stop the services with:

```sh
docker compose --env-file .env -f deploy/docker-compose.yml down
```

To run the adaptive limiter, set `LIMITER_MODE=adaptive` in `.env` and restart the services.

## Run locally

Start Redis and the mock backend in separate terminals. From the repository root:

```sh
go run ./deploy/mock-backend
```

In another terminal, configure and start the gateway. For PowerShell:

```powershell
$env:JWT_SECRET = "replace-with-a-long-random-secret"
$env:REDIS_URL = "localhost:6379"
$env:BACKEND_URL = "http://localhost:8081"
$env:LIMITER_MODE = "static"
go run ./cmd/gateway
```

To use adaptive limiting, set `$env:LIMITER_MODE = "adaptive"` before starting the gateway.

## Configuration

All settings are read from environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | Address and port for the gateway HTTP server. |
| `JWT_SECRET` | Required | Shared secret for JWT verification and request HMAC verification. |
| `REDIS_URL` | `localhost:6379` | Redis address. |
| `REDIS_POOL_SIZE` | `10` | Redis connection pool size. |
| `BACKEND_URL` | `http://localhost:8081` | HTTP(S) URL of the backend to which allowed `/api/*` requests are proxied. |
| `LIMITER_MODE` | `static` | Rate-limit mode: `static` or `adaptive`. |
| `BASE_LIMIT` | `100` | Maximum requests per client within the configured window at the base risk level. |
| `WINDOW_MS` | `1000` | Sliding-window duration in milliseconds. |
| `NONCE_WINDOW_SEC` | `300` | Allowed timestamp drift, in seconds, in either direction. |

The Compose configuration sets `BACKEND_URL` and `REDIS_URL` to the internal service addresses. It reads `JWT_SECRET` and `LIMITER_MODE` from the root `.env` file.

## Calling the API

Every request to `/api/*` must include:

| Header | Purpose |
| --- | --- |
| `Authorization` | JWT bearer token signed with HS256. Its claims must include a non-empty `client_id`. |
| `X-Signature` | Lowercase hexadecimal HMAC-SHA256 signature of the canonical request. |
| `X-Timestamp` | Current Unix timestamp in seconds. |
| `X-Nonce` | A unique nonce for the client within the replay window. |

The client ID used for rate limiting comes from the JWT; `X-Client-ID` is not used to establish client identity.

The canonical request consists of these newline-separated fields:

```text
HTTP_METHOD
URL_PATH
CANONICAL_QUERY
CANONICAL_HEADERS

SHA256_BODY_HEX
```

Query parameters are ordered by key, and values for each key are sorted; each is represented as `key=value`, with entries joined by `&`. The body is hashed with SHA-256 and represented as lowercase hexadecimal. If `X-Signed-Headers` is supplied, it is a semicolon-separated list of header names; those names are lowercased and sorted, and their trimmed values are included in the canonical headers field. Leave that field empty when no additional headers are being signed.

The signature is `HMAC-SHA256(JWT_SECRET, canonical_request)`, encoded as lowercase hexadecimal. The JWT must use HS256 and include the same `client_id` that should own the request. Include an expiration claim (`exp`) in client-issued tokens.

For example, a request to the mock backend can use `GET /api/users`, provided the JWT, timestamp, unique nonce, and signature are generated for that exact method and path. A missing or invalid JWT returns `401`; a missing or invalid signature returns `403`; missing replay headers return `400`; an expired timestamp or duplicate nonce returns `429`. Rate limiting can also return `429`, and a denied request with elevated risk can return `401` (step-up required) or `403` (blocked).

Check the public health endpoint with:

```sh
curl http://localhost:8080/health
```

## Rate limiting and risk policy

- `static` mode applies the same Redis-backed sliding-window limit to each client.
- `adaptive` mode reduces the effective limit as the risk score rises, subject to a 10% minimum fraction of the base limit.
- When the limiter denies a request, the policy returns `429` below a risk score of 60, `401` from 60 up to (but not including) 85, and `403` at 85 or above.

The current behavioral tracker keeps its rolling request history in gateway process memory. Redis stores limiter and replay state. In a multi-instance deployment, process-local risk histories are therefore not shared between gateway instances.

## Tests

Run the Go test suite from the repository root:

```sh
go test ./...
```

## Benchmarks

The benchmark suite uses Docker Compose, [k6](https://k6.io/), and Go. It runs legitimate steady/bursty traffic, attack-generator scenarios, and a volumetric scenario against both limiter modes. Create the root `.env` file as described above, then run from a Bash-compatible shell:

```sh
bash benchmarks/run_all.sh
```

Pass `-b` to build the Docker images before running:

```sh
bash benchmarks/run_all.sh -b
```

The script writes k6 JSON and attack-generator logs to `benchmarks/results/`. To summarize the k6 results and generate the latency comparison plot, install Python with `numpy` and `matplotlib`, then run:

```sh
python analysis/main.py
```

The plot is saved to `analysis/plots/latency_profile.png`.

## Project structure

```text
cmd/gateway/       Gateway application entry point
internal/          Authentication, configuration, limiters, policy, replay,
                   risk tracking, server, and request signing
deploy/            Dockerfiles and Docker Compose configuration
benchmarks/        Load scenarios and attack-scenario generator
analysis/          Benchmark result parsing and plotting
```

## Security note

WardGate is a prototype, not a production security boundary. It uses a shared secret for JWT and request-signature verification, and its behavioral risk history is process-local. Deploy behind TLS, protect and rotate secrets, and review the authentication, key-management, Redis, and multi-instance requirements for your environment before exposing it to untrusted traffic.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
