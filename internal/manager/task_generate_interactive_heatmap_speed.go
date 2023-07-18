package manager

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/file/video"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/generate"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
)

type GenerateInteractiveHeatmapSpeedTask struct {
	Scene     models.Scene
	DrawRange bool
	Overwrite bool

	repository          models.Repository
	fileNamingAlgorithm models.HashAlgorithm
	Paths               *paths.Paths
}

func (t *GenerateInteractiveHeatmapSpeedTask) GetDescription() string {
	return fmt.Sprintf("Generating heatmap and interactive speed for %s", t.Scene.Path)
}

func (t *GenerateInteractiveHeatmapSpeedTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	videoFile := t.Scene.Files.Primary()
	if videoFile == nil {
		return
	}

	videoChecksum := t.Scene.GetHash(t.fileNamingAlgorithm)
	funscriptPath := video.GetFunscriptPath(t.Scene.Path)
	heatmapPath := t.Paths.Scene.GetInteractiveHeatmapPath(videoChecksum)
	duration := videoFile.Duration

	median, err := generate.GenerateInteractiveHeatmapSpeed(funscriptPath, heatmapPath, duration, t.DrawRange)
	if err != nil {
		logger.Errorf("error generating heatmap: %s", err.Error())
		return
	}

	r := t.repository
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		videoFile.InteractiveSpeed = &median
		return r.File.Update(ctx, videoFile)
	}); err != nil && ctx.Err() == nil {
		logger.Error(err.Error())
	}
}

func (t *GenerateInteractiveHeatmapSpeedTask) required() bool {
	primaryFile := t.Scene.Files.Primary()
	if primaryFile == nil || !primaryFile.Interactive {
		return false
	}

	if t.Overwrite {
		return true
	}

	sceneHash := t.Scene.GetHash(t.fileNamingAlgorithm)
	return !t.doesHeatmapExist(sceneHash) || primaryFile.InteractiveSpeed == nil
}

func (t *GenerateInteractiveHeatmapSpeedTask) doesHeatmapExist(sceneChecksum string) bool {
	if sceneChecksum == "" {
		return false
	}

	imageExists, _ := fsutil.FileExists(instance.Paths.Scene.GetInteractiveHeatmapPath(sceneChecksum))
	return imageExists
}
