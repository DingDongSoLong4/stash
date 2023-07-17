package api

import (
	"context"

	"github.com/stashapp/stash/pkg/job"
)

func makeJobStatusUpdate(t JobStatusUpdateType, j job.Job) *JobStatusUpdate {
	return &JobStatusUpdate{
		Type: t,
		Job:  jobToJobModel(j),
	}
}

func (r *subscriptionResolver) JobsSubscribe(ctx context.Context) (<-chan *JobStatusUpdate, error) {
	msg := make(chan *JobStatusUpdate, 100)

	subscription := r.manager.JobManager.Subscribe(ctx)

	go func() {
		for {
			select {
			case j := <-subscription.NewJob:
				msg <- makeJobStatusUpdate(JobStatusUpdateTypeAdd, j)
			case j := <-subscription.RemovedJob:
				msg <- makeJobStatusUpdate(JobStatusUpdateTypeRemove, j)
			case j := <-subscription.UpdatedJob:
				msg <- makeJobStatusUpdate(JobStatusUpdateTypeUpdate, j)
			case <-ctx.Done():
				close(msg)
				return
			}
		}
	}()

	return msg, nil
}

func (r *subscriptionResolver) ScanCompleteSubscribe(ctx context.Context) (<-chan bool, error) {
	return r.manager.ScanSubscribe(ctx), nil
}
