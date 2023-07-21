package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jmoiron/sqlx"
)

var AppSchemaVersion uint = 48

var (
	// ErrDatabaseNotInitialized indicates that the database is not
	// initialized, usually due to an incomplete configuration.
	ErrDatabaseNotInitialized = errors.New("database not initialized")
)

// ErrMigrationNeeded indicates that a database migration is needed
// before the database can be initialized
type MigrationNeededError struct {
	CurrentSchemaVersion uint
}

func (e *MigrationNeededError) Error() string {
	return fmt.Sprintf("database schema version %d does not match required schema version %d", e.CurrentSchemaVersion, AppSchemaVersion)
}

type MismatchedSchemaVersionError struct {
	CurrentSchemaVersion uint
}

func (e *MismatchedSchemaVersionError) Error() string {
	return fmt.Sprintf("schema version %d is incompatible with required schema version %d", e.CurrentSchemaVersion, AppSchemaVersion)
}

type DBType string

const (
	SQLiteDB   = "SQLite"
	PostgresDB = "PostgreSQL"
)

type UnsupportedForDBTypeError struct {
	Operation string
	DBType    DBType
}

func (e *UnsupportedForDBTypeError) Error() string {
	return fmt.Sprintf("%s does not support %s", e.DBType, e.Operation)
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

	db    *sqlx.DB
	dbUrl string

	DBType DBType

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
		if databaseSchemaVersion > AppSchemaVersion {
			return &MismatchedSchemaVersionError{
				CurrentSchemaVersion: databaseSchemaVersion,
			}
		}

		// if migration is needed, then don't open the connection
		if databaseSchemaVersion != AppSchemaVersion {
			return &MigrationNeededError{
				CurrentSchemaVersion: databaseSchemaVersion,
			}
		}
	}

	switch db.DBType {
	case SQLiteDB:
		const disableForeignKeys = false
		db.db, err = db.sqliteOpenDB(disableForeignKeys)
		if err != nil {
			return err
		}
	case PostgresDB:
		db.db, err = db.postgresOpenDB()
		if err != nil {
			return err
		}
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

func (db *Database) Reset() error {
	db.lockNoCtx()
	defer db.unlock()

	switch db.DBType {
	case SQLiteDB:
		return db.sqliteReset()
	default:
		return &UnsupportedForDBTypeError{
			Operation: "resetting",
			DBType:    db.DBType,
		}
	}
}

// Backup the database. Will open a temporary database connection if necessary.
func (db *Database) Backup(backupPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	switch db.DBType {
	case SQLiteDB:
		return db.sqliteBackup(backupPath)
	default:
		return &UnsupportedForDBTypeError{
			Operation: "backups",
			DBType:    db.DBType,
		}
	}
}

func (db *Database) Restore(backupPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	switch db.DBType {
	case SQLiteDB:
		return db.sqliteRestore(backupPath)
	default:
		return &UnsupportedForDBTypeError{
			Operation: "restoring from backup",
			DBType:    db.DBType,
		}
	}
}

func (db *Database) Anonymise(outPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	switch db.DBType {
	case SQLiteDB:
		return db.sqliteAnonymise(outPath)
	default:
		return &UnsupportedForDBTypeError{
			Operation: "anonymising",
			DBType:    db.DBType,
		}
	}
}

func (db *Database) Url() string {
	return db.dbUrl
}

func (db *Database) SetUrl(dbPath string) error {
	db.lockNoCtx()
	defer db.unlock()

	// ensure db is closed
	if db.db != nil {
		return errors.New("database is open")
	}

	db.dbUrl = dbPath

	if strings.HasPrefix(dbPath, "postgresql://") {
		db.DBType = PostgresDB
	} else {
		db.DBType = SQLiteDB
	}

	return nil
}

func (db *Database) Version() uint {
	return db.schemaVersion
}

func (db *Database) getMigrate() (*migrate.Migrate, error) {
	switch db.DBType {
	case SQLiteDB:
		return db.sqliteGetMigrate()
	case PostgresDB:
		return db.postgresGetMigrate()
	default:
		panic(fmt.Sprintf("unknown dbType: %s", db.DBType))
	}
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
	db.lockNoCtx()
	defer db.unlock()

	switch db.DBType {
	case SQLiteDB:
		return db.sqliteVacuum(ctx)
	default:
		return &UnsupportedForDBTypeError{
			Operation: "Vacuuming",
			DBType:    db.DBType,
		}
	}
}

func (db *Database) runMigrations() error {
	switch db.DBType {
	case SQLiteDB:
		return db.sqliteRunMigrations()
	case PostgresDB:
		return db.postgresRunMigrations()
	default:
		panic(fmt.Sprintf("unknown dbType: %s", db.DBType))
	}
}
