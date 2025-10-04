package netutil_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This integration test always runs and reaches out to api.ipify.org.
// It validates that the service returns a syntactically valid IPv4 or IPv6 address.
func TestIpify_Integration_ReturnsValidIP(t *testing.T) {
	t.Helper()

	const ipifyURL = "https://api.ipify.org"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipifyURL, nil)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.GreaterOrEqual(t, resp.StatusCode, 200)
	require.Less(t, resp.StatusCode, 300)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	ipStr := string(b)
	ip := net.ParseIP(ipStr)
	require.NotNil(t, ip, "expected a valid IPv4 or IPv6 address, got %q", ipStr)
}
