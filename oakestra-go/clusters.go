package oakestra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ClustersService handles communication with the cluster-related endpoints
// of the Oakestra System Manager API.
type ClustersService struct {
	client *Client
}

// ClusterMetricPoint is a single CPU/memory measurement for a cluster.
type ClusterMetricPoint struct {
	Timestamp float64  `json:"timestamp"`
	Value     *float64 `json:"value"` // pointer to handle null entries
}

// StringOrNumber decodes a JSON field that different Oakestra API versions
// encode inconsistently as either a string or a number (e.g. Cluster.Port).
type StringOrNumber struct {
	s string
}

// UnmarshalJSON implements json.Unmarshaler. It resolves the string/number
// ambiguity once at decode time so String() is a plain field access.
func (v *StringOrNumber) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		v.s = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.s = s
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		v.s = fmt.Sprintf("%d", int(f))
		return nil
	}
	v.s = string(data)
	return nil
}

// String returns the value as a printable string regardless of whether the
// API sent it as a JSON string or a JSON number. It returns "" if the value
// was null or never set.
func (v StringOrNumber) String() string {
	return v.s
}

// Cluster represents a cluster returned by the /api/clusters/ endpoint.
type Cluster struct {
	ClusterID       string         `json:"_id"`
	ClusterName     string         `json:"cluster_name"`
	CandidateName   string         `json:"candidate_name"`
	ClusterIP       string         `json:"ip"`
	Port            StringOrNumber `json:"port"`
	Active          bool           `json:"active"`
	ActiveNodes     int            `json:"active_nodes"`
	ClusterLocation string         `json:"cluster_location"`

	// Resource totals.
	TotalCPUCores int `json:"total_cpu_cores"`
	TotalGPUCores int `json:"total_gpu_cores"`
	VCPUs         int `json:"vcpus"`
	VGPUs         int `json:"vgpus"`
	MemoryInMB    int `json:"memory_in_mb"`
	VRAM          int `json:"vram"`

	// Current utilisation.
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	GPUPercent    float64 `json:"gpu_percent"`
	GPUTemp       float64 `json:"gpu_temp"`
	VRAMPercent   float64 `json:"vram_percent"`

	// Capabilities.
	Virtualization  []string `json:"virtualization"`
	CSIDrivers      []string `json:"csi_drivers"`
	SupportedAddons []string `json:"supported_addons"`
	GPUDrivers      []string `json:"gpu_drivers"`

	LastModifiedTimestamp float64 `json:"last_modified_timestamp"`
}

// ClusterListOptions narrows a Clusters.List call.
type ClusterListOptions struct {
	// ActiveOnly restricts the result to active clusters only.
	ActiveOnly bool
}

// List returns clusters, optionally restricted to active ones.
func (s *ClustersService) List(ctx context.Context, opts *ClusterListOptions) ([]*Cluster, *Response, error) {
	endpoint := "api/clusters/"
	if opts != nil && opts.ActiveOnly {
		endpoint += "active"
	}

	req, err := s.client.NewRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	var clusters []*Cluster
	resp, err := s.client.Do(ctx, req, &clusters)
	if err != nil {
		return nil, resp, err
	}
	return clusters, resp, nil
}

// FindByNameOrID returns the cluster matching nameOrID (cluster_name,
// candidate_name, or _id). It returns *MultipleMatchesError if more than one
// cluster shares that name or candidate name. Clusters have no dedicated
// get-by-ID endpoint to try first the way Applications/Services do, so this
// scans the full list, but an ID match is still unique and authoritative and
// is returned immediately.
func (s *ClustersService) FindByNameOrID(ctx context.Context, nameOrID string) (*Cluster, *Response, error) {
	all, resp, err := s.List(ctx, &ClusterListOptions{ActiveOnly: false})
	if err != nil {
		return nil, resp, err
	}

	for _, cl := range all {
		if cl.ClusterID == nameOrID {
			return cl, resp, nil
		}
	}

	var matches []*Cluster
	for _, cl := range all {
		if cl.ClusterName == nameOrID || cl.CandidateName == nameOrID {
			matches = append(matches, cl)
		}
	}
	switch len(matches) {
	case 0:
		return nil, resp, &NotFoundError{Kind: "cluster", Query: nameOrID}
	case 1:
		return matches[0], resp, nil
	default:
		var ms []Match
		for _, m := range matches {
			ms = append(ms, Match{ID: m.ClusterID, Name: m.ClusterName, Detail: m.CandidateName})
		}
		return nil, resp, &MultipleMatchesError{Kind: "cluster", Query: nameOrID, Matches: ms}
	}
}
