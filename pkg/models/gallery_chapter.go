package models

import "context"

type GalleryChapterFinder interface {
	// TODO - rename this to Find and remove existing method
	FindMany(ctx context.Context, ids []int) ([]*GalleryChapter, error)
}

type GalleryChapterReader interface {
	GalleryChapterFinder
	Find(ctx context.Context, id int) (*GalleryChapter, error)
	FindByGalleryID(ctx context.Context, galleryID int) ([]*GalleryChapter, error)
}

type GalleryChapterWriter interface {
	Create(ctx context.Context, newGalleryChapter *GalleryChapter) error
	Update(ctx context.Context, updatedGalleryChapter *GalleryChapter) error
	Destroy(ctx context.Context, id int) error
}

type GalleryChapterReaderWriter interface {
	GalleryChapterReader
	GalleryChapterWriter
}
