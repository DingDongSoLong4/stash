package api

import (
	"context"

	"github.com/stashapp/stash/internal/dlna"
)

func (r *queryResolver) DlnaStatus(ctx context.Context) (*dlna.Status, error) {
	return r.manager.DLNAService.Status(), nil
}
