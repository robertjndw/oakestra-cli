// Package oakestra is a Go client library for the Oakestra System Manager API.
//
// It is intentionally config-agnostic: the caller supplies a base URL and a
// way to authenticate (a static token, a username/password pair, or a custom
// AuthProvider). The library never reads files or environment variables on
// its own, which keeps it usable both from a CLI with its own config file
// and from any other Go program.
package oakestra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultSystemManagerPort is the port the Oakestra System Manager listens
// on by default.
const DefaultSystemManagerPort = 10000

// DefaultTokenTTL mirrors the Root Orchestrator's own re-login cadence: a
// cached token is reused for this long before a fresh login is attempted.
const DefaultTokenTTL = 10 * time.Second

// defaultUserAgent identifies the library in the User-Agent header when the
// caller does not set one of their own.
const defaultUserAgent = "oakestra-go"

// BaseURLForHost builds the default System Manager base URL for a bare host
// or IP, e.g. BaseURLForHost("1.2.3.4") returns "http://1.2.3.4:10000".
func BaseURLForHost(host string) string {
	return fmt.Sprintf("http://%s:%d", host, DefaultSystemManagerPort)
}

// Client is an Oakestra System Manager API client. Create one with
// NewClient.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	userAgent  string
	auth       AuthProvider

	Applications *ApplicationsService
	Services     *ServicesService
	Clusters     *ClustersService
}

// Option configures a Client. Pass one or more to NewClient.
type Option func(*Client) error

// WithBaseURL sets the System Manager base URL, e.g. "http://1.2.3.4:10000".
// This option is required; NewClient returns an error without it.
func WithBaseURL(rawURL string) Option {
	return func(c *Client) error {
		if rawURL == "" {
			return fmt.Errorf("oakestra: base URL must not be empty")
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("oakestra: parsing base URL %q: %w", rawURL, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("oakestra: base URL %q must be absolute (e.g. http://host:port)", rawURL)
		}
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
		c.baseURL = u
		return nil
	}
}

// WithHTTPClient overrides the underlying *http.Client. The default matches
// the timeout the Oakestra CLI has always used: 30 seconds.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return fmt.Errorf("oakestra: http client must not be nil")
		}
		c.httpClient = hc
		return nil
	}
}

// WithUserAgent overrides the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// WithAuth sets a custom AuthProvider, for callers that need something
// beyond a static token or username/password login (e.g. OIDC).
func WithAuth(auth AuthProvider) Option {
	return func(c *Client) error {
		c.auth = auth
		return nil
	}
}

// WithToken authenticates every request with a fixed bearer token, skipping
// the login flow entirely.
func WithToken(token string) Option {
	return WithAuth(StaticToken(token))
}

// WithBasicLogin authenticates by POSTing username/password to
// /api/auth/login and caching the returned token for DefaultTokenTTL (or the
// duration set by WithTokenTTL). This is the option most CLI-style callers
// want.
func WithBasicLogin(username, password string) Option {
	return func(c *Client) error {
		c.auth = newLoginTokenSource(c, username, password, DefaultTokenTTL)
		return nil
	}
}

// WithTokenTTL overrides how long a token obtained via WithBasicLogin is
// cached before the client logs in again. It has no effect unless
// WithBasicLogin is also used, and must appear after WithBasicLogin in the
// option list.
func WithTokenTTL(ttl time.Duration) Option {
	return func(c *Client) error {
		lts, ok := c.auth.(*loginTokenSource)
		if !ok {
			return fmt.Errorf("oakestra: WithTokenTTL requires WithBasicLogin to be set first")
		}
		lts.setTTL(ttl)
		return nil
	}
}

// NewClient builds an Oakestra API client. WithBaseURL is required; without
// it NewClient returns an error.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.baseURL == nil {
		return nil, fmt.Errorf("oakestra: WithBaseURL is required")
	}

	c.Applications = &ApplicationsService{client: c}
	c.Services = &ServicesService{client: c}
	c.Clusters = &ClustersService{client: c}
	return c, nil
}

// BaseURL returns the client's configured base URL.
func (c *Client) BaseURL() *url.URL {
	u := *c.baseURL
	return &u
}

// Response wraps an *http.Response returned by Do. The body has already
// been fully read and closed by the time it is returned to the caller;
// RawBody holds those bytes so callers that need them (e.g. an endpoint
// with no typed response) don't have to re-read Body.
type Response struct {
	*http.Response
	RawBody []byte
}

// Token returns the client's current bearer token, obtaining or refreshing
// it via the configured AuthProvider as needed. This exists for callers
// that need to authenticate a request against another Oakestra component
// sharing the same credentials (e.g. an addon API on a different port)
// rather than the System Manager this client is bound to; ordinary System
// Manager calls never need it since NewRequest/Do attach it automatically.
// It returns an error if the client has no AuthProvider configured.
func (c *Client) Token(ctx context.Context) (string, error) {
	if c.auth == nil {
		return "", fmt.Errorf("oakestra: client has no AuthProvider configured")
	}
	return c.auth.Token(ctx)
}

// NewRequest builds an HTTP request against the client's base URL. path is
// resolved relative to the base URL, so it should not start with a slash
// unless it is meant to replace the base URL's path entirely (it isn't, for
// any endpoint this library or its callers use).
//
// The request is not yet authenticated: Do attaches the bearer token
// immediately before sending, so the token used reflects the time of the
// call rather than the time NewRequest was built.
func (c *Client) NewRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	rel, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return nil, fmt.Errorf("oakestra: parsing request path %q: %w", path, err)
	}
	u := c.baseURL.ResolveReference(rel)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("oakestra: marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("oakestra: creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

// Do sends req, decoding a successful JSON response into v (if v is
// non-nil) and translating connection failures and non-2xx responses into
// typed errors. It attaches an Authorization header from the client's
// AuthProvider unless one is already set on req.
func (c *Client) Do(ctx context.Context, req *http.Request, v any) (*Response, error) {
	if req.Header.Get("Authorization") == "" && c.auth != nil {
		token, err := c.auth.Token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		if isConnectionError(err) {
			return nil, &ConnectionError{BaseURL: c.baseURL.String(), Err: err}
		}
		return nil, fmt.Errorf("oakestra: %s %s: %w", req.Method, req.URL, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("oakestra: reading response body: %w", err)
	}
	// Re-wrap the body so callers inspecting *Response can still read it if
	// they want to (it is already drained above).
	httpResp.Body = io.NopCloser(bytes.NewReader(data))
	resp := &Response{Response: httpResp, RawBody: data}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
			return resp, &AuthError{StatusCode: httpResp.StatusCode, Body: string(data)}
		}
		return resp, &ErrorResponse{
			Method:     req.Method,
			URL:        req.URL.String(),
			StatusCode: httpResp.StatusCode,
			Body:       string(data),
		}
	}

	if v != nil && len(data) > 0 {
		if err := smartUnmarshal(data, v); err != nil {
			return resp, fmt.Errorf("oakestra: decoding response from %s %s: %w", req.Method, req.URL, err)
		}
	}
	return resp, nil
}
