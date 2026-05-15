package models

import (
	"time"

	"github.com/uptrace/bun"
)

type PaymentEvent struct {
	bun.BaseModel `bun:"table:payment_events"`

	ID        string         `bun:",pk,type:uuid"`
	PaymentID string         `bun:",type:uuid,notnull"`
	EventType string         `bun:",notnull"`
	Data      map[string]any `bun:",type:jsonb"`
	CreatedAt time.Time      `bun:",notnull"`
}
