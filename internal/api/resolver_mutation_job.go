package api

import (
	"context"
	"strconv"
)

func (r *mutationResolver) StopJob(ctx context.Context, jobID string) (bool, error) {
	idInt, err := strconv.Atoi(jobID)
	if err != nil {
		return false, err
	}
	r.manager.JobManager.CancelJob(idInt)

	return true, nil
}

func (r *mutationResolver) StopAllJobs(ctx context.Context) (bool, error) {
	r.manager.JobManager.CancelAll()
	return true, nil
}
