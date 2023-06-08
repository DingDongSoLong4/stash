package performer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/jsonschema"
	"github.com/stashapp/stash/pkg/sliceutil/stringslice"
	"github.com/stashapp/stash/pkg/utils"
)

type Importer struct {
	ReaderWriter        models.PerformerReaderWriter
	TagWriter           models.TagReaderWriter
	Input               jsonschema.Performer
	MissingRefBehaviour models.ImportMissingRefEnum

	ID        int
	performer models.Performer
	imageData []byte
}

func (i *Importer) PreImport(ctx context.Context) error {
	i.performer = performerJSONToPerformer(i.Input)

	if err := i.populateTags(ctx); err != nil {
		return err
	}

	var err error
	if len(i.Input.Image) > 0 {
		i.imageData, err = utils.ProcessBase64Image(i.Input.Image)
		if err != nil {
			return fmt.Errorf("invalid image: %v", err)
		}
	}

	return nil
}

func (i *Importer) populateTags(ctx context.Context) error {
	if len(i.Input.Tags) > 0 {

		tags, err := importTags(ctx, i.TagWriter, i.Input.Tags, i.MissingRefBehaviour)
		if err != nil {
			return err
		}

		for _, p := range tags {
			i.performer.TagIDs.Add(p.ID)
		}
	}

	return nil
}

func importTags(ctx context.Context, tagWriter models.TagReaderWriter, names []string, missingRefBehaviour models.ImportMissingRefEnum) ([]*models.Tag, error) {
	tags, err := tagWriter.FindByNames(ctx, names, false)
	if err != nil {
		return nil, err
	}

	var pluckedNames []string
	for _, tag := range tags {
		pluckedNames = append(pluckedNames, tag.Name)
	}

	missingTags := stringslice.StrFilter(names, func(name string) bool {
		return !stringslice.StrInclude(pluckedNames, name)
	})

	if len(missingTags) > 0 {
		if missingRefBehaviour == models.ImportMissingRefEnumFail {
			return nil, fmt.Errorf("tags [%s] not found", strings.Join(missingTags, ", "))
		}

		if missingRefBehaviour == models.ImportMissingRefEnumCreate {
			createdTags, err := createTags(ctx, tagWriter, missingTags)
			if err != nil {
				return nil, fmt.Errorf("error creating tags: %v", err)
			}

			tags = append(tags, createdTags...)
		}

		// ignore if MissingRefBehaviour set to Ignore
	}

	return tags, nil
}

func createTags(ctx context.Context, tagWriter models.TagReaderWriter, names []string) ([]*models.Tag, error) {
	var ret []*models.Tag
	for _, name := range names {
		newTag := models.NewTag(name)

		err := tagWriter.Create(ctx, newTag)
		if err != nil {
			return nil, err
		}

		ret = append(ret, newTag)
	}

	return ret, nil
}

func (i *Importer) PostImport(ctx context.Context, id int) error {
	if len(i.imageData) > 0 {
		if err := i.ReaderWriter.UpdateImage(ctx, id, i.imageData); err != nil {
			return fmt.Errorf("error setting performer image: %v", err)
		}
	}

	return nil
}

func (i *Importer) Name() string {
	return i.Input.Name
}

func (i *Importer) FindExistingID(ctx context.Context) (*int, error) {
	// use disambiguation as well
	performerFilter := models.PerformerFilterType{
		Name: &models.StringCriterionInput{
			Value:    i.Input.Name,
			Modifier: models.CriterionModifierEquals,
		},
	}

	if i.Input.Disambiguation != "" {
		performerFilter.Disambiguation = &models.StringCriterionInput{
			Value:    i.Input.Disambiguation,
			Modifier: models.CriterionModifierEquals,
		}
	}

	pp := 1
	findFilter := models.FindFilterType{
		PerPage: &pp,
	}

	existing, _, err := i.ReaderWriter.Query(ctx, &performerFilter, &findFilter)
	if err != nil {
		return nil, err
	}

	if len(existing) > 0 {
		id := existing[0].ID
		return &id, nil
	}

	return nil, nil
}

func (i *Importer) Create(ctx context.Context) (*int, error) {
	err := i.ReaderWriter.Create(ctx, &i.performer)
	if err != nil {
		return nil, fmt.Errorf("error creating performer: %v", err)
	}

	id := i.performer.ID
	return &id, nil
}

func (i *Importer) Update(ctx context.Context, id int) error {
	performer := i.performer
	performer.ID = id
	err := i.ReaderWriter.Update(ctx, &performer)
	if err != nil {
		return fmt.Errorf("error updating existing performer: %v", err)
	}

	return nil
}

func performerJSONToPerformer(performerJSON jsonschema.Performer) models.Performer {
	newPerformer := models.Performer{
		Name:           performerJSON.Name,
		Disambiguation: performerJSON.Disambiguation,
		URL:            performerJSON.URL,
		Ethnicity:      performerJSON.Ethnicity,
		Country:        performerJSON.Country,
		EyeColor:       performerJSON.EyeColor,
		Measurements:   performerJSON.Measurements,
		FakeTits:       performerJSON.FakeTits,
		CareerLength:   performerJSON.CareerLength,
		Tattoos:        performerJSON.Tattoos,
		Piercings:      performerJSON.Piercings,
		Aliases:        models.NewRelatedStrings(performerJSON.Aliases),
		Twitter:        performerJSON.Twitter,
		Instagram:      performerJSON.Instagram,
		Details:        performerJSON.Details,
		HairColor:      performerJSON.HairColor,
		Favorite:       performerJSON.Favorite,
		IgnoreAutoTag:  performerJSON.IgnoreAutoTag,
		CreatedAt:      performerJSON.CreatedAt.GetTime(),
		UpdatedAt:      performerJSON.UpdatedAt.GetTime(),

		TagIDs:   models.NewRelatedIDs([]int{}),
		StashIDs: models.NewRelatedStashIDs(performerJSON.StashIDs),
	}

	if performerJSON.Gender != "" {
		v := models.GenderEnum(performerJSON.Gender)
		newPerformer.Gender = &v
	}

	if performerJSON.Circumcised != "" {
		v := models.CircumisedEnum(performerJSON.Circumcised)
		newPerformer.Circumcised = &v
	}

	if performerJSON.Birthdate != "" {
		date, err := models.ParseDate(performerJSON.Birthdate)
		if err == nil {
			newPerformer.Birthdate = &date
		}
	}
	if performerJSON.Rating != 0 {
		newPerformer.Rating = &performerJSON.Rating
	}
	if performerJSON.DeathDate != "" {
		date, err := models.ParseDate(performerJSON.DeathDate)
		if err == nil {
			newPerformer.DeathDate = &date
		}
	}

	if performerJSON.Weight != 0 {
		newPerformer.Weight = &performerJSON.Weight
	}

	if performerJSON.PenisLength != 0 {
		newPerformer.PenisLength = &performerJSON.PenisLength
	}

	if performerJSON.Height != "" {
		h, err := strconv.Atoi(performerJSON.Height)
		if err == nil {
			newPerformer.Height = &h
		} else {
			logger.Warnf("error parsing height %q: %v", performerJSON.Height, err)
		}
	}

	return newPerformer
}
