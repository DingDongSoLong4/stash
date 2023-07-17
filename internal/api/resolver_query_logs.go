package api

import (
	"context"
)

func (r *queryResolver) Logs(ctx context.Context) ([]*LogEntry, error) {
	logCache := r.manager.Logger.GetLogCache()
	ret := make([]*LogEntry, len(logCache))

	for i, entry := range logCache {
		ret[i] = &LogEntry{
			Time:    entry.Time,
			Level:   getLogLevel(entry.Type),
			Message: entry.Message,
		}
	}

	return ret, nil
}
