package oakestra

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer starts an httptest server and returns it alongside a Client
// pointed at it, authenticated with a static token so tests that don't care
// about login can ignore auth entirely.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := NewClient(WithBaseURL(srv.URL), WithToken("test-token"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return srv, c
}
