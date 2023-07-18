package manager

import (
	"github.com/stashapp/stash/pkg/generate"
)

func (s *Manager) NewGenerator(overwrite bool) *generate.Generator {
	return &generate.Generator{
		Encoder:     s.FFMpeg,
		Config:      s.Config,
		FFProbe:     s.FFProbe,
		LockManager: s.ReadLockManager,
		Paths:       s.Paths,
		Overwrite:   overwrite,
	}
}
