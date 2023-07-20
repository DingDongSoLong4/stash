package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3mig "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"

	// register custom migrations
	"github.com/stashapp/stash/pkg/db/migrations"
)

const (
	// Number of database connections to use
	// The same value is used for both the maximum and idle limit,
	// to prevent opening connections on the fly which has a notieable performance penalty.
	// Fewer connections use less memory, more connections increase performance,
	// but have diminishing returns.
	// 10 was found to be a good tradeoff.
	dbConns = 10
	// Idle connection timeout, in seconds
	// Closes a connection after a period of inactivity, which saves on memory and
	// causes the sqlite -wal and -shm files to be automatically deleted.
	dbConnTimeout = 30
)

var appSchemaVersion uint = 48

//go:embed migrations/*.sql
var migrationsBox embed.FS

var (
	// ErrDatabaseNotInitialized indicates that the database is not
	// initialized, usually due to an incomplete configuration.
	ErrDatabaseNotInitialized = errors.New("database not initialized")
)

// ErrMigrationNeeded indicates that a database migration is needed
// before the database can be initialized
type MigrationNeededError struct {
	CurrentSchemaVersion  uint
	RequiredSchemaVersion uint
}

func (e *MigrationNeededError) Error() string {
	return fmt.Sprintf("database schema version %d does not match required schema version %d", e.CurrentSchemaVersion, e.RequiredSchemaVersion)
}

type MismatchedSchemaVersionError struct {
	CurrentSchemaVersion  uint
	RequiredSchemaVersion uint
}

func (e *MismatchedSchemaVersionError) Error() string {
	return fmt.Sprintf("schema version %d is incompatible with required schema version %d", e.CurrentSchemaVersion, e.RequiredSchemaVersion)
}

type Database struct {
	Blobs          *BlobStore
	File           *FileStore
	Folder         *FolderStore
	Image          *ImageStore
	Gallery        *GalleryStore
	GalleryChapter *GalleryChapterStore
	Scene          *SceneStore
	SceneMarker    *SceneMarkerStore
	Performer      *PerformerStore
	Studio         *StudioStore
	Tag            *TagStore
	Movie          *MovieStore
	SavedFilter    *SavedFilterStore

	db     *sqlx.DB
	dbPath string

	schemaVersion uint

	lockChan chan struct{}
}

func NewDatabase() *Database {
	fileStore := NewFileStore()
	folderStore := NewFolderStore()
	blobStore := NewBlobStore(BlobStoreOptions{})

	db := &Database{
		Blobs:          blobStore,
		File:           fileStore,
		Folder:         folderStore,
		Scene:          NewSceneStore(fileStore, blobStore),
		SceneMarker:    NewSceneMarkerStore(),
		Image:          NewImageStore(fileStore),
		Gallery:        NewGalleryStore(fileStore, folderStore),
		GalleryChapter: NewGalleryChapterStore(),
		Performer:      NewPerformerStore(blobStore),
		Studio:         NewStudioStore(blobStore),
		Tag:            NewTagStore(blobStore),
		Movie:          NewMovieStore(blobStore),
		SavedFilter:    NewSavedFilterStore(),

		lockChan: make(chan struct{}, 1),
	}

	return db
}

func (db *Database) SetBlobStoreOptions(options BlobStoreOptions) {
	*db.Blobs = *NewBlobStore(options)
}

// Ready returns an error if the database is not ready to begin transactions.
func (db *Database) Ready() error {
	if db.db == nil {
		return ErrDatabaseNotInitialized
	}

	return nil
}

// Open initializes the database. If the database is new, then it
// performs a full migration to the latest schema version. Otherwise, any
// necessary migrations must be run separately using Migrate.
func (db *Database) Open() error {
	db.lockNoCtx()
	defer db.unlock()

	return db.open()
}

func (db *Database) open() error {
	if db.db != nil {
		err := db.db.Close()
		if err != nil {
			return fmt.Errorf("closing existing database connection: %w", err)
		}
		db.db = nil
	}

	databaseSchemaVersion, err := db.getDatabaseSchemaVersion()
	if err != nil {
		return fmt.Errorf("getting database schema version: %w", err)
	}

	db.schemaVersion = databaseSchemaVersion

	if databaseSchemaVersion == 0 {
		// new database, just run the migrations
		if err := db.runMigrations(); err != nil {
			return fmt.Errorf("error running initial schema migrations: %v", err)
		}
	} else {
		if databaseSchemaVersion > appSchemaVersion {
			return &MismatchedSchemaVersionError{
				CurrentSchemaVersion:  databaseSchemaVersion,
				RequiredSchemaVersion: appSchemaVersion,
			}
		}

		// if migration is needed, then don't open the connection
		if databaseSchemaVersion != appSchemaVersion {
			return &MigrationNeededError{
				CurrentSchemaVersion:  databaseSchemaVersion,
				RequiredSchemaVersion: appSchemaVersion,
			}
		}
	}

	const disableForeignKeys = false
	db.db, err = db.openDB(disableForeignKeys)
	if err != nil {
		return err
	}

	return nil
}

// lock locks the database for writing.
// This method will block until the lock is acquired or the context is cancelled.
func (db *Database) lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case db.lockChan <- struct{}{}:
		return nil
	}
}

// lock locks the database for writing. This method will block until the lock is acquired.
func (db *Database) lockNoCtx() {
	db.lockChan <- struct{}{}
}

// unlock unlocks the database
func (db *Database) unlock() {
	// will block the caller if the lock is not held, so check first
	select {
	case <-db.lockChan:
		return
	default:
		panic("database is not locked")
	}
}

func (db *Database) Close() error {
	db.lockNoCtx()
	defer db.unlock()

	return db.close()
}

func (db *Database) close() error {
	if db.db != nil {
		if err := db.db.Close(); err != nil {
			return err
		}

		db.db = nil
	}

	return nil
}

func (db *Database) openDB(disableForeignKeys bool) (*sqlx.DB, error) {
	// https://github.com/mattn/go-sqlite3
	url := "file:" + db.dbPath + "?_journal=WAL&_sync=NORMAL&_busy_timeout=50"
	if !disableForeignKeys {
		url += "&_fk=true"
	}

	conn, err := sqlx.Open(sqlite3Driver, url)
	conn.SetMaxOpenConns(dbConns)
	conn.SetMaxIdleConns(dbConns)
	conn.SetConnMaxIdleTime(dbConnTimeout * time.Second)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return conn, nil
}

func (db *Database) Remove() error {
	db.lockNoCtx()
	defer db.unlock()

	return db.remove()
}

func (db *Database) remove() error {
	databasePath := db.dbPath

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

func (db *Database) Reset() error {
	db.lockNoCtx()
	defer db.unlock()

	if err := db.remove(); err != nil {
		return err
	}

	if err := db.open(); err != nil {
		return fmt.Errorf("initializing new database: %w", err)
	}

	return nil
}

// Backup the database. Will open a temporary database connection if necessary.
func (db *Database) Backup(backupPath string) error {
	conn := db.db
	if conn == nil {
		var err error
		const disableForeignKeys = false
		conn, err = db.openDB(disableForeignKeys)
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

func (db *Database) Anonymise(outPath string) error {
	logger.Infof("Anonymising database to %s", outPath)
	anon, err := NewAnonymiser(db, outPath)
	if err != nil {
		return err
	}

	return anon.Anonymise(context.Background())
}

func (db *Database) RestoreFromBackup(backupPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	// ensure db is closed
	err := db.close()
	if err != nil {
		return fmt.Errorf("closing database: %w", err)
	}

	logger.Infof("Restoring from backup database %s", backupPath)
	return os.Rename(backupPath, db.dbPath)
}

func (db *Database) AppSchemaVersion() uint {
	return appSchemaVersion
}

func (db *Database) DatabasePath() string {
	return db.dbPath
}

func (db *Database) SetDatabasePath(dbPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	// ensure db is closed
	if db.db != nil {
		return errors.New("database is open")
	}

	db.dbPath = dbPath

	return nil
}

func (db *Database) DatabaseBackupPath(backupDirectoryPath string) string {
	fn := fmt.Sprintf("%s.%d.%s", filepath.Base(db.dbPath), db.schemaVersion, time.Now().Format("20060102_150405"))

	if backupDirectoryPath != "" {
		return filepath.Join(backupDirectoryPath, fn)
	}

	return fn
}

func (db *Database) AnonymousDatabasePath(backupDirectoryPath string) string {
	fn := fmt.Sprintf("%s.anonymous.%d.%s", filepath.Base(db.dbPath), db.schemaVersion, time.Now().Format("20060102_150405"))

	if backupDirectoryPath != "" {
		return filepath.Join(backupDirectoryPath, fn)
	}

	return fn
}

func (db *Database) Version() uint {
	return db.schemaVersion
}

func (db *Database) getMigrate() (*migrate.Migrate, error) {
	migrations, err := iofs.New(migrationsBox, "migrations")
	if err != nil {
		return nil, err
	}

	const disableForeignKeys = true
	conn, err := db.openDB(disableForeignKeys)
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
		db.dbPath,
		driver,
	)
}

func (db *Database) getDatabaseSchemaVersion() (uint, error) {
	m, err := db.getMigrate()
	if err != nil {
		return 0, err
	}
	defer m.Close()

	ret, _, _ := m.Version()
	return ret, nil
}

// Migrate the database. Will use temporary database connections,
// the database must be reopened to be used afterwards.
func (db *Database) Migrate() error {
	db.lockNoCtx()
	defer db.unlock()

	return db.runMigrations()
}

// Vacuum runs a VACUUM on the database, rebuilding the database file into a minimal amount of disk space.
func (db *Database) Vacuum(ctx context.Context) error {
	conn := db.db
	if conn == nil {
		return ErrDatabaseNotInitialized
	}

	_, err := conn.ExecContext(ctx, "VACUUM")
	return err
}

func (db *Database) runMigrations() error {
	ctx := context.Background()

	m, err := db.getMigrate()
	if err != nil {
		return err
	}
	defer m.Close()

	databaseSchemaVersion, _, _ := m.Version()
	stepNumber := appSchemaVersion - databaseSchemaVersion
	if stepNumber != 0 {
		logger.Infof("Migrating database from version %d to %d", databaseSchemaVersion, appSchemaVersion)

		// run each migration individually, and run custom migrations as needed
		var i uint = 1
		for ; i <= stepNumber; i++ {
			newVersion := databaseSchemaVersion + i

			// run pre migrations as needed
			if err := db.runCustomMigration(ctx, migrations.PreMigrations[newVersion]); err != nil {
				return fmt.Errorf("running pre migration for schema version %d: %w", newVersion, err)
			}

			err = m.Steps(1)
			if err != nil {
				// migration failed
				return err
			}

			// run post migrations as needed
			if err := db.runCustomMigration(ctx, migrations.PostMigrations[newVersion]); err != nil {
				return fmt.Errorf("running post migration for schema version %d: %w", newVersion, err)
			}
		}
	}

	// update the schema version
	db.schemaVersion, _, _ = m.Version()

	// optimize database after migration

	const disableForeignKeys = false
	conn, err := db.openDB(disableForeignKeys)
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

func (db *Database) runCustomMigration(ctx context.Context, fn migrations.CustomMigrationFunc) error {
	if fn == nil {
		return nil
	}

	const disableForeignKeys = false
	d, err := db.openDB(disableForeignKeys)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := fn(ctx, d); err != nil {
		return err
	}

	return nil
}
