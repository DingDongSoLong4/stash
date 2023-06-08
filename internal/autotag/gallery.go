package autotag

import (
	"context"

	"github.com/stashapp/stash/pkg/gallery"
	"github.com/stashapp/stash/pkg/match"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sliceutil/intslice"
)

func getGalleryFileTagger(s *models.Gallery, cache *match.Cache) tagger {
	var path string
	if s.Path != "" {
		path = s.Path
	}

	// only trim the extension if gallery is file-based
	trimExt := s.PrimaryFileID != nil

	return tagger{
		ID:      s.ID,
		Type:    "gallery",
		Name:    s.DisplayName(),
		Path:    path,
		trimExt: trimExt,
		cache:   cache,
	}
}

// GalleryPerformers tags the provided gallery with performers whose name matches the gallery's path.
func GalleryPerformers(ctx context.Context, s *models.Gallery, rw models.GalleryReaderWriter, performerReader models.PerformerReader, cache *match.Cache) error {
	t := getGalleryFileTagger(s, cache)

	return t.tagPerformers(ctx, performerReader, func(subjectID, otherID int) (bool, error) {
		if err := s.LoadPerformerIDs(ctx, rw); err != nil {
			return false, err
		}
		existing := s.PerformerIDs.List()

		if intslice.IntInclude(existing, otherID) {
			return false, nil
		}

		if err := gallery.AddPerformer(ctx, rw, s, otherID); err != nil {
			return false, err
		}

		return true, nil
	})
}

// GalleryStudios tags the provided gallery with the first studio whose name matches the gallery's path.
//
// Gallerys will not be tagged if studio is already set.
func GalleryStudios(ctx context.Context, s *models.Gallery, rw models.GalleryReaderWriter, studioReader models.StudioReader, cache *match.Cache) error {
	if s.StudioID != nil {
		// don't modify
		return nil
	}

	t := getGalleryFileTagger(s, cache)

	return t.tagStudios(ctx, studioReader, func(subjectID, otherID int) (bool, error) {
		return addGalleryStudio(ctx, rw, s, otherID)
	})
}

// GalleryTags tags the provided gallery with tags whose name matches the gallery's path.
func GalleryTags(ctx context.Context, s *models.Gallery, rw models.GalleryReaderWriter, tagReader models.TagReader, cache *match.Cache) error {
	t := getGalleryFileTagger(s, cache)

	return t.tagTags(ctx, tagReader, func(subjectID, otherID int) (bool, error) {
		if err := s.LoadTagIDs(ctx, rw); err != nil {
			return false, err
		}
		existing := s.TagIDs.List()

		if intslice.IntInclude(existing, otherID) {
			return false, nil
		}

		if err := gallery.AddTag(ctx, rw, s, otherID); err != nil {
			return false, err
		}

		return true, nil
	})
}
