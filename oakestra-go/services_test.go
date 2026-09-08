package oakestra

import (
	"context"
	"net/http"
	"testing"
)

func TestServicesService_Get_MergesFieldNamesAcrossEndpoints(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The detail endpoint uses app_name/app_ns instead of
		// application_name/application_namespace.
		_, _ = w.Write([]byte(`{"microserviceID":"1","app_name":"a","app_ns":"ns"}`))
	})

	svc, _, err := c.Services.Get(context.Background(), "1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if svc.GetApplicationName() != "a" {
		t.Fatalf("GetApplicationName() = %q, want %q", svc.GetApplicationName(), "a")
	}
	if svc.GetApplicationNamespace() != "ns" {
		t.Fatalf("GetApplicationNamespace() = %q, want %q", svc.GetApplicationNamespace(), "ns")
	}
}

func TestServicesService_List_FiltersByApplicationClientSide(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/app-1" {
			t.Errorf("path = %q, want /api/services/app-1", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// Simulate the endpoint returning more than requested.
		_, _ = w.Write([]byte(`[
			{"microserviceID":"1","applicationID":"app-1"},
			{"microserviceID":"2","applicationID":"app-2"}
		]`))
	})

	svcs, _, err := c.Services.List(context.Background(), &ServiceListOptions{ApplicationID: "app-1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(svcs) != 1 || svcs[0].MicroserviceID != "1" {
		t.Fatalf("svcs = %+v, want only microservice 1", svcs)
	}
}

func TestServicesService_DeployInstance(t *testing.T) {
	var gotMethod, gotPath string
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	if _, err := c.Services.DeployInstance(context.Background(), "svc-1"); err != nil {
		t.Fatalf("DeployInstance: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/service/svc-1/instance" {
		t.Fatalf("got %s %s, want POST /api/service/svc-1/instance", gotMethod, gotPath)
	}
}

func TestServicesService_UndeployInstance(t *testing.T) {
	var gotMethod, gotPath string
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	if _, err := c.Services.UndeployInstance(context.Background(), "svc-1", 2); err != nil {
		t.Fatalf("UndeployInstance: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/service/svc-1/instance/2" {
		t.Fatalf("got %s %s, want DELETE /api/service/svc-1/instance/2", gotMethod, gotPath)
	}
}
