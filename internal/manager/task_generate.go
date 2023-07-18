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
	repository models.Repository
	input      models.GenerateMetadataInput

	overwrite      bool
	parallelTasks  int
	fileNamingAlgo models.HashAlgorithm
	previewPreset  models.PreviewPreset
	paths          *paths.Paths
	generator      *generate.Generator
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

	logger.Infof("Generate started with %d parallel tasks", j.parallelTasks)

	queue := make(chan Task, generateQueueSize)
	go func() {
		defer close(queue)

		var totals totalsGenerate
		sceneIDs, err := stringslice.StringSliceToIntSlice(j.input.SceneIDs)
		if err != nil {
			logger.Error(err.Error())
		}
		markerIDs, err := stringslice.StringSliceToIntSlice(j.input.MarkerIDs)
		if err != nil {
			logger.Error(err.Error())
		}

		r := j.repository
		if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
			qb := r.Scene
			if len(j.input.SceneIDs) == 0 && len(j.input.MarkerIDs) == 0 {
				totals = j.queueTasks(ctx, queue)
			} else {
				if len(j.input.SceneIDs) > 0 {
					scenes, err = qb.FindMany(ctx, sceneIDs)
					for _, s := range scenes {
						if err := s.LoadFiles(ctx, qb); err != nil {
							return err
						}

						j.queueSceneJobs(ctx, s, queue, &totals)
					}
				}

				if len(j.input.MarkerIDs) > 0 {
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
		if j.input.Covers {
			logMsg += fmt.Sprintf(" %d covers", totals.covers)
		}
		if j.input.Sprites {
			logMsg += fmt.Sprintf(" %d sprites", totals.sprites)
		}
		if j.input.Previews {
			logMsg += fmt.Sprintf(" %d previews", totals.previews)
		}
		if j.input.ImagePreviews {
			logMsg += fmt.Sprintf(" %d image previews", totals.imagePreviews)
		}
		if j.input.Markers {
			logMsg += fmt.Sprintf(" %d markers", totals.markers)
		}
		if j.input.Transcodes {
			logMsg += fmt.Sprintf(" %d transcodes", totals.transcodes)
		}
		if j.input.Phashes {
			logMsg += fmt.Sprintf(" %d phashes", totals.phashes)
		}
		if j.input.InteractiveHeatmapsSpeeds {
			logMsg += fmt.Sprintf(" %d heatmaps & speeds", totals.interactiveHeatmapSpeeds)
		}
		if j.input.ClipPreviews {
			logMsg += fmt.Sprintf(" %d image clip previews", totals.clipPreviews)
		}
		if logMsg == "Generating" {
			logMsg = "Nothing selected to generate"
		}
		logger.Infof(logMsg)

		progress.SetTotal(int(totals.tasks))
	}()

	wg := sizedwaitgroup.New(j.parallelTasks)

	// Start measuring how long the generate has taken. (consider moving this up)
	start := time.Now()
	if err = j.paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("could not create temporary directory: %v", err)
	}

	defer func() {
		if err := j.paths.Generated.EmptyTmpDir(); err != nil {
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

	r := j.repository

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
	for more := j.input.ClipPreviews; more; {
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
	r := j.repository

	if j.input.Covers {
		task := &GenerateCoverTask{
			repository: r,
			Scene:      *scene,
			Overwrite:  j.overwrite,
		}

		if task.required(ctx) {
			totals.covers++
			totals.tasks++
			queue <- task
		}
	}

	if j.input.Sprites {
		task := &GenerateSpriteTask{
			Scene:               *scene,
			Overwrite:           j.overwrite,
			FileNamingAlgorithm: j.fileNamingAlgo,
			Paths:               j.paths,
			generator:           j.generator,
		}

		if task.required() {
			totals.sprites++
			totals.tasks++
			queue <- task
		}
	}

	generatePreviewOptions := j.input.PreviewOptions
	if generatePreviewOptions == nil {
		generatePreviewOptions = &models.GeneratePreviewOptionsInput{}
	}
	options := getGeneratePreviewOptions(*generatePreviewOptions)

	if j.input.Previews {
		task := &GeneratePreviewTask{
			Scene:               *scene,
			ImagePreview:        j.input.ImagePreviews,
			Options:             options,
			Overwrite:           j.overwrite,
			fileNamingAlgorithm: j.fileNamingAlgo,
			generator:           j.generator,
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

	if j.input.Markers {
		task := &GenerateMarkersTask{
			repository:          r,
			Scene:               scene,
			Overwrite:           j.overwrite,
			fileNamingAlgorithm: j.fileNamingAlgo,
			ImagePreview:        j.input.MarkerImagePreviews,
			Screenshot:          j.input.MarkerScreenshots,

			generator: j.generator,
		}

		markers := task.markersNeeded(ctx)
		if markers > 0 {
			totals.markers += int64(markers)
			totals.tasks++

			queue <- task
		}
	}

	if j.input.Transcodes {
		forceTranscode := j.input.ForceTranscodes
		task := &GenerateTranscodeTask{
			Scene:               *scene,
			Overwrite:           j.overwrite,
			Force:               forceTranscode,
			fileNamingAlgorithm: j.fileNamingAlgo,
			g:                   j.generator,
		}
		if task.required() {
			totals.transcodes++
			totals.tasks++
			queue <- task
		}
	}

	if j.input.Phashes {
		// generate for all files in scene
		for _, f := range scene.Files.List() {
			task := &GeneratePhashTask{
				repository:          r,
				File:                f,
				fileNamingAlgorithm: j.fileNamingAlgo,
				Overwrite:           j.overwrite,
			}

			if task.required() {
				totals.phashes++
				totals.tasks++
				queue <- task
			}
		}
	}

	if j.input.InteractiveHeatmapsSpeeds {
		task := &GenerateInteractiveHeatmapSpeedTask{
			repository:          r,
			Scene:               *scene,
			Overwrite:           j.overwrite,
			fileNamingAlgorithm: j.fileNamingAlgo,
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
		repository:          j.repository,
		Marker:              marker,
		Overwrite:           j.overwrite,
		fileNamingAlgorithm: j.fileNamingAlgo,
		generator:           j.generator,
	}
	totals.markers++
	totals.tasks++
	queue <- task
}

func (j *GenerateJob) queueImageJob(image *models.Image, queue chan<- Task, totals *totalsGenerate) {
	task := &GenerateClipPreviewTask{
		Image:         *image,
		Overwrite:     j.overwrite,
		PreviewPreset: j.previewPreset,
		Paths:         j.paths,
		generator:     j.generator,
	}

	if task.required() {
		totals.clipPreviews++
		totals.tasks++
		queue <- task
	}
}
