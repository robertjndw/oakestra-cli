package oakestra

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestStringOrNumber_HandlesBothShapes(t *testing.T) {
	var asString StringOrNumber
	if err := json.Unmarshal([]byte(`"10100"`), &asString); err != nil {
		t.Fatalf("Unmarshal string: %v", err)
	}
	if asString.String() != "10100" {
		t.Fatalf("String() = %q, want %q", asString.String(), "10100")
	}

	var asNumber StringOrNumber
	if err := json.Unmarshal([]byte(`10100`), &asNumber); err != nil {
		t.Fatalf("Unmarshal number: %v", err)
	}
	if asNumber.String() != "10100" {
		t.Fatalf("String() = %q, want %q", asNumber.String(), "10100")
	}

	var asNull StringOrNumber
	if err := json.Unmarshal([]byte(`null`), &asNull); err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	if asNull.String() != "" {
		t.Fatalf("String() = %q, want empty string for null", asNull.String())
	}
}

func TestClustersService_List_ActiveOnly(t *testing.T) {
	var gotPath string
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"_id":"1","cluster_name":"c1","port":10100}]`))
	})

	clusters, _, err := c.Clusters.List(context.Background(), &ClusterListOptions{ActiveOnly: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != "/api/clusters/active" {
		t.Fatalf("path = %q, want /api/clusters/active", gotPath)
	}
	if len(clusters) != 1 || clusters[0].Port.String() != "10100" {
		t.Fatalf("clusters = %+v", clusters)
	}
}

func TestClustersService_FindByNameOrID(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"_id":"1","cluster_name":"c1"},
			{"_id":"2","cluster_name":"c2","candidate_name":"candidate-2"}
		]`))
	})

	byName, _, err := c.Clusters.FindByNameOrID(context.Background(), "c1")
	if err != nil || byName.ClusterID != "1" {
		t.Fatalf("FindByNameOrID(c1) = %+v, %v", byName, err)
	}

	byCandidate, _, err := c.Clusters.FindByNameOrID(context.Background(), "candidate-2")
	if err != nil || byCandidate.ClusterID != "2" {
		t.Fatalf("FindByNameOrID(candidate-2) = %+v, %v", byCandidate, err)
	}

	_, _, err = c.Clusters.FindByNameOrID(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected an error for a missing cluster")
	}
}

func TestClustersService_FindByNameOrID_MultipleMatches(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"_id":"1","cluster_name":"dup"},
			{"_id":"2","cluster_name":"dup"}
		]`))
	})

	_, _, err := c.Clusters.FindByNameOrID(context.Background(), "dup")
	var multi *MultipleMatchesError
	if !errors.As(err, &multi) {
		t.Fatalf("err = %v (%T), want *MultipleMatchesError", err, err)
	}
	if len(multi.Matches) != 2 {
		t.Fatalf("Matches = %+v, want 2 entries", multi.Matches)
	}
}
