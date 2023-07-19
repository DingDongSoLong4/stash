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

	"github.com/stashapp/stash/internal/identify"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

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

func (s *Manager) Scan(ctx context.Context, input models.ScanMetadataInput) (int, error) {
	scanJob := ScanJob{
		Input:   input,
		Manager: s,
	}

	return s.JobManager.Add(ctx, "Scanning...", &scanJob), nil
}

func (s *Manager) Import(ctx context.Context) (int, error) {
	metadataPath := s.Config.GetMetadataPath()
	if metadataPath == "" {
		return 0, errors.New("metadata path must be set in config")
	}

	t := ImportTask{
		BaseDir:             metadataPath,
		Reset:               true,
		DuplicateBehaviour:  models.ImportDuplicateEnumFail,
		MissingRefBehaviour: models.ImportMissingRefEnumFail,
		Repository:          s.Repository,
		Resetter:            s.Database,
		FileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
	}

	return s.RunSingleTask(ctx, &t), nil
}

func (s *Manager) ImportObjects(ctx context.Context, input models.ImportObjectsInput) (int, error) {
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
		BaseDir:             baseDir,
		TmpZip:              tmpZip,
		Reset:               false,
		DuplicateBehaviour:  input.DuplicateBehaviour,
		MissingRefBehaviour: input.MissingRefBehaviour,
		Repository:          s.Repository,
		Resetter:            s.Database,
		FileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
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
			Full:                true,
			Repository:          s.Repository,
			FileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
			Paths:               s.Paths,
			DownloadStore:       s.DownloadStore,
		}
		task.Start(ctx)
	})

	return s.JobManager.Add(ctx, "Exporting...", j), nil
}

func (s *Manager) ExportObjects(ctx context.Context, input models.ExportObjectsInput, baseURL string) (*string, error) {
	includeDeps := false
	if input.IncludeDependencies != nil {
		includeDeps = *input.IncludeDependencies
	}

	t := ExportTask{
		Input:               input,
		IncludeDependencies: includeDeps,
		Repository:          s.Repository,
		FileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
		Paths:               s.Paths,
		DownloadStore:       s.DownloadStore,
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

func (s *Manager) Generate(ctx context.Context, input models.GenerateMetadataInput) (int, error) {
	if err := s.Paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("could not generate temporary directory: %v", err)
	}

	j := &GenerateJob{
		Input:               input,
		ParallelTasks:       s.Config.GetParallelTasksWithAutoDetection(),
		Repository:          s.Repository,
		SceneService:        s.SceneService,
		FileNamingAlgorithm: s.Config.GetVideoFileNamingAlgorithm(),
		PreviewAudio:        s.Config.GetPreviewAudio(),
		PreviewPreset:       s.Config.GetPreviewPreset(),
		Paths:               s.Paths,
		FFMpeg:              s.FFMpeg,
		FFProbe:             s.FFProbe,
		Generator:           s.NewGenerator(false),
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
			Scene:        *scene,
			ScreenshotAt: at,
			Overwrite:    true,
			Repository:   s.Repository,
		}

		task.Start(ctx)

		logger.Infof("Generate screenshot finished")
	})

	return s.JobManager.Add(ctx, fmt.Sprintf("Generating screenshot for scene id %s", sceneId), j)
}

func (s *Manager) AutoTag(ctx context.Context, input models.AutoTagMetadataInput) int {
	j := AutoTagJob{
		Input:      input,
		Repository: s.Repository,
	}

	return s.JobManager.Add(ctx, "Auto-tagging...", &j)
}

func (s *Manager) Identify(ctx context.Context, input identify.Options) int {
	j := IdentifyJob{
		Input:            input,
		StashBoxes:       s.Config.GetStashBoxes(),
		Repository:       s.Repository,
		ScraperCache:     s.ScraperCache,
		PostHookExecutor: s.PluginCache,
	}

	return s.JobManager.Add(ctx, "Identifying...", &j)
}

func (s *Manager) Clean(ctx context.Context, input models.CleanMetadataInput) int {
	j := CleanJob{
		Input:        input,
		Repository:   s.Repository,
		SceneService: s.SceneService,
		ImageService: s.ImageService,
		Paths:        s.Paths,
		PluginCache:  s.PluginCache,
		ScanSubs:     s.scanSubs,
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

			task := MigrateHashTask{
				Scene:               scene,
				Paths:               s.Paths,
				FileNamingAlgorithm: fileNamingAlgo,
			}
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

func (s *Manager) StashBoxBatchPerformerTag(ctx context.Context, input models.StashBoxBatchPerformerTagInput) int {
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
								Performer:      performer,
								Refresh:        input.Refresh,
								Box:            box,
								ExcludedFields: input.ExcludeFields,
								Repository:     s.Repository,
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
						Name:           &input.PerformerNames[i],
						Refresh:        input.Refresh,
						Box:            box,
						ExcludedFields: input.ExcludeFields,
						Repository:     s.Repository,
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
						Performer:      performer,
						Refresh:        input.Refresh,
						Box:            box,
						ExcludedFields: input.ExcludeFields,
						Repository:     s.Repository,
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
