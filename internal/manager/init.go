package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/desktop"
	"github.com/stashapp/stash/internal/dlna"
	"github.com/stashapp/stash/internal/log"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/file"
	file_image "github.com/stashapp/stash/pkg/file/image"
	"github.com/stashapp/stash/pkg/file/video"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/gallery"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/scraper"
	"github.com/stashapp/stash/pkg/session"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/utils"
	"github.com/stashapp/stash/ui"
)

// Only called once at startup
func Initialize() (*Manager, error) {
	ctx := context.TODO()

	cfg, err := config.Initialize()
	if err != nil {
		return nil, fmt.Errorf("initializing configuration: %w", err)
	}

	l := initLog(cfg)
	initProfiling(cfg.GetCPUProfilePath())

	db := sqlite.NewDatabase()
	repo := db.Repository()

	// start with empty paths
	mgrPaths := &paths.Paths{}

	ffMpeg := &ffmpeg.FFMpeg{}
	ffProbe := &ffmpeg.FFProbe{}

	sessionStore := session.NewStore(cfg)

	readLockMgr := fsutil.NewReadLockManager()
	pluginCache := plugin.NewCache(cfg, sessionStore)

	scraperRepository := scraper.NewRepository(repo)
	scraperCache := scraper.NewCache(cfg, scraperRepository)

	streamManager := ffmpeg.NewStreamManager(ffMpeg, ffProbe, cfg, readLockMgr)

	sceneService := &scene.Service{
		File:             db.File,
		Repository:       db.Scene,
		MarkerRepository: db.SceneMarker,
		PluginCache:      pluginCache,
		Paths:            mgrPaths,
		Config:           cfg,
	}

	imageService := &image.Service{
		File:       db.File,
		Repository: db.Image,
	}

	galleryService := &gallery.Service{
		Repository:   db.Gallery,
		ImageFinder:  db.Image,
		ImageService: imageService,
		File:         db.File,
		Folder:       db.Folder,
	}

	sceneServer := &SceneServer{
		TxnManager:       repo,
		SceneCoverGetter: repo.Scene,
	}

	dlnaRepository := dlna.NewRepository(repo)
	dlnaService := dlna.NewService(dlnaRepository, cfg, sceneServer)

	scanner := &file.Scanner{
		Repository: file.NewRepository(repo),
		FileDecorators: []file.Decorator{
			&file.FilteredDecorator{
				Decorator: &video.Decorator{
					FFProbe: ffProbe,
				},
				Filter: file.FilterFunc(videoFileFilter),
			},
			&file.FilteredDecorator{
				Decorator: &file_image.Decorator{
					FFProbe: ffProbe,
				},
				Filter: file.FilterFunc(imageFileFilter),
			},
		},
		FingerprintCalculator: &fingerprintCalculator{cfg},
		FS:                    &file.OsFS{},
	}

	cleaner := &file.Cleaner{
		FS:         &file.OsFS{},
		Repository: file.NewRepository(repo),
		Handlers: []file.CleanHandler{
			&cleanHandler{},
		},
	}

	mgr := &Manager{
		Config: cfg,
		Logger: l,

		Paths: mgrPaths,

		FFMpeg:  ffMpeg,
		FFProbe: ffProbe,

		JobManager:      initJobManager(cfg),
		ReadLockManager: readLockMgr,

		DownloadStore: NewDownloadStore(),
		SessionStore:  sessionStore,

		PluginCache:  pluginCache,
		ScraperCache: scraperCache,

		StreamManager: streamManager,
		DLNAService:   dlnaService,

		Database:   db,
		Repository: repo,

		SceneService:   sceneService,
		ImageService:   imageService,
		GalleryService: galleryService,

		Scanner: scanner,
		Cleaner: cleaner,

		scanSubs: &subscriptionManager{},
	}

	if !cfg.IsNewSystem() {
		logger.Infof("using config file: %s", cfg.GetConfigFile())

		err := cfg.Validate()
		if err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}

		if err := mgr.postInit(ctx); err != nil {
			return nil, err
		}

		mgr.checkSecurityTripwire()
	} else {
		cfgFile := cfg.GetConfigFile()
		if cfgFile != "" {
			cfgFile += " "
		}

		logger.Warnf("config file %snot found. Assuming new system...", cfgFile)
	}

	instance = mgr
	return mgr, nil
}

func initLog(cfg *config.Instance) *log.Logger {
	l := log.NewLogger()
	l.Init(cfg.GetLogFile(), cfg.GetLogOut(), cfg.GetLogLevel())
	logger.Logger = l

	return l
}

func initProfiling(cpuProfilePath string) {
	if cpuProfilePath == "" {
		return
	}

	f, err := os.Create(cpuProfilePath)
	if err != nil {
		logger.Fatalf("unable to create cpu profile file: %s", err.Error())
	}

	logger.Infof("profiling to %s", cpuProfilePath)

	// StopCPUProfile is called on manager shutdown
	if err = pprof.StartCPUProfile(f); err != nil {
		logger.Warnf("could not start CPU profiling: %v", err)
	}
}

func formatDuration(t time.Duration) string {
	return fmt.Sprintf("%02.f:%02.f:%02.f", t.Hours(), t.Minutes(), t.Seconds())
}

func initJobManager(cfg *config.Instance) *job.Manager {
	ret := job.NewManager()

	// desktop notifications
	ctx := context.Background()
	c := ret.Subscribe(context.Background())
	go func() {
		for {
			select {
			case j := <-c.RemovedJob:
				if cfg.GetNotificationsEnabled() {
					cleanDesc := strings.TrimRight(j.Description, ".")

					if j.StartTime == nil {
						// Task was never started
						return
					}

					timeElapsed := j.EndTime.Sub(*j.StartTime)
					desktop.SendNotification("Task Finished", "Task \""+cleanDesc+"\" is finished in "+formatDuration(timeElapsed)+".")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return ret
}

func (s *Manager) checkSecurityTripwire() {
	if err := session.CheckExternalAccessTripwire(s.Config); err != nil {
		session.LogExternalAccessError(*err)
	}
}

// postInit initialises the paths, caches and database after the initial
// configuration has been set. Should only be called if the configuration
// is valid.
func (s *Manager) postInit(ctx context.Context) error {
	s.RefreshConfig()
	s.SessionStore.Reset()

	s.RefreshPluginCache()
	s.RefreshScraperCache()
	s.RefreshStreamManager()
	s.RefreshDLNA()

	s.SetBlobStoreOptions()

	s.writeStashIcon()

	// clear the downloads and tmp directories
	// #1021 - only clear these directories if the generated folder is non-empty
	if s.Config.GetGeneratedPath() != "" {
		const deleteTimeout = 1 * time.Second

		utils.Timeout(func() {
			if err := fsutil.EmptyDir(s.Paths.Generated.Downloads); err != nil {
				logger.Warnf("could not empty Downloads directory: %v", err)
			}
			if err := fsutil.EnsureDir(s.Paths.Generated.Tmp); err != nil {
				logger.Warnf("could not create Tmp directory: %v", err)
			} else {
				if err := fsutil.EmptyDir(s.Paths.Generated.Tmp); err != nil {
					logger.Warnf("could not empty Tmp directory: %v", err)
				}
			}
		}, deleteTimeout, func(done chan struct{}) {
			logger.Info("Please wait. Deleting temporary files...") // print
			<-done                                                  // and wait for deletion
			logger.Info("Temporary files deleted.")
		})
	}

	if err := s.Database.Open(s.Config.GetDatabasePath()); err != nil {
		var migrationNeededErr *sqlite.MigrationNeededError
		if errors.As(err, &migrationNeededErr) {
			logger.Warn(err)
		} else {
			return err
		}
	}

	// Set the proxy if defined in config
	if s.Config.GetProxy() != "" {
		os.Setenv("HTTP_PROXY", s.Config.GetProxy())
		os.Setenv("HTTPS_PROXY", s.Config.GetProxy())
		os.Setenv("NO_PROXY", s.Config.GetNoProxy())
		logger.Info("Using HTTP Proxy")
	}

	if err := s.initFFMpeg(ctx); err != nil {
		return fmt.Errorf("error initializing FFMpeg subsystem: %v", err)
	}

	return nil
}

func (s *Manager) writeStashIcon() {
	iconPath := filepath.Join(s.Config.GetConfigPath(), "icon.png")
	err := os.WriteFile(iconPath, ui.FaviconProvider.GetFaviconPng(), 0644)
	if err != nil {
		logger.Errorf("Couldn't write icon file: %s", err.Error())
	}
}

func (s *Manager) initFFMpeg(ctx context.Context) error {
	// use same directory as config path
	configDirectory := s.Config.GetConfigPath()
	paths := []string{
		configDirectory,
		paths.GetStashHomeDirectory(),
	}
	ffmpegPath, ffprobePath := ffmpeg.GetPaths(paths)

	if ffmpegPath == "" || ffprobePath == "" {
		logger.Infof("couldn't find FFMpeg, attempting to download it")
		if err := ffmpeg.Download(ctx, configDirectory); err != nil {
			msg := `Unable to locate / automatically download FFMpeg

Check the readme for download links.
The FFMpeg and FFProbe binaries should be placed in %s

The error was: %s
`
			logger.Errorf(msg, configDirectory, err)
			return err
		} else {
			// After download get new paths for ffmpeg and ffprobe
			ffmpegPath, ffprobePath = ffmpeg.GetPaths(paths)
		}
	}

	s.FFMpeg.Configure(ctx, ffmpegPath)
	s.FFProbe.Configure(ffprobePath)

	return nil
}
