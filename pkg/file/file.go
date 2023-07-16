package file

import (
	"context"

	"github.com/stashapp/stash/pkg/models"
)

// Repository provides access to storage methods for files and folders.
type Repository struct {
	models.Database

	File   models.FileReaderWriter
	Folder models.FolderReaderWriter
}

func NewRepository(repo models.Repository) Repository {
	return Repository{
		Database: repo.Database,
		File:     repo.File,
		Folder:   repo.Folder,
	}
}

// Decorator wraps the Decorate method to add additional functionality while scanning files.
type Decorator interface {
	Decorate(ctx context.Context, fs models.FS, f models.File) (models.File, error)
	IsMissingMetadata(ctx context.Context, fs models.FS, f models.File) bool
}

type FilteredDecorator struct {
	Decorator
	Filter
}

// Decorate runs the decorator if the filter accepts the file.
func (d *FilteredDecorator) Decorate(ctx context.Context, fs models.FS, f models.File) (models.File, error) {
	if d.Accept(ctx, f) {
		return d.Decorator.Decorate(ctx, fs, f)
	}
	return f, nil
}

func (d *FilteredDecorator) IsMissingMetadata(ctx context.Context, fs models.FS, f models.File) bool {
	if d.Accept(ctx, f) {
		return d.Decorator.IsMissingMetadata(ctx, fs, f)
	}

	return false
}
