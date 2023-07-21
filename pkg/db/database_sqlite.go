package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3mig "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"

	migrations "github.com/stashapp/stash/pkg/db/migrations_sqlite"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
)

const (
	// Number of database connections to use
	// The same value is used for both the maximum and idle limit,
	// to prevent opening connections on the fly which has a notieable performance penalty.
	// Fewer connections use less memory, more connections increase performance,
	// but have diminishing returns.
	// 10 was found to be a good tradeoff.
	sqliteDBConns = 10
	// Idle connection timeout, in seconds
	// Closes a connection after a period of inactivity, which saves on memory and
	// causes the sqlite -wal and -shm files to be automatically deleted.
	sqliteDBConnTimeout = 30
)

//go:embed migrations_sqlite/*.sql
var sqliteMigrationsBox embed.FS

func (db *Database) SetSQLitePath(dbPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	// ensure db is closed
	if db.db != nil {
		return errors.New("database is open")
	}

	db.dbUrl = dbPath
	db.DBType = SQLiteDB

	return nil
}

func (db *Database) sqliteOpenDB(disableForeignKeys bool) (*sqlx.DB, error) {
	// https://github.com/mattn/go-sqlite3
	url := "file:" + db.dbUrl + "?_journal=WAL&_sync=NORMAL&_busy_timeout=50"
	if !disableForeignKeys {
		url += "&_fk=true"
	}

	conn, err := sqlx.Open(sqlite3Driver, url)
	conn.SetMaxOpenConns(sqliteDBConns)
	conn.SetMaxIdleConns(sqliteDBConns)
	conn.SetConnMaxIdleTime(sqliteDBConnTimeout * time.Second)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return conn, nil
}

func (db *Database) sqliteRemove() error {
	databasePath := db.dbUrl

	err := db.close()
	if err != nil {
		return fmt.Errorf("closing database: %w", err)
	}

	err = os.Remove(databasePath)
	if err != nil {
		return fmt.Errorf("removing database: %w", err)
	}

	// remove the -shm, -wal files ( if they exist )
	walFiles := []string{databasePath + "-shm", databasePath + "-wal"}
	for _, wf := range walFiles {
		if exists, _ := fsutil.FileExists(wf); exists {
			err = os.Remove(wf)
			if err != nil {
				return fmt.Errorf("removing database: %w", err)
			}
		}
	}

	return nil
}

func (db *Database) sqliteReset() error {
	if err := db.sqliteRemove(); err != nil {
		return err
	}

	if err := db.open(); err != nil {
		return fmt.Errorf("initializing new database: %w", err)
	}

	return nil
}

// Backup the database. Will open a temporary database connection if necessary.
func (db *Database) sqliteBackup(backupPath string) error {
	conn := db.db
	if conn == nil {
		var err error
		const disableForeignKeys = false
		conn, err = db.sqliteOpenDB(disableForeignKeys)
		if err != nil {
			return err
		}
		defer conn.Close()
	}

	logger.Infof("Backing up database to %s", backupPath)
	_, err := conn.Exec(`VACUUM INTO "` + backupPath + `"`)
	if err != nil {
		return fmt.Errorf("vacuum failed: %v", err)
	}

	return nil
}

func (db *Database) sqliteAnonymise(outPath string) error {
	logger.Infof("Anonymising database to %s", outPath)
	anon, err := NewAnonymiser(db, outPath)
	if err != nil {
		return err
	}

	return anon.Anonymise(context.Background())
}

func (db *Database) sqliteRestore(backupPath string) error {
	// ensure db is closed
	err := db.close()
	if err != nil {
		return err
	}

	logger.Infof("Restoring from backup database %s", backupPath)
	return os.Rename(backupPath, db.dbUrl)
}

func (db *Database) sqliteGetMigrate() (*migrate.Migrate, error) {
	migrations, err := iofs.New(sqliteMigrationsBox, "migrations_sqlite")
	if err != nil {
		return nil, err
	}

	const disableForeignKeys = true
	conn, err := db.sqliteOpenDB(disableForeignKeys)
	if err != nil {
		return nil, err
	}

	driver, err := sqlite3mig.WithInstance(conn.DB, &sqlite3mig.Config{})
	if err != nil {
		return nil, err
	}

	// use sqlite3Driver so that migration has access to durationToTinyInt
	return migrate.NewWithInstance(
		"iofs",
		migrations,
		db.dbUrl,
		driver,
	)
}

// Vacuum runs a VACUUM on the database, rebuilding the database file into a minimal amount of disk space.
func (db *Database) sqliteVacuum(ctx context.Context) error {
	conn := db.db
	if conn == nil {
		return ErrDatabaseNotInitialized
	}

	_, err := conn.ExecContext(ctx, "VACUUM")
	return err
}

func (db *Database) sqliteRunMigrations() error {
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
			if err := db.sqliteRunCustomMigration(ctx, migrations.PreMigrations[newVersion]); err != nil {
				return fmt.Errorf("running pre migration for schema version %d: %w", newVersion, err)
			}

			err = m.Steps(1)
			if err != nil {
				// migration failed
				return err
			}

			// run post migrations as needed
			if err := db.sqliteRunCustomMigration(ctx, migrations.PostMigrations[newVersion]); err != nil {
				return fmt.Errorf("running post migration for schema version %d: %w", newVersion, err)
			}
		}
	}

	// update the schema version
	db.schemaVersion, _, _ = m.Version()

	// optimize database after migration

	const disableForeignKeys = false
	conn, err := db.sqliteOpenDB(disableForeignKeys)
	if err != nil {
		return fmt.Errorf("reopening the database: %w", err)
	}
	defer conn.Close()

	logger.Info("Optimizing database")
	_, err = conn.Exec("ANALYZE")
	if err != nil {
		logger.Warnf("error while performing post-migration optimization: %v", err)
	}
	_, err = conn.Exec("VACUUM")
	if err != nil {
		logger.Warnf("error while performing post-migration vacuum: %v", err)
	}

	return nil
}

func (db *Database) sqliteRunCustomMigration(ctx context.Context, fn migrations.CustomMigrationFunc) error {
	if fn == nil {
		return nil
	}

	const disableForeignKeys = false
	d, err := db.sqliteOpenDB(disableForeignKeys)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := fn(ctx, d); err != nil {
		return err
	}

	return nil
}

func (db *Database) sqliteIsLocked(err error) bool {
	var sqliteError sqlite3.Error
	if errors.As(err, &sqliteError) {
		return sqliteError.Code == sqlite3.ErrBusy
	}
	return false
}
