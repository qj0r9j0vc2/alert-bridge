package pagerduty

import (
	"context"
	"fmt"
	"time"

	"github.com/PagerDuty/go-pagerduty"
)

// RESTClient defines the interface for PagerDuty REST API v2 operations.
type RESTClient interface {
	// GetService retrieves service details for validation and health checks.
	GetService(ctx context.Context, serviceID string) (*pagerduty.Service, error)

	// CreateIncidentNote adds a note to an existing incident.
	CreateIncidentNote(ctx context.Context, incidentID, content string) (*pagerduty.IncidentNote, error)

	// GetIncident retrieves full incident details.
	GetIncident(ctx context.Context, incidentID string) (*pagerduty.Incident, error)

	// CreateIncidentResponderRequest adds responders to an incident.
	CreateIncidentResponderRequest(ctx context.Context, incidentID, message string, responderRequestTargets []pagerduty.ResponderRequestTarget) (*pagerduty.ResponderRequest, error)
}

// restClient implements RESTClient using the go-pagerduty SDK.
type restClient struct {
	client      *pagerduty.Client
	retryPolicy *RetryPolicy
	fromEmail   string
}

// NewRESTClient creates a new REST API client with retry policy.
func NewRESTClient(apiToken string, fromEmail string, retryPolicy *RetryPolicy) RESTClient {
	if retryPolicy == nil {
		retryPolicy = DefaultRetryPolicy()
	}

	return &restClient{
		client:      pagerduty.NewClient(apiToken),
		retryPolicy: retryPolicy,
		fromEmail:   fromEmail,
	}
}

// GetService retrieves service details with 5-second timeout and retry logic.
func (r *restClient) GetService(ctx context.Context, serviceID string) (*pagerduty.Service, error) {
	// Apply 5-second timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var service *pagerduty.Service
	var err error

	// Execute with retry
	retryErr := r.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		service, err = r.client.GetServiceWithContext(ctx, serviceID, &pagerduty.GetServiceOptions{})
		return err
	})

	if retryErr != nil {
		return nil, fmt.Errorf("GetService failed for service %s: %w", serviceID, retryErr)
	}

	return service, nil
}

// CreateIncidentNote adds a note to an incident with retry logic and fromEmail attribution.
func (r *restClient) CreateIncidentNote(ctx context.Context, incidentID, content string) (*pagerduty.IncidentNote, error) {
	// Apply 5-second timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var note *pagerduty.IncidentNote
	var err error

	// Create note struct
	noteToCreate := pagerduty.IncidentNote{
		Content: content,
	}

	// Execute with retry
	retryErr := r.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		note, err = r.client.CreateIncidentNoteWithContext(ctx, incidentID, noteToCreate)
		return err
	})

	if retryErr != nil {
		return nil, fmt.Errorf("CreateIncidentNote failed for incident %s: %w", incidentID, retryErr)
	}

	return note, nil
}

// GetIncident retrieves full incident details with retry logic.
func (r *restClient) GetIncident(ctx context.Context, incidentID string) (*pagerduty.Incident, error) {
	// Apply 5-second timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var incident *pagerduty.Incident
	var err error

	// Execute with retry
	retryErr := r.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		incident, err = r.client.GetIncidentWithContext(ctx, incidentID)
		return err
	})

	if retryErr != nil {
		return nil, fmt.Errorf("GetIncident failed for incident %s: %w", incidentID, retryErr)
	}

	return incident, nil
}

// CreateIncidentResponderRequest adds responders to an incident with retry logic.
func (r *restClient) CreateIncidentResponderRequest(ctx context.Context, incidentID, message string, responderRequestTargets []pagerduty.ResponderRequestTarget) (*pagerduty.ResponderRequest, error) {
	// Apply 5-second timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var responderReq *pagerduty.ResponderRequest
	var err error

	// Build request
	request := pagerduty.ResponderRequest{
		Message:                 message,
		ResponderRequestTargets: responderRequestTargets,
	}

	// Execute with retry
	retryErr := r.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		responderReq, err = r.client.CreateIncidentResponderRequestWithContext(ctx, incidentID, request)
		return err
	})

	if retryErr != nil {
		return nil, fmt.Errorf("CreateIncidentResponderRequest failed for incident %s: %w", incidentID, retryErr)
	}

	return responderReq, nil
}
