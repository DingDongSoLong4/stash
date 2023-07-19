package manager

import (
	"context"
	"errors"
	"net/http"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/internal/static"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/txn"
	"github.com/stashapp/stash/pkg/utils"
)

func (s *Manager) KillRunningStreams(scene *models.Scene) {
	s.ReadLockManager.Cancel(scene.Path)

	sceneHash := scene.GetHash(s.Config.GetVideoFileNamingAlgorithm())

	if sceneHash == "" {
		return
	}

	transcodePath := s.Paths.Scene.GetTranscodePath(sceneHash)
	s.ReadLockManager.Cancel(transcodePath)
}

type SceneServer struct {
	TxnManager       txn.Manager
	SceneCoverGetter models.SceneReader

	Config          *config.Config
	Paths           *paths.Paths
	SceneService    SceneService
	ReadLockManager *fsutil.ReadLockManager
}

func (s *SceneServer) StreamDirect(scene *models.Scene, w http.ResponseWriter, r *http.Request) {
	// #3526 - return 404 if the scene does not have any files
	if scene.Path == "" {
		http.Error(w, http.StatusText(404), 404)
		return
	}

	sceneHash := scene.GetHash(s.Config.GetVideoFileNamingAlgorithm())

	filepath := s.Paths.Scene.GetStreamPath(scene.Path, sceneHash)
	streamRequestCtx := ffmpeg.NewStreamRequestContext(w, r)

	// #2579 - hijacking and closing the connection here causes video playback to fail in Safari
	// We trust that the request context will be closed, so we don't need to call Cancel on the
	// returned context here.
	_ = s.ReadLockManager.ReadLock(streamRequestCtx, filepath)
	http.ServeFile(w, r, filepath)
}

func (s *SceneServer) ServeScreenshot(scene *models.Scene, w http.ResponseWriter, r *http.Request) {
	var cover []byte
	readTxnErr := txn.WithReadTxn(r.Context(), s.TxnManager, func(ctx context.Context) error {
		cover, _ = s.SceneCoverGetter.GetCover(ctx, scene.ID)
		return nil
	})
	if errors.Is(readTxnErr, context.Canceled) {
		return
	}
	if readTxnErr != nil {
		logger.Warnf("read transaction error on fetch screenshot: %v", readTxnErr)
	}

	if cover == nil {
		// fallback to legacy image if present
		if scene.Path != "" {
			sceneHash := scene.GetHash(s.Config.GetVideoFileNamingAlgorithm())
			filepath := s.Paths.Scene.GetLegacyScreenshotPath(sceneHash)

			// fall back to the scene image blob if the file isn't present
			screenshotExists, _ := fsutil.FileExists(filepath)
			if screenshotExists {
				if r.URL.Query().Has("t") {
					w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				http.ServeFile(w, r, filepath)
				return
			}
		}

		// fallback to default cover if none found
		cover = static.ReadAll(static.DefaultSceneImage)
	}

	utils.ServeImage(w, r, cover)
}
