package manager

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/plugin"
)

func (s *Manager) RunPluginTask(ctx context.Context, pluginID string, taskName string, args []*plugin.PluginArgInput) int {
	j := job.MakeJobExec(func(jobCtx context.Context, progress *job.Progress) {
		pluginProgress := make(chan float64)
		task, err := s.PluginCache.CreateTask(ctx, pluginID, taskName, args, pluginProgress)
		if err != nil {
			logger.Errorf("error creating plugin task: %v", err)
			return
		}

		err = task.Start()
		if err != nil {
			logger.Errorf("error running plugin task: %v", err)
			return
		}

		done := make(chan bool)
		go func() {
			defer close(done)
			task.Wait()

			output := task.GetResult()
			if output == nil {
				logger.Debug("Plugin returned no result")
			} else {
				if output.Error != nil {
					logger.Errorf("Plugin returned error: %s", *output.Error)
				} else if output.Output != nil {
					logger.Debugf("Plugin returned: %v", output.Output)
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			case p := <-pluginProgress:
				progress.SetPercent(p)
			case <-jobCtx.Done():
				if err := task.Stop(); err != nil {
					logger.Errorf("error stopping plugin operation: %v", err)
				}
				return
			}
		}
	})

	return s.JobManager.Add(ctx, fmt.Sprintf("Running plugin task: %s", taskName), j)
}
