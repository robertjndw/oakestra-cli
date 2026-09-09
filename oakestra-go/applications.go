package oakestra

import (
	"context"
	"net/http"
)

// ApplicationsService handles communication with the application-related
// endpoints of the Oakestra System Manager API.
type ApplicationsService struct {
	client *Client
}

// Application represents an Oakestra application.
type Application struct {
	ApplicationID        string   `json:"applicationID"`
	ApplicationName      string   `json:"application_name"`
	ApplicationNamespace string   `json:"application_namespace"`
	ApplicationDesc      string   `json:"application_desc"`
	Microservices        []string `json:"microservices"`
}

// List returns all applications.
func (s *ApplicationsService) List(ctx context.Context) ([]Application, *Response, error) {
	req, err := s.client.NewRequest(ctx, http.MethodGet, "api/applications", nil)
	if err != nil {
		return nil, nil, err
	}
	var apps []Application
	resp, err := s.client.Do(ctx, req, &apps)
	if err != nil {
		return nil, resp, err
	}
	return apps, resp, nil
}

// Get returns a single application by ID.
func (s *ApplicationsService) Get(ctx context.Context, applicationID string) (*Application, *Response, error) {
	req, err := s.client.NewRequest(ctx, http.MethodGet, "api/application/"+applicationID, nil)
	if err != nil {
		return nil, nil, err
	}
	var app Application
	resp, err := s.client.Do(ctx, req, &app)
	if err != nil {
		return nil, resp, err
	}
	return &app, resp, nil
}

// Create deploys a new application from an SLA payload.
func (s *ApplicationsService) Create(ctx context.Context, sla any) ([]Application, *Response, error) {
	req, err := s.client.NewRequest(ctx, http.MethodPost, "api/application", sla)
	if err != nil {
		return nil, nil, err
	}
	var apps []Application
	resp, err := s.client.Do(ctx, req, &apps)
	if err != nil {
		return nil, resp, err
	}
	return apps, resp, nil
}

// Delete removes an application by ID.
func (s *ApplicationsService) Delete(ctx context.Context, applicationID string) (*Response, error) {
	req, err := s.client.NewRequest(ctx, http.MethodDelete, "api/application/"+applicationID, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, req, nil)
}

// ResolveByNameOrID accepts an application ID or name. On a name lookup it
// returns *MultipleMatchesError if more than one application shares that
// name, and *NotFoundError if none do.
func (s *ApplicationsService) ResolveByNameOrID(ctx context.Context, idOrName string) (*Application, *Response, error) {
	return resolveByNameOrID(ctx, "application", idOrName, s.Get, s.List,
		func(a *Application) string { return a.ApplicationName },
		func(a *Application) Match {
			return Match{ID: a.ApplicationID, Name: a.ApplicationName, Detail: a.ApplicationNamespace}
		},
	)
}
