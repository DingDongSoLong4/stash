package manager

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type importer interface {
	PreImport(ctx context.Context) error
	PostImport(ctx context.Context, id int) error
	Name() string
	FindExistingID(ctx context.Context) (*int, error)
	Create(ctx context.Context) (*int, error)
	Update(ctx context.Context, id int) error
}

func performImport(ctx context.Context, i importer, duplicateBehaviour models.ImportDuplicateEnum) error {
	if err := i.PreImport(ctx); err != nil {
		return err
	}

	// try to find an existing object with the same name
	name := i.Name()
	existing, err := i.FindExistingID(ctx)
	if err != nil {
		return fmt.Errorf("error finding existing objects: %v", err)
	}

	var id int

	if existing != nil {
		if duplicateBehaviour == models.ImportDuplicateEnumFail {
			return fmt.Errorf("existing object with name '%s'", name)
		} else if duplicateBehaviour == models.ImportDuplicateEnumIgnore {
			logger.Infof("Skipping existing object %q", name)
			return nil
		}

		// must be overwriting
		id = *existing
		if err := i.Update(ctx, id); err != nil {
			return fmt.Errorf("error updating existing object: %v", err)
		}
	} else {
		// creating
		createdID, err := i.Create(ctx)
		if err != nil {
			return fmt.Errorf("error creating object: %v", err)
		}

		id = *createdID
	}

	if err := i.PostImport(ctx, id); err != nil {
		return err
	}

	return nil
}
