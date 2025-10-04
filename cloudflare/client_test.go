package cloudflare_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jsirianni/cloudflare-go/cloudflare"
	"github.com/stretchr/testify/require"
)

type recorded struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

func newTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
}

func mustClient(t *testing.T, opts ...cloudflare.Option) *cloudflare.Client {
	t.Helper()
	c, err := cloudflare.New(opts...)
	require.NoError(t, err)
	return c
}

func TestFindZoneID_Success(t *testing.T) {
	var got recorded
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = capture(r)
		require.Equal(t, "/zones", r.URL.Path)
		require.Equal(t, "name=example.com", r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  []map[string]any{{"id": "abc123", "name": "example.com"}},
		})
	})
	defer srv.Close()

	c := mustClient(t, cloudflare.WithAPIToken("tok"), cloudflare.WithBaseURL(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	id, err := c.FindZoneID(ctx, "example.com")
	require.NoError(t, err)
	require.Equal(t, "abc123", id)
	// header assertions
	require.Equal(t, "application/json", got.Header.Get("Content-Type"))
	require.NotEmpty(t, got.Header.Get("User-Agent"))
	require.Equal(t, "Bearer "+"tok", got.Header.Get("Authorization"))
}

func TestFindZoneID_NotFound(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  []any{},
		})
	})
	defer srv.Close()

	c := mustClient(t, cloudflare.WithAPIToken("tok"), cloudflare.WithBaseURL(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.FindZoneID(ctx, "missing.com")
	require.Error(t, err)
}

func TestGetARecord_FoundAndNotFound(t *testing.T) {
	calls := 0
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/zones/zid/dns_records", r.URL.Path)
		q := r.URL.Query()
		if q.Get("name") == "a.example.com" && q.Get("type") == "A" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": []map[string]any{{
					"id":      "rid",
					"type":    "A",
					"name":    "a.example.com",
					"content": "203.0.113.1",
					"ttl":     300,
					"proxied": false,
				}},
			})
			return
		}
		// Not found
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  []any{},
		})
	})
	defer srv.Close()

	c := mustClient(t, cloudflare.WithAPIToken("tok"), cloudflare.WithBaseURL(srv.URL))
	ctx := context.Background()

	rec, err := c.GetARecord(ctx, "zid", "a.example.com")
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, "rid", rec.ID)

	rec, err = c.GetARecord(ctx, "zid", "b.example.com")
	require.NoError(t, err)
	require.Nil(t, rec)
	require.GreaterOrEqual(t, calls, 2)
}

func TestCreateAndUpdateARecord_Success(t *testing.T) {
	var created, updated bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/zones/zid/dns_records" {
			created = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"id":      "rid",
					"type":    "A",
					"name":    "home",
					"content": "203.0.113.9",
					"ttl":     1,
					"proxied": false,
				},
			})
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/zones/zid/dns_records/rid" {
			updated = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"id":      "rid",
					"type":    "A",
					"name":    "home",
					"content": "203.0.113.10",
					"ttl":     300,
					"proxied": true,
				},
			})
			return
		}
		http.NotFound(w, r)
	})
	defer srv.Close()

	c := mustClient(t, cloudflare.WithAPIToken("tok"), cloudflare.WithBaseURL(srv.URL))
	ctx := context.Background()

	rec, err := c.CreateARecord(ctx, "zid", cloudflare.DNSRecord{Type: "A", Name: "home", Content: "203.0.113.9", TTL: 1, Proxied: false})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, rec)
	require.Equal(t, "rid", rec.ID)

	rec, err = c.UpdateARecord(ctx, "zid", "rid", cloudflare.DNSRecord{Type: "A", Name: "home", Content: "203.0.113.10", TTL: 300, Proxied: true})
	require.NoError(t, err)
	require.True(t, updated)
	require.NotNil(t, rec)
	require.Equal(t, "203.0.113.10", rec.Content)
}

func TestGlobalKeyAuthHeaders(t *testing.T) {
	var got recorded
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = capture(r)
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []any{map[string]any{"id": "z"}}})
	})
	defer srv.Close()

	c := mustClient(t, cloudflare.WithGlobalKey("me@example.com", "key123"), cloudflare.WithBaseURL(srv.URL))
	ctx := context.Background()
	_, _ = c.FindZoneID(ctx, "example.com")

	require.Equal(t, "me@example.com", got.Header.Get("X-Auth-Email"))
	require.Equal(t, "key123", got.Header.Get("X-Auth-Key"))
	require.NotEmpty(t, got.Header.Get("User-Agent"))
	require.Equal(t, "application/json", got.Header.Get("Content-Type"))
}

func capture(r *http.Request) recorded {
	var body string
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}
	return recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: body}
}
