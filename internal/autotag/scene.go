package autotag

import (
	"context"

	"github.com/stashapp/stash/pkg/match"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/sliceutil/intslice"
)

func getSceneFileTagger(s *models.Scene, cache *match.Cache) tagger {
	return tagger{
		ID:    s.ID,
		Type:  "scene",
		Name:  s.DisplayName(),
		Path:  s.Path,
		cache: cache,
	}
}

// ScenePerformers tags the provided scene with performers whose name matches the scene's path.
func ScenePerformers(ctx context.Context, s *models.Scene, rw models.SceneReaderWriter, performerReader models.PerformerReader, cache *match.Cache) error {
	t := getSceneFileTagger(s, cache)

	return t.tagPerformers(ctx, performerReader, func(subjectID, otherID int) (bool, error) {
		if err := s.LoadPerformerIDs(ctx, rw); err != nil {
			return false, err
		}
		existing := s.PerformerIDs.List()

		if intslice.IntInclude(existing, otherID) {
			return false, nil
		}

		if err := scene.AddPerformer(ctx, rw, s, otherID); err != nil {
			return false, err
		}

		return true, nil
	})
}

// SceneStudios tags the provided scene with the first studio whose name matches the scene's path.
//
// Scenes will not be tagged if studio is already set.
func SceneStudios(ctx context.Context, s *models.Scene, rw models.SceneReaderWriter, studioReader models.StudioReader, cache *match.Cache) error {
	if s.StudioID != nil {
		// don't modify
		return nil
	}

	t := getSceneFileTagger(s, cache)

	return t.tagStudios(ctx, studioReader, func(subjectID, otherID int) (bool, error) {
		return addSceneStudio(ctx, rw, s, otherID)
	})
}

// SceneTags tags the provided scene with tags whose name matches the scene's path.
func SceneTags(ctx context.Context, s *models.Scene, rw models.SceneReaderWriter, tagReader models.TagReader, cache *match.Cache) error {
	t := getSceneFileTagger(s, cache)

	return t.tagTags(ctx, tagReader, func(subjectID, otherID int) (bool, error) {
		if err := s.LoadTagIDs(ctx, rw); err != nil {
			return false, err
		}
		existing := s.TagIDs.List()

		if intslice.IntInclude(existing, otherID) {
			return false, nil
		}

		if err := scene.AddTag(ctx, rw, s, otherID); err != nil {
			return false, err
		}

		return true, nil
	})
}
