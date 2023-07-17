package api

import (
	"context"
)

func (r *mutationResolver) ReloadScrapers(ctx context.Context) (bool, error) {
	r.manager.RefreshScraperCache()
	return true, nil
}
