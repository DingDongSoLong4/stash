package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/remeh/sizedwaitgroup"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/generate"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/sliceutil/stringslice"
)

const generateQueueSize = 200000

type GenerateJob struct {
	Input     models.GenerateMetadataInput
	Overwrite bool

	ParallelTasks       int
	Repository          models.Repository
	FileNamingAlgorithm models.HashAlgorithm
	PreviewPreset       models.PreviewPreset
	Paths               *paths.Paths
	Generator           *generate.Generator
}

type totalsGenerate struct {
	covers                   int64
	sprites                  int64
	previews                 int64
	imagePreviews            int64
	markers                  int64
	transcodes               int64
	phashes                  int64
	interactiveHeatmapSpeeds int64
	clipPreviews             int64

	tasks int
}

func (j *GenerateJob) Execute(ctx context.Context, progress *job.Progress) {
	var scenes []*models.Scene
	var err error
	var markers []*models.SceneMarker

	logger.Infof("Generate started with %d parallel tasks", j.ParallelTasks)

	queue := make(chan Task, generateQueueSize)
	go func() {
		defer close(queue)

		var totals totalsGenerate
		sceneIDs, err := stringslice.StringSliceToIntSlice(j.Input.SceneIDs)
		if err != nil {
			logger.Error(err.Error())
		}
		markerIDs, err := stringslice.StringSliceToIntSlice(j.Input.MarkerIDs)
		if err != nil {
			logger.Error(err.Error())
		}

		r := j.Repository
		if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
			qb := r.Scene
			if len(j.Input.SceneIDs) == 0 && len(j.Input.MarkerIDs) == 0 {
				totals = j.queueTasks(ctx, queue)
			} else {
				if len(j.Input.SceneIDs) > 0 {
					scenes, err = qb.FindMany(ctx, sceneIDs)
					for _, s := range scenes {
						if err := s.LoadFiles(ctx, qb); err != nil {
							return err
						}

						j.queueSceneJobs(ctx, s, queue, &totals)
					}
				}

				if len(j.Input.MarkerIDs) > 0 {
					markers, err = r.SceneMarker.FindMany(ctx, markerIDs)
					if err != nil {
						return err
					}
					for _, m := range markers {
						j.queueMarkerJob(m, queue, &totals)
					}
				}
			}

			return nil
		}); err != nil && ctx.Err() == nil {
			logger.Error(err.Error())
			return
		}

		logMsg := "Generating"
		if j.Input.Covers {
			logMsg += fmt.Sprintf(" %d covers", totals.covers)
		}
		if j.Input.Sprites {
			logMsg += fmt.Sprintf(" %d sprites", totals.sprites)
		}
		if j.Input.Previews {
			logMsg += fmt.Sprintf(" %d previews", totals.previews)
		}
		if j.Input.ImagePreviews {
			logMsg += fmt.Sprintf(" %d image previews", totals.imagePreviews)
		}
		if j.Input.Markers {
			logMsg += fmt.Sprintf(" %d markers", totals.markers)
		}
		if j.Input.Transcodes {
			logMsg += fmt.Sprintf(" %d transcodes", totals.transcodes)
		}
		if j.Input.Phashes {
			logMsg += fmt.Sprintf(" %d phashes", totals.phashes)
		}
		if j.Input.InteractiveHeatmapsSpeeds {
			logMsg += fmt.Sprintf(" %d heatmaps & speeds", totals.interactiveHeatmapSpeeds)
		}
		if j.Input.ClipPreviews {
			logMsg += fmt.Sprintf(" %d image clip previews", totals.clipPreviews)
		}
		if logMsg == "Generating" {
			logMsg = "Nothing selected to generate"
		}
		logger.Infof(logMsg)

		progress.SetTotal(int(totals.tasks))
	}()

	wg := sizedwaitgroup.New(j.ParallelTasks)

	// Start measuring how long the generate has taken. (consider moving this up)
	start := time.Now()
	if err = j.Paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("could not create temporary directory: %v", err)
	}

	defer func() {
		if err := j.Paths.Generated.EmptyTmpDir(); err != nil {
			logger.Warnf("failure emptying temporary directory: %v", err)
		}
	}()

	for f := range queue {
		if job.IsCancelled(ctx) {
			break
		}

		wg.Add()
		// #1879 - need to make a copy of f - otherwise there is a race condition
		// where f is changed when the goroutine runs
		localTask := f
		go progress.ExecuteTask(localTask.GetDescription(), func() {
			localTask.Start(ctx)
			wg.Done()
			progress.Increment()
		})
	}

	wg.Wait()

	if job.IsCancelled(ctx) {
		logger.Info("Stopping due to user request")
		return
	}

	elapsed := time.Since(start)
	logger.Info(fmt.Sprintf("Generate finished (%s)", elapsed))
}

func (j *GenerateJob) queueTasks(ctx context.Context, queue chan<- Task) totalsGenerate {
	var totals totalsGenerate

	const batchSize = 1000

	findFilter := models.BatchFindFilter(batchSize)

	r := j.Repository

	for more := true; more; {
		if job.IsCancelled(ctx) {
			return totals
		}

		scenes, err := scene.Query(ctx, r.Scene, nil, findFilter)
		if err != nil {
			logger.Errorf("Error encountered queuing files to scan: %s", err.Error())
			return totals
		}

		for _, ss := range scenes {
			if job.IsCancelled(ctx) {
				return totals
			}

			if err := ss.LoadFiles(ctx, r.Scene); err != nil {
				logger.Errorf("Error encountered queuing files to scan: %s", err.Error())
				return totals
			}

			j.queueSceneJobs(ctx, ss, queue, &totals)
		}

		if len(scenes) != batchSize {
			more = false
		} else {
			*findFilter.Page++
		}
	}

	*findFilter.Page = 1
	for more := j.Input.ClipPreviews; more; {
		if job.IsCancelled(ctx) {
			return totals
		}

		images, err := image.Query(ctx, r.Image, nil, findFilter)
		if err != nil {
			logger.Errorf("Error encountered queuing files to scan: %s", err.Error())
			return totals
		}

		for _, ss := range images {
			if job.IsCancelled(ctx) {
				return totals
			}

			if err := ss.LoadFiles(ctx, r.Image); err != nil {
				logger.Errorf("Error encountered queuing files to scan: %s", err.Error())
				return totals
			}

			j.queueImageJob(ss, queue, &totals)
		}

		if len(images) != batchSize {
			more = false
		} else {
			*findFilter.Page++
		}
	}

	return totals
}

func getGeneratePreviewOptions(optionsInput models.GeneratePreviewOptionsInput) generate.PreviewOptions {
	config := config.GetInstance()

	ret := generate.PreviewOptions{
		Segments:        config.GetPreviewSegments(),
		SegmentDuration: config.GetPreviewSegmentDuration(),
		ExcludeStart:    config.GetPreviewExcludeStart(),
		ExcludeEnd:      config.GetPreviewExcludeEnd(),
		Preset:          config.GetPreviewPreset().String(),
		Audio:           config.GetPreviewAudio(),
	}

	if optionsInput.PreviewSegments != nil {
		ret.Segments = *optionsInput.PreviewSegments
	}

	if optionsInput.PreviewSegmentDuration != nil {
		ret.SegmentDuration = *optionsInput.PreviewSegmentDuration
	}

	if optionsInput.PreviewExcludeStart != nil {
		ret.ExcludeStart = *optionsInput.PreviewExcludeStart
	}

	if optionsInput.PreviewExcludeEnd != nil {
		ret.ExcludeEnd = *optionsInput.PreviewExcludeEnd
	}

	if optionsInput.PreviewPreset != nil {
		ret.Preset = optionsInput.PreviewPreset.String()
	}

	return ret
}

func (j *GenerateJob) queueSceneJobs(ctx context.Context, scene *models.Scene, queue chan<- Task, totals *totalsGenerate) {
	r := j.Repository

	if j.Input.Covers {
		task := &GenerateCoverTask{
			Scene:      *scene,
			Overwrite:  j.Overwrite,
			Repository: r,
		}

		if task.required(ctx) {
			totals.covers++
			totals.tasks++
			queue <- task
		}
	}

	if j.Input.Sprites {
		task := &GenerateSpriteTask{
			Scene:               *scene,
			Overwrite:           j.Overwrite,
			Paths:               j.Paths,
			FileNamingAlgorithm: j.FileNamingAlgorithm,
			Generator:           j.Generator,
		}

		if task.required() {
			totals.sprites++
			totals.tasks++
			queue <- task
		}
	}

	generatePreviewOptions := j.Input.PreviewOptions
	if generatePreviewOptions == nil {
		generatePreviewOptions = &models.GeneratePreviewOptionsInput{}
	}
	options := getGeneratePreviewOptions(*generatePreviewOptions)

	if j.Input.Previews {
		task := &GeneratePreviewTask{
			Scene:               *scene,
			Options:             options,
			ImagePreview:        j.Input.ImagePreviews,
			Overwrite:           j.Overwrite,
			FileNamingAlgorithm: j.FileNamingAlgorithm,
			Generator:           j.Generator,
		}

		if task.required() {
			if task.videoPreviewRequired() {
				totals.previews++
			}
			if task.imagePreviewRequired() {
				totals.imagePreviews++
			}

			totals.tasks++
			queue <- task
		}
	}

	if j.Input.Markers {
		task := &GenerateMarkersTask{
			Scene:               scene,
			ImagePreview:        j.Input.MarkerImagePreviews,
			Screenshot:          j.Input.MarkerScreenshots,
			Overwrite:           j.Overwrite,
			Repository:          r,
			FileNamingAlgorithm: j.FileNamingAlgorithm,
			Generator:           j.Generator,
		}

		markers := task.markersNeeded(ctx)
		if markers > 0 {
			totals.markers += int64(markers)
			totals.tasks++

			queue <- task
		}
	}

	if j.Input.Transcodes {
		forceTranscode := j.Input.ForceTranscodes
		task := &GenerateTranscodeTask{
			Scene:               *scene,
			Overwrite:           j.Overwrite,
			Force:               forceTranscode,
			FileNamingAlgorithm: j.FileNamingAlgorithm,
			SceneService:        GetInstance().SceneService,
			Generator:           j.Generator,
		}
		if task.required() {
			totals.transcodes++
			totals.tasks++
			queue <- task
		}
	}

	if j.Input.Phashes {
		// generate for all files in scene
		for _, f := range scene.Files.List() {
			task := &GeneratePhashTask{
				File:                f,
				Overwrite:           j.Overwrite,
				Repository:          r,
				FileNamingAlgorithm: j.FileNamingAlgorithm,
			}

			if task.required() {
				totals.phashes++
				totals.tasks++
				queue <- task
			}
		}
	}

	if j.Input.InteractiveHeatmapsSpeeds {
		task := &GenerateInteractiveHeatmapSpeedTask{
			Scene:               *scene,
			DrawRange:           config.GetInstance().GetDrawFunscriptHeatmapRange(),
			Overwrite:           j.Overwrite,
			Repository:          r,
			FileNamingAlgorithm: j.FileNamingAlgorithm,
			Paths:               j.Paths,
		}

		if task.required() {
			totals.interactiveHeatmapSpeeds++
			totals.tasks++
			queue <- task
		}
	}
}

func (j *GenerateJob) queueMarkerJob(marker *models.SceneMarker, queue chan<- Task, totals *totalsGenerate) {
	task := &GenerateMarkersTask{
		Marker:              marker,
		Overwrite:           j.Overwrite,
		Repository:          j.Repository,
		FileNamingAlgorithm: j.FileNamingAlgorithm,
		Generator:           j.Generator,
	}
	totals.markers++
	totals.tasks++
	queue <- task
}

func (j *GenerateJob) queueImageJob(image *models.Image, queue chan<- Task, totals *totalsGenerate) {
	task := &GenerateClipPreviewTask{
		Image:         *image,
		Overwrite:     j.Overwrite,
		Paths:         j.Paths,
		PreviewPreset: j.PreviewPreset,
		Generator:     j.Generator,
	}

	if task.required() {
		totals.clipPreviews++
		totals.tasks++
		queue <- task
	}
}
