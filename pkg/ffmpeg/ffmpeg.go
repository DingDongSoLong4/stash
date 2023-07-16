// Package ffmpeg provides a wrapper around the ffmpeg and ffprobe executables.
package ffmpeg

import (
	"context"
	"errors"
	"os/exec"

	stashExec "github.com/stashapp/stash/pkg/exec"
)

var ErrFFMpegUnconfigured = errors.New("ffmpeg not configured")

// FFMpeg provides an interface to ffmpeg.
type FFMpeg struct {
	ffmpeg         string
	hwCodecSupport []VideoCodec
}

func (f *FFMpeg) Configure(ctx context.Context, path string) {
	f.ffmpeg = path

	f.initHWSupport(ctx)
}

func (f *FFMpeg) ensureConfigured() error {
	if f.ffmpeg == "" {
		return ErrFFMpegUnconfigured
	}
	return nil
}

// Returns an exec.Cmd that can be used to run ffmpeg using args.
func (f *FFMpeg) command(ctx context.Context, args []string) *exec.Cmd {
	return stashExec.CommandContext(ctx, string(f.ffmpeg), args...)
}
