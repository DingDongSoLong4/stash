package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/stashapp/stash/internal/dlna"
	"github.com/stashapp/stash/internal/log"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
	"github.com/stashapp/stash/pkg/scraper"
	"github.com/stashapp/stash/pkg/session"
	"github.com/stashapp/stash/pkg/sqlite"

	// register custom migrations
	_ "github.com/stashapp/stash/pkg/sqlite/migrations"
)

// The fields of this struct are read-only after initialization,
// i.e. the values the pointers point to may change but
// the pointers themselves will not.
type Manager struct {
	Config *config.Config
	Logger *log.Logger

	Paths *paths.Paths

	FFMpeg  *ffmpeg.FFMpeg
	FFProbe *ffmpeg.FFProbe

	JobManager      *job.Manager
	ReadLockManager *fsutil.ReadLockManager

	SessionStore  *session.Store
	DownloadStore *DownloadStore

	PluginCache  *plugin.Cache
	ScraperCache *scraper.Cache

	StreamManager *ffmpeg.StreamManager
	DLNAService   *dlna.Service

	Database   *sqlite.Database
	Repository models.Repository

	SceneService   SceneService
	ImageService   ImageService
	GalleryService GalleryService

	Scanner *file.Scanner
	Cleaner *file.Cleaner

	scanSubs *subscriptionManager
}

var instance *Manager

func GetInstance() *Manager {
	if instance == nil {
		panic("manager not initialized")
	}
	return instance
}

func (s *Manager) SetBlobStoreOptions() {
	storageType := s.Config.GetBlobsStorage()
	blobsPath := s.Config.GetBlobsPath()

	s.Database.SetBlobStoreOptions(sqlite.BlobStoreOptions{
		UseFilesystem: storageType == models.BlobStorageTypeFilesystem,
		UseDatabase:   storageType == models.BlobStorageTypeDatabase,
		Path:          blobsPath,
	})
}

func (s *Manager) RefreshConfig() {
	*s.Paths = paths.NewPaths(s.Config.GetGeneratedPath(), s.Config.GetBlobsPath())
	config := s.Config
	if config.Validate() == nil {
		if err := fsutil.EnsureDir(s.Paths.Generated.Screenshots); err != nil {
			logger.Warnf("could not create directory for Screenshots: %v", err)
		}
		if err := fsutil.EnsureDir(s.Paths.Generated.Vtt); err != nil {
			logger.Warnf("could not create directory for VTT: %v", err)
		}
		if err := fsutil.EnsureDir(s.Paths.Generated.Markers); err != nil {
			logger.Warnf("could not create directory for Markers: %v", err)
		}
		if err := fsutil.EnsureDir(s.Paths.Generated.Transcodes); err != nil {
			logger.Warnf("could not create directory for Transcodes: %v", err)
		}
		if err := fsutil.EnsureDir(s.Paths.Generated.Downloads); err != nil {
			logger.Warnf("could not create directory for Downloads: %v", err)
		}
		if err := fsutil.EnsureDir(s.Paths.Generated.InteractiveHeatmap); err != nil {
			logger.Warnf("could not create directory for Interactive Heatmaps: %v", err)
		}
	}
}

// RefreshSessionStore refreshes the session store configuration.
// Call this when the max session age changes.
func (s *Manager) RefreshSessionStore() {
	s.SessionStore.Configure(s.Config.GetMaxSessionAge())
}

// RefreshPluginCache refreshes the plugin cache.
// Call this when the plugin configuration changes.
func (s *Manager) RefreshPluginCache() {
	s.PluginCache.ReloadPlugins()
}

// RefreshScraperCache refreshes the scraper cache.
// Call this when the scraper configuration changes.
func (s *Manager) RefreshScraperCache() {
	s.ScraperCache.ReloadScrapers()
}

// RefreshStreamManager refreshes the stream manager.
// Call this when the cache directory changes.
func (s *Manager) RefreshStreamManager() {
	s.StreamManager.Configure(s.Config.GetCachePath())
}

// RefreshDLNA starts/stops the DLNA service as needed.
func (s *Manager) RefreshDLNA() {
	dlnaService := s.DLNAService
	enabled := s.Config.GetDLNADefaultEnabled()
	if !enabled && dlnaService.IsRunning() {
		dlnaService.Stop(nil)
	} else if enabled && !dlnaService.IsRunning() {
		if err := dlnaService.Start(nil); err != nil {
			logger.Warnf("error starting DLNA service: %v", err)
		}
	}
}

func setSetupDefaults(input *models.SetupInput) {
	if input.ConfigLocation == "" {
		input.ConfigLocation = filepath.Join(fsutil.GetHomeDirectory(), ".stash", "config.yml")
	}

	configDir := filepath.Dir(input.ConfigLocation)
	if input.GeneratedLocation == "" {
		input.GeneratedLocation = filepath.Join(configDir, "generated")
	}
	if input.CacheLocation == "" {
		input.CacheLocation = filepath.Join(configDir, "cache")
	}

	if input.DatabaseFile == "" {
		input.DatabaseFile = filepath.Join(configDir, "stash-go.sqlite")
	}
}

func (s *Manager) Setup(ctx context.Context, input models.SetupInput) error {
	setSetupDefaults(&input)
	c := s.Config

	// create the config directory if it does not exist
	// don't do anything if config is already set in the environment
	if !config.FileEnvSet() {
		// #3304 - if config path is relative, it breaks the ffmpeg/ffprobe
		// paths since they must not be relative. The config file property is
		// resolved to an absolute path when stash is run normally, so convert
		// relative paths to absolute paths during setup.
		configFile, _ := filepath.Abs(input.ConfigLocation)

		configDir := filepath.Dir(configFile)

		if exists, _ := fsutil.DirExists(configDir); !exists {
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("error creating config directory: %v", err)
			}
		}

		if err := fsutil.Touch(configFile); err != nil {
			return fmt.Errorf("error creating config file: %v", err)
		}

		s.Config.SetConfigFile(configFile)
	}

	// create the generated directory if it does not exist
	if !c.HasOverride(config.Generated) {
		if exists, _ := fsutil.DirExists(input.GeneratedLocation); !exists {
			if err := os.MkdirAll(input.GeneratedLocation, 0755); err != nil {
				return fmt.Errorf("error creating generated directory: %v", err)
			}
		}

		s.Config.Set(config.Generated, input.GeneratedLocation)
	}

	// create the cache directory if it does not exist
	if !c.HasOverride(config.Cache) {
		if exists, _ := fsutil.DirExists(input.CacheLocation); !exists {
			if err := os.MkdirAll(input.CacheLocation, 0755); err != nil {
				return fmt.Errorf("error creating cache directory: %v", err)
			}
		}

		s.Config.Set(config.Cache, input.CacheLocation)
	}

	// if blobs path was provided then use filesystem based blob storage
	if input.BlobsLocation != "" {
		if !c.HasOverride(config.BlobsPath) {
			if exists, _ := fsutil.DirExists(input.BlobsLocation); !exists {
				if err := os.MkdirAll(input.BlobsLocation, 0755); err != nil {
					return fmt.Errorf("error creating blobs directory: %v", err)
				}
			}
		}

		s.Config.Set(config.BlobsPath, input.BlobsLocation)
		s.Config.Set(config.BlobsStorage, models.BlobStorageTypeFilesystem)
	} else {
		s.Config.Set(config.BlobsStorage, models.BlobStorageTypeDatabase)
	}

	// set the configuration
	if !c.HasOverride(config.Database) {
		s.Config.Set(config.Database, input.DatabaseFile)
	}

	s.Config.Set(config.Stash, input.Stashes)

	if err := s.Config.SetInitialConfig(); err != nil {
		return fmt.Errorf("error setting initial configuration: %v", err)
	}

	if err := s.Config.Write(); err != nil {
		return fmt.Errorf("error writing configuration file: %v", err)
	}

	// finish initialization
	if err := s.postInit(ctx); err != nil {
		return fmt.Errorf("error completing initialization: %v", err)
	}

	s.Config.FinalizeSetup()

	return nil
}

func (s *Manager) Migrate(ctx context.Context, input models.MigrateInput) error {
	database := s.Database

	// always backup so that we can roll back to the previous version if
	// migration fails
	backupPath := input.BackupPath
	if backupPath == "" {
		backupPath = database.DatabaseBackupPath(s.Config.GetBackupDirectoryPath())
	} else {
		// check if backup path is a filename or path
		// filename goes into backup directory, path is kept as is
		filename := filepath.Base(backupPath)
		if backupPath == filename {
			backupPath = filepath.Join(s.Config.GetBackupDirectoryPathOrDefault(), filename)
		}
	}

	// perform database backup
	if err := database.Backup(backupPath); err != nil {
		return fmt.Errorf("error backing up database: %s", err)
	}

	if err := database.Migrate(); err != nil {
		errStr := fmt.Sprintf("error performing migration: %s", err)

		// roll back to the backed up version
		restoreErr := database.RestoreFromBackup(backupPath)
		if restoreErr != nil {
			errStr = fmt.Sprintf("ERROR: unable to restore database from backup after migration failure: %s\n%s", restoreErr.Error(), errStr)
		} else {
			errStr = "An error occurred migrating the database to the latest schema version. The backup database file was automatically renamed to restore the database.\n" + errStr
		}

		return errors.New(errStr)
	}

	// reopen after migration
	if err := database.Open(); err != nil {
		return fmt.Errorf("error reopening database: %s", err)
	}

	// if no backup path was provided, then delete the created backup
	if input.BackupPath == "" {
		if err := os.Remove(backupPath); err != nil {
			logger.Warnf("error removing unwanted database backup (%s): %s", backupPath, err.Error())
		}
	}

	return nil
}

func (s *Manager) BackupDatabase(download bool) (string, string, error) {
	var backupPath string
	var backupName string
	if download {
		backupDir := s.Paths.Generated.Downloads
		if err := fsutil.EnsureDir(backupDir); err != nil {
			return "", "", fmt.Errorf("could not create backup directory %v: %w", backupDir, err)
		}
		f, err := os.CreateTemp(backupDir, "backup*.sqlite")
		if err != nil {
			return "", "", err
		}

		backupPath = f.Name()
		backupName = s.Database.DatabaseBackupPath("")
		f.Close()
	} else {
		backupDir := s.Config.GetBackupDirectoryPathOrDefault()
		if backupDir != "" {
			if err := fsutil.EnsureDir(backupDir); err != nil {
				return "", "", fmt.Errorf("could not create backup directory %v: %w", backupDir, err)
			}
		}
		backupPath = s.Database.DatabaseBackupPath(backupDir)
		backupName = filepath.Base(backupPath)
	}

	err := s.Database.Backup(backupPath)
	if err != nil {
		return "", "", err
	}

	return backupPath, backupName, nil
}

func (s *Manager) AnonymiseDatabase(download bool) (string, string, error) {
	var outPath string
	var outName string
	if download {
		outDir := s.Paths.Generated.Downloads
		if err := fsutil.EnsureDir(outDir); err != nil {
			return "", "", fmt.Errorf("could not create output directory %v: %w", outDir, err)
		}
		f, err := os.CreateTemp(outDir, "anonymous*.sqlite")
		if err != nil {
			return "", "", err
		}

		outPath = f.Name()
		outName = s.Database.DatabaseBackupPath("")
		f.Close()
	} else {
		outDir := s.Config.GetBackupDirectoryPathOrDefault()
		if outDir != "" {
			if err := fsutil.EnsureDir(outDir); err != nil {
				return "", "", fmt.Errorf("could not create output directory %v: %w", outDir, err)
			}
		}
		outPath = s.Database.AnonymousDatabasePath(outDir)
		outName = filepath.Base(outPath)
	}

	err := s.Database.Anonymise(outPath)
	if err != nil {
		return "", "", err
	}

	return outPath, outName, nil
}

func (s *Manager) GetSystemStatus() *models.SystemStatus {
	database := s.Database
	status := models.SystemStatusEnumOk
	dbSchema := int(database.Version())
	dbPath := database.DatabasePath()
	appSchema := int(database.AppSchemaVersion())
	configFile := s.Config.GetConfigFile()

	if s.Config.IsNewSystem() {
		status = models.SystemStatusEnumSetup
	} else if dbSchema < appSchema {
		status = models.SystemStatusEnumNeedsMigration
	}

	return &models.SystemStatus{
		DatabaseSchema: &dbSchema,
		DatabasePath:   &dbPath,
		AppSchema:      appSchema,
		Status:         status,
		ConfigPath:     &configFile,
	}
}

// Shutdown gracefully stops the manager
func (s *Manager) Shutdown() {
	// stop any profiling at exit
	pprof.StopCPUProfile()

	// TODO: Each part of the manager needs to gracefully stop at some point

	s.StreamManager.Shutdown()

	err := s.Database.Close()
	if err != nil {
		logger.Errorf("Error closing database: %s", err)
	}
}
