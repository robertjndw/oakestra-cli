package oakestra

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestClient_Token(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tok, err := c.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "test-token" {
		t.Fatalf("Token() = %q, want %q", tok, "test-token")
	}
}

func TestClient_Token_NoAuthProvider(t *testing.T) {
	c, err := NewClient(WithBaseURL("http://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.Token(context.Background()); err == nil {
		t.Fatal("expected an error when no AuthProvider is configured")
	}
}

func TestNewClient_RequiresBaseURL(t *testing.T) {
	if _, err := NewClient(); err == nil {
		t.Fatal("expected an error when WithBaseURL is omitted")
	}
}

func TestNewClient_RejectsInvalidBaseURL(t *testing.T) {
	if _, err := NewClient(WithBaseURL("not-a-url")); err == nil {
		t.Fatal("expected an error for a base URL with no scheme/host")
	}
}

func TestDo_AttachesBearerToken(t *testing.T) {
	var gotAuth string
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	req, err := c.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if _, err := c.Do(context.Background(), req, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
}

func TestDo_NonOKStatusReturnsErrorResponse(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	defer srv.Close()

	req, err := c.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = c.Do(context.Background(), req, nil)
	var errResp *ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("Do err = %v (%T), want *ErrorResponse", err, err)
	}
	if errResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", errResp.StatusCode, http.StatusInternalServerError)
	}
}

func TestDo_UnauthorizedReturnsAuthError(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	})
	defer srv.Close()

	req, err := c.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = c.Do(context.Background(), req, nil)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("Do err = %v (%T), want *AuthError", err, err)
	}
}

func TestDo_ConnectionRefusedReturnsConnectionError(t *testing.T) {
	// Port 0 with no listener guarantees an unreachable, syntactically valid
	// base URL without depending on a specific closed port being free.
	c, err := NewClient(WithBaseURL("http://127.0.0.1:1"), WithToken("t"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = c.Do(context.Background(), req, nil)
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("Do err = %v (%T), want *ConnectionError", err, err)
	}
}

func TestDo_CanceledContextIsNotAConnectionError(t *testing.T) {
	// http.Client.Do always wraps its error in *url.Error, which satisfies
	// net.Error regardless of cause, so a canceled context (nothing wrong
	// with reachability) must not be reported as one. Otherwise a caller
	// that cancels its own request gets the misleading "orchestrator not
	// reachable" hint.
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req, err := c.NewRequest(ctx, http.MethodGet, "api/applications", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = c.Do(ctx, req, nil)
	if err == nil {
		t.Fatal("expected an error from a canceled context")
	}
	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		t.Fatalf("Do err = %v, should not be classified as *ConnectionError", err)
	}
}

func TestSmartUnmarshal_HandlesJSONEncodedStringBody(t *testing.T) {
	type app struct {
		Name string `json:"name"`
	}
	// The API sometimes wraps the body in a JSON string.
	wrapped := []byte(`"[{\"name\":\"a\"}]"`)
	var apps []app
	if err := smartUnmarshal(wrapped, &apps); err != nil {
		t.Fatalf("smartUnmarshal: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != "a" {
		t.Fatalf("apps = %+v, want one app named a", apps)
	}

	direct := []byte(`[{"name":"b"}]`)
	apps = nil
	if err := smartUnmarshal(direct, &apps); err != nil {
		t.Fatalf("smartUnmarshal: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != "b" {
		t.Fatalf("apps = %+v, want one app named b", apps)
	}
}
