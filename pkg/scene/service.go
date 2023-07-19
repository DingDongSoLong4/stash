package scene

import (
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
)

type Config interface {
	GetVideoFileNamingAlgorithm() models.HashAlgorithm
	GetMaxStreamingTranscodeSize() models.StreamingResolutionEnum
}

type Service struct {
	File             models.FileReaderWriter
	Repository       models.SceneReaderWriter
	MarkerRepository models.SceneMarkerReaderWriter

	Config      Config
	FFProbe     *ffmpeg.FFProbe
	Paths       *paths.Paths
	PluginCache *plugin.Cache
}
