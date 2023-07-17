package api

import (
	"context"

	"github.com/stashapp/stash/pkg/plugin"
)

func (r *queryResolver) Plugins(ctx context.Context) ([]*plugin.Plugin, error) {
	return r.manager.PluginCache.ListPlugins(), nil
}

func (r *queryResolver) PluginTasks(ctx context.Context) ([]*plugin.PluginTask, error) {
	return r.manager.PluginCache.ListPluginTasks(), nil
}
