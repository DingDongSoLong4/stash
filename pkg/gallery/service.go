package gallery

import (
	"context"

	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/models"
)

type ImageService interface {
	Destroy(ctx context.Context, i *models.Image, fileDeleter *image.FileDeleter, deleteGenerated, deleteFile bool) error
	DestroyZipImages(ctx context.Context, zipFile models.File, fileDeleter *image.FileDeleter, deleteGenerated bool) ([]*models.Image, error)
}

type Service struct {
	Repository   models.GalleryReaderWriter
	ImageFinder  models.ImageReader
	ImageService ImageService
	File         models.FileReaderWriter
	Folder       models.FolderReaderWriter
}
