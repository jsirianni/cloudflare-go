### Cloudflare Dynamic DNS - AI Guide

This document briefs AI tools and future developers on the repository's purpose, architecture, layout, public APIs, testing strategy, and CI. Copy/paste this into prompts to accelerate safe, consistent changes.

### Purpose and Goals

- Keep a Cloudflare DNS A record synchronized with the host's current public IP.
- Provide a clean, reusable Go client for the Cloudflare v4 API using only the standard library.
- Favor composability, strong typing, context-driven cancellation, and an options pattern that supports extension.

### High-Level Behavior

- Discover public IPv4 via ipify.org.
- Resolve target zone by name.
- Read DNS A record for the FQDN and reconcile: no-op if equal, otherwise create or update.

### Repository Layout

- `cloudflare/`: Reusable Cloudflare API client
  - `client.go`: Client, options pattern, auth headers, HTTP and URL handling
  - `dns.go`: Types and methods for zones and DNS records (A record focus)
  - `client_test.go`: Unit tests using `httptest.Server` (no real network)
- `cmd/cloudflare/`: CLI that wires flags/env to `cloudflare` package and performs the dynamic DNS flow
- `internal/netutil/`:
  - `ip.go`: `DiscoverIPv4ViaIpify` to fetch the public IPv4 with context and validation
  - `ip_test.go`: Unit tests with mocked transport (no real network)
  - `ip_integration_test.go`: Integration test that calls ipify directly; validates IPv4 or IPv6
- `.github/workflows/ci.yml`: CI with build, test, gosec, staticcheck, and revive
- `README.md`: Usage docs for CLI and library

### Public API (cloudflare package)

- Client construction (options-only API):
  - `New(opts ...Option) (*Client, error)`
  - Options:
    - `WithAPIToken(token string)`
    - `WithGlobalKey(email, key string)`
    - `WithBaseURL(url string)`
    - `WithTimeout(d time.Duration)`
    - `WithTLSConfig(cfg *tls.Config)`
    - `WithHTTPClient(c *http.Client)`

- Auth guarantees: exactly one of API token or global key+email must be set; both or neither return an error.

- DNS/Zone operations:
  - `FindZoneID(ctx, zoneName string) (string, error)`
  - `GetARecord(ctx, zoneID, fqdn string) (*DNSRecord, error)`
  - `CreateARecord(ctx, zoneID string, payload DNSRecord) (*DNSRecord, error)`
  - `UpdateARecord(ctx, zoneID, recordID string, payload DNSRecord) (*DNSRecord, error)`

- Types:
  - `type DNSRecord { ID, Type, Name, Content string; TTL int; Proxied bool }`
  - `type Zone { ID, Name string }`

### CLI Behavior (cmd/cloudflare)

- Flags with env fallbacks: `-zone`, `-name`, `-ttl`, `-proxied`, `-email`, `-global-key`, `-api-token`, `-timeout`.
- Validation is centralized in `validateInputs`.
- Flow: build context with timeout and OS signal cancel → construct client based on provided auth → discover WAN IP via `netutil.DiscoverIPv4ViaIpify` → find zone → get A record → no-op/update/create.

### Design Principles

- Standard library only (production code); external dependencies allowed in test files.
- Context everywhere; graceful cancellation and timeouts.
- Options pattern for configuration; clean separation of required vs optional params.
- Strong typing for request/response payloads; JSON unmarshalling via structs.
- Centralized constants for HTTP header names and timeouts.
- URL handling via `url.URL.ResolveReference` rather than manual string concat.
- Clear, actionable errors; fail fast on misconfiguration.

### Testing Strategy

- Unit tests (always run):
  - `internal/netutil/ip_test.go`: table-driven tests using a stub `http.Transport` to simulate success/failure/timeouts/cancellations.
  - `cloudflare/client_test.go`: `httptest.Server` mocks Cloudflare endpoints; validates paths, query strings, headers, JSON handling, and behaviors (found/not found/create/update).
- Integration tests (always run, internet required):
  - `internal/netutil/ip_integration_test.go`: hits ipify.org and asserts IPv4 or IPv6 parseable.
- No real Cloudflare API integration tests yet; these would require credentials and will be added later.

### CI

- `.github/workflows/ci.yml` defines jobs:
  - `build`: `go build ./...`
  - `test`: `go test ./... -v`
  - `gosec`: installs `gosec` and runs it across the repo
  - `staticcheck`: installs `staticcheck` and runs it across the repo
  - `revive`: installs `revive` and runs it with `-set_exit_status`
- `actions/setup-go@v5` uses `go-version-file: go.mod` for Go version pinning and caching.

### Making Changes (Guidance for AI Tools)

- Preserve the options pattern in `cloudflare/client.go`; add new options as `WithX(...) Option` without breaking existing API.
- For new Cloudflare endpoints:
  - Add request/response structs in the `cloudflare` package.
  - Use `Client.buildURL` to resolve paths and `Client.do` for HTTP with headers and context.
  - Add unit tests in `cloudflare/` using `httptest.Server` that validate request paths, query parameters, headers, and response parsing.
- For new CLI features:
  - Add flags and env fallbacks consistently; extend `validateInputs` when necessary.
  - Keep the main flow readable with early returns for error cases.
- For net utilities:
  - Keep network calls context-aware; avoid global state.
  - Ensure test coverage includes cancellation and deadline scenarios via mocked transports.

### Future Extensions

- DNS AAAA support (IPv6) mirroring the A record flow.
- Additional Cloudflare resources (e.g., TXT records, proxied settings, page rules) following the same patterns.
- Optional retries with backoff for transient HTTP failures (non-2xx, 429, 5xx), bounded by context deadlines.


