package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/golang-migrate/migrate/v4"
	postgresmig "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	migrations "github.com/stashapp/stash/pkg/db/migrations_postgres"
	"github.com/stashapp/stash/pkg/logger"
)

const (
	// Max number of database connections to use
	postgresDBConns = 10
	// Number of idle database connections to use
	postgresDBIdleConns = 10
	// Idle connection timeout, in seconds
	postgresDBConnTimeout = 30

	postgresDriver = "pgx"
)

//go:embed migrations_postgres/*.sql
var postgresMigrationsBox embed.FS

func (db *Database) SetPostgresUrl(dbUrl string) error {
	db.lockNoCtx()
	defer db.unlock()

	// ensure db is closed
	if db.db != nil {
		return errors.New("database is open")
	}

	db.dbUrl = dbUrl
	db.DBType = PostgresDB

	return nil
}

func (db *Database) postgresOpenDB() (*sqlx.DB, error) {
	conn, err := sqlx.Open(postgresDriver, db.dbUrl)
	conn.SetMaxOpenConns(postgresDBConns)
	conn.SetMaxIdleConns(postgresDBIdleConns)
	conn.SetConnMaxIdleTime(postgresDBConnTimeout * time.Second)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return conn, nil
}

func (db *Database) postgresGetMigrate() (*migrate.Migrate, error) {
	migrations, err := iofs.New(postgresMigrationsBox, "migrations_postgres")
	if err != nil {
		return nil, err
	}

	conn, err := db.postgresOpenDB()
	if err != nil {
		return nil, err
	}

	driver, err := postgresmig.WithInstance(conn.DB, &postgresmig.Config{})
	if err != nil {
		return nil, err
	}

	return migrate.NewWithInstance(
		"iofs",
		migrations,
		db.dbUrl,
		driver,
	)
}

func (db *Database) postgresRunMigrations() error {
	ctx := context.Background()

	m, err := db.getMigrate()
	if err != nil {
		return err
	}
	defer m.Close()

	databaseSchemaVersion, _, _ := m.Version()
	stepNumber := AppSchemaVersion - databaseSchemaVersion
	if stepNumber != 0 {
		logger.Infof("Migrating database from version %d to %d", databaseSchemaVersion, AppSchemaVersion)

		// run each migration individually, and run custom migrations as needed
		var i uint = 1
		for ; i <= stepNumber; i++ {
			newVersion := databaseSchemaVersion + i

			// run pre migrations as needed
			if err := db.postgresRunCustomMigration(ctx, migrations.PreMigrations[newVersion]); err != nil {
				return fmt.Errorf("running pre migrations for schema version %d: %w", newVersion, err)
			}

			err := m.Steps(1)
			if err != nil {
				// migration failed
				return err
			}

			// run post migrations as needed
			if err := db.postgresRunCustomMigration(ctx, migrations.PostMigrations[newVersion]); err != nil {
				return fmt.Errorf("running post migrations for schema version %d: %w", newVersion, err)
			}
		}
	}

	// update the schema version
	db.schemaVersion, _, _ = m.Version()

	return nil
}

func (db *Database) postgresRunCustomMigration(ctx context.Context, fn migrations.CustomMigrationFunc) error {
	if fn == nil {
		return nil
	}

	d, err := db.postgresOpenDB()
	if err != nil {
		return err
	}

	defer d.Close()
	if err := fn(ctx, d); err != nil {
		return err
	}

	return nil
}
