package gallery

import (
	"context"

	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/models"
)

type ImageService interface {
	Destroy(ctx context.Context, i *models.Image, fileDeleter *image.FileDeleter, deleteGenerated, deleteFile bool) error
	DestroyZipImages(ctx context.Context, zipFile file.File, fileDeleter *image.FileDeleter, deleteGenerated bool) ([]*models.Image, error)
}

type ChapterRepository interface {
	ChapterFinder
	ChapterDestroyer

	Update(ctx context.Context, updatedObject models.GalleryChapter) (*models.GalleryChapter, error)
}

type Service struct {
	Repository   models.GalleryReaderWriter
	ImageFinder  models.ImageReader
	ImageService ImageService
	File         file.Store
	Folder       file.FolderStore
}
