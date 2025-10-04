### Cloudflare Dynamic DNS (Go)

This project provides a Go implementation of a Cloudflare Dynamic DNS updater and a reusable Cloudflare API client.

> Notice: This is a personal passion project developed by [jsirianni](https://github.com/jsirianni) with the assistance of AI. Use at your own risk.

### Requirements

- Go 1.25+
- Cloudflare API Token (recommended) or Global API Key + Email

### Installation

```bash
go install github.com/jsirianni/cloudflare-go/cmd/cloudflare@latest
```

This installs the `cloudflare` CLI.

### CLI Usage

```bash
cloudflare \
  -zone example.com \
  -name home \
  -ttl 300 \
  -proxied=false \
  -api-token $CF_API_TOKEN
```

Environment variables are supported (flags override env):

- `ZONE`, `NAME`, `TTL`, `PROXIED`
- `CF_API_TOKEN` (preferred)
- Or `CF_EMAIL` + `CF_GLOBAL_KEY`

Other options:

- `-timeout` (default 30s)
 

### Behavior

- Discovers current public IPv4 via the IP echo URL
- Resolves Cloudflare Zone ID by `-zone`
- Gets existing A record for `NAME.ZONE`
- If record exists and matches current IP, exits with "No change" (success)
- Else updates or creates the A record to point to the current IP

 

### Library Usage

Import the `cloudflare` package to use the API client in your own projects:

```go
package main

import (
    "context"
    "fmt"
    "github.com/jsirianni/cloudflare-go/cloudflare"
)

func main() {
    c, err := cloudflare.New(cloudflare.WithAPIToken("<token>"))
    if err != nil { panic(err) }

    ctx := context.Background()
    zoneID, err := c.FindZoneID(ctx, "example.com")
    if err != nil { panic(err) }

    // Create or update an A record
    rec := cloudflare.DNSRecord{Type: "A", Name: "home", Content: "203.0.113.42", TTL: 300, Proxied: false}
    created, err := c.CreateARecord(ctx, zoneID, rec)
    if err != nil { panic(err) }
    fmt.Println("created id:", created.ID)
}
```

### Design Notes

- Standard library only; context-aware with timeouts and clean cancellation
- Options pattern for client configuration (`WithAPIToken`, `WithBaseURL`, `WithTimeout`, etc.)
- Strong input validation and explicit types for API payloads/responses
- Structured for extension to additional Cloudflare endpoints

### License

MIT — see `LICENSE`.


"# cloudflare-go" 
