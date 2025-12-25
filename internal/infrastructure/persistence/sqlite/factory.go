package sqlite

import "database/sql"

// Repositories holds all SQLite repository implementations.
type Repositories struct {
	Alert    *AlertRepository
	AckEvent *AckEventRepository
	Silence  *SilenceRepository
}

// NewRepositories creates all SQLite repositories with a shared database connection.
// This factory ensures all repositories use the same DB instance for transactions
// and connection pooling.
func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Alert:    NewAlertRepository(db),
		AckEvent: NewAckEventRepository(db),
		Silence:  NewSilenceRepository(db),
	}
}
