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

type GenerateSpriteTask struct {
	Scene     models.Scene
	Overwrite bool

	Paths               *paths.Paths
	FileNamingAlgorithm models.HashAlgorithm
	Generator           *generate.Generator
}

func (t *GenerateSpriteTask) GetDescription() string {
	return fmt.Sprintf("Generating sprites for %s", t.Scene.Path)
}

func (t *GenerateSpriteTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	ffprobe := instance.FFProbe
	videoFile, err := ffprobe.NewVideoFile(t.Scene.Path)
	if err != nil {
		logger.Errorf("error reading video file: %v", err)
		return
	}

	videoHash := t.Scene.GetHash(t.FileNamingAlgorithm)

	if t.spriteImageRequired() {
		if err := t.Generator.SpriteImage(context.TODO(), videoFile, videoHash); err != nil {
			logger.Errorf("error generating sprite image: %v", err)
			logErrorOutput(err)
			return
		}
	}

	if t.spriteVTTRequired() {
		if err := t.Generator.SpriteVTT(context.TODO(), videoFile, videoHash); err != nil {
			logger.Errorf("error generating sprite vtt: %v", err)
			logErrorOutput(err)
			return
		}
	}
}

// required returns true if the sprite needs to be generated
func (t *GenerateSpriteTask) required() bool {
	return t.spriteImageRequired() || t.spriteVTTRequired()
}

func (t *GenerateSpriteTask) spriteImageRequired() bool {
	if t.Scene.Path == "" {
		return false
	}

	if t.Overwrite {
		return true
	}

	sceneHash := t.Scene.GetHash(t.FileNamingAlgorithm)
	if sceneHash == "" {
		return false
	}

	exists, _ := fsutil.FileExists(t.Paths.Scene.GetSpriteImageFilePath(sceneHash))
	return !exists
}

func (t *GenerateSpriteTask) spriteVTTRequired() bool {
	if t.Scene.Path == "" {
		return false
	}

	if t.Overwrite {
		return true
	}

	sceneHash := t.Scene.GetHash(t.FileNamingAlgorithm)
	if sceneHash == "" {
		return false
	}

	exists, _ := fsutil.FileExists(t.Paths.Scene.GetSpriteVttFilePath(sceneHash))
	return !exists
}
