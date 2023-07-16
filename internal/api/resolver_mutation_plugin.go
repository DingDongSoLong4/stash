package api

import (
	"context"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/plugin"
)

func (r *mutationResolver) RunPluginTask(ctx context.Context, pluginID string, taskName string, args []*plugin.PluginArgInput) (string, error) {
	manager.GetInstance().RunPluginTask(ctx, pluginID, taskName, args)
	return "todo", nil
}

func (r *mutationResolver) ReloadPlugins(ctx context.Context) (bool, error) {
	manager.GetInstance().RefreshPluginCache()
	return true, nil
}
