package manager

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
	"github.com/stashapp/stash/pkg/scene"
)

type CleanJob struct {
	Input models.CleanMetadataInput

	Repository   models.Repository
	SceneService SceneService
	ImageService ImageService
	Paths        *paths.Paths
	PluginCache  *plugin.Cache
	ScanSubs     *subscriptionManager
}

func (j *CleanJob) Execute(ctx context.Context, progress *job.Progress) {
	if job.IsCancelled(ctx) {
		logger.Info("Stopping due to user request")
		return
	}

	start := time.Now()

	logger.Infof("Starting cleaning of tracked files")
	if j.Input.DryRun {
		logger.Infof("Running in Dry Mode")
	}

	cfg := config.GetInstance()

	stashPaths := cfg.GetStashPaths()
	videoFileNamingAlgorithm := cfg.GetVideoFileNamingAlgorithm()

	extensionMatcher := &file.ExtensionMatcher{
		CreateImageClipsFromVideos: cfg.IsCreateImageClipsFromVideos(),
		StashPaths:                 stashPaths,
	}

	cleaner := &file.Cleaner{
		Paths: j.Input.Paths,
		Handlers: []file.CleanHandler{
			&cleanHandler{
				repository:     j.Repository,
				sceneService:   j.SceneService,
				imageService:   j.ImageService,
				paths:          j.Paths,
				pluginCache:    j.PluginCache,
				fileNamingAlgo: videoFileNamingAlgorithm,
			},
		},
		PathFilter: &cleanFilter{
			stashPaths:        stashPaths,
			generatedPath:     cfg.GetGeneratedPath(),
			videoExcludeRegex: generateRegexps(cfg.GetExcludes()),
			imageExcludeRegex: generateRegexps(cfg.GetImageExcludes()),
			extensionMatcher:  extensionMatcher,
		},
		DryRun:     j.Input.DryRun,
		Repository: file.NewRepository(j.Repository),
		FS:         &file.OsFS{},
	}

	cleaner.Execute(ctx, progress)

	if job.IsCancelled(ctx) {
		logger.Info("Stopping due to user request")
		return
	}

	j.cleanEmptyGalleries(ctx)

	j.ScanSubs.notify()
	elapsed := time.Since(start)
	logger.Info(fmt.Sprintf("Finished Cleaning (%s)", elapsed))
}

func (j *CleanJob) cleanEmptyGalleries(ctx context.Context) {
	const batchSize = 1000
	var toClean []int
	findFilter := models.BatchFindFilter(batchSize)
	r := j.Repository
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		found := true
		for found {
			emptyGalleries, _, err := r.Gallery.Query(ctx, &models.GalleryFilterType{
				ImageCount: &models.IntCriterionInput{
					Value:    0,
					Modifier: models.CriterionModifierEquals,
				},
			}, findFilter)

			if err != nil {
				return err
			}

			found = len(emptyGalleries) > 0

			for _, g := range emptyGalleries {
				if g.Path == "" {
					continue
				}

				if len(j.Input.Paths) > 0 && !fsutil.IsPathInDirs(j.Input.Paths, g.Path) {
					continue
				}

				logger.Infof("Gallery has 0 images. Marking to clean: %s", g.DisplayName())
				toClean = append(toClean, g.ID)
			}

			*findFilter.Page++
		}

		return nil
	}); err != nil {
		logger.Errorf("Error finding empty galleries: %v", err)
		return
	}

	if !j.Input.DryRun {
		for _, id := range toClean {
			j.deleteGallery(ctx, r, id)
		}
	}
}

func (j *CleanJob) deleteGallery(ctx context.Context, r models.Repository, id int) {
	qb := r.Gallery
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		g, err := qb.Find(ctx, id)
		if err != nil {
			return err
		}

		if g == nil {
			return fmt.Errorf("gallery with id %d not found", id)
		}

		if err := g.LoadPrimaryFile(ctx, r.File); err != nil {
			return err
		}

		if err := qb.Destroy(ctx, id); err != nil {
			return err
		}

		j.PluginCache.RegisterPostHooks(ctx, id, plugin.GalleryDestroyPost, plugin.GalleryDestroyInput{
			Checksum: g.PrimaryChecksum(),
			Path:     g.Path,
		}, nil)

		return nil
	}); err != nil {
		logger.Errorf("Error deleting gallery from database: %s", err.Error())
	}
}

type cleanFilter struct {
	stashPaths        models.StashConfigs
	generatedPath     string
	videoExcludeRegex []*regexp.Regexp
	imageExcludeRegex []*regexp.Regexp
	extensionMatcher  *file.ExtensionMatcher
}

func (f *cleanFilter) Accept(ctx context.Context, path string, info fs.FileInfo) bool {
	//  #1102 - clean anything in generated path
	generatedPath := f.generatedPath

	var stash *models.StashConfig
	fileOrFolder := "File"

	if info.IsDir() {
		fileOrFolder = "Folder"
		stash = f.stashPaths.GetStashFromDirPath(path)
	} else {
		stash = f.stashPaths.GetStashFromPath(path)
	}

	if stash == nil {
		logger.Infof("%s not in any stash library directories. Marking to clean: \"%s\"", fileOrFolder, path)
		return false
	}

	if fsutil.IsPathInDir(generatedPath, path) {
		logger.Infof("%s is in generated path. Marking to clean: \"%s\"", fileOrFolder, path)
		return false
	}

	if info.IsDir() {
		return !f.shouldCleanFolder(path, stash)
	}

	return !f.shouldCleanFile(path, info, stash)
}

func (f *cleanFilter) shouldCleanFolder(path string, s *models.StashConfig) bool {
	// only delete folders where it is excluded from everything
	pathExcludeTest := path + string(filepath.Separator)
	if (s.ExcludeVideo || matchFileRegex(pathExcludeTest, f.videoExcludeRegex)) && (s.ExcludeImage || matchFileRegex(pathExcludeTest, f.imageExcludeRegex)) {
		logger.Infof("Folder is excluded from both video and image. Marking to clean: \"%s\"", path)
		return true
	}

	return false
}

func (f *cleanFilter) shouldCleanFile(path string, info fs.FileInfo, stash *models.StashConfig) bool {
	switch {
	case info.IsDir() || f.extensionMatcher.IsZip(path):
		return f.shouldCleanGallery(path, stash)
	case f.extensionMatcher.UseAsVideo(path):
		return f.shouldCleanVideoFile(path, stash)
	case f.extensionMatcher.UseAsImage(path):
		return f.shouldCleanImage(path, stash)
	default:
		logger.Infof("File extension does not match any media extensions. Marking to clean: \"%s\"", path)
		return true
	}
}

func (f *cleanFilter) shouldCleanVideoFile(path string, stash *models.StashConfig) bool {
	if stash.ExcludeVideo {
		logger.Infof("File in stash library that excludes video. Marking to clean: \"%s\"", path)
		return true
	}

	if matchFileRegex(path, f.videoExcludeRegex) {
		logger.Infof("File matched regex. Marking to clean: \"%s\"", path)
		return true
	}

	return false
}

func (f *cleanFilter) shouldCleanGallery(path string, stash *models.StashConfig) bool {
	if stash.ExcludeImage {
		logger.Infof("File in stash library that excludes images. Marking to clean: \"%s\"", path)
		return true
	}

	if matchFileRegex(path, f.imageExcludeRegex) {
		logger.Infof("File matched regex. Marking to clean: \"%s\"", path)
		return true
	}

	return false
}

func (f *cleanFilter) shouldCleanImage(path string, stash *models.StashConfig) bool {
	if stash.ExcludeImage {
		logger.Infof("File in stash library that excludes images. Marking to clean: \"%s\"", path)
		return true
	}

	if matchFileRegex(path, f.imageExcludeRegex) {
		logger.Infof("File matched regex. Marking to clean: \"%s\"", path)
		return true
	}

	return false
}

type cleanHandler struct {
	repository   models.Repository
	sceneService SceneService
	imageService ImageService

	paths          *paths.Paths
	pluginCache    *plugin.Cache
	fileNamingAlgo models.HashAlgorithm
}

func (h *cleanHandler) HandleFile(ctx context.Context, fileDeleter *file.Deleter, fileID models.FileID) error {
	if err := h.handleRelatedScenes(ctx, fileDeleter, fileID); err != nil {
		return err
	}
	if err := h.handleRelatedGalleries(ctx, fileID); err != nil {
		return err
	}
	if err := h.handleRelatedImages(ctx, fileDeleter, fileID); err != nil {
		return err
	}

	return nil
}

func (h *cleanHandler) HandleFolder(ctx context.Context, fileDeleter *file.Deleter, folderID models.FolderID) error {
	return h.deleteRelatedFolderGalleries(ctx, folderID)
}

func (h *cleanHandler) handleRelatedScenes(ctx context.Context, fileDeleter *file.Deleter, fileID models.FileID) error {
	sceneQB := h.repository.Scene
	scenes, err := sceneQB.FindByFileID(ctx, fileID)
	if err != nil {
		return err
	}

	sceneFileDeleter := &scene.FileDeleter{
		Deleter:        fileDeleter,
		FileNamingAlgo: h.fileNamingAlgo,
		Paths:          h.paths,
	}

	for _, scene := range scenes {
		if err := scene.LoadFiles(ctx, sceneQB); err != nil {
			return err
		}

		// only delete if the scene has no other files
		if len(scene.Files.List()) <= 1 {
			logger.Infof("Deleting scene %q since it has no other related files", scene.DisplayName())
			if err := h.sceneService.Destroy(ctx, scene, sceneFileDeleter, true, false); err != nil {
				return err
			}

			h.pluginCache.RegisterPostHooks(ctx, scene.ID, plugin.SceneDestroyPost, plugin.SceneDestroyInput{
				Checksum: scene.Checksum,
				OSHash:   scene.OSHash,
				Path:     scene.Path,
			}, nil)
		} else {
			// set the primary file to a remaining file
			var newPrimaryID models.FileID
			for _, f := range scene.Files.List() {
				if f.ID != fileID {
					newPrimaryID = f.ID
					break
				}
			}

			scenePartial := models.NewScenePartial()
			scenePartial.PrimaryFileID = &newPrimaryID

			if _, err := h.repository.Scene.UpdatePartial(ctx, scene.ID, scenePartial); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *cleanHandler) handleRelatedGalleries(ctx context.Context, fileID models.FileID) error {
	qb := h.repository.Gallery
	galleries, err := qb.FindByFileID(ctx, fileID)
	if err != nil {
		return err
	}

	for _, g := range galleries {
		if err := g.LoadFiles(ctx, qb); err != nil {
			return err
		}

		// only delete if the gallery has no other files
		if len(g.Files.List()) <= 1 {
			logger.Infof("Deleting gallery %q since it has no other related files", g.DisplayName())
			if err := qb.Destroy(ctx, g.ID); err != nil {
				return err
			}

			h.pluginCache.RegisterPostHooks(ctx, g.ID, plugin.GalleryDestroyPost, plugin.GalleryDestroyInput{
				Checksum: g.PrimaryChecksum(),
				Path:     g.Path,
			}, nil)
		} else {
			// set the primary file to a remaining file
			var newPrimaryID models.FileID
			for _, f := range g.Files.List() {
				if f.Base().ID != fileID {
					newPrimaryID = f.Base().ID
					break
				}
			}

			galleryPartial := models.NewGalleryPartial()
			galleryPartial.PrimaryFileID = &newPrimaryID

			if _, err := h.repository.Gallery.UpdatePartial(ctx, g.ID, galleryPartial); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *cleanHandler) deleteRelatedFolderGalleries(ctx context.Context, folderID models.FolderID) error {
	qb := h.repository.Gallery
	galleries, err := qb.FindByFolderID(ctx, folderID)
	if err != nil {
		return err
	}

	for _, g := range galleries {
		logger.Infof("Deleting folder-based gallery %q since the folder no longer exists", g.DisplayName())
		if err := qb.Destroy(ctx, g.ID); err != nil {
			return err
		}

		h.pluginCache.RegisterPostHooks(ctx, g.ID, plugin.GalleryDestroyPost, plugin.GalleryDestroyInput{
			// No checksum for folders
			// Checksum: g.Checksum(),
			Path: g.Path,
		}, nil)
	}

	return nil
}

func (h *cleanHandler) handleRelatedImages(ctx context.Context, fileDeleter *file.Deleter, fileID models.FileID) error {
	imageQB := h.repository.Image
	images, err := imageQB.FindByFileID(ctx, fileID)
	if err != nil {
		return err
	}

	imageFileDeleter := &image.FileDeleter{
		Deleter: fileDeleter,
		Paths:   h.paths,
	}

	for _, i := range images {
		if err := i.LoadFiles(ctx, imageQB); err != nil {
			return err
		}

		if len(i.Files.List()) <= 1 {
			logger.Infof("Deleting image %q since it has no other related files", i.DisplayName())
			if err := h.imageService.Destroy(ctx, i, imageFileDeleter, true, false); err != nil {
				return err
			}

			h.pluginCache.RegisterPostHooks(ctx, i.ID, plugin.ImageDestroyPost, plugin.ImageDestroyInput{
				Checksum: i.Checksum,
				Path:     i.Path,
			}, nil)
		} else {
			// set the primary file to a remaining file
			var newPrimaryID models.FileID
			for _, f := range i.Files.List() {
				if f.Base().ID != fileID {
					newPrimaryID = f.Base().ID
					break
				}
			}

			imagePartial := models.NewImagePartial()
			imagePartial.PrimaryFileID = &newPrimaryID

			if _, err := h.repository.Image.UpdatePartial(ctx, i.ID, imagePartial); err != nil {
				return err
			}
		}
	}

	return nil
}
