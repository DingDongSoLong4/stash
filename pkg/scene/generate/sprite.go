package generate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/utils"
)

const (
	spriteScreenshotWidth = 160

	spriteRows   = 9
	spriteCols   = 9
	spriteChunks = spriteRows * spriteCols
)

func (g Generator) SpriteImage(ctx context.Context, videoFile *ffmpeg.VideoFile, hash string) error {
	lockCtx := g.LockManager.ReadLock(ctx, videoFile.Path)
	defer lockCtx.Cancel()

	if err := g.generateSpriteImage(lockCtx, videoFile, hash); err != nil {
		return err
	}

	return nil
}

func (g Generator) SpriteVTT(ctx context.Context, videoFile *ffmpeg.VideoFile, hash string) error {
	lockCtx := g.LockManager.ReadLock(ctx, videoFile.Path)
	defer lockCtx.Cancel()

	if err := g.generateSpriteVTT(lockCtx, videoFile, hash); err != nil {
		return err
	}

	return nil
}

func (g Generator) generateSpriteImage(lockCtx *fsutil.LockContext, videoFile *ffmpeg.VideoFile, hash string) error {
	output := g.ScenePaths.GetSpriteImageFilePath(hash)
	if !g.Overwrite {
		if exists, _ := fsutil.FileExists(output); exists {
			return nil
		}
	}

	useSlowSeek, err := g.useSlowSeek(videoFile)
	if err != nil {
		return err
	}

	var images []image.Image
	if !useSlowSeek {
		images, err = g.generateSprites(lockCtx, videoFile)
	} else {
		images, err = g.generateSpritesSlow(lockCtx, videoFile)
	}
	if err != nil {
		return err
	}

	if len(images) == 0 {
		return errors.New("images slice is empty")
	}

	if err := imaging.Save(g.combineSpriteImages(images), output); err != nil {
		return err
	}

	logger.Debug("created sprite image: ", output)

	return nil
}

func (g Generator) generateSpriteVTT(lockCtx *fsutil.LockContext, videoFile *ffmpeg.VideoFile, hash string) error {
	output := g.ScenePaths.GetSpriteVttFilePath(hash)
	if !g.Overwrite {
		if exists, _ := fsutil.FileExists(output); exists {
			return nil
		}
	}

	useSlowSeek, err := g.useSlowSeek(videoFile)
	if err != nil {
		return err
	}

	spriteImagePath := g.ScenePaths.GetSpriteImageFilePath(hash)
	fn := g.spriteVTT(videoFile, spriteImagePath, useSlowSeek)
	err = g.generateFile(lockCtx, g.ScenePaths, vttPattern, output, fn)
	if err != nil {
		return err
	}

	logger.Debug("created sprite vtt: ", output)

	return nil
}

func (g Generator) useSlowSeek(vf *ffmpeg.VideoFile) (bool, error) {
	// For files with small duration / low frame count, try to seek using frame number instead of seconds
	// some files can have FrameCount == 0, only use SlowSeek if duration < 5
	if vf.VideoStreamDuration < 5 || (0 < vf.FrameCount && vf.FrameCount <= int64(spriteChunks)) {
		if vf.VideoStreamDuration <= 0 {
			return false, fmt.Errorf("duration (%.3f) / frame count (%d) invalid", vf.VideoStreamDuration, vf.FrameCount)
		}
		logger.Warnf("[generator] video %s very short (%.3fs, %d frames), using frame seeking", vf.Path, vf.VideoStreamDuration, vf.FrameCount)
		return true, nil
	}

	return false, nil
}

func (g Generator) combineSpriteImages(images []image.Image) image.Image {
	// Combine all of the thumbnails into a sprite image
	width := images[0].Bounds().Size().X
	height := images[0].Bounds().Size().Y
	canvasWidth := width * spriteCols
	canvasHeight := height * spriteRows
	montage := imaging.New(canvasWidth, canvasHeight, color.NRGBA{})
	for index := 0; index < len(images); index++ {
		x := width * (index % spriteCols)
		y := height * int(math.Floor(float64(index)/float64(spriteRows)))
		img := images[index]
		montage = imaging.Paste(montage, img, image.Pt(x, y))
	}

	return montage
}

func (g Generator) generateSprites(lockCtx *fsutil.LockContext, vf *ffmpeg.VideoFile) ([]image.Image, error) {
	input := vf.Path
	logger.Infof("[generator] generating sprite image for %s", input)

	// generate `spriteChunks` thumbnails
	stepSize := vf.VideoStreamDuration / float64(spriteChunks)

	var images []image.Image
	for i := 0; i < spriteChunks; i++ {
		time := float64(i) * stepSize

		img, err := g.spriteScreenshot(lockCtx, input, time)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}

	return images, nil
}

func (g Generator) generateSpritesSlow(lockCtx *fsutil.LockContext, vf *ffmpeg.VideoFile) ([]image.Image, error) {
	input := vf.Path
	frameCount := vf.FrameCount

	// do an actual frame count of the file (number of frames = read frames)
	fc, err := g.FFProbe.GetReadFrameCount(input)
	if err == nil {
		if fc != frameCount {
			logger.Warnf("[generator] updating framecount (%d) for %s with read frames count (%d)", frameCount, input, fc)
			frameCount = fc
		}
	}

	logger.Infof("[generator] generating sprite image for %s (%d frames)", input, frameCount)

	stepFrame := float64(frameCount-1) / float64(spriteChunks)

	var images []image.Image
	for i := 0; i < spriteChunks; i++ {
		// generate exactly `spriteChunks` thumbnails, using duplicate frames if needed
		frame := math.Round(float64(i) * stepFrame)
		if frame >= math.MaxInt || frame <= math.MinInt {
			return nil, errors.New("invalid frame number conversion")
		}

		img, err := g.spriteScreenshotSlow(lockCtx, input, int(frame))
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}

	return images, nil
}

func (g Generator) spriteScreenshot(lockCtx *fsutil.LockContext, input string, seconds float64) (image.Image, error) {
	ssOptions := transcoder.ScreenshotOptions{
		OutputPath: "-",
		OutputType: transcoder.ScreenshotOutputTypeBMP,
		Width:      spriteScreenshotWidth,
	}

	args := transcoder.ScreenshotTime(input, seconds, ssOptions)

	return g.generateImage(lockCtx, args)
}

func (g Generator) spriteScreenshotSlow(lockCtx *fsutil.LockContext, input string, frame int) (image.Image, error) {
	ssOptions := transcoder.ScreenshotOptions{
		OutputPath: "-",
		OutputType: transcoder.ScreenshotOutputTypeBMP,
		Width:      spriteScreenshotWidth,
	}

	args := transcoder.ScreenshotFrame(input, frame, ssOptions)

	return g.generateImage(lockCtx, args)
}

func (g Generator) generateImage(lockCtx *fsutil.LockContext, args ffmpeg.Args) (image.Image, error) {
	out, err := g.Encoder.GenerateOutput(lockCtx, args, nil)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decoding image from ffmpeg: %w", err)
	}

	return img, nil
}

func (g Generator) spriteVTT(videoFile *ffmpeg.VideoFile, spriteImagePath string, slowSeek bool) generateFn {
	return func(lockCtx *fsutil.LockContext, tmpFn string) error {
		logger.Infof("[generator] generating sprite vtt for %s", videoFile.Path)

		spriteImage, err := os.Open(spriteImagePath)
		if err != nil {
			return err
		}
		defer spriteImage.Close()
		spriteImageName := filepath.Base(spriteImagePath)
		image, _, err := image.DecodeConfig(spriteImage)
		if err != nil {
			return err
		}
		width := image.Width / spriteCols
		height := image.Height / spriteRows

		var stepSize float64
		if !slowSeek {
			// this is actually what is generated....
			// stepSize = videoFile.VideoStreamDuration / float64(spriteChunks)

			framerate, numberOfFrames, err := g.calculateFrameRate(lockCtx, videoFile)
			if err != nil {
				return err
			}

			nthFrame := numberOfFrames / spriteChunks
			stepSize = float64(nthFrame) / framerate
		} else {
			framerate, _, err := g.calculateFrameRate(lockCtx, videoFile)
			if err != nil {
				return err
			}

			stepSize = float64(videoFile.FrameCount-1) / float64(spriteChunks)
			stepSize /= framerate
		}

		vttLines := []string{"WEBVTT", ""}
		for index := 0; index < spriteChunks; index++ {
			x := width * (index % spriteCols)
			y := height * int(math.Floor(float64(index)/float64(spriteRows)))
			startTime := utils.GetVTTTime(float64(index) * stepSize)
			endTime := utils.GetVTTTime(float64(index+1) * stepSize)

			vttLines = append(vttLines, startTime+" --> "+endTime)
			vttLines = append(vttLines, fmt.Sprintf("%s#xywh=%d,%d,%d,%d", spriteImageName, x, y, width, height))
			vttLines = append(vttLines, "")
		}
		vtt := strings.Join(vttLines, "\n")

		return os.WriteFile(tmpFn, []byte(vtt), 0644)
	}
}

func (g Generator) calculateFrameRate(ctx context.Context, videoFile *ffmpeg.VideoFile) (float64, int, error) {
	videoStream := videoFile.VideoStream
	if videoStream == nil {
		return 0, 0, errors.New("missing video stream")
	}

	framerate := videoFile.FrameRate

	if !isValidFramerate(framerate) {
		framerate, _ = strconv.ParseFloat(videoStream.RFrameRate, 64)
	}

	numberOfFrames, _ := strconv.Atoi(videoStream.NbFrames)

	if numberOfFrames == 0 && isValidFramerate(framerate) && videoFile.VideoStreamDuration > 0 { // TODO: test
		numberOfFrames = int(framerate * videoFile.VideoStreamDuration)
	}

	// If we are missing the frame count or frame rate then seek through the file and extract the info with regex
	if numberOfFrames == 0 || !isValidFramerate(framerate) {
		info, err := g.Encoder.CalculateFrameRate(ctx, videoFile)
		if err != nil {
			logger.Warnf("error calculating frame rate: %v", err)
		} else {
			if numberOfFrames == 0 {
				numberOfFrames = info.NumberOfFrames
			}
			if !isValidFramerate(framerate) {
				framerate = info.FrameRate
			}
		}
	}

	// Something is seriously wrong with this file
	if numberOfFrames == 0 || !isValidFramerate(framerate) {
		logger.Errorf(
			"cannot calculate frame rate: nb_frames <%s> framerate <%f> duration <%f>",
			videoStream.NbFrames,
			framerate,
			videoFile.VideoStreamDuration,
		)
	}

	return framerate, numberOfFrames, nil
}

// isValidFramerate ensures the given value is a valid number (not NaN) which is not equal to 0
func isValidFramerate(value float64) bool {
	return !math.IsNaN(value) && value != 0
}
