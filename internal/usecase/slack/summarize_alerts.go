package slack

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// SummarizeAlertsUseCase computes alert summary statistics.
type SummarizeAlertsUseCase struct {
	alertRepo repository.AlertRepository
}

// NewSummarizeAlertsUseCase creates a new summarize alerts use case.
func NewSummarizeAlertsUseCase(alertRepo repository.AlertRepository) *SummarizeAlertsUseCase {
	return &SummarizeAlertsUseCase{
		alertRepo: alertRepo,
	}
}

// Execute computes summary statistics from non-resolved alerts within the specified period.
// If period is 0, all active alerts are included.
// Otherwise, only alerts fired within the period are included.
func (uc *SummarizeAlertsUseCase) Execute(ctx context.Context, period time.Duration) (*entity.AlertSummary, error) {
	// Fetch all active (non-resolved) alerts
	alerts, err := uc.alertRepo.GetActiveAlerts(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get active alerts: %w", err)
	}

	// Filter by period if specified
	if period > 0 {
		cutoff := time.Now().Add(-period)
		alerts = uc.filterAlertsByTime(alerts, cutoff)
	}

	summary := entity.NewAlertSummary()
	summary.TotalAlerts = len(alerts)

	// Track acknowledgers for counting
	acknowledgerCounts := make(map[string]int)

	// Aggregate statistics from alerts
	for _, alert := range alerts {
		// Count by severity
		summary.AlertsBySeverity[alert.Severity]++

		// Count by state
		summary.AlertsByState[alert.State]++

		// Count by instance
		if alert.Instance != "" {
			summary.AlertsByInstance[alert.Instance]++
		}

		// Count acknowledgers (only for acknowledged alerts)
		if alert.IsAcked() && alert.AckedBy != "" {
			acknowledgerCounts[alert.AckedBy]++
		}
	}

	// Convert acknowledger counts to sorted list
	summary.TopAcknowledgers = uc.buildTopAcknowledgers(acknowledgerCounts)

	return summary, nil
}

// filterAlertsByTime returns only alerts fired on or after the cutoff time.
func (uc *SummarizeAlertsUseCase) filterAlertsByTime(alerts []*entity.Alert, cutoff time.Time) []*entity.Alert {
	filtered := make([]*entity.Alert, 0, len(alerts))
	for _, alert := range alerts {
		if !alert.FiredAt.Before(cutoff) {
			filtered = append(filtered, alert)
		}
	}
	return filtered
}

// buildTopAcknowledgers converts a map of acknowledger counts to a sorted slice.
// Returns up to 5 top acknowledgers sorted by count descending.
func (uc *SummarizeAlertsUseCase) buildTopAcknowledgers(counts map[string]int) []entity.UserAckCount {
	result := make([]entity.UserAckCount, 0, len(counts))

	for user, count := range counts {
		result = append(result, entity.UserAckCount{
			UserName: user,
			Count:    count,
		})
	}

	// Sort by count descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	// Return top 5
	if len(result) > 5 {
		result = result[:5]
	}

	return result
}
