package api

import (
	"context"
	"time"
)

func (r *mutationResolver) EnableDlna(ctx context.Context, input EnableDLNAInput) (bool, error) {
	err := r.manager.DLNAService.Start(parseMinutes(input.Duration))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) DisableDlna(ctx context.Context, input DisableDLNAInput) (bool, error) {
	r.manager.DLNAService.Stop(parseMinutes(input.Duration))
	return true, nil
}

func (r *mutationResolver) AddTempDlnaip(ctx context.Context, input AddTempDLNAIPInput) (bool, error) {
	r.manager.DLNAService.AddTempDLNAIP(input.Address, parseMinutes(input.Duration))
	return true, nil
}

func (r *mutationResolver) RemoveTempDlnaip(ctx context.Context, input RemoveTempDLNAIPInput) (bool, error) {
	ret := r.manager.DLNAService.RemoveTempDLNAIP(input.Address)
	return ret, nil
}

func parseMinutes(minutes *int) *time.Duration {
	var ret *time.Duration
	if minutes != nil {
		d := time.Duration(*minutes) * time.Minute
		ret = &d
	}

	return ret
}
