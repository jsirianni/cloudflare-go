package netutil

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// DiscoverIPv4ViaIpify fetches the public IPv4 address using ipify.org.
func DiscoverIPv4ViaIpify(ctx context.Context, client *http.Client) (string, error) {
	const ipifyURL = "https://api.ipify.org"
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipifyURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ip := string(b)
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return "", errors.New("invalid IPv4 response")
	}
	return parsed.String(), nil
}
