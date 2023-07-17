package paths

import (
	"path/filepath"
	"strconv"
)

type sceneMarkerPaths struct {
	*generatedPaths
}

func newSceneMarkerPaths(gp *generatedPaths) *sceneMarkerPaths {
	return &sceneMarkerPaths{
		generatedPaths: gp,
	}
}

func (sp *sceneMarkerPaths) GetVideoPreviewPath(checksum string, seconds int) string {
	return filepath.Join(sp.Markers, checksum, strconv.Itoa(seconds)+".mp4")
}

func (sp *sceneMarkerPaths) GetWebpPreviewPath(checksum string, seconds int) string {
	return filepath.Join(sp.Markers, checksum, strconv.Itoa(seconds)+".webp")
}

func (sp *sceneMarkerPaths) GetScreenshotPath(checksum string, seconds int) string {
	return filepath.Join(sp.Markers, checksum, strconv.Itoa(seconds)+".jpg")
}
