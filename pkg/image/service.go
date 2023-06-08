package image

import (
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
)

type Service struct {
	File       file.Store
	Repository models.ImageReaderWriter
}
