package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Payment struct {
	bun.BaseModel `bun:"table:payments"`

	ID                string  `bun:",pk,type:uuid,default:gen_random_uuid()"`
	MerchantID        string  `bun:",type:uuid,notnull"`
	IdempotencyKey    string  `bun:",notnull"`
	CardLast4         string  `bun:",notnull"`
	ExpiryMonth       int     `bun:",notnull"`
	ExpiryYear        int     `bun:",notnull"`
	Amount            int64   `bun:",notnull"`
	Currency          string  `bun:",notnull"`
	Status            string  `bun:",notnull"`
	DeclineReason     *string `bun:""`
	BankTransactionID *string `bun:""`

	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}
