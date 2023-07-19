package manager

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scraper/stashbox"
	"github.com/stashapp/stash/pkg/sliceutil/stringslice"
	"github.com/stashapp/stash/pkg/utils"
)

type StashBoxPerformerTagTask struct {
	Name           *string
	Performer      *models.Performer
	Refresh        bool
	Box            *models.StashBox
	ExcludedFields []string

	Repository models.Repository
}

func (t *StashBoxPerformerTagTask) Start(ctx context.Context) {
	t.stashBoxPerformerTag(ctx)
}

func (t *StashBoxPerformerTagTask) Description() string {
	var name string
	if t.Name != nil {
		name = *t.Name
	} else if t.Performer != nil {
		name = t.Performer.Name
	}

	return fmt.Sprintf("Tagging performer %s from stash-box", name)
}

func (t *StashBoxPerformerTagTask) stashBoxPerformerTag(ctx context.Context) {
	var performer *models.ScrapedPerformer
	var err error

	stashboxRepository := stashbox.NewRepository(t.Repository)
	client := stashbox.NewClient(*t.Box, stashboxRepository)

	if t.Refresh {
		var performerID string
		for _, id := range t.Performer.StashIDs.List() {
			if id.Endpoint == t.Box.Endpoint {
				performerID = id.StashID
			}
		}
		if performerID != "" {
			performer, err = client.FindStashBoxPerformerByID(ctx, performerID)
		}
	} else {
		var name string
		if t.Name != nil {
			name = *t.Name
		} else {
			name = t.Performer.Name
		}
		performer, err = client.FindStashBoxPerformerByName(ctx, name)
	}

	if err != nil {
		logger.Errorf("Error fetching performer data from stash-box: %s", err.Error())
		return
	}

	excluded := map[string]bool{}
	for _, field := range t.ExcludedFields {
		excluded[field] = true
	}

	if performer != nil {
		if t.Performer != nil {
			partial := t.getPartial(performer, excluded)

			r := t.Repository
			txnErr := r.WithTxn(ctx, func(ctx context.Context) error {
				_, err := r.Performer.UpdatePartial(ctx, t.Performer.ID, partial)

				if len(performer.Images) > 0 && !excluded["image"] {
					image, err := utils.ReadImageFromURL(ctx, performer.Images[0])
					if err == nil {
						err = r.Performer.UpdateImage(ctx, t.Performer.ID, image)
						if err != nil {
							return err
						}
					} else {
						logger.Warnf("Failed to read performer image: %v", err)
					}
				}

				if err == nil {
					var name string
					if performer.Name != nil {
						name = *performer.Name
					}
					logger.Infof("Updated performer %s", name)
				}
				return err
			})
			if txnErr != nil {
				logger.Warnf("failure to execute partial update of performer: %v", txnErr)
			}
		} else if t.Name != nil && performer.Name != nil {
			newPerformer := models.NewPerformer()

			newPerformer.Name = *performer.Name
			newPerformer.Disambiguation = getString(performer.Disambiguation)
			newPerformer.Details = getString(performer.Details)
			newPerformer.Birthdate = getDate(performer.Birthdate)
			newPerformer.DeathDate = getDate(performer.DeathDate)
			newPerformer.CareerLength = getString(performer.CareerLength)
			newPerformer.Country = getString(performer.Country)
			newPerformer.Ethnicity = getString(performer.Ethnicity)
			newPerformer.EyeColor = getString(performer.EyeColor)
			newPerformer.HairColor = getString(performer.HairColor)
			newPerformer.FakeTits = getString(performer.FakeTits)
			newPerformer.Height = getIntPtr(performer.Height)
			newPerformer.Weight = getIntPtr(performer.Weight)
			newPerformer.Instagram = getString(performer.Instagram)
			newPerformer.Measurements = getString(performer.Measurements)
			newPerformer.Piercings = getString(performer.Piercings)
			newPerformer.Tattoos = getString(performer.Tattoos)
			newPerformer.Twitter = getString(performer.Twitter)
			newPerformer.URL = getString(performer.URL)
			newPerformer.StashIDs = models.NewRelatedStashIDs([]models.StashID{
				{
					Endpoint: t.Box.Endpoint,
					StashID:  *performer.RemoteSiteID,
				},
			})

			if performer.Aliases != nil {
				newPerformer.Aliases = models.NewRelatedStrings(stringslice.FromString(*performer.Aliases, ","))
			}

			if performer.Gender != nil {
				v := models.GenderEnum(getString(performer.Gender))
				newPerformer.Gender = &v
			}

			r := t.Repository
			err := r.WithTxn(ctx, func(ctx context.Context) error {
				err := r.Performer.Create(ctx, &newPerformer)
				if err != nil {
					return err
				}

				if len(performer.Images) > 0 {
					image, imageErr := utils.ReadImageFromURL(ctx, performer.Images[0])
					if imageErr != nil {
						return imageErr
					}
					err = r.Performer.UpdateImage(ctx, newPerformer.ID, image)
				}
				return err
			})
			if err != nil {
				logger.Errorf("Failed to save performer %s: %s", *t.Name, err.Error())
			} else {
				logger.Infof("Saved performer %s", *t.Name)
			}
		}
	} else {
		var name string
		if t.Name != nil {
			name = *t.Name
		} else if t.Performer != nil {
			name = t.Performer.Name
		}
		logger.Infof("No match found for %s", name)
	}
}

func (t *StashBoxPerformerTagTask) getPartial(performer *models.ScrapedPerformer, excluded map[string]bool) models.PerformerPartial {
	partial := models.NewPerformerPartial()

	if performer.Aliases != nil && !excluded["aliases"] {
		partial.Aliases = &models.UpdateStrings{
			Values: stringslice.FromString(*performer.Aliases, ","),
			Mode:   models.RelationshipUpdateModeSet,
		}
	}
	if performer.Birthdate != nil && *performer.Birthdate != "" && !excluded["birthdate"] {
		value := getDate(performer.Birthdate)
		partial.Birthdate = models.NewOptionalDate(*value)
	}
	if performer.DeathDate != nil && *performer.DeathDate != "" && !excluded["deathdate"] {
		value := getDate(performer.DeathDate)
		partial.DeathDate = models.NewOptionalDate(*value)
	}
	if performer.CareerLength != nil && !excluded["career_length"] {
		partial.CareerLength = models.NewOptionalString(*performer.CareerLength)
	}
	if performer.Country != nil && !excluded["country"] {
		partial.Country = models.NewOptionalString(*performer.Country)
	}
	if performer.Ethnicity != nil && !excluded["ethnicity"] {
		partial.Ethnicity = models.NewOptionalString(*performer.Ethnicity)
	}
	if performer.EyeColor != nil && !excluded["eye_color"] {
		partial.EyeColor = models.NewOptionalString(*performer.EyeColor)
	}
	if performer.HairColor != nil && !excluded["hair_color"] {
		partial.HairColor = models.NewOptionalString(*performer.HairColor)
	}
	if performer.FakeTits != nil && !excluded["fake_tits"] {
		partial.FakeTits = models.NewOptionalString(*performer.FakeTits)
	}
	if performer.Gender != nil && !excluded["gender"] {
		partial.Gender = models.NewOptionalString(*performer.Gender)
	}
	if performer.Height != nil && !excluded["height"] {
		h, err := strconv.Atoi(*performer.Height)
		if err == nil {
			partial.Height = models.NewOptionalInt(h)
		}
	}
	if performer.Weight != nil && !excluded["weight"] {
		w, err := strconv.Atoi(*performer.Weight)
		if err == nil {
			partial.Weight = models.NewOptionalInt(w)
		}
	}
	if performer.Instagram != nil && !excluded["instagram"] {
		partial.Instagram = models.NewOptionalString(*performer.Instagram)
	}
	if performer.Measurements != nil && !excluded["measurements"] {
		partial.Measurements = models.NewOptionalString(*performer.Measurements)
	}
	if excluded["name"] && performer.Name != nil {
		partial.Name = models.NewOptionalString(*performer.Name)
	}
	if performer.Disambiguation != nil && !excluded["disambiguation"] {
		partial.Disambiguation = models.NewOptionalString(*performer.Disambiguation)
	}
	if performer.Piercings != nil && !excluded["piercings"] {
		partial.Piercings = models.NewOptionalString(*performer.Piercings)
	}
	if performer.Tattoos != nil && !excluded["tattoos"] {
		partial.Tattoos = models.NewOptionalString(*performer.Tattoos)
	}
	if performer.Twitter != nil && !excluded["twitter"] {
		partial.Twitter = models.NewOptionalString(*performer.Twitter)
	}
	if performer.URL != nil && !excluded["url"] {
		partial.URL = models.NewOptionalString(*performer.URL)
	}
	if !t.Refresh {
		// #3547 - need to overwrite the stash id for the endpoint, but preserve
		// existing stash ids for other endpoints
		partial.StashIDs = &models.UpdateStashIDs{
			StashIDs: t.Performer.StashIDs.List(),
			Mode:     models.RelationshipUpdateModeSet,
		}

		partial.StashIDs.Set(models.StashID{
			Endpoint: t.Box.Endpoint,
			StashID:  *performer.RemoteSiteID,
		})
	}

	return partial
}

func getDate(val *string) *models.Date {
	if val == nil {
		return nil
	}

	ret, err := models.ParseDate(*val)
	if err != nil {
		return nil
	}
	return &ret
}

func getString(val *string) string {
	if val == nil {
		return ""
	} else {
		return *val
	}
}

func getIntPtr(val *string) *int {
	if val == nil {
		return nil
	} else {
		v, err := strconv.Atoi(*val)
		if err != nil {
			return nil
		}

		return &v
	}
}
