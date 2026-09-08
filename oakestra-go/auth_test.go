package oakestra

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoginTokenSource_CachesWithinTTL(t *testing.T) {
	var logins int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			atomic.AddInt32(&logins, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewClient(WithBaseURL(srv.URL), WithBasicLogin("Admin", "Admin"), WithTokenTTL(time.Minute))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	for i := 0; i < 5; i++ {
		req, err := client.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		if _, err := client.Do(context.Background(), req, nil); err != nil {
			t.Fatalf("Do: %v", err)
		}
	}
	if got := atomic.LoadInt32(&logins); got != 1 {
		t.Fatalf("login count = %d, want 1 (token should be cached)", got)
	}
}

func TestLoginTokenSource_RefetchesAfterTTL(t *testing.T) {
	var logins int32
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			atomic.AddInt32(&logins, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	client, err := NewClient(WithBaseURL(srv.URL), WithBasicLogin("Admin", "Admin"), WithTokenTTL(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	do := func() {
		req, err := client.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		if _, err := client.Do(context.Background(), req, nil); err != nil {
			t.Fatalf("Do: %v", err)
		}
	}

	do()
	time.Sleep(20 * time.Millisecond)
	do()

	if got := atomic.LoadInt32(&logins); got != 2 {
		t.Fatalf("login count = %d, want 2 (token should be refetched after TTL)", got)
	}
}

func TestLoginTokenSource_RejectedLoginReturnsAuthError(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("bad credentials"))
	})
	defer srv.Close()

	client, err := NewClient(WithBaseURL(srv.URL), WithBasicLogin("Admin", "wrong"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	req, err := client.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = client.Do(context.Background(), req, nil)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("Do err = %v (%T), want *AuthError", err, err)
	}
}

func TestTwoClients_DoNotShareTokens(t *testing.T) {
	// This is the regression the old package-level loginToken/lastLogin
	// globals made impossible to avoid: two clients targeting two different
	// orchestrators must never see each other's cached token.
	tokenFor := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/auth/login" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"token":"` + name + `"}`))
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+name {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}

	srvA, _ := newTestServer(t, tokenFor("token-a"))
	srvB, _ := newTestServer(t, tokenFor("token-b"))
	defer srvA.Close()
	defer srvB.Close()

	clientA, err := NewClient(WithBaseURL(srvA.URL), WithBasicLogin("Admin", "Admin"))
	if err != nil {
		t.Fatalf("NewClient A: %v", err)
	}
	clientB, err := NewClient(WithBaseURL(srvB.URL), WithBasicLogin("Admin", "Admin"))
	if err != nil {
		t.Fatalf("NewClient B: %v", err)
	}

	for _, c := range []*Client{clientA, clientB} {
		req, err := c.NewRequest(context.Background(), http.MethodGet, "api/applications", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		if _, err := c.Do(context.Background(), req, nil); err != nil {
			t.Fatalf("Do: %v (each client should authenticate against its own server independently)", err)
		}
	}
}
