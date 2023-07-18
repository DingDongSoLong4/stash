package manager

import (
	"github.com/stashapp/stash/pkg/generate"
)

func (s *Manager) NewGenerator(overwrite bool) *generate.Generator {
	return &generate.Generator{
		Encoder:      s.FFMpeg,
		FFMpegConfig: s.Config,
		FFProbe:      s.FFProbe,
		LockManager:  s.ReadLockManager,
		MarkerPaths:  s.Paths.SceneMarkers,
		ScenePaths:   s.Paths.Scene,
		Overwrite:    overwrite,
	}
}
