package manager

import (
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

// MigrateHashTask renames generated files between oshash and MD5 based on the
// value of the fileNamingAlgorithm flag.
type MigrateHashTask struct {
	Scene *models.Scene

	FileNamingAlgorithm models.HashAlgorithm
}

// Start starts the task.
func (t *MigrateHashTask) Start() {
	if t.Scene.OSHash == "" || t.Scene.Checksum == "" {
		// nothing to do
		return
	}

	oshash := t.Scene.OSHash
	checksum := t.Scene.Checksum

	oldHash := oshash
	newHash := checksum
	if t.FileNamingAlgorithm == models.HashAlgorithmOshash {
		oldHash = checksum
		newHash = oshash
	}

	scene.MigrateHash(instance.Paths, oldHash, newHash)
}
