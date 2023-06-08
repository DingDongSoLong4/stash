package scene

import (
	"context"

	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
)

type MarkerFinder interface {
	FindBySceneID(ctx context.Context, sceneID int) ([]*models.SceneMarker, error)
}

type MarkerDestroyer interface {
	FindBySceneID(ctx context.Context, sceneID int) ([]*models.SceneMarker, error)
	Destroy(ctx context.Context, id int) error
}

type Config interface {
	GetVideoFileNamingAlgorithm() models.HashAlgorithm
}

type MarkerRepository interface {
	MarkerFinder
	MarkerDestroyer

	Update(ctx context.Context, updatedObject *models.SceneMarker) error
}

type Service struct {
	File             file.Store
	Repository       models.SceneReaderWriter
	MarkerRepository MarkerRepository
	PluginCache      *plugin.Cache

	Paths  *paths.Paths
	Config Config
}
