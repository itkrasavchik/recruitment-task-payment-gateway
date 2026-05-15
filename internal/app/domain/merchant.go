package domain

import "time"

type Merchant struct {
	ID        string
	Name      string
	APIKey    string //nolint:gosec // not a hardcoded credential
	CreatedAt time.Time
}
