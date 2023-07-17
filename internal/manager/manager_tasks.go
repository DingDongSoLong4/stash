package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql"

	"github.com/stashapp/stash/internal/identify"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

func useAsVideo(pathname string) bool {
	config := config.GetInstance()
	if config.IsCreateImageClipsFromVideos() && config.GetStashPaths().GetStashFromDirPath(pathname).ExcludeVideo {
		return false
	}
	return isVideo(pathname)
}

func useAsImage(pathname string) bool {
	config := config.GetInstance()
	if config.IsCreateImageClipsFromVideos() && config.GetStashPaths().GetStashFromDirPath(pathname).ExcludeVideo {
		return isImage(pathname) || isVideo(pathname)
	}
	return isImage(pathname)
}

func isZip(pathname string) bool {
	gExt := config.GetInstance().GetGalleryExtensions()
	return fsutil.MatchExtension(pathname, gExt)
}

func isVideo(pathname string) bool {
	vidExt := config.GetInstance().GetVideoExtensions()
	return fsutil.MatchExtension(pathname, vidExt)
}

func isImage(pathname string) bool {
	imgExt := config.GetInstance().GetImageExtensions()
	return fsutil.MatchExtension(pathname, imgExt)
}

func getScanPaths(inputPaths []string) []*config.StashConfig {
	stashPaths := config.GetInstance().GetStashPaths()

	if len(inputPaths) == 0 {
		return stashPaths
	}

	var ret config.StashConfigs
	for _, p := range inputPaths {
		s := stashPaths.GetStashFromDirPath(p)
		if s == nil {
			logger.Warnf("%s is not in the configured stash paths", p)
			continue
		}

		// make a copy, changing the path
		ss := *s
		ss.Path = p
		ret = append(ret, &ss)
	}

	return ret
}

// ScanSubscribe subscribes to a notification that is triggered when a
// scan or clean is complete.
func (s *Manager) ScanSubscribe(ctx context.Context) <-chan bool {
	return s.scanSubs.subscribe(ctx)
}

func (s *Manager) RunSingleTask(ctx context.Context, t Task) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) {
		t.Start(ctx)
	})

	return s.JobManager.Add(ctx, t.GetDescription(), j)
}

type ScanMetadataInput struct {
	Paths []string `json:"paths"`

	config.ScanMetadataOptions `mapstructure:",squash"`

	// Filter options for the scan
	Filter *ScanMetaDataFilterInput `json:"filter"`
}

// Filter options for meta data scannning
type ScanMetaDataFilterInput struct {
	// If set, files with a modification time before this time point are ignored by the scan
	MinModTime *time.Time `json:"minModTime"`
}

func (s *Manager) Scan(ctx context.Context, input ScanMetadataInput) (int, error) {
	scanJob := ScanJob{
		scanner:       s.Scanner,
		input:         input,
		subscriptions: s.scanSubs,
	}

	return s.JobManager.Add(ctx, "Scanning...", &scanJob), nil
}

func (s *Manager) Import(ctx context.Context) (int, error) {
	metadataPath := s.Config.GetMetadataPath()
	if metadataPath == "" {
		return 0, errors.New("metadata path must be set in config")
	}

	t := ImportTask{
		repository:          s.Repository,
		resetter:            s.Database,
		BaseDir:             metadataPath,
		Reset:               true,
		DuplicateBehaviour:  ImportDuplicateEnumFail,
		MissingRefBehaviour: models.ImportMissingRefEnumFail,
		fileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
	}

	return s.RunSingleTask(ctx, &t), nil
}

type ImportObjectsInput struct {
	File                graphql.Upload              `json:"file"`
	DuplicateBehaviour  ImportDuplicateEnum         `json:"duplicateBehaviour"`
	MissingRefBehaviour models.ImportMissingRefEnum `json:"missingRefBehaviour"`
}

func (s *Manager) ImportObjects(ctx context.Context, input ImportObjectsInput) (int, error) {
	baseDir, err := s.Paths.Generated.TempDir("import")
	if err != nil {
		logger.Errorf("error creating temporary directory for import: %s", err.Error())
		return 0, err
	}

	tmpZip := ""
	if input.File.File != nil {
		tmpZip = filepath.Join(baseDir, "import.zip")
		out, err := os.Create(tmpZip)
		if err != nil {
			return 0, err
		}

		_, err = io.Copy(out, input.File.File)
		out.Close()
		if err != nil {
			return 0, err
		}
	}

	t := ImportTask{
		repository:          s.Repository,
		resetter:            s.Database,
		BaseDir:             baseDir,
		TmpZip:              tmpZip,
		Reset:               false,
		DuplicateBehaviour:  input.DuplicateBehaviour,
		MissingRefBehaviour: input.MissingRefBehaviour,
		fileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
	}

	return s.RunSingleTask(ctx, &t), nil
}

func (s *Manager) Export(ctx context.Context) (int, error) {
	metadataPath := s.Config.GetMetadataPath()
	if metadataPath == "" {
		return 0, errors.New("metadata path must be set in config")
	}

	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) {
		task := ExportTask{
			repository:          s.Repository,
			full:                true,
			fileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
		}
		task.Start(ctx)
	})

	return s.JobManager.Add(ctx, "Exporting...", j), nil
}

func (s *Manager) ExportObjects(ctx context.Context, input ExportObjectsInput, baseURL string) (*string, error) {
	includeDeps := false
	if input.IncludeDependencies != nil {
		includeDeps = *input.IncludeDependencies
	}

	t := ExportTask{
		repository:          s.Repository,
		fileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
		scenes:              newExportSpec(input.Scenes),
		images:              newExportSpec(input.Images),
		performers:          newExportSpec(input.Performers),
		movies:              newExportSpec(input.Movies),
		tags:                newExportSpec(input.Tags),
		studios:             newExportSpec(input.Studios),
		galleries:           newExportSpec(input.Galleries),
		includeDependencies: includeDeps,
	}

	t.Start(ctx)

	if t.DownloadHash != "" {
		// generate timestamp
		suffix := time.Now().Format("20060102-150405")
		ret := baseURL + "/downloads/" + t.DownloadHash + "/export" + suffix + ".zip"
		return &ret, nil
	}

	return nil, nil
}

func (s *Manager) Generate(ctx context.Context, input GenerateMetadataInput) (int, error) {
	if err := s.Paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("could not generate temporary directory: %v", err)
	}

	j := &GenerateJob{
		repository: s.Repository,
		input:      input,
	}

	return s.JobManager.Add(ctx, "Generating...", j), nil
}

func (s *Manager) GenerateDefaultScreenshot(ctx context.Context, sceneId string) int {
	return s.generateScreenshot(ctx, sceneId, nil)
}

func (s *Manager) GenerateScreenshot(ctx context.Context, sceneId string, at float64) int {
	return s.generateScreenshot(ctx, sceneId, &at)
}

// generate default screenshot if at is nil
func (s *Manager) generateScreenshot(ctx context.Context, sceneId string, at *float64) int {
	if err := s.Paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("failure generating screenshot: %v", err)
	}

	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) {
		sceneIdInt, err := strconv.Atoi(sceneId)
		if err != nil {
			logger.Errorf("Error parsing scene id %s: %v", sceneId, err)
			return
		}

		var scene *models.Scene
		if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
			scene, err = s.Repository.Scene.Find(ctx, sceneIdInt)
			if err != nil {
				return err
			}
			if scene == nil {
				return fmt.Errorf("scene with id %s not found", sceneId)
			}

			return scene.LoadPrimaryFile(ctx, s.Repository.File)
		}); err != nil {
			logger.Errorf("error finding scene for screenshot generation: %v", err)
			return
		}

		task := GenerateCoverTask{
			repository:   s.Repository,
			Scene:        *scene,
			ScreenshotAt: at,
			Overwrite:    true,
		}

		task.Start(ctx)

		logger.Infof("Generate screenshot finished")
	})

	return s.JobManager.Add(ctx, fmt.Sprintf("Generating screenshot for scene id %s", sceneId), j)
}

type AutoTagMetadataInput struct {
	// Paths to tag, null for all files
	Paths []string `json:"paths"`
	// IDs of performers to tag files with, or "*" for all
	Performers []string `json:"performers"`
	// IDs of studios to tag files with, or "*" for all
	Studios []string `json:"studios"`
	// IDs of tags to tag files with, or "*" for all
	Tags []string `json:"tags"`
}

func (s *Manager) AutoTag(ctx context.Context, input AutoTagMetadataInput) int {
	j := autoTagJob{
		repository: s.Repository,
		input:      input,
	}

	return s.JobManager.Add(ctx, "Auto-tagging...", &j)
}

func (s *Manager) Identify(ctx context.Context, input identify.Options) int {
	j := IdentifyJob{
		repository:       s.Repository,
		postHookExecutor: s.PluginCache,
		scraperCache:     s.ScraperCache,

		input:      input,
		stashBoxes: s.Config.GetStashBoxes(),
	}

	return s.JobManager.Add(ctx, "Identifying...", &j)
}

type CleanMetadataInput struct {
	Paths []string `json:"paths"`
	// Do a dry run. Don't delete any files
	DryRun bool `json:"dryRun"`
}

func (s *Manager) Clean(ctx context.Context, input CleanMetadataInput) int {
	j := cleanJob{
		cleaner:      s.Cleaner,
		repository:   s.Repository,
		sceneService: s.SceneService,
		imageService: s.ImageService,
		input:        input,
		scanSubs:     s.scanSubs,
	}

	return s.JobManager.Add(ctx, "Cleaning...", &j)
}

func (s *Manager) MigrateHash(ctx context.Context) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) {
		fileNamingAlgo := s.Config.GetVideoFileNamingAlgorithm()
		logger.Infof("Migrating generated files for %s naming hash", fileNamingAlgo.String())

		var scenes []*models.Scene
		if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
			var err error
			scenes, err = s.Repository.Scene.All(ctx)
			return err
		}); err != nil {
			logger.Errorf("failed to fetch list of scenes for migration: %s", err.Error())
			return
		}

		var wg sync.WaitGroup
		total := len(scenes)
		progress.SetTotal(total)

		for _, scene := range scenes {
			progress.Increment()
			if job.IsCancelled(ctx) {
				logger.Info("Stopping due to user request")
				return
			}

			if scene == nil {
				logger.Errorf("nil scene, skipping migrate")
				continue
			}

			wg.Add(1)

			task := MigrateHashTask{Scene: scene, fileNamingAlgorithm: fileNamingAlgo}
			go func() {
				task.Start()
				wg.Done()
			}()

			wg.Wait()
		}

		logger.Info("Finished migrating")
	})

	return s.JobManager.Add(ctx, "Migrating scene hashes...", j)
}

// If neither performer_ids nor performer_names are set, tag all performers
type StashBoxBatchPerformerTagInput struct {
	// Stash endpoint to use for the performer tagging
	Endpoint int `json:"endpoint"`
	// Fields to exclude when executing the performer tagging
	ExcludeFields []string `json:"exclude_fields"`
	// Refresh performers already tagged by StashBox if true. Only tag performers with no StashBox tagging if false
	Refresh bool `json:"refresh"`
	// If set, only tag these performer ids
	PerformerIds []string `json:"performer_ids"`
	// If set, only tag these performer names
	PerformerNames []string `json:"performer_names"`
}

func (s *Manager) StashBoxBatchPerformerTag(ctx context.Context, input StashBoxBatchPerformerTagInput) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) {
		logger.Infof("Initiating stash-box batch performer tag")

		boxes := s.Config.GetStashBoxes()
		if input.Endpoint < 0 || input.Endpoint >= len(boxes) {
			logger.Error(fmt.Errorf("invalid stash_box_index %d", input.Endpoint))
			return
		}
		box := boxes[input.Endpoint]

		var tasks []StashBoxPerformerTagTask

		// The gocritic linter wants to turn this ifElseChain into a switch.
		// however, such a switch would contain quite large blocks for each section
		// and would arguably be hard to read.
		//
		// This is why we mark this section nolint. In principle, we should look to
		// rewrite the section at some point, to avoid the linter warning.
		if len(input.PerformerIds) > 0 { //nolint:gocritic
			if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
				performerQuery := s.Repository.Performer

				for _, performerID := range input.PerformerIds {
					if id, err := strconv.Atoi(performerID); err == nil {
						performer, err := performerQuery.Find(ctx, id)
						if err == nil {
							err = performer.LoadStashIDs(ctx, performerQuery)
						}

						if err == nil {
							tasks = append(tasks, StashBoxPerformerTagTask{
								performer:       performer,
								refresh:         input.Refresh,
								box:             box,
								excluded_fields: input.ExcludeFields,
							})
						} else {
							return err
						}
					}
				}
				return nil
			}); err != nil {
				logger.Error(err.Error())
			}
		} else if len(input.PerformerNames) > 0 {
			for i := range input.PerformerNames {
				if len(input.PerformerNames[i]) > 0 {
					tasks = append(tasks, StashBoxPerformerTagTask{
						name:            &input.PerformerNames[i],
						refresh:         input.Refresh,
						box:             box,
						excluded_fields: input.ExcludeFields,
					})
				}
			}
		} else { //nolint:gocritic
			// The gocritic linter wants to fold this if-block into the else on the line above.
			// However, this doesn't really help with readability of the current section. Mark it
			// as nolint for now. In the future we'd like to rewrite this code by factoring some of
			// this into separate functions.
			if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
				performerQuery := s.Repository.Performer
				var performers []*models.Performer
				var err error
				if input.Refresh {
					performers, err = performerQuery.FindByStashIDStatus(ctx, true, box.Endpoint)
				} else {
					performers, err = performerQuery.FindByStashIDStatus(ctx, false, box.Endpoint)
				}
				if err != nil {
					return fmt.Errorf("error querying performers: %v", err)
				}

				for _, performer := range performers {
					if err := performer.LoadStashIDs(ctx, performerQuery); err != nil {
						return fmt.Errorf("error loading stash ids for performer %s: %v", performer.Name, err)
					}

					tasks = append(tasks, StashBoxPerformerTagTask{
						performer:       performer,
						refresh:         input.Refresh,
						box:             box,
						excluded_fields: input.ExcludeFields,
					})
				}
				return nil
			}); err != nil {
				logger.Error(err.Error())
				return
			}
		}

		if len(tasks) == 0 {
			return
		}

		progress.SetTotal(len(tasks))

		logger.Infof("Starting stash-box batch operation for %d performers", len(tasks))

		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			progress.ExecuteTask(task.Description(), func() {
				task.Start(ctx)
				wg.Done()
			})

			progress.Increment()
		}
	})

	return s.JobManager.Add(ctx, "Batch stash-box performer tag...", j)
}
