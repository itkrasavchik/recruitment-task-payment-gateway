package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Merchant struct {
	bun.BaseModel `bun:"table:merchants"`

	ID        string    `bun:",pk,type:uuid,default:gen_random_uuid()"`
	Name      string    `bun:",notnull"`
	APIKey    string    `bun:"api_key,notnull,unique"` //nolint:gosec // not a hardcoded credential
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}
