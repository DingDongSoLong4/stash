package manager

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/generate"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
)

type GenerateClipPreviewTask struct {
	Image     models.Image
	Overwrite bool

	PreviewPreset models.PreviewPreset
	Paths         *paths.Paths
	generator     *generate.Generator
}

func (t *GenerateClipPreviewTask) GetDescription() string {
	return fmt.Sprintf("Generating Preview for image Clip %s", t.Image.Path)
}

func (t *GenerateClipPreviewTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	ffprobe := instance.FFProbe
	videoFile, err := ffprobe.NewVideoFile(t.Image.Path)
	if err != nil {
		logger.Errorf("error reading video file: %v", err)
		return
	}

	checksum := t.Image.Checksum
	err = t.generator.ClipPreview(context.TODO(), videoFile, checksum, models.DefaultGthumbWidth, t.PreviewPreset)
	if err != nil {
		logger.Errorf("error generating image preview: %v", err)
		return
	}
}

func (t *GenerateClipPreviewTask) required() bool {
	_, ok := t.Image.Files.Primary().(*models.VideoFile)
	if !ok {
		return false
	}

	if t.Overwrite {
		return true
	}

	checksum := t.Image.Checksum
	if checksum == "" {
		return false
	}

	prevPath := t.Paths.Generated.GetClipPreviewPath(checksum, models.DefaultGthumbWidth)
	if exists, _ := fsutil.FileExists(prevPath); exists {
		return false
	}

	return true
}
