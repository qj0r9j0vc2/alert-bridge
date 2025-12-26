package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// QueryAlertStatusUseCase handles alert status queries.
type QueryAlertStatusUseCase struct {
	alertRepo repository.AlertRepository
}

// NewQueryAlertStatusUseCase creates a new query alert status use case.
func NewQueryAlertStatusUseCase(alertRepo repository.AlertRepository) *QueryAlertStatusUseCase {
	return &QueryAlertStatusUseCase{
		alertRepo: alertRepo,
	}
}

// Execute queries active alerts optionally filtered by severity.
// Severity parameter: "critical", "warning", "info", or "" for all severities.
func (uc *QueryAlertStatusUseCase) Execute(ctx context.Context, severity string) ([]*entity.Alert, error) {
	// Parse and validate severity
	parsedSeverity := uc.parseSeverity(severity)

	// Query alerts from repository
	alerts, err := uc.alertRepo.GetActiveAlerts(ctx, parsedSeverity)
	if err != nil {
		return nil, fmt.Errorf("failed to get active alerts: %w", err)
	}

	return alerts, nil
}

// parseSeverity normalizes severity input.
// Accepts: "critical", "warning", "info" (case-insensitive)
// Returns: normalized severity or "" for all severities
func (uc *QueryAlertStatusUseCase) parseSeverity(severity string) string {
	normalized := strings.ToLower(strings.TrimSpace(severity))

	switch normalized {
	case "critical", "crit":
		return "critical"
	case "warning", "warn":
		return "warning"
	case "info", "information":
		return "info"
	default:
		// Empty or invalid input returns empty string (all severities)
		return ""
	}
}
