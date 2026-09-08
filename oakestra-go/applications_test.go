package oakestra

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestApplicationsService_List(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/applications" {
			t.Errorf("path = %q, want /api/applications", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"applicationID":"1","application_name":"a","application_namespace":"ns"}]`))
	})

	apps, _, err := c.Applications.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(apps) != 1 || apps[0].ApplicationName != "a" {
		t.Fatalf("apps = %+v, want one app named a", apps)
	}
}

func TestApplicationsService_ResolveByNameOrID_MultipleMatches(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/application/dup":
			w.WriteHeader(http.StatusNotFound)
		case "/api/applications":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"applicationID":"1","application_name":"dup","application_namespace":"ns1"},
				{"applicationID":"2","application_name":"dup","application_namespace":"ns2"}
			]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	_, _, err := c.Applications.ResolveByNameOrID(context.Background(), "dup")
	var multi *MultipleMatchesError
	if !errors.As(err, &multi) {
		t.Fatalf("err = %v (%T), want *MultipleMatchesError", err, err)
	}
	if len(multi.Matches) != 2 {
		t.Fatalf("Matches = %+v, want 2 entries", multi.Matches)
	}
}

func TestApplicationsService_ResolveByNameOrID_NotFound(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/application/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/api/applications":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	_, _, err := c.Applications.ResolveByNameOrID(context.Background(), "missing")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v (%T), want *NotFoundError", err, err)
	}
}

func TestApplicationsService_ResolveByNameOrID_DirectIDHit(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/application/1" {
			t.Errorf("path = %q, want /api/application/1", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"applicationID":"1","application_name":"a"}`))
	})

	app, _, err := c.Applications.ResolveByNameOrID(context.Background(), "1")
	if err != nil {
		t.Fatalf("ResolveByNameOrID: %v", err)
	}
	if app.ApplicationID != "1" {
		t.Fatalf("ApplicationID = %q, want %q", app.ApplicationID, "1")
	}
}
