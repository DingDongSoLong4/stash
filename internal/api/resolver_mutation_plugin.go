package api

import (
	"context"

	"github.com/stashapp/stash/pkg/plugin"
)

func (r *mutationResolver) RunPluginTask(ctx context.Context, pluginID string, taskName string, args []*plugin.PluginArgInput) (string, error) {
	r.manager.RunPluginTask(ctx, pluginID, taskName, args)
	return "todo", nil
}

func (r *mutationResolver) ReloadPlugins(ctx context.Context) (bool, error) {
	r.manager.RefreshPluginCache()
	return true, nil
}
