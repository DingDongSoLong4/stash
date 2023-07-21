package migrations_postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type CustomMigrationFunc func(ctx context.Context, db *sqlx.DB) error

var PreMigrations = map[uint]CustomMigrationFunc{}
var PostMigrations = map[uint]CustomMigrationFunc{}
