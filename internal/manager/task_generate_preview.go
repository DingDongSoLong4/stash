package manager

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/generate"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
)

type GeneratePreviewTask struct {
	Scene        models.Scene
	Options      generate.PreviewOptions
	ImagePreview bool
	Overwrite    bool

	Paths               *paths.Paths
	FileNamingAlgorithm models.HashAlgorithm
	FFProbe             *ffmpeg.FFProbe
	Generator           *generate.Generator
}

func (t *GeneratePreviewTask) GetDescription() string {
	return fmt.Sprintf("Generating preview for %s", t.Scene.Path)
}

func (t *GeneratePreviewTask) Start(ctx context.Context) {
	videoChecksum := t.Scene.GetHash(t.FileNamingAlgorithm)

	if t.videoPreviewRequired() {
		videoFile, err := t.FFProbe.NewVideoFile(t.Scene.Path)
		if err != nil {
			logger.Errorf("error reading video file: %v", err)
			return
		}

		if err := t.generateVideo(videoChecksum, videoFile.VideoStreamDuration, videoFile.FrameRate); err != nil {
			logger.Errorf("error generating preview: %v", err)
			logErrorOutput(err)
			return
		}
	}

	if t.imagePreviewRequired() {
		if err := t.generateWebp(videoChecksum); err != nil {
			logger.Errorf("error generating preview webp: %v", err)
			logErrorOutput(err)
		}
	}
}

func (t *GeneratePreviewTask) generateVideo(videoChecksum string, videoDuration float64, videoFrameRate float64) error {
	videoFilename := t.Scene.Path
	useVsync2 := false

	if videoFrameRate <= 0.01 {
		logger.Errorf("[generator] Video framerate very low/high (%f) most likely vfr so using -vsync 2", videoFrameRate)
		useVsync2 = true
	}

	if err := t.Generator.PreviewVideo(context.TODO(), videoFilename, videoDuration, videoChecksum, t.Options, false, useVsync2); err != nil {
		logger.Warnf("[generator] failed generating scene preview, trying fallback")
		if err := t.Generator.PreviewVideo(context.TODO(), videoFilename, videoDuration, videoChecksum, t.Options, true, useVsync2); err != nil {
			return err
		}
	}

	return nil
}

func (t *GeneratePreviewTask) generateWebp(videoChecksum string) error {
	videoFilename := t.Scene.Path
	return t.Generator.PreviewWebp(context.TODO(), videoFilename, videoChecksum)
}

func (t *GeneratePreviewTask) required() bool {
	return t.videoPreviewRequired() || t.imagePreviewRequired()
}

func (t *GeneratePreviewTask) videoPreviewRequired() bool {
	if t.Scene.Path == "" {
		return false
	}

	if t.Overwrite {
		return true
	}

	sceneChecksum := t.Scene.GetHash(t.FileNamingAlgorithm)
	if sceneChecksum == "" {
		return false
	}

	exists, _ := fsutil.FileExists(t.Paths.Scene.GetVideoPreviewPath(sceneChecksum))
	return !exists
}

func (t *GeneratePreviewTask) imagePreviewRequired() bool {
	if !t.ImagePreview {
		return false
	}

	if t.Scene.Path == "" {
		return false
	}

	if t.Overwrite {
		return true
	}

	sceneChecksum := t.Scene.GetHash(t.FileNamingAlgorithm)
	if sceneChecksum == "" {
		return false
	}

	exists, _ := fsutil.FileExists(t.Paths.Scene.GetWebpPreviewPath(sceneChecksum))
	return !exists
}
