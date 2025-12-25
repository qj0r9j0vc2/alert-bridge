package dto

import (
	"time"
)

// PagerDutyWebhookV3 represents the PagerDuty V3 webhook payload.
// See: https://developer.pagerduty.com/docs/webhooks/v3-overview/
type PagerDutyWebhookV3 struct {
	Messages []PagerDutyWebhookMessage `json:"messages"`
}

// PagerDutyWebhookMessage represents a single message in the webhook payload.
type PagerDutyWebhookMessage struct {
	ID        string                `json:"id"`
	Event     PagerDutyWebhookEvent `json:"event"`
	CreatedOn time.Time             `json:"created_on"`
}

// PagerDutyWebhookEvent represents the event data in a webhook message.
type PagerDutyWebhookEvent struct {
	ID           string                    `json:"id"`
	EventType    string                    `json:"event_type"`
	ResourceType string                    `json:"resource_type"`
	OccurredAt   time.Time                 `json:"occurred_at"`
	Agent        *PagerDutyAgent           `json:"agent,omitempty"`
	Client       *PagerDutyClient          `json:"client,omitempty"`
	Data         PagerDutyWebhookEventData `json:"data"`
}

// PagerDutyWebhookEventData represents the event data.
type PagerDutyWebhookEventData struct {
	ID              string                      `json:"id"`
	Type            string                      `json:"type"`
	Self            string                      `json:"self"`
	HTMLURL         string                      `json:"html_url"`
	Number          int                         `json:"number"`
	Status          string                      `json:"status"`
	IncidentKey     string                      `json:"incident_key"`
	CreatedAt       time.Time                   `json:"created_at"`
	Title           string                      `json:"title"`
	Service         *PagerDutyServiceRef        `json:"service,omitempty"`
	Assignees       []PagerDutyUserRef          `json:"assignees,omitempty"`
	Acknowledgers   []PagerDutyAcknowledgerRef  `json:"acknowledgers,omitempty"`
	LastStatusChangeAt time.Time               `json:"last_status_change_at"`
	LastStatusChangeBy *PagerDutyUserRef       `json:"last_status_change_by,omitempty"`
	Priority        *PagerDutyPriorityRef       `json:"priority,omitempty"`
	Urgency         string                      `json:"urgency"`
	ResolveReason   string                      `json:"resolve_reason,omitempty"`
}

// PagerDutyAgent represents the agent that triggered the event.
type PagerDutyAgent struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Self  string `json:"self"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PagerDutyClient represents the client that triggered the event.
type PagerDutyClient struct {
	Name string `json:"name"`
}

// PagerDutyServiceRef represents a service reference.
type PagerDutyServiceRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
	Summary string `json:"summary"`
}

// PagerDutyUserRef represents a user reference.
type PagerDutyUserRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
	Summary string `json:"summary"`
	Email   string `json:"email,omitempty"`
}

// PagerDutyAcknowledgerRef represents an acknowledger reference.
type PagerDutyAcknowledgerRef struct {
	Acknowledger PagerDutyUserRef `json:"acknowledger"`
	AcknowledgedAt time.Time      `json:"acknowledged_at"`
}

// PagerDutyPriorityRef represents a priority reference.
type PagerDutyPriorityRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
	Summary string `json:"summary"`
}

// HandlePagerDutyWebhookInput represents the input for handling a PagerDuty webhook.
type HandlePagerDutyWebhookInput struct {
	EventType    string
	IncidentID   string
	IncidentKey  string // Maps to our alert fingerprint/ID
	UserEmail    string
	UserName     string
	UserID       string
	Status       string
	ResolveReason string
}

// HandlePagerDutyWebhookOutput represents the result of handling a PagerDuty webhook.
type HandlePagerDutyWebhookOutput struct {
	Processed bool
	AlertID   string
	Message   string
}

// IsSupportedEventType checks if the event type should be processed.
func IsSupportedEventType(eventType string) bool {
	supportedTypes := map[string]bool{
		// Existing event types
		"incident.acknowledged":   true,
		"incident.resolved":       true,
		"incident.unacknowledged": true,
		"incident.reassigned":     true,
		// New event types (Phase 6: US4)
		"incident.escalated":               true,
		"incident.priority_updated":        true,
		"incident.responder_added":         true,
		"incident.status_update_published": true,
	}
	return supportedTypes[eventType]
}
