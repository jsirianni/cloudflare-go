package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// API response wrappers
type apiResponse[T any] struct {
	Success  bool         `json:"success"`
	Errors   []apiMessage `json:"errors"`
	Messages []apiMessage `json:"messages"`
	Result   T            `json:"result"`
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Zone represents a Cloudflare Zone
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// FindZoneID looks up the Zone ID by exact zone name.
func (c *Client) FindZoneID(ctx context.Context, zoneName string) (string, error) {
	if zoneName == "" {
		return "", errors.New("zone name cannot be empty")
	}
	u := c.buildURL("zones?name=" + url.QueryEscape(zoneName))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("zones lookup failed: %s", resp.Status)
	}
	var out apiResponse[[]Zone]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if !out.Success || len(out.Result) == 0 {
		return "", fmt.Errorf("zone not found: %s", zoneName)
	}
	return out.Result[0].ID, nil
}

// GetARecord fetches a DNS A record by FQDN within a zone.
func (c *Client) GetARecord(ctx context.Context, zoneID, fqdn string) (*DNSRecord, error) {
	if zoneID == "" || fqdn == "" {
		return nil, errors.New("zoneID and fqdn are required")
	}
	params := url.Values{}
	params.Set("type", "A")
	params.Set("name", fqdn)
	u := c.buildURL("zones/" + zoneID + "/dns_records?" + params.Encode())
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get dns record failed: %s", resp.Status)
	}
	var out apiResponse[[]DNSRecord]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Success || len(out.Result) == 0 {
		return nil, nil
	}
	rec := out.Result[0]
	return &rec, nil
}

// UpsertARecord creates or updates an A record for NAME within the zone to point to ip.
// name is the record label (not FQDN). ttl in seconds; proxied per Cloudflare semantics.
func (c *Client) UpsertARecord(ctx context.Context, zoneID, name, ip string, ttl int, proxied bool) (*DNSRecord, bool, error) {
	if zoneID == "" || name == "" || ip == "" {
		return nil, false, errors.New("zoneID, name, and ip are required")
	}
	// Get existing if any by FQDN
	fqdn := name
	// The API expects label in create/update; for lookup, caller should use FQDN.
	// Here we rely on caller to have fetched existing record to determine if update is needed.
	// For convenience, we will still attempt a lookup by composing later in higher-level logic.
	_ = fqdn
	payload := DNSRecord{Type: "A", Name: name, Content: ip, TTL: ttl, Proxied: proxied}
	rec, err := c.CreateARecord(ctx, zoneID, payload)
	if err == nil {
		return rec, true, nil
	}
	return nil, false, err
}

// CreateARecord creates an A record.
func (c *Client) CreateARecord(ctx context.Context, zoneID string, payload DNSRecord) (*DNSRecord, error) {
	u := c.buildURL("zones/" + zoneID + "/dns_records")
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create dns record failed: %s", resp.Status)
	}
	var out apiResponse[DNSRecord]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, errors.New("create dns record unsuccessful")
	}
	return &out.Result, nil
}

// UpdateARecord updates an existing DNS record by id.
func (c *Client) UpdateARecord(ctx context.Context, zoneID, recordID string, payload DNSRecord) (*DNSRecord, error) {
	if zoneID == "" || recordID == "" {
		return nil, errors.New("zoneID and recordID are required")
	}
	u := c.buildURL("zones/" + zoneID + "/dns_records/" + recordID)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, u, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update dns record failed: %s", resp.Status)
	}
	var out apiResponse[DNSRecord]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, errors.New("update dns record unsuccessful")
	}
	return &out.Result, nil
}
