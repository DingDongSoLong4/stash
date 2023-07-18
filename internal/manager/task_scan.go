package manager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"time"

	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/stashapp/stash/pkg/file"
	file_image "github.com/stashapp/stash/pkg/file/image"
	"github.com/stashapp/stash/pkg/file/video"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/gallery"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/scene"
)

type ScanJob struct {
	Manager *Manager
	Input   models.ScanMetadataInput
}

func (j *ScanJob) Execute(ctx context.Context, progress *job.Progress) {
	if job.IsCancelled(ctx) {
		logger.Info("Stopping due to user request")
		return
	}

	start := time.Now()

	mgr := j.Manager
	cfg := mgr.Config
	repo := mgr.Repository

	stashPaths := cfg.GetStashPaths()
	processes := cfg.GetParallelTasksWithAutoDetection()
	videoFileNamingAlgorithm := cfg.GetVideoFileNamingAlgorithm()
	sequentialScanning := cfg.GetSequentialScanning()
	createGalleriesFromFolders := cfg.GetCreateGalleriesFromFolders()

	paths := mgr.Paths
	pluginCache := mgr.PluginCache
	extensionMatcher := &file.ExtensionMatcher{
		CreateImageClipsFromVideos: cfg.IsCreateImageClipsFromVideos(),
		StashPaths:                 stashPaths,
	}

	const taskQueueSize = 200000
	taskQueue := job.NewTaskQueue(ctx, progress, taskQueueSize, processes)

	var minModTime time.Time
	if j.Input.Filter != nil && j.Input.Filter.MinModTime != nil {
		minModTime = *j.Input.Filter.MinModTime
	}

	scanner := &file.Scanner{
		Paths: getScanPaths(stashPaths, j.Input.Paths),
		FileDecorators: []file.Decorator{
			&file.FilteredDecorator{
				Decorator: &video.Decorator{
					FFProbe: mgr.FFProbe,
				},
				Filter: extensionMatcher.VideoFilterFunc(),
			},
			&file.FilteredDecorator{
				Decorator: &file_image.Decorator{
					FFProbe: mgr.FFProbe,
				},
				Filter: extensionMatcher.ImageFilterFunc(),
			},
		},
		Handlers: []file.Handler{
			&file.FilteredHandler{
				Filter: extensionMatcher.ImageFilterFunc(),
				Handler: &image.ScanHandler{
					CreatorUpdater: repo.Image,
					GalleryFinder:  repo.Gallery,
					PluginCache:    pluginCache,
					ScanGenerator: &imageGenerator{
						taskQueue:           taskQueue,
						progress:            progress,
						manager:             mgr,
						input:               j.Input,
						paths:               paths,
						sequentialScanning:  sequentialScanning,
						transcodeInputArgs:  cfg.GetTranscodeInputArgs(),
						transcodeOutputArgs: cfg.GetTranscodeOutputArgs(),
						previewPreset:       cfg.GetPreviewPreset(),
					},
					CreateGalleriesFromFolders: createGalleriesFromFolders,
					Paths:                      paths,
				},
			},
			&file.FilteredHandler{
				Filter: extensionMatcher.GalleryFilterFunc(),
				Handler: &gallery.ScanHandler{
					CreatorUpdater:     repo.Gallery,
					SceneFinderUpdater: repo.Scene,
					ImageFinderUpdater: repo.Image,
					PluginCache:        pluginCache,
				},
			},
			&file.FilteredHandler{
				Filter: extensionMatcher.VideoFilterFunc(),
				Handler: &scene.ScanHandler{
					CreatorUpdater: repo.Scene,
					CaptionUpdater: repo.File,
					PluginCache:    pluginCache,
					ScanGenerator: &sceneGenerators{
						taskQueue:           taskQueue,
						progress:            progress,
						manager:             mgr,
						repository:          repo,
						input:               j.Input,
						paths:               paths,
						fileNamingAlgorithm: videoFileNamingAlgorithm,
						sequentialScanning:  sequentialScanning,
					},
					FileNamingAlgorithm: videoFileNamingAlgorithm,
					Paths:               paths,
				},
			},
		},
		ZipFileExtensions: cfg.GetGalleryExtensions(),
		ScanFilters: []file.PathFilter{
			&scanFilter{
				repository:        repo,
				stashPaths:        stashPaths,
				generatedPath:     cfg.GetGeneratedPath(),
				videoExcludeRegex: generateRegexps(cfg.GetExcludes()),
				imageExcludeRegex: generateRegexps(cfg.GetImageExcludes()),
				minModTime:        minModTime,
				extensionMatcher:  extensionMatcher,
			},
		},
		HandlerRequiredFilters: []file.Filter{
			&handlerRequiredFilter{
				repository:                 repo,
				folderCache:                lru.New(processes * 2),
				videoFileNamingAlgorithm:   videoFileNamingAlgorithm,
				createGalleriesFromFolders: createGalleriesFromFolders,
				extensionMatcher:           extensionMatcher,
			},
		},
		Repository: file.NewRepository(repo),
		FingerprintCalculator: &fingerprintCalculator{
			Config:           cfg,
			ExtensionMatcher: extensionMatcher,
		},
		FS:            &file.OsFS{},
		ParallelTasks: processes,
	}

	scanner.Execute(ctx, progress)

	taskQueue.Close()

	if job.IsCancelled(ctx) {
		logger.Info("Stopping due to user request")
		return
	}

	elapsed := time.Since(start)
	logger.Info(fmt.Sprintf("Scan finished (%s)", elapsed))

	mgr.scanSubs.notify()
}

func getScanPaths(stashPaths models.StashConfigs, inputPaths []string) []string {
	var ret []string

	for _, p := range inputPaths {
		s := stashPaths.GetStashFromDirPath(p)
		if s == nil {
			logger.Warnf("%s is not in the configured stash paths", p)
			continue
		}

		ret = append(ret, s.Path)
	}

	return ret
}

type fileCounter interface {
	CountByFileID(ctx context.Context, fileID models.FileID) (int, error)
}

// handlerRequiredFilter returns true if a File's handler needs to be executed despite the file not being updated.
type handlerRequiredFilter struct {
	repository models.Repository

	folderCache *lru.LRU

	videoFileNamingAlgorithm   models.HashAlgorithm
	createGalleriesFromFolders bool

	extensionMatcher *file.ExtensionMatcher
}

func (f *handlerRequiredFilter) Accept(ctx context.Context, ff models.File) bool {
	path := ff.Base().Path
	isVideoFile := f.extensionMatcher.UseAsVideo(path)
	isImageFile := f.extensionMatcher.UseAsImage(path)
	isZipFile := f.extensionMatcher.IsZip(path)

	var counter fileCounter

	switch {
	case isVideoFile:
		// return true if there are no scenes associated
		counter = f.repository.Scene
	case isImageFile:
		counter = f.repository.Image
	case isZipFile:
		counter = f.repository.Gallery
	}

	if counter == nil {
		return false
	}

	n, err := counter.CountByFileID(ctx, ff.Base().ID)
	if err != nil {
		// just ignore
		return false
	}

	// execute handler if there are no related objects
	if n == 0 {
		return true
	}

	// if create galleries from folder is enabled and the file is not in a zip
	// file, then check if there is a folder-based gallery for the file's
	// directory
	if isImageFile && f.createGalleriesFromFolders && ff.Base().ZipFileID == nil {
		// only do this for the first time it encounters the folder
		// the first instance should create the gallery
		_, found := f.folderCache.Get(ctx, ff.Base().ParentFolderID.String())
		if found {
			// should already be handled
			return false
		}

		g, _ := f.repository.Gallery.FindByFolderID(ctx, ff.Base().ParentFolderID)
		f.folderCache.Add(ctx, ff.Base().ParentFolderID.String(), true)

		if len(g) == 0 {
			// no folder gallery. Return true so that it creates one.
			return true
		}
	}

	if isVideoFile {
		// TODO - check if the cover exists
		// hash := scene.GetHash(ff, f.videoFileNamingAlgorithm)
		// ssPath := instance.Paths.Scene.GetScreenshotPath(hash)
		// if exists, _ := fsutil.FileExists(ssPath); !exists {
		// 	// if not, check if the file is a primary file for a scene
		// 	scenes, err := f.SceneFinder.FindByPrimaryFileID(ctx, ff.Base().ID)
		// 	if err != nil {
		// 		// just ignore
		// 		return false
		// 	}

		// 	if len(scenes) > 0 {
		// 		// if it is, then it needs to be re-generated
		// 		return true
		// 	}
		// }

		// clean captions - scene handler handles this as well, but
		// unchanged files aren't processed by the scene handler
		videoFile, _ := ff.(*models.VideoFile)
		if videoFile != nil {
			if err := video.CleanCaptions(ctx, videoFile, f.repository, f.repository.File); err != nil {
				logger.Errorf("Error cleaning captions: %v", err)
			}
		}
	}

	return false
}

type scanFilter struct {
	repository        models.Repository
	stashPaths        models.StashConfigs
	generatedPath     string
	videoExcludeRegex []*regexp.Regexp
	imageExcludeRegex []*regexp.Regexp
	minModTime        time.Time
	extensionMatcher  *file.ExtensionMatcher
}

func (f *scanFilter) Accept(ctx context.Context, path string, info fs.FileInfo) bool {
	if fsutil.IsPathInDir(f.generatedPath, path) {
		logger.Warnf("Skipping %q as it overlaps with the generated folder", path)
		return false
	}

	// exit early on cutoff
	if info.Mode().IsRegular() && info.ModTime().Before(f.minModTime) {
		return false
	}

	isVideoFile := f.extensionMatcher.UseAsVideo(path)
	isImageFile := f.extensionMatcher.UseAsImage(path)
	isZipFile := f.extensionMatcher.IsZip(path)

	// handle caption files
	if fsutil.MatchExtension(path, video.CaptionExts) {
		// we don't include caption files in the file scan, but we do need
		// to handle them
		video.AssociateCaptions(ctx, path, f.repository, f.repository.File)

		return false
	}

	if !info.IsDir() && !isVideoFile && !isImageFile && !isZipFile {
		logger.Debugf("Skipping %s as it does not match any known file extensions", path)
		return false
	}

	// #1756 - skip zero length files
	if !info.IsDir() && info.Size() == 0 {
		logger.Infof("Skipping zero-length file: %s", path)
		return false
	}

	s := f.stashPaths.GetStashFromDirPath(path)

	if s == nil {
		logger.Debugf("Skipping %s as it is not in the stash library", path)
		return false
	}

	// shortcut: skip the directory entirely if it matches both exclusion patterns
	// add a trailing separator so that it correctly matches against patterns like path/.*
	pathExcludeTest := path + string(filepath.Separator)
	if (matchFileRegex(pathExcludeTest, f.videoExcludeRegex)) && (s.ExcludeImage || matchFileRegex(pathExcludeTest, f.imageExcludeRegex)) {
		logger.Debugf("Skipping directory %s as it matches video and image exclusion patterns", path)
		return false
	}

	if isVideoFile && (s.ExcludeVideo || matchFileRegex(path, f.videoExcludeRegex)) {
		logger.Debugf("Skipping %s as it matches video exclusion patterns", path)
		return false
	} else if (isImageFile || isZipFile) && (s.ExcludeImage || matchFileRegex(path, f.imageExcludeRegex)) {
		logger.Debugf("Skipping %s as it matches image exclusion patterns", path)
		return false
	}

	return true
}

type imageGenerator struct {
	taskQueue *job.TaskQueue
	progress  *job.Progress

	manager             *Manager
	input               models.ScanMetadataInput
	paths               *paths.Paths
	sequentialScanning  bool
	transcodeInputArgs  []string
	transcodeOutputArgs []string
	previewPreset       models.PreviewPreset
}

func (g *imageGenerator) Generate(ctx context.Context, i *models.Image, f models.File) error {
	const overwrite = false

	progress := g.progress
	t := g.input
	path := f.Base().Path

	if t.ScanGenerateThumbnails {
		// this should be quick, so always generate sequentially
		if err := g.generateThumbnail(ctx, i, f); err != nil {
			logger.Errorf("Error generating thumbnail for %s: %v", path, err)
		}
	}

	// avoid adding a task if the file isn't a video file
	_, isVideo := f.(*models.VideoFile)
	if isVideo && t.ScanGenerateClipPreviews {
		// this is a bit of a hack: the task requires files to be loaded, but
		// we don't really need to since we already have the file
		ii := *i
		ii.Files = models.NewRelatedFiles([]models.File{f})

		progress.AddTotal(1)
		previewsFn := func(ctx context.Context) {
			taskPreview := GenerateClipPreviewTask{
				Image:     ii,
				Overwrite: overwrite,
			}

			taskPreview.Start(ctx)
			progress.Increment()
		}

		if g.sequentialScanning {
			previewsFn(ctx)
		} else {
			g.taskQueue.Add(fmt.Sprintf("Generating preview for %s", path), previewsFn)
		}
	}

	return nil
}

func (g *imageGenerator) generateThumbnail(ctx context.Context, i *models.Image, f models.File) error {
	thumbPath := g.paths.Generated.GetThumbnailPath(i.Checksum, models.DefaultGthumbWidth)
	exists, _ := fsutil.FileExists(thumbPath)
	if exists {
		return nil
	}

	path := f.Base().Path

	asFrame, ok := f.(models.VisualFile)
	if !ok {
		return fmt.Errorf("file %s does not implement Frame", path)
	}

	if asFrame.GetHeight() <= models.DefaultGthumbWidth && asFrame.GetWidth() <= models.DefaultGthumbWidth {
		return nil
	}

	logger.Debugf("Generating thumbnail for %s", path)

	clipPreviewOptions := image.ClipPreviewOptions{
		InputArgs:  g.transcodeInputArgs,
		OutputArgs: g.transcodeOutputArgs,
		Preset:     g.previewPreset.String(),
	}

	encoder := image.NewThumbnailEncoder(g.manager.FFMpeg, g.manager.FFProbe, clipPreviewOptions)
	data, err := encoder.GetThumbnail(f, models.DefaultGthumbWidth)

	if err != nil {
		// don't log for animated images
		if !errors.Is(err, image.ErrNotSupportedForThumbnail) {
			return fmt.Errorf("getting thumbnail for image %s: %w", path, err)
		}
		return nil
	}

	err = fsutil.WriteFile(thumbPath, data)
	if err != nil {
		return fmt.Errorf("writing thumbnail for image %s: %w", path, err)
	}

	return nil
}

type sceneGenerators struct {
	taskQueue *job.TaskQueue
	progress  *job.Progress

	manager             *Manager
	repository          models.Repository
	input               models.ScanMetadataInput
	paths               *paths.Paths
	fileNamingAlgorithm models.HashAlgorithm
	sequentialScanning  bool
}

func (g *sceneGenerators) Generate(ctx context.Context, s *models.Scene, f *models.VideoFile) error {
	const overwrite = false

	progress := g.progress
	t := g.input
	path := f.Path

	generator := g.manager.NewGenerator(overwrite)

	if t.ScanGenerateSprites {
		progress.AddTotal(1)
		spriteFn := func(ctx context.Context) {
			taskSprite := GenerateSpriteTask{
				Scene:               *s,
				Overwrite:           overwrite,
				FileNamingAlgorithm: g.fileNamingAlgorithm,
				Paths:               g.paths,
				generator:           generator,
			}
			taskSprite.Start(ctx)
			progress.Increment()
		}

		if g.sequentialScanning {
			spriteFn(ctx)
		} else {
			g.taskQueue.Add(fmt.Sprintf("Generating sprites for %s", path), spriteFn)
		}
	}

	if t.ScanGeneratePhashes {
		progress.AddTotal(1)
		phashFn := func(ctx context.Context) {
			taskPhash := GeneratePhashTask{
				repository:          g.repository,
				File:                f,
				fileNamingAlgorithm: g.fileNamingAlgorithm,
				Overwrite:           overwrite,
			}
			taskPhash.Start(ctx)
			progress.Increment()
		}

		if g.sequentialScanning {
			phashFn(ctx)
		} else {
			g.taskQueue.Add(fmt.Sprintf("Generating phash for %s", path), phashFn)
		}
	}

	if t.ScanGeneratePreviews {
		progress.AddTotal(1)
		previewsFn := func(ctx context.Context) {
			options := getGeneratePreviewOptions(models.GeneratePreviewOptionsInput{})

			taskPreview := GeneratePreviewTask{
				Scene:               *s,
				ImagePreview:        t.ScanGenerateImagePreviews,
				Options:             options,
				Overwrite:           overwrite,
				fileNamingAlgorithm: g.fileNamingAlgorithm,
				generator:           generator,
			}
			taskPreview.Start(ctx)
			progress.Increment()
		}

		if g.sequentialScanning {
			previewsFn(ctx)
		} else {
			g.taskQueue.Add(fmt.Sprintf("Generating preview for %s", path), previewsFn)
		}
	}

	if t.ScanGenerateCovers {
		progress.AddTotal(1)
		g.taskQueue.Add(fmt.Sprintf("Generating cover for %s", path), func(ctx context.Context) {
			taskCover := GenerateCoverTask{
				repository: g.repository,
				Scene:      *s,
				Overwrite:  overwrite,
			}
			taskCover.Start(ctx)
			progress.Increment()
		})
	}

	return nil
}
