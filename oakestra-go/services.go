package oakestra

import (
	"context"
	"fmt"
	"net/http"
)

// ServicesService handles communication with the microservice-related
// endpoints of the Oakestra System Manager API.
type ServicesService struct {
	client *Client
}

// MetricPoint is a single timestamped resource measurement.
type MetricPoint struct {
	Value     string `json:"value"`
	Timestamp string `json:"timestamp"`
}

// ServiceInstance is a single running instance of a service.
type ServiceInstance struct {
	InstanceNumber  int           `json:"instance_number"`
	Status          string        `json:"status"`
	StatusDetail    string        `json:"status_detail"`
	PublicIP        string        `json:"publicip"` // note: lowercase 'ip' in the API
	ClusterID       string        `json:"cluster_id"`
	ClusterLocation string        `json:"cluster_location"`
	HostIP          string        `json:"host_ip"`
	HostPort        int           `json:"host_port"`
	Disk            string        `json:"disk"`
	CPUPercent      string        `json:"cpu_percent"`
	MemoryPercent   string        `json:"memory_percent"`
	CPUHistory      []MetricPoint `json:"cpu_history"`
	MemoryHistory   []MetricPoint `json:"memory_history"`
	Logs            string        `json:"logs"`
	WorkerID        string        `json:"worker_id"`
}

// Service represents a microservice.
//
// The API uses different field names depending on the endpoint:
//   - GET /api/services/ uses application_name / application_namespace
//   - GET /api/service/{id} uses app_name / app_ns
//
// Both sets are decoded; call GetApplicationName() / GetApplicationNamespace()
// to retrieve the correct value regardless of which endpoint populated it.
type Service struct {
	MicroserviceID        string            `json:"microserviceID"`
	MicroserviceName      string            `json:"microservice_name"`
	MicroserviceNamespace string            `json:"microservice_namespace"`
	ApplicationID         string            `json:"applicationID"`
	ApplicationName       string            `json:"application_name"` // list endpoint
	AppName               string            `json:"app_name"`         // detail endpoint
	ApplicationNamespace  string            `json:"application_namespace"`
	AppNs                 string            `json:"app_ns"`
	InstanceList          []ServiceInstance `json:"instance_list"`
	Status                string            `json:"status"`
	StatusDetail          string            `json:"status_detail"`
	// Deployment config (populated on the detail endpoint).
	Port           string   `json:"port"`
	Code           string   `json:"code"`
	Virtualization string   `json:"virtualization"`
	Memory         int      `json:"memory"`
	VCPUs          int      `json:"vcpus"`
	VGPUs          int      `json:"vgpus"`
	Environment    []string `json:"environment"`
	RRip           string   `json:"RR_ip"`
}

// GetApplicationName returns the application name regardless of which API
// endpoint populated the struct.
func (s Service) GetApplicationName() string {
	if s.ApplicationName != "" {
		return s.ApplicationName
	}
	return s.AppName
}

// GetApplicationNamespace returns the application namespace regardless of
// which API endpoint populated the struct.
func (s Service) GetApplicationNamespace() string {
	if s.ApplicationNamespace != "" {
		return s.ApplicationNamespace
	}
	return s.AppNs
}

// ServiceListOptions narrows a Services.List call.
type ServiceListOptions struct {
	// ApplicationID, if set, restricts the result to services belonging to
	// this application.
	ApplicationID string
}

// Get returns a single service by ID.
func (s *ServicesService) Get(ctx context.Context, serviceID string) (*Service, *Response, error) {
	req, err := s.client.NewRequest(ctx, http.MethodGet, "api/service/"+serviceID, nil)
	if err != nil {
		return nil, nil, err
	}
	var svc Service
	resp, err := s.client.Do(ctx, req, &svc)
	if err != nil {
		return nil, resp, err
	}
	return &svc, resp, nil
}

// List returns services, optionally filtered to one application.
func (s *ServicesService) List(ctx context.Context, opts *ServiceListOptions) ([]*Service, *Response, error) {
	endpoint := "api/services/"
	appID := ""
	if opts != nil {
		appID = opts.ApplicationID
	}
	if appID != "" {
		endpoint += appID
	}

	req, err := s.client.NewRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	var svcs []*Service
	resp, err := s.client.Do(ctx, req, &svcs)
	if err != nil {
		return nil, resp, err
	}

	// The endpoint can return more than the requested application, so
	// re-filter client-side.
	if appID != "" {
		filtered := svcs[:0]
		for _, svc := range svcs {
			if svc.ApplicationID == appID {
				filtered = append(filtered, svc)
			}
		}
		svcs = filtered
	}
	return svcs, resp, nil
}

// DeployInstance starts a new instance of the given service.
func (s *ServicesService) DeployInstance(ctx context.Context, serviceID string) (*Response, error) {
	req, err := s.client.NewRequest(ctx, http.MethodPost, "api/service/"+serviceID+"/instance", nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, req, nil)
}

// UndeployInstance stops one instance of the given service.
func (s *ServicesService) UndeployInstance(ctx context.Context, serviceID string, instanceID int) (*Response, error) {
	endpoint := fmt.Sprintf("api/service/%s/instance/%d", serviceID, instanceID)
	req, err := s.client.NewRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, req, nil)
}

// ResolveByNameOrID accepts a service ID or name. On a name lookup it
// returns *MultipleMatchesError if more than one service shares that name,
// and *NotFoundError if none do.
func (s *ServicesService) ResolveByNameOrID(ctx context.Context, idOrName string) (*Service, *Response, error) {
	// Try direct ID lookup first.
	svc, resp, err := s.Get(ctx, idOrName)
	if err == nil {
		return svc, resp, nil
	}

	// Fall back to name search.
	all, listResp, err := s.List(ctx, nil)
	if err != nil {
		return nil, listResp, err
	}
	var matches []*Service
	for _, svc := range all {
		if svc.MicroserviceName == idOrName {
			matches = append(matches, svc)
		}
	}
	switch len(matches) {
	case 0:
		return nil, listResp, &NotFoundError{Kind: "service", Query: idOrName}
	case 1:
		return matches[0], listResp, nil
	default:
		var ms []Match
		for _, m := range matches {
			ms = append(ms, Match{ID: m.MicroserviceID, Name: m.MicroserviceName, Detail: m.GetApplicationName()})
		}
		return nil, listResp, &MultipleMatchesError{Kind: "service", Query: idOrName, Matches: ms}
	}
}
