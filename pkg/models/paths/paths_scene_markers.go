package paths

import (
	"path/filepath"
	"strconv"
)

type sceneMarkerPaths struct {
	gp *generatedPaths
}

func newSceneMarkerPaths(gp *generatedPaths) *sceneMarkerPaths {
	return &sceneMarkerPaths{
		gp: gp,
	}
}

func (sp *sceneMarkerPaths) GetVideoPreviewPath(checksum string, seconds int) string {
	return filepath.Join(sp.gp.Markers, checksum, strconv.Itoa(seconds)+".mp4")
}

func (sp *sceneMarkerPaths) GetWebpPreviewPath(checksum string, seconds int) string {
	return filepath.Join(sp.gp.Markers, checksum, strconv.Itoa(seconds)+".webp")
}

func (sp *sceneMarkerPaths) GetScreenshotPath(checksum string, seconds int) string {
	return filepath.Join(sp.gp.Markers, checksum, strconv.Itoa(seconds)+".jpg")
}
