package paths

import (
	"path/filepath"

	"github.com/stashapp/stash/pkg/fsutil"
)

type scenePaths struct {
	gp *generatedPaths
}

func newScenePaths(gp *generatedPaths) *scenePaths {
	return &scenePaths{
		gp: gp,
	}
}

func (sp *scenePaths) GetLegacyScreenshotPath(checksum string) string {
	return filepath.Join(sp.gp.Screenshots, checksum+".jpg")
}

func (sp *scenePaths) GetTranscodePath(checksum string) string {
	return filepath.Join(sp.gp.Transcodes, checksum+".mp4")
}

func (sp *scenePaths) GetStreamPath(scenePath string, checksum string) string {
	transcodePath := sp.GetTranscodePath(checksum)
	transcodeExists, _ := fsutil.FileExists(transcodePath)
	if transcodeExists {
		return transcodePath
	}
	return scenePath
}

func (sp *scenePaths) GetVideoPreviewPath(checksum string) string {
	return filepath.Join(sp.gp.Screenshots, checksum+".mp4")
}

func (sp *scenePaths) GetWebpPreviewPath(checksum string) string {
	return filepath.Join(sp.gp.Screenshots, checksum+".webp")
}

func (sp *scenePaths) GetSpriteImageFilePath(checksum string) string {
	return filepath.Join(sp.gp.Vtt, checksum+"_sprite.jpg")
}

func (sp *scenePaths) GetSpriteVttFilePath(checksum string) string {
	return filepath.Join(sp.gp.Vtt, checksum+"_thumbs.vtt")
}

func (sp *scenePaths) GetInteractiveHeatmapPath(checksum string) string {
	return filepath.Join(sp.gp.InteractiveHeatmap, checksum+".png")
}
