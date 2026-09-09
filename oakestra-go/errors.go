package oakestra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// ErrorResponse is returned when the API responds with a non-2xx status
// that isn't an authentication failure (see AuthError).
type ErrorResponse struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("oakestra: %s %s returned HTTP %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

// AuthError is returned when a login attempt is rejected, or when a request
// fails with 401/403.
type AuthError struct {
	StatusCode int
	Body       string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("oakestra: authentication failed (HTTP %d): %s", e.StatusCode, e.Body)
}

// ConnectionError is returned when the client could not reach the
// configured base URL at all (connection refused, DNS failure, timeout).
// Unwrap returns the underlying network error.
type ConnectionError struct {
	BaseURL string
	Err     error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("oakestra: %s not reachable: %v", e.BaseURL, e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// NotFoundError is returned when a resource lookup by ID or name matches
// nothing.
type NotFoundError struct {
	// Kind is the resource type, e.g. "application", "service", "cluster".
	Kind  string
	Query string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("oakestra: no %s found with ID or name %q", e.Kind, e.Query)
}

// Match is one ambiguous result reported by MultipleMatchesError.
type Match struct {
	ID   string
	Name string
	// Detail is extra context to disambiguate matches with the same name,
	// e.g. an application's namespace.
	Detail string
}

// MultipleMatchesError is returned when a name lookup matches more than one
// resource. Callers should ask the user to specify the resource by ID
// instead.
type MultipleMatchesError struct {
	Kind    string
	Query   string
	Matches []Match
}

func (e *MultipleMatchesError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "oakestra: multiple %ss named %q, use the ID instead:", e.Kind, e.Query)
	for _, m := range e.Matches {
		if m.Detail != "" {
			fmt.Fprintf(&b, "\n  %s (%s)", m.ID, m.Detail)
		} else {
			fmt.Fprintf(&b, "\n  %s", m.ID)
		}
	}
	return b.String()
}

// isConnectionError reports whether err represents a failure to reach the
// server at all (as opposed to the server responding with an error status).
func isConnectionError(err error) bool {
	// http.Client.Do always wraps its error in *url.Error, which itself
	// satisfies net.Error (it forwards Timeout()/Temporary() to whatever it
	// wraps). That means a bare net.Error check below would call nearly any
	// failure a connection error, including an explicitly canceled context
	// that has nothing to do with reachability, so rule that out first. A
	// context deadline exceeded is still treated as a connection error
	// though, since that's what http.Client.Timeout produces on a real
	// dial/read timeout.
	if errors.Is(err, context.Canceled) {
		return false
	}

	// *net.OpError and *net.DNSError (dial/lookup failures) both satisfy
	// net.Error, so a single check covers them too.
	var netErr net.Error
	return errors.As(err, &netErr)
}

// readBodyForError best-effort reads a response body for inclusion in an
// error message. It never returns an error itself.
func readBodyForError(resp *http.Response) string {
	data, _ := io.ReadAll(resp.Body)
	return string(data)
}
