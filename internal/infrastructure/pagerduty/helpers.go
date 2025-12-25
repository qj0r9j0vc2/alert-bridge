package pagerduty

import (
	"fmt"
	"strings"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// formatIncidentNote creates a formatted incident note with attribution.
// Combines the ack event note with alert context and user information.
func formatIncidentNote(alert *entity.Alert, ackEvent *entity.AckEvent, fromEmail string) string {
	var parts []string

	// Add user attribution if available
	if ackEvent.UserName != "" {
		parts = append(parts, fmt.Sprintf("Acknowledged by: %s", ackEvent.UserName))
	} else if fromEmail != "" {
		parts = append(parts, fmt.Sprintf("Acknowledged via: %s", fromEmail))
	} else {
		parts = append(parts, "Acknowledged via alert-bridge")
	}

	// Add the actual note content
	if ackEvent.Note != "" {
		parts = append(parts, "", "Comment:", ackEvent.Note)
	}

	// Add alert context
	parts = append(parts, "", "Alert Details:")
	parts = append(parts, fmt.Sprintf("- Name: %s", alert.Name))
	if alert.Instance != "" {
		parts = append(parts, fmt.Sprintf("- Instance: %s", alert.Instance))
	}
	if alert.Fingerprint != "" {
		parts = append(parts, fmt.Sprintf("- Fingerprint: %s", alert.Fingerprint))
	}

	return strings.Join(parts, "\n")
}
