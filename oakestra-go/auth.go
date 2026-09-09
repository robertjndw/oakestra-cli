package oakestra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AuthProvider supplies the bearer token attached to every request. It's
// the extension point for authentication schemes this library doesn't know
// about (OIDC, a token read from a file the caller manages themselves,
// etc.). Implement it and pass it via WithAuth.
type AuthProvider interface {
	// Token returns a valid bearer token, fetching or refreshing it as
	// needed.
	Token(ctx context.Context) (string, error)
}

// StaticToken is an AuthProvider that always returns the same token. Use it
// via WithToken when the caller already holds a valid token and wants to
// skip the login flow.
type StaticToken string

// Token implements AuthProvider.
func (t StaticToken) Token(_ context.Context) (string, error) {
	return string(t), nil
}

// loginTokenSource authenticates via POST /api/auth/login and caches the
// resulting token for ttl before logging in again. State lives on the
// instance (not in package globals) so that two clients targeting two
// different orchestrators never share a cached token.
type loginTokenSource struct {
	client   *Client
	username string
	password string

	mu        sync.Mutex
	ttl       time.Duration
	token     string
	fetchedAt time.Time
}

func newLoginTokenSource(c *Client, username, password string, ttl time.Duration) *loginTokenSource {
	return &loginTokenSource{client: c, username: username, password: password, ttl: ttl}
}

func (l *loginTokenSource) setTTL(ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ttl = ttl
}

// Token implements AuthProvider.
func (l *loginTokenSource) Token(ctx context.Context) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.token != "" && time.Since(l.fetchedAt) < l.ttl {
		return l.token, nil
	}

	payload := map[string]string{
		"username": l.username,
		"password": l.password,
	}
	req, err := l.client.NewRequest(ctx, http.MethodPost, "api/auth/login", payload)
	if err != nil {
		return "", err
	}

	// Bypass Do: it would recurse back into Token to authenticate this very
	// request. The login endpoint needs no Authorization header.
	httpResp, err := l.client.httpClient.Do(req)
	if err != nil {
		if isConnectionError(err) {
			return "", &ConnectionError{BaseURL: l.client.baseURL.String(), Err: err}
		}
		return "", fmt.Errorf("oakestra: login request: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	var result struct {
		Token string `json:"token"`
	}
	if httpResp.StatusCode != http.StatusOK {
		body := readBodyForError(httpResp)
		return "", &AuthError{StatusCode: httpResp.StatusCode, Body: body}
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("oakestra: parsing login response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("oakestra: login response had no 'token' field")
	}

	l.token = result.Token
	l.fetchedAt = time.Now()
	return l.token, nil
}
