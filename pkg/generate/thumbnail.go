package generate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/models"
)

const (
	// thumbnailWidth   = 320
	thumbnailQuality = 5
)

// Thumbnail returns the thumbnail image of the provided image resized to
// the provided max size. It resizes based on the largest X/Y direction.
// It returns nil and an error if an error occurs reading, decoding or encoding
// the image, or if the image is not suitable for thumbnails.
func (g Generator) Thumbnail(ctx context.Context, f models.File, maxSize int) ([]byte, error) {
	reader, err := f.Open(&file.OsFS{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}

	data := buf.Bytes()

	if imageFile, ok := f.(*models.ImageFile); ok {
		format := imageFile.Format
		animated := format == formatGif

		// #2266 - if image is webp, then determine if it is animated
		if format == formatWebP {
			animated = isWebPAnimated(data)
		}

		// #2266 - don't generate a thumbnail for animated images
		if animated {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
		}
	}

	// Videofiles can only be thumbnailed with ffmpeg
	if _, ok := f.(*models.VideoFile); ok {
		return g.thumbnailFFMpeg(ctx, &buf, maxSize)
	}

	var vips *vipsEncoder

	// vips has issues loading files from stdin on Windows
	if runtime.GOOS != "windows" {
		vips = getVipsEncoder()
	}

	if vips != nil {
		return vips.Thumbnail(ctx, &buf, maxSize)
	} else {
		return g.thumbnailFFMpeg(ctx, &buf, maxSize)
	}
}

// ClipPreview generates a preview clip of videoFile resized to
// the provided max size. It resizes based on the largest X/Y direction.
// It is hardcoded to 30 seconds maximum right now.
func (g Generator) ClipPreview(ctx context.Context, videoFile *ffmpeg.VideoFile, hash string, maxSize int, preset models.PreviewPreset) error {
	lockCtx := g.LockManager.ReadLock(ctx, videoFile.Path)
	defer lockCtx.Cancel()

	output := g.Paths.Generated.GetClipPreviewPath(hash, maxSize)
	if !g.Overwrite {
		if exists, _ := fsutil.FileExists(output); exists {
			return nil
		}
	}

	fn := g.clipPreview(videoFile, maxSize, preset.String())
	return g.generateFile(lockCtx, webmPattern, output, fn)
}

func (g *Generator) thumbnailFFMpeg(ctx context.Context, image io.Reader, maxSize int) ([]byte, error) {
	args := transcoder.ImageThumbnail("-", transcoder.ImageThumbnailOptions{
		OutputFormat:  ffmpeg.ImageFormatJpeg,
		OutputPath:    "-",
		MaxDimensions: maxSize,
		Quality:       thumbnailQuality,
	})

	return g.Encoder.GenerateOutput(ctx, args, image)
}

func (g *Generator) clipPreview(videoFile *ffmpeg.VideoFile, maxSize int, preset string) generateFn {
	if videoFile.Width <= maxSize {
		maxSize = videoFile.Width
	}
	duration := videoFile.VideoStreamDuration
	if duration > 30.0 {
		duration = 30.0
	}

	return func(lockCtx *fsutil.LockContext, tmpFn string) error {
		var thumbFilter ffmpeg.VideoFilter
		thumbFilter = thumbFilter.ScaleMaxSize(maxSize)

		var thumbArgs ffmpeg.Args
		thumbArgs = thumbArgs.VideoFilter(thumbFilter)

		thumbArgs = append(thumbArgs,
			"-pix_fmt", "yuv420p",
			"-preset", preset,
			"-crf", "25",
			"-threads", "4",
			"-strict", "-2",
			"-f", "webm",
		)

		if videoFile.FrameRate <= 0.01 {
			thumbArgs = append(thumbArgs, "-vsync", "2")
		}

		thumbOptions := transcoder.TranscodeOptions{
			OutputPath: tmpFn,
			StartTime:  0,
			Duration:   duration,

			XError:   true,
			SlowSeek: false,

			VideoCodec: ffmpeg.VideoCodecVP9,
			VideoArgs:  thumbArgs,

			ExtraInputArgs:  g.Config.GetTranscodeInputArgs(),
			ExtraOutputArgs: g.Config.GetTranscodeOutputArgs(),
		}

		args := transcoder.Transcode(videoFile.Path, thumbOptions)

		return g.Encoder.Generate(lockCtx, args)
	}
}
