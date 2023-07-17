package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/internal/identify"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

func (r *mutationResolver) MetadataScan(ctx context.Context, input models.ScanMetadataInput) (string, error) {
	jobID, err := r.manager.Scan(ctx, input)

	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataImport(ctx context.Context) (string, error) {
	jobID, err := r.manager.Import(ctx)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) ImportObjects(ctx context.Context, input models.ImportObjectsInput) (string, error) {
	jobID, err := r.manager.ImportObjects(ctx, input)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataExport(ctx context.Context) (string, error) {
	jobID, err := r.manager.Export(ctx)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) ExportObjects(ctx context.Context, input models.ExportObjectsInput) (*string, error) {
	baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

	return r.manager.ExportObjects(ctx, input, baseURL)
}

func (r *mutationResolver) MetadataGenerate(ctx context.Context, input models.GenerateMetadataInput) (string, error) {
	jobID, err := r.manager.Generate(ctx, input)

	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAutoTag(ctx context.Context, input models.AutoTagMetadataInput) (string, error) {
	jobID := r.manager.AutoTag(ctx, input)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataIdentify(ctx context.Context, input identify.Options) (string, error) {
	jobID := r.manager.Identify(ctx, input)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataClean(ctx context.Context, input models.CleanMetadataInput) (string, error) {
	jobID := r.manager.Clean(ctx, input)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MigrateHashNaming(ctx context.Context) (string, error) {
	jobID := r.manager.MigrateHash(ctx)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) BackupDatabase(ctx context.Context, input BackupDatabaseInput) (*string, error) {
	// if download is true, then backup to temporary file and return a link
	download := input.Download != nil && *input.Download

	backupPath, backupName, err := r.manager.BackupDatabase(download)
	if err != nil {
		logger.Errorf("Error backing up database: %v", err)
		return nil, err
	}

	if download {
		downloadHash, err := r.manager.DownloadStore.RegisterFile(backupPath, "", false)
		if err != nil {
			return nil, fmt.Errorf("error registering file for download: %w", err)
		}
		logger.Debugf("Generated backup file %s with hash %s", backupPath, downloadHash)

		baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

		ret := baseURL + "/downloads/" + downloadHash + "/" + backupName
		return &ret, nil
	} else {
		logger.Infof("Successfully backed up database to: %s", backupPath)
	}

	return nil, nil
}

func (r *mutationResolver) AnonymiseDatabase(ctx context.Context, input AnonymiseDatabaseInput) (*string, error) {
	// if download is true, then save to temporary file and return a link
	download := input.Download != nil && *input.Download

	outPath, outName, err := r.manager.AnonymiseDatabase(download)
	if err != nil {
		logger.Errorf("Error anonymising database: %v", err)
		return nil, err
	}

	if download {
		downloadHash, err := r.manager.DownloadStore.RegisterFile(outPath, "", false)
		if err != nil {
			return nil, fmt.Errorf("error registering file for download: %w", err)
		}
		logger.Debugf("Generated anonymised file %s with hash %s", outPath, downloadHash)

		baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

		ret := baseURL + "/downloads/" + downloadHash + "/" + outName
		return &ret, nil
	} else {
		logger.Infof("Successfully anonymised database to: %s", outPath)
	}

	return nil, nil
}
