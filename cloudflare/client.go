// Package cloudflare provides a minimal, extensible Cloudflare v4 API client
// with standard-library-only dependencies, context-aware HTTP requests,
// and an options pattern for clean configuration and future growth.
package cloudflare

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://api.cloudflare.com/client/v4"
	defaultUserAgent = "cloudflare-go/1.0 (+github.com/jsirianni/cloudflare-go)"

	// Timeouts
	dialTimeout           = 10 * time.Second
	keepAlive             = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	expectContinueTimeout = 1 * time.Second
	defaultHTTPTimeout    = 30 * time.Second

	// Header keys
	headerContentType = "Content-Type"
	headerUserAgent   = "User-Agent"
	headerAuthEmail   = "X-Auth-Email"
	headerAuthKey     = "X-Auth-Key"
	headerAuthz       = "Authorization"
)

// AuthMode defines the authentication method used by the client.
type AuthMode int

const (
	// AuthGlobalKey uses X-Auth-Email and X-Auth-Key headers.
	AuthGlobalKey AuthMode = iota
	// AuthAPIToken uses an OAuth2-style Bearer token in Authorization header.
	AuthAPIToken
)

// Options holds optional configuration for the Client.
type Options struct {
	// BaseURL allows overriding the Cloudflare API base URL.
	BaseURL string
	// HTTPClient allows injecting a custom http.Client.
	HTTPClient *http.Client
	// UserAgent allows customizing the User-Agent header.
	UserAgent string
	// TLSConfig allows customizing TLS settings.
	TLSConfig *tls.Config
	// Timeout applies if HTTPClient is nil; otherwise caller configures their client.
	Timeout time.Duration
	// APIToken if set, configures token-based authentication.
	APIToken string
	// GlobalKey and Email for legacy auth.
	GlobalKey string
	Email     string
}

// Option is a functional option for configuring Options.
type Option func(*Options)

// WithBaseURL sets a custom API base URL.
func WithBaseURL(base string) Option { return func(o *Options) { o.BaseURL = base } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(o *Options) { o.HTTPClient = c } }

// WithUserAgent sets a custom user agent.
func WithUserAgent(ua string) Option { return func(o *Options) { o.UserAgent = ua } }

// WithTLSConfig sets TLS configuration used by the default transport.
func WithTLSConfig(cfg *tls.Config) Option { return func(o *Options) { o.TLSConfig = cfg } }

// WithTimeout sets default timeout if using the internal http.Client.
func WithTimeout(d time.Duration) Option { return func(o *Options) { o.Timeout = d } }

// WithAPIToken sets the API token for token-based authentication.
func WithAPIToken(token string) Option { return func(o *Options) { o.APIToken = token } }

// WithGlobalKey sets the global key and email for legacy authentication.
func WithGlobalKey(email, key string) Option {
	return func(o *Options) { o.Email, o.GlobalKey = email, key }
}

// Client is a Cloudflare API client.
type Client struct {
	authMode   AuthMode
	email      string
	globalKey  string
	apiToken   string
	baseURL    *url.URL
	httpClient *http.Client
	userAgent  string
}

// New constructs a new Cloudflare client. Exactly one of (email+globalKey) or (apiToken) must be provided.
// Example:
//
//	New("user@example.com", "<global-key>")
//	New("", "", WithAPIToken("<token>"))
func New(opts ...Option) (*Client, error) {
	options := Options{}
	for _, opt := range opts {
		opt(&options)
	}

	// Determine auth mode and validate mutual exclusivity
	var mode AuthMode
	haveGlobal := options.Email != "" && options.GlobalKey != ""
	haveToken := options.APIToken != ""
	if haveGlobal && haveToken {
		return nil, errors.New("invalid auth: specify either global key (with email) or api token, not both")
	}
	if haveGlobal {
		mode = AuthGlobalKey
	} else if haveToken {
		mode = AuthAPIToken
	} else {
		return nil, errors.New("missing auth: provide global key+email or api token")
	}

	base := options.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAlive,
			}).DialContext,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ExpectContinueTimeout: expectContinueTimeout,
			TLSClientConfig:       options.TLSConfig,
		}
		httpClient = &http.Client{Timeout: options.Timeout}
		if httpClient.Timeout == 0 {
			httpClient.Timeout = defaultHTTPTimeout
		}
		httpClient.Transport = transport
	}

	userAgent := options.UserAgent
	if strings.TrimSpace(userAgent) == "" {
		userAgent = defaultUserAgent
	}

	c := &Client{
		authMode:   mode,
		email:      options.Email,
		globalKey:  options.GlobalKey,
		baseURL:    parsed,
		httpClient: httpClient,
		userAgent:  userAgent,
	}
	if mode == AuthAPIToken {
		c.apiToken = options.APIToken
	}

	return c, nil
}

// do sends an HTTP request to the Cloudflare API with proper headers and context.
func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.Clone(ctx)
	req.Header.Set(headerContentType, "application/json")
	req.Header.Set(headerUserAgent, c.userAgent)
	switch c.authMode {
	case AuthGlobalKey:
		if c.email == "" || c.globalKey == "" {
			return nil, errors.New("missing global key credentials")
		}
		req.Header.Set(headerAuthEmail, c.email)
		req.Header.Set(headerAuthKey, c.globalKey)
	case AuthAPIToken:
		if c.apiToken == "" {
			return nil, errors.New("missing API token")
		}
		req.Header.Set(headerAuthz, "Bearer "+c.apiToken)
	default:
		return nil, errors.New("unknown auth mode")
	}
	return c.httpClient.Do(req)
}

// buildURL joins the base URL with the given path (which may start with '/').
func (c *Client) buildURL(p string) string {
	// Use standard library url.URL joining
	rel, _ := url.Parse(p)
	return c.baseURL.ResolveReference(rel).String()
}
