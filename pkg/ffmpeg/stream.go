package ffmpeg

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

const (
	MimeWebmVideo string = "video/webm"
	MimeWebmAudio string = "audio/webm"
	MimeMkvVideo  string = "video/x-matroska"
	MimeMkvAudio  string = "audio/x-matroska"
	MimeMp4Video  string = "video/mp4"
	MimeMp4Audio  string = "audio/mp4"
)

type StreamManager struct {
	cacheDir string
	encoder  *FFMpeg
	ffprobe  *FFProbe

	config      StreamManagerConfig
	lockManager *fsutil.ReadLockManager

	context    context.Context
	cancelFunc context.CancelFunc

	runningStreams map[string]*runningStream
	streamsMutex   sync.Mutex
}

type StreamManagerConfig interface {
	GetMaxStreamingTranscodeSize() models.StreamingResolutionEnum
	GetLiveTranscodeInputArgs() []string
	GetLiveTranscodeOutputArgs() []string
	GetTranscodeHardwareAcceleration() bool
}

func NewStreamManager(encoder *FFMpeg, ffprobe *FFProbe, config StreamManagerConfig, lockManager *fsutil.ReadLockManager) *StreamManager {
	return &StreamManager{
		encoder:     encoder,
		ffprobe:     ffprobe,
		config:      config,
		lockManager: lockManager,
	}
}

// Configure configures or reconfigures the stream manager with a new cacheDir.
func (sm *StreamManager) Configure(cacheDir string) {
	sm.Shutdown()

	sm.cacheDir = cacheDir
	if cacheDir == "" {
		logger.Warn("Cache directory unset. Live HLS/DASH transcoding will be disabled.")
	} else {
		sm.runningStreams = make(map[string]*runningStream)

		ctx, cancel := context.WithCancel(context.Background())

		sm.context = ctx
		sm.cancelFunc = cancel

		go func() {
			for {
				select {
				case <-time.After(monitorInterval):
					sm.monitorStreams()
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

// Shutdown shuts down the stream manager, killing any running transcoding processes and removing all cached files.
func (sm *StreamManager) Shutdown() {
	if sm.cancelFunc != nil {
		sm.cancelFunc()
	}
	sm.stopAndRemoveAll()
}

type StreamRequestContext struct {
	context.Context
	ResponseWriter http.ResponseWriter
}

func NewStreamRequestContext(w http.ResponseWriter, r *http.Request) *StreamRequestContext {
	return &StreamRequestContext{
		Context:        r.Context(),
		ResponseWriter: w,
	}
}

func (c *StreamRequestContext) Cancel() {
	hj, ok := (c.ResponseWriter).(http.Hijacker)
	if !ok {
		return
	}

	// hijack and close the connection
	conn, bw, _ := hj.Hijack()
	if conn != nil {
		if bw != nil {
			// notify end of stream
			_, err := bw.WriteString("0\r\n")
			if err != nil {
				logger.Warnf("unable to write end of stream: %v", err)
			}
			_, err = bw.WriteString("\r\n")
			if err != nil {
				logger.Warnf("unable to write end of stream: %v", err)
			}

			// flush the buffer, but don't wait indefinitely
			timeout := make(chan struct{}, 1)
			go func() {
				_ = bw.Flush()
				close(timeout)
			}()

			const waitTime = time.Second

			select {
			case <-timeout:
			case <-time.After(waitTime):
				logger.Warnf("unable to flush buffer - closing connection")
			}
		}

		conn.Close()
	}
}
