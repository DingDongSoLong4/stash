package generate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	stashExec "github.com/stashapp/stash/pkg/exec"
	"github.com/stashapp/stash/pkg/logger"
)

type vipsEncoder string

var vipsPath string
var vipsOnce sync.Once

func getVipsEncoder() *vipsEncoder {
	vipsOnce.Do(func() {
		vipsPath, _ = exec.LookPath("vips")
	})
	if vipsPath != "" {
		return (*vipsEncoder)(&vipsPath)
	} else {
		return nil
	}
}

func (e vipsEncoder) Thumbnail(ctx context.Context, image io.Reader, maxSize int) ([]byte, error) {
	args := []string{
		"thumbnail_source",
		"[descriptor=0]",
		".jpg[Q=70,strip]",
		fmt.Sprint(maxSize),
		"--size", "down",
	}
	data, err := e.run(ctx, args, image)

	return data, err
}

func (e vipsEncoder) run(ctx context.Context, args []string, stdin io.Reader) ([]byte, error) {
	cmd := stashExec.CommandContext(ctx, string(e), args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	err := cmd.Wait()

	if err != nil {
		// error message should be in the stderr stream
		logger.Errorf("image encoder error when running command <%s>: %s", strings.Join(cmd.Args, " "), stderr.String())
		return nil, err
	}

	return stdout.Bytes(), nil
}
