package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/plugin"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/sliceutil/intslice"
	"github.com/stashapp/stash/pkg/sliceutil/stringslice"
	"github.com/stashapp/stash/pkg/utils"
)

func (r *mutationResolver) getScene(ctx context.Context, id int) (ret *models.Scene, err error) {
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Scene.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneCreate(ctx context.Context, input models.SceneCreateInput) (ret *models.Scene, err error) {
	translator := changesetTranslator{
		inputMap: getUpdateInputMap(ctx),
	}

	fileIDs, err := translator.fileIDSliceFromStringSlice(input.FileIds, "file_ids")
	if err != nil {
		return nil, fmt.Errorf("converting file ids: %w", err)
	}

	// Populate a new scene from the input
	newScene := models.NewScene()

	newScene.Title = translator.string(input.Title, "title")
	newScene.Code = translator.string(input.Code, "code")
	newScene.Details = translator.string(input.Details, "details")
	newScene.Director = translator.string(input.Director, "director")
	newScene.Rating = translator.ratingConversionInt(input.Rating, input.Rating100)
	newScene.Organized = translator.bool(input.Organized, "organized")
	newScene.StashIDs = models.NewRelatedStashIDs(input.StashIds)

	newScene.Date, err = translator.datePtr(input.Date, "date")
	if err != nil {
		return nil, fmt.Errorf("converting date: %w", err)
	}
	newScene.StudioID, err = translator.intPtrFromString(input.StudioID, "studio_id")
	if err != nil {
		return nil, fmt.Errorf("converting studio id: %w", err)
	}

	if input.Urls != nil {
		newScene.URLs = models.NewRelatedStrings(input.Urls)
	} else if input.URL != nil {
		newScene.URLs = models.NewRelatedStrings([]string{*input.URL})
	}

	newScene.PerformerIDs, err = translator.relatedIds(input.PerformerIds, "performer_ids")
	if err != nil {
		return nil, fmt.Errorf("converting performer ids: %w", err)
	}
	newScene.TagIDs, err = translator.relatedIds(input.TagIds, "tag_ids")
	if err != nil {
		return nil, fmt.Errorf("converting tag ids: %w", err)
	}
	newScene.GalleryIDs, err = translator.relatedIds(input.GalleryIds, "gallery_ids")
	if err != nil {
		return nil, fmt.Errorf("converting gallery ids: %w", err)
	}

	newScene.Movies, err = translator.relatedMovies(input.Movies, "movies")
	if err != nil {
		return nil, fmt.Errorf("converting movies: %w", err)
	}

	var coverImageData []byte
	if input.CoverImage != nil {
		var err error
		coverImageData, err = utils.ProcessImageInput(ctx, *input.CoverImage)
		if err != nil {
			return nil, err
		}
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.Resolver.sceneService.Create(ctx, &newScene, fileIDs, coverImageData)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneUpdate(ctx context.Context, input models.SceneUpdateInput) (ret *models.Scene, err error) {
	translator := changesetTranslator{
		inputMap: getUpdateInputMap(ctx),
	}

	// Start the transaction and save the scene
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.sceneUpdate(ctx, input, translator)
		return err
	}); err != nil {
		return nil, err
	}

	r.hookExecutor.ExecutePostHooks(ctx, ret.ID, plugin.SceneUpdatePost, input, translator.getFields())
	return r.getScene(ctx, ret.ID)
}

func (r *mutationResolver) ScenesUpdate(ctx context.Context, input []*models.SceneUpdateInput) (ret []*models.Scene, err error) {
	inputMaps := getUpdateInputMaps(ctx)

	// Start the transaction and save the scenes
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		for i, scene := range input {
			translator := changesetTranslator{
				inputMap: inputMaps[i],
			}

			thisScene, err := r.sceneUpdate(ctx, *scene, translator)
			if err != nil {
				return err
			}

			ret = append(ret, thisScene)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// execute post hooks outside of txn
	var newRet []*models.Scene
	for i, scene := range ret {
		translator := changesetTranslator{
			inputMap: inputMaps[i],
		}

		r.hookExecutor.ExecutePostHooks(ctx, scene.ID, plugin.SceneUpdatePost, input, translator.getFields())

		scene, err = r.getScene(ctx, scene.ID)
		if err != nil {
			return nil, err
		}

		newRet = append(newRet, scene)
	}

	return newRet, nil
}

func scenePartialFromInput(input models.SceneUpdateInput, translator changesetTranslator) (*models.ScenePartial, error) {
	updatedScene := models.NewScenePartial()

	updatedScene.Title = translator.optionalString(input.Title, "title")
	updatedScene.Code = translator.optionalString(input.Code, "code")
	updatedScene.Details = translator.optionalString(input.Details, "details")
	updatedScene.Director = translator.optionalString(input.Director, "director")
	updatedScene.Rating = translator.ratingConversionOptional(input.Rating, input.Rating100)
	updatedScene.OCounter = translator.optionalInt(input.OCounter, "o_counter")
	updatedScene.PlayCount = translator.optionalInt(input.PlayCount, "play_count")
	updatedScene.PlayDuration = translator.optionalFloat64(input.PlayDuration, "play_duration")
	updatedScene.Organized = translator.optionalBool(input.Organized, "organized")
	updatedScene.StashIDs = translator.updateStashIDs(input.StashIds, "stash_ids")

	var err error

	updatedScene.Date, err = translator.optionalDate(input.Date, "date")
	if err != nil {
		return nil, fmt.Errorf("converting date: %w", err)
	}
	updatedScene.StudioID, err = translator.optionalIntFromString(input.StudioID, "studio_id")
	if err != nil {
		return nil, fmt.Errorf("converting studio id: %w", err)
	}

	if translator.hasField("urls") {
		updatedScene.URLs = &models.UpdateStrings{
			Values: input.Urls,
			Mode:   models.RelationshipUpdateModeSet,
		}
	} else if translator.hasField("url") {
		updatedScene.URLs = &models.UpdateStrings{
			Values: []string{*input.URL},
			Mode:   models.RelationshipUpdateModeSet,
		}
	}

	updatedScene.PrimaryFileID, err = translator.fileIDPtrFromString(input.PrimaryFileID, "primary_file_id")
	if err != nil {
		return nil, fmt.Errorf("converting primary file id: %w", err)
	}

	updatedScene.PerformerIDs, err = translator.updateIds(input.PerformerIds, "performer_ids")
	if err != nil {
		return nil, fmt.Errorf("converting performer ids: %w", err)
	}
	updatedScene.TagIDs, err = translator.updateIds(input.TagIds, "tag_ids")
	if err != nil {
		return nil, fmt.Errorf("converting tag ids: %w", err)
	}
	updatedScene.GalleryIDs, err = translator.updateIds(input.GalleryIds, "gallery_ids")
	if err != nil {
		return nil, fmt.Errorf("converting gallery ids: %w", err)
	}

	updatedScene.MovieIDs, err = translator.updateMovieIDs(input.Movies, "movies")
	if err != nil {
		return nil, fmt.Errorf("converting movies: %w", err)
	}

	return &updatedScene, nil
}

func (r *mutationResolver) sceneUpdate(ctx context.Context, input models.SceneUpdateInput, translator changesetTranslator) (*models.Scene, error) {
	sceneID, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, err
	}

	qb := r.repository.Scene

	originalScene, err := qb.Find(ctx, sceneID)
	if err != nil {
		return nil, err
	}

	if originalScene == nil {
		return nil, fmt.Errorf("scene with id %d not found", sceneID)
	}

	// Populate scene from the input
	updatedScene, err := scenePartialFromInput(input, translator)
	if err != nil {
		return nil, err
	}

	// ensure that title is set where scene has no file
	if updatedScene.Title.Set && updatedScene.Title.Value == "" {
		if err := originalScene.LoadFiles(ctx, r.repository.Scene); err != nil {
			return nil, err
		}

		if len(originalScene.Files.List()) == 0 {
			return nil, errors.New("title must be set if scene has no files")
		}
	}

	if updatedScene.PrimaryFileID != nil {
		newPrimaryFileID := *updatedScene.PrimaryFileID

		// if file hash has changed, we should migrate generated files
		// after commit
		if err := originalScene.LoadFiles(ctx, r.repository.Scene); err != nil {
			return nil, err
		}

		// ensure that new primary file is associated with scene
		var f *models.VideoFile
		for _, ff := range originalScene.Files.List() {
			if ff.ID == newPrimaryFileID {
				f = ff
			}
		}

		if f == nil {
			return nil, fmt.Errorf("file with id %d not associated with scene", newPrimaryFileID)
		}
	}

	var coverImageData []byte
	if input.CoverImage != nil {
		var err error
		coverImageData, err = utils.ProcessImageInput(ctx, *input.CoverImage)
		if err != nil {
			return nil, err
		}
	}

	scene, err := qb.UpdatePartial(ctx, sceneID, *updatedScene)
	if err != nil {
		return nil, err
	}

	if err := r.sceneUpdateCoverImage(ctx, scene, coverImageData); err != nil {
		return nil, err
	}

	return scene, nil
}

func (r *mutationResolver) sceneUpdateCoverImage(ctx context.Context, s *models.Scene, coverImageData []byte) error {
	if len(coverImageData) > 0 {
		qb := r.repository.Scene

		// update cover table
		if err := qb.UpdateCover(ctx, s.ID, coverImageData); err != nil {
			return err
		}
	}

	return nil
}

func (r *mutationResolver) BulkSceneUpdate(ctx context.Context, input BulkSceneUpdateInput) ([]*models.Scene, error) {
	sceneIDs, err := stringslice.StringSliceToIntSlice(input.Ids)
	if err != nil {
		return nil, err
	}

	translator := changesetTranslator{
		inputMap: getUpdateInputMap(ctx),
	}

	// Populate scene from the input
	updatedScene := models.NewScenePartial()

	updatedScene.Title = translator.optionalString(input.Title, "title")
	updatedScene.Code = translator.optionalString(input.Code, "code")
	updatedScene.Details = translator.optionalString(input.Details, "details")
	updatedScene.Director = translator.optionalString(input.Director, "director")
	updatedScene.Rating = translator.ratingConversionOptional(input.Rating, input.Rating100)
	updatedScene.Organized = translator.optionalBool(input.Organized, "organized")

	updatedScene.Date, err = translator.optionalDate(input.Date, "date")
	if err != nil {
		return nil, fmt.Errorf("converting date: %w", err)
	}
	updatedScene.StudioID, err = translator.optionalIntFromString(input.StudioID, "studio_id")
	if err != nil {
		return nil, fmt.Errorf("converting studio id: %w", err)
	}

	if translator.hasField("urls") {
		updatedScene.URLs = &models.UpdateStrings{
			Values: input.Urls.Values,
			Mode:   input.Urls.Mode,
		}
	} else if translator.hasField("url") {
		updatedScene.URLs = &models.UpdateStrings{
			Values: []string{*input.URL},
			Mode:   models.RelationshipUpdateModeSet,
		}
	}

	updatedScene.PerformerIDs, err = translator.updateIdsBulk(input.PerformerIds, "performer_ids")
	if err != nil {
		return nil, fmt.Errorf("converting performer ids: %w", err)
	}
	updatedScene.TagIDs, err = translator.updateIdsBulk(input.TagIds, "tag_ids")
	if err != nil {
		return nil, fmt.Errorf("converting tag ids: %w", err)
	}
	updatedScene.GalleryIDs, err = translator.updateIdsBulk(input.GalleryIds, "gallery_ids")
	if err != nil {
		return nil, fmt.Errorf("converting gallery ids: %w", err)
	}

	updatedScene.MovieIDs, err = translator.updateMovieIDsBulk(input.MovieIds, "movie_ids")
	if err != nil {
		return nil, fmt.Errorf("converting movie ids: %w", err)
	}

	ret := []*models.Scene{}

	// Start the transaction and save the scenes
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		for _, sceneID := range sceneIDs {
			scene, err := qb.UpdatePartial(ctx, sceneID, updatedScene)
			if err != nil {
				return err
			}

			ret = append(ret, scene)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// execute post hooks outside of txn
	var newRet []*models.Scene
	for _, scene := range ret {
		r.hookExecutor.ExecutePostHooks(ctx, scene.ID, plugin.SceneUpdatePost, input, translator.getFields())

		scene, err = r.getScene(ctx, scene.ID)
		if err != nil {
			return nil, err
		}

		newRet = append(newRet, scene)
	}

	return newRet, nil
}

func (r *mutationResolver) SceneDestroy(ctx context.Context, input models.SceneDestroyInput) (bool, error) {
	sceneID, err := strconv.Atoi(input.ID)
	if err != nil {
		return false, err
	}

	fileNamingAlgo := r.config.GetVideoFileNamingAlgorithm()

	var s *models.Scene
	fileDeleter := &scene.FileDeleter{
		Deleter:        file.NewDeleter(),
		FileNamingAlgo: fileNamingAlgo,
		Paths:          r.manager.Paths,
	}

	deleteGenerated := utils.IsTrue(input.DeleteGenerated)
	deleteFile := utils.IsTrue(input.DeleteFile)

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene
		var err error
		s, err = qb.Find(ctx, sceneID)
		if err != nil {
			return err
		}

		if s == nil {
			return fmt.Errorf("scene with id %d not found", sceneID)
		}

		// kill any running encoders
		r.manager.KillRunningStreams(s)

		return r.sceneService.Destroy(ctx, s, fileDeleter, deleteGenerated, deleteFile)
	}); err != nil {
		fileDeleter.Rollback()
		return false, err
	}

	// perform the post-commit actions
	fileDeleter.Commit()

	// call post hook after performing the other actions
	r.hookExecutor.ExecutePostHooks(ctx, s.ID, plugin.SceneDestroyPost, plugin.SceneDestroyInput{
		SceneDestroyInput: input,
		Checksum:          s.Checksum,
		OSHash:            s.OSHash,
		Path:              s.Path,
	}, nil)

	return true, nil
}

func (r *mutationResolver) ScenesDestroy(ctx context.Context, input models.ScenesDestroyInput) (bool, error) {
	var scenes []*models.Scene
	fileNamingAlgo := r.config.GetVideoFileNamingAlgorithm()

	fileDeleter := &scene.FileDeleter{
		Deleter:        file.NewDeleter(),
		FileNamingAlgo: fileNamingAlgo,
		Paths:          r.manager.Paths,
	}

	deleteGenerated := utils.IsTrue(input.DeleteGenerated)
	deleteFile := utils.IsTrue(input.DeleteFile)

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		for _, id := range input.Ids {
			sceneID, _ := strconv.Atoi(id)

			s, err := qb.Find(ctx, sceneID)
			if err != nil {
				return err
			}
			if s == nil {
				return fmt.Errorf("scene with id %d not found", sceneID)
			}

			scenes = append(scenes, s)

			// kill any running encoders
			r.manager.KillRunningStreams(s)

			if err := r.sceneService.Destroy(ctx, s, fileDeleter, deleteGenerated, deleteFile); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		fileDeleter.Rollback()
		return false, err
	}

	// perform the post-commit actions
	fileDeleter.Commit()

	for _, scene := range scenes {
		// call post hook after performing the other actions
		r.hookExecutor.ExecutePostHooks(ctx, scene.ID, plugin.SceneDestroyPost, plugin.ScenesDestroyInput{
			ScenesDestroyInput: input,
			Checksum:           scene.Checksum,
			OSHash:             scene.OSHash,
			Path:               scene.Path,
		}, nil)
	}

	return true, nil
}

func (r *mutationResolver) SceneAssignFile(ctx context.Context, input AssignSceneFileInput) (bool, error) {
	sceneID, err := strconv.Atoi(input.SceneID)
	if err != nil {
		return false, fmt.Errorf("converting scene ID: %w", err)
	}

	fileIDInt, err := strconv.Atoi(input.FileID)
	if err != nil {
		return false, fmt.Errorf("converting file ID: %w", err)
	}

	fileID := models.FileID(fileIDInt)

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.Resolver.sceneService.AssignFile(ctx, sceneID, fileID)
	}); err != nil {
		return false, fmt.Errorf("assigning file to scene: %w", err)
	}

	return true, nil
}

func (r *mutationResolver) SceneMerge(ctx context.Context, input SceneMergeInput) (*models.Scene, error) {
	srcIDs, err := stringslice.StringSliceToIntSlice(input.Source)
	if err != nil {
		return nil, fmt.Errorf("converting source IDs: %w", err)
	}

	destID, err := strconv.Atoi(input.Destination)
	if err != nil {
		return nil, fmt.Errorf("converting destination ID %s: %w", input.Destination, err)
	}

	var values *models.ScenePartial
	if input.Values != nil {
		translator := changesetTranslator{
			inputMap: getNamedUpdateInputMap(ctx, "input.values"),
		}

		values, err = scenePartialFromInput(*input.Values, translator)
		if err != nil {
			return nil, err
		}
	} else {
		v := models.NewScenePartial()
		values = &v
	}

	var coverImageData []byte
	if input.Values.CoverImage != nil {
		var err error
		coverImageData, err = utils.ProcessImageInput(ctx, *input.Values.CoverImage)
		if err != nil {
			return nil, err
		}
	}

	var ret *models.Scene
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.Resolver.sceneService.Merge(ctx, srcIDs, destID, *values); err != nil {
			return err
		}

		ret, err = r.Resolver.repository.Scene.Find(ctx, destID)
		if err != nil {
			return err
		}
		if ret == nil {
			return fmt.Errorf("scene with id %d not found", destID)
		}

		return r.sceneUpdateCoverImage(ctx, ret, coverImageData)
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) getSceneMarker(ctx context.Context, id int) (ret *models.SceneMarker, err error) {
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.SceneMarker.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneMarkerCreate(ctx context.Context, input SceneMarkerCreateInput) (*models.SceneMarker, error) {
	primaryTagID, err := strconv.Atoi(input.PrimaryTagID)
	if err != nil {
		return nil, err
	}

	sceneID, err := strconv.Atoi(input.SceneID)
	if err != nil {
		return nil, err
	}

	// Populate a new scene marker from the input
	newSceneMarker := models.NewSceneMarker()

	newSceneMarker.Title = input.Title
	newSceneMarker.Seconds = input.Seconds
	newSceneMarker.PrimaryTagID = primaryTagID
	newSceneMarker.SceneID = sceneID

	tagIDs, err := stringslice.StringSliceToIntSlice(input.TagIds)
	if err != nil {
		return nil, err
	}

	err = r.changeMarker(ctx, create, &newSceneMarker, tagIDs)
	if err != nil {
		return nil, err
	}

	r.hookExecutor.ExecutePostHooks(ctx, newSceneMarker.ID, plugin.SceneMarkerCreatePost, input, nil)
	return r.getSceneMarker(ctx, newSceneMarker.ID)
}

func (r *mutationResolver) SceneMarkerUpdate(ctx context.Context, input SceneMarkerUpdateInput) (*models.SceneMarker, error) {
	sceneMarkerID, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, err
	}

	primaryTagID, err := strconv.Atoi(input.PrimaryTagID)
	if err != nil {
		return nil, err
	}

	sceneID, err := strconv.Atoi(input.SceneID)
	if err != nil {
		return nil, err
	}

	// Populate scene marker from the input
	currentTime := time.Now()
	updatedSceneMarker := models.SceneMarker{
		ID:           sceneMarkerID,
		Title:        input.Title,
		Seconds:      input.Seconds,
		SceneID:      sceneID,
		PrimaryTagID: primaryTagID,
		UpdatedAt:    currentTime,
	}

	tagIDs, err := stringslice.StringSliceToIntSlice(input.TagIds)
	if err != nil {
		return nil, err
	}

	err = r.changeMarker(ctx, update, &updatedSceneMarker, tagIDs)
	if err != nil {
		return nil, err
	}

	translator := changesetTranslator{
		inputMap: getUpdateInputMap(ctx),
	}
	r.hookExecutor.ExecutePostHooks(ctx, updatedSceneMarker.ID, plugin.SceneMarkerUpdatePost, input, translator.getFields())
	return r.getSceneMarker(ctx, updatedSceneMarker.ID)
}

func (r *mutationResolver) SceneMarkerDestroy(ctx context.Context, id string) (bool, error) {
	markerID, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}

	fileNamingAlgo := r.config.GetVideoFileNamingAlgorithm()

	fileDeleter := &scene.FileDeleter{
		Deleter:        file.NewDeleter(),
		FileNamingAlgo: fileNamingAlgo,
		Paths:          r.manager.Paths,
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.SceneMarker
		sqb := r.repository.Scene

		marker, err := qb.Find(ctx, markerID)

		if err != nil {
			return err
		}

		if marker == nil {
			return fmt.Errorf("scene marker with id %d not found", markerID)
		}

		s, err := sqb.Find(ctx, marker.SceneID)
		if err != nil {
			return err
		}

		if s == nil {
			return fmt.Errorf("scene with id %d not found", marker.SceneID)
		}

		return scene.DestroyMarker(ctx, s, marker, qb, fileDeleter)
	}); err != nil {
		fileDeleter.Rollback()
		return false, err
	}

	// perform the post-commit actions
	fileDeleter.Commit()

	r.hookExecutor.ExecutePostHooks(ctx, markerID, plugin.SceneMarkerDestroyPost, id, nil)

	return true, nil
}

func (r *mutationResolver) changeMarker(ctx context.Context, changeType int, changedMarker *models.SceneMarker, tagIDs []int) error {
	fileNamingAlgo := r.config.GetVideoFileNamingAlgorithm()

	fileDeleter := &scene.FileDeleter{
		Deleter:        file.NewDeleter(),
		FileNamingAlgo: fileNamingAlgo,
		Paths:          r.manager.Paths,
	}

	// Start the transaction and save the scene marker
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.SceneMarker
		sqb := r.repository.Scene

		switch changeType {
		case create:
			err := qb.Create(ctx, changedMarker)
			if err != nil {
				return err
			}
		case update:
			// check to see if timestamp was changed
			existingMarker, err := qb.Find(ctx, changedMarker.ID)
			if err != nil {
				return err
			}
			if existingMarker == nil {
				return fmt.Errorf("scene marker with id %d not found", changedMarker.ID)
			}

			err = qb.Update(ctx, changedMarker)
			if err != nil {
				return err
			}

			s, err := sqb.Find(ctx, existingMarker.SceneID)
			if err != nil {
				return err
			}
			if s == nil {
				return fmt.Errorf("scene with id %d not found", existingMarker.ID)
			}

			// remove the marker preview if the timestamp was changed
			if existingMarker.Seconds != changedMarker.Seconds {
				seconds := int(existingMarker.Seconds)
				if err := fileDeleter.MarkMarkerFiles(s, seconds); err != nil {
					return err
				}
			}
		}

		// Save the marker tags
		// If this tag is the primary tag, then let's not add it.
		tagIDs = intslice.IntExclude(tagIDs, []int{changedMarker.PrimaryTagID})
		return qb.UpdateTags(ctx, changedMarker.ID, tagIDs)
	}); err != nil {
		fileDeleter.Rollback()
		return err
	}

	// perform the post-commit actions
	fileDeleter.Commit()
	return nil
}

func (r *mutationResolver) SceneSaveActivity(ctx context.Context, id string, resumeTime *float64, playDuration *float64) (ret bool, err error) {
	sceneID, err := strconv.Atoi(id)
	if err != nil {
		return false, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		ret, err = qb.SaveActivity(ctx, sceneID, resumeTime, playDuration)
		return err
	}); err != nil {
		return false, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneIncrementPlayCount(ctx context.Context, id string) (ret int, err error) {
	sceneID, err := strconv.Atoi(id)
	if err != nil {
		return 0, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		ret, err = qb.IncrementWatchCount(ctx, sceneID)
		return err
	}); err != nil {
		return 0, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneIncrementO(ctx context.Context, id string) (ret int, err error) {
	sceneID, err := strconv.Atoi(id)
	if err != nil {
		return 0, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		ret, err = qb.IncrementOCounter(ctx, sceneID)
		return err
	}); err != nil {
		return 0, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneDecrementO(ctx context.Context, id string) (ret int, err error) {
	sceneID, err := strconv.Atoi(id)
	if err != nil {
		return 0, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		ret, err = qb.DecrementOCounter(ctx, sceneID)
		return err
	}); err != nil {
		return 0, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneResetO(ctx context.Context, id string) (ret int, err error) {
	sceneID, err := strconv.Atoi(id)
	if err != nil {
		return 0, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Scene

		ret, err = qb.ResetOCounter(ctx, sceneID)
		return err
	}); err != nil {
		return 0, err
	}

	return ret, nil
}

func (r *mutationResolver) SceneGenerateScreenshot(ctx context.Context, id string, at *float64) (string, error) {
	if at != nil {
		r.manager.GenerateScreenshot(ctx, id, *at)
	} else {
		r.manager.GenerateDefaultScreenshot(ctx, id)
	}

	return "todo", nil
}
