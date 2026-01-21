package entity

// AlertSummary holds aggregated statistics about alerts.
type AlertSummary struct {
	// TotalAlerts is the total number of non-resolved alerts.
	TotalAlerts int

	// AlertsBySeverity maps severity to count.
	AlertsBySeverity map[AlertSeverity]int

	// AlertsByState maps state to count.
	AlertsByState map[AlertState]int

	// AlertsByInstance maps instance name to count.
	AlertsByInstance map[string]int

	// TopAcknowledgers lists users who acknowledged the most alerts.
	TopAcknowledgers []UserAckCount
}

// UserAckCount represents acknowledgment count for a user.
type UserAckCount struct {
	// UserName is the display name of the user.
	UserName string

	// UserEmail is the email of the user.
	UserEmail string

	// Count is the number of acknowledgments.
	Count int
}

// NewAlertSummary creates an empty alert summary.
func NewAlertSummary() *AlertSummary {
	return &AlertSummary{
		AlertsBySeverity: make(map[AlertSeverity]int),
		AlertsByState:    make(map[AlertState]int),
		AlertsByInstance: make(map[string]int),
		TopAcknowledgers: []UserAckCount{},
	}
}

// TopInstance returns the instance with the most alerts.
// Returns empty string if no instances found.
func (s *AlertSummary) TopInstance() string {
	var topInstance string
	var maxCount int
	for instance, count := range s.AlertsByInstance {
		if count > maxCount {
			maxCount = count
			topInstance = instance
		}
	}
	return topInstance
}

// TopInstanceCount returns the count for the top instance.
func (s *AlertSummary) TopInstanceCount() int {
	var maxCount int
	for _, count := range s.AlertsByInstance {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

// TopAcknowledger returns the user who acknowledged the most alerts.
// Returns empty UserAckCount if no acknowledgers found.
func (s *AlertSummary) TopAcknowledger() UserAckCount {
	if len(s.TopAcknowledgers) == 0 {
		return UserAckCount{}
	}
	return s.TopAcknowledgers[0]
}

// CriticalCount returns the count of critical alerts.
func (s *AlertSummary) CriticalCount() int {
	return s.AlertsBySeverity[SeverityCritical]
}

// WarningCount returns the count of warning alerts.
func (s *AlertSummary) WarningCount() int {
	return s.AlertsBySeverity[SeverityWarning]
}

// InfoCount returns the count of info alerts.
func (s *AlertSummary) InfoCount() int {
	return s.AlertsBySeverity[SeverityInfo]
}

// ActiveCount returns the count of active (unacknowledged) alerts.
func (s *AlertSummary) ActiveCount() int {
	return s.AlertsByState[StateActive]
}

// AcknowledgedCount returns the count of acknowledged alerts.
func (s *AlertSummary) AcknowledgedCount() int {
	return s.AlertsByState[StateAcked]
}
