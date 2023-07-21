package migrations_sqlite

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type CustomMigrationFunc func(ctx context.Context, db *sqlx.DB) error

var PreMigrations = map[uint]CustomMigrationFunc{
	32: pre32,
	48: pre48,
}
var PostMigrations = map[uint]CustomMigrationFunc{
	12: post12,
	32: post32,
	34: post34,
	42: post42,
	45: post45,
}
