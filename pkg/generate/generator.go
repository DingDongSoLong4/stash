package generate

import (
	"errors"
	"fmt"
	"os"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/models/paths"
)

const (
	mp4Pattern  = "*.mp4"
	webpPattern = "*.webp"
	webmPattern = "*.webm"
	jpgPattern  = "*.jpg"
	txtPattern  = "*.txt"
	vttPattern  = "*.vtt"
)

var ErrUnsupportedFormat = errors.New("unsupported format")

type Config interface {
	GetTranscodeInputArgs() []string
	GetTranscodeOutputArgs() []string
}

type Generator struct {
	Encoder     *ffmpeg.FFMpeg
	Config      Config
	FFProbe     *ffmpeg.FFProbe
	LockManager *fsutil.ReadLockManager
	Paths       *paths.Paths
	Overwrite   bool
}

type generateFn func(lockCtx *fsutil.LockContext, tmpFn string) error

func (g Generator) tempFile(pattern string) (*os.File, error) {
	tmpFile, err := g.Paths.Generated.TempFile(pattern) // tmp output in case the process ends abruptly
	if err != nil {
		return nil, fmt.Errorf("creating temporary file: %w", err)
	}
	_ = tmpFile.Close()
	return tmpFile, err
}

// generateFile performs a generate operation by generating a temporary file using pattern, then
// moving it to output on success.
func (g Generator) generateFile(lockCtx *fsutil.LockContext, pattern string, output string, generateFn generateFn) error {
	tmpFile, err := g.tempFile(pattern) // tmp output in case the process ends abruptly
	if err != nil {
		return err
	}

	tmpFn := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpFn)
	}()

	if err := generateFn(lockCtx, tmpFn); err != nil {
		return err
	}

	// check if generated empty file
	stat, err := os.Stat(tmpFn)
	if err != nil {
		return fmt.Errorf("error getting file stat: %w", err)
	}

	if stat.Size() == 0 {
		return fmt.Errorf("ffmpeg command produced no output")
	}

	if err := fsutil.SafeMove(tmpFn, output); err != nil {
		return fmt.Errorf("moving %s to %s", tmpFn, output)
	}

	return nil
}

// generateBytes performs a generate operation by generating a temporary file using pattern, returns the contents, then deletes it.
func (g Generator) generateBytes(lockCtx *fsutil.LockContext, pattern string, generateFn generateFn) ([]byte, error) {
	tmpFile, err := g.tempFile(pattern) // tmp output in case the process ends abruptly
	if err != nil {
		return nil, err
	}

	tmpFn := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpFn)
	}()

	if err := generateFn(lockCtx, tmpFn); err != nil {
		return nil, err
	}

	defer os.Remove(tmpFn)
	return os.ReadFile(tmpFn)
}
