// Command cloudflare provides a CLI to synchronize a Cloudflare DNS A record
// with the machine's current public IP, using the reusable cloudflare client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jsirianni/cloudflare-go/cloudflare"
	"github.com/jsirianni/cloudflare-go/internal/netutil"
)

func main() {
	var (
		zone      = flag.String("zone", envOr("ZONE", ""), "Cloudflare zone (apex domain)")
		name      = flag.String("name", envOr("NAME", ""), "Record name/label within the zone")
		ttl       = flag.Int("ttl", envOrInt("TTL", 1), "TTL in seconds (1=auto)")
		proxied   = flag.Bool("proxied", envOrBool("PROXIED", false), "Whether the record is proxied")
		email     = flag.String("email", envOr("CF_EMAIL", ""), "Cloudflare account email (Global Key auth)")
		globalKey = flag.String("global-key", envOr("CF_GLOBAL_KEY", ""), "Cloudflare Global API Key")
		apiToken  = flag.String("api-token", envOr("CF_API_TOKEN", ""), "Cloudflare API Token (preferred)")
		timeout   = flag.Duration("timeout", envOrDuration("TIMEOUT", 30*time.Second), "Overall timeout")
	)
	flag.Parse()

	if err := run(*zone, *name, *ttl, *proxied, *email, *globalKey, *apiToken, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(zone, name string, ttl int, proxied bool, email, globalKey, apiToken string, timeout time.Duration) error {
	if err := validateInputs(zone, name, ttl, email, globalKey, apiToken); err != nil {
		return err
	}

	// Context with cancel on interrupt and deadline
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ctx = withSignalCancel(ctx, cancel)

	// Construct client
	var (
		c   *cloudflare.Client
		err error
	)
	if apiToken != "" && email == "" && globalKey == "" {
		c, err = cloudflare.New(cloudflare.WithAPIToken(apiToken))
	} else if apiToken == "" && email != "" && globalKey != "" {
		c, err = cloudflare.New(cloudflare.WithGlobalKey(email, globalKey))
	} else {
		return errors.New("provide either api-token or email+global-key, not both")
	}
	if err != nil {
		return err
	}

	// Discover IP
	wanIP, err := netutil.DiscoverIPv4ViaIpify(ctx, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		return fmt.Errorf("could not determine WAN IP: %w", err)
	}

	// Resolve zone ID
	zoneID, err := c.FindZoneID(ctx, zone)
	if err != nil {
		return err
	}

	fqdn := name + "." + zone
	rec, err := c.GetARecord(ctx, zoneID, fqdn)
	if err != nil {
		return err
	}
	payload := cloudflare.DNSRecord{Type: "A", Name: name, Content: wanIP, TTL: ttl, Proxied: proxied}
	if rec != nil {
		if rec.Content == wanIP {
			fmt.Printf("No change: %s already points to %s\n", fqdn, wanIP)
			return nil
		}
		if _, err := c.UpdateARecord(ctx, zoneID, rec.ID, payload); err != nil {
			return err
		}
		fmt.Printf("Updated A %s -> %s\n", fqdn, wanIP)
		return nil
	}
	if _, err := c.CreateARecord(ctx, zoneID, payload); err != nil {
		return err
	}
	fmt.Printf("Created A %s -> %s\n", fqdn, wanIP)
	return nil
}

func validateInputs(zone, name string, ttl int, email, globalKey, apiToken string) error {
	if strings.TrimSpace(zone) == "" {
		return errors.New("zone is required")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if ttl < 0 {
		return errors.New("ttl must be >= 0 (1 for auto)")
	}
	haveGlobal := email != "" && globalKey != ""
	haveToken := apiToken != ""
	if haveGlobal && haveToken {
		return errors.New("provide either api-token or email+global-key, not both")
	}
	if !haveGlobal && !haveToken {
		return errors.New("missing credentials: set CF_API_TOKEN or CF_EMAIL and CF_GLOBAL_KEY")
	}
	return nil
}

func withSignalCancel(ctx context.Context, cancel context.CancelFunc) context.Context {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return n
		}
	}
	return def
}

func envOrBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "t", "true", "y", "yes":
			return true
		case "0", "f", "false", "n", "no":
			return false
		}
	}
	return def
}

func envOrDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
