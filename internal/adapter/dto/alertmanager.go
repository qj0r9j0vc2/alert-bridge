package dto

import (
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AlertmanagerWebhook represents the webhook payload from Prometheus Alertmanager.
// See: https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type AlertmanagerWebhook struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Status            string            `json:"status"` // "firing" or "resolved"
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []AlertmanagerAlert `json:"alerts"`
}

// AlertmanagerAlert represents a single alert in the Alertmanager webhook payload.
type AlertmanagerAlert struct {
	Status       string            `json:"status"` // "firing" or "resolved"
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// ProcessAlertInput represents the input for processing an alert.
type ProcessAlertInput struct {
	Fingerprint  string
	Name         string
	Instance     string
	Target       string
	Summary      string
	Description  string
	Severity     entity.AlertSeverity
	Status       string // "firing" or "resolved"
	Labels       map[string]string
	Annotations  map[string]string
	FiredAt      time.Time
}

// ToProcessAlertInput converts an AlertmanagerAlert to ProcessAlertInput.
func ToProcessAlertInput(alert AlertmanagerAlert) ProcessAlertInput {
	return ProcessAlertInput{
		Fingerprint: alert.Fingerprint,
		Name:        alert.Labels["alertname"],
		Instance:    alert.Labels["instance"],
		Target:      alert.Labels["job"],
		Summary:     alert.Annotations["summary"],
		Description: alert.Annotations["description"],
		Severity:    mapSeverity(alert.Labels["severity"]),
		Status:      alert.Status,
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
		FiredAt:     alert.StartsAt,
	}
}

// mapSeverity converts Alertmanager severity label to entity.AlertSeverity.
func mapSeverity(severity string) entity.AlertSeverity {
	switch severity {
	case "critical", "page":
		return entity.SeverityCritical
	case "warning", "warn":
		return entity.SeverityWarning
	default:
		return entity.SeverityInfo
	}
}

// IsFiring returns true if the alert status is "firing".
func (a *AlertmanagerAlert) IsFiring() bool {
	return a.Status == "firing"
}

// IsResolved returns true if the alert status is "resolved".
func (a *AlertmanagerAlert) IsResolved() bool {
	return a.Status == "resolved"
}

// ProcessAlertOutput represents the result of processing an alert.
type ProcessAlertOutput struct {
	AlertID             string
	IsNew               bool
	IsSilenced          bool
	NotificationsSent   []string
	NotificationsFailed []NotificationError
}

// NotificationError represents a failed notification attempt.
type NotificationError struct {
	NotifierName string
	Error        error
}
