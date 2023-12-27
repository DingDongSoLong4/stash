package ffmpeg

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	stashExec "github.com/stashapp/stash/pkg/exec"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
)

func GetPaths(paths []string) (string, string) {
	// find ffmpeg in $PATH
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		ffmpegPath = ""
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		ffprobePath = ""
	}

	if ffmpegPath != "" && ffprobePath != "" {
		if verifyFFMpegFlags(ffmpegPath) {
			return ffmpegPath, ffprobePath
		}
	}

	// find ffmpeg in the given paths
	ffmpegPath = fsutil.FindInPaths(paths, getFFMpegFilename())
	ffprobePath = fsutil.FindInPaths(paths, getFFProbeFilename())

	if ffmpegPath != "" && ffprobePath != "" {
		// return absolute paths (exec.LookPath already does this)
		ffmpegPath, _ = filepath.Abs(ffmpegPath)
		ffprobePath, _ = filepath.Abs(ffprobePath)

		if verifyFFMpegFlags(ffmpegPath) {
			return ffmpegPath, ffprobePath
		}
	}

	return "", ""
}

func Download(ctx context.Context, destDir string) error {
	urls := getFFmpegURLs()
	if urls == nil {
		return fmt.Errorf("ffmpeg auto-download is unsupported on this platform")
	}

	for _, url := range urls {
		err := downloadSingle(ctx, destDir, url)
		if err != nil {
			return err
		}
	}

	return nil
}

type progressReader struct {
	io.Reader
	lastProgress int64
	bytesRead    int64
	total        int64
}

func (r *progressReader) Read(p []byte) (int, error) {
	read, err := r.Reader.Read(p)
	if err == nil {
		r.bytesRead += int64(read)
		if r.total > 0 {
			progress := int64(float64(r.bytesRead) / float64(r.total) * 100)
			if progress/5 > r.lastProgress {
				logger.Infof("%d%% downloaded...", progress)
				r.lastProgress = progress / 5
			}
		}
	}

	return read, err
}

func downloadSingle(ctx context.Context, destDir string, url string) error {
	logger.Infof("Downloading %s...", url)

	// Make the HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}

	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	reader := &progressReader{
		Reader: resp.Body,
		total:  resp.ContentLength,
	}

	out, err := os.CreateTemp(destDir, ".ffmpeg-download-*.zip")
	if err != nil {
		return err
	}
	outName := out.Name()
	defer os.Remove(outName)

	// Write the response to the archive file location
	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}
	logger.Info("Downloading complete")

	mime := resp.Header.Get("Content-Type")
	if mime != "application/zip" { // try detecting MIME type since some servers don't return the correct one
		data := make([]byte, 500) // http.DetectContentType only reads up to 500 bytes
		_, _ = out.ReadAt(data, 0)
		mime = http.DetectContentType(data)
	}

	if mime == "application/zip" {
		logger.Infof("Unzipping %s...", outName)
		if err := unzip(outName, destDir); err != nil {
			return err
		}

		// On non-windows platforms set file permissions
		if runtime.GOOS != "windows" {
			ffmpegPath := filepath.Join(destDir, getFFMpegFilename())
			if exists, _ := fsutil.FileExists(ffmpegPath); exists {
				if err := os.Chmod(ffmpegPath, 0755); err != nil {
					return err
				}
			}

			ffprobePath := filepath.Join(destDir, getFFProbeFilename())
			if exists, _ := fsutil.FileExists(ffprobePath); exists {
				if err := os.Chmod(ffprobePath, 0755); err != nil {
					return err
				}
			}

			// TODO: In future possible clear xattr to allow running on osx without user intervention
			// TODO: this however may not be required.
			// xattr -c /path/to/binary -- xattr.Remove(path, "com.apple.quarantine")
		}
	} else {
		return fmt.Errorf("downloaded file is not a zip file")
	}

	return nil
}

func getFFmpegURLs() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"https://evermeet.cx/ffmpeg/getrelease/zip", "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip"}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return []string{"https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v4.2.1/ffmpeg-4.2.1-linux-64.zip", "https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v4.2.1/ffprobe-4.2.1-linux-64.zip"}
		case "arm":
			return []string{"https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v4.2.1/ffmpeg-4.2.1-linux-armhf-32.zip", "https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v4.2.1/ffprobe-4.2.1-linux-armhf-32.zip"}
		case "arm64":
			return []string{"https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v4.2.1/ffmpeg-4.2.1-linux-arm-64.zip", "https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download/v4.2.1/ffprobe-4.2.1-linux-arm-64.zip"}
		}
	case "windows":
		return []string{"https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"}
	}
	return nil
}

func getFFMpegFilename() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func getFFProbeFilename() string {
	if runtime.GOOS == "windows" {
		return "ffprobe.exe"
	}
	return "ffprobe"
}

// Checks if the specified ffmpeg binary was compiled with the correct flags
func verifyFFMpegFlags(ffmpegPath string) bool {
	if ffmpegPath == "" {
		return false
	}

	cmd := stashExec.Command(ffmpegPath, "-version")
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	output := string(bytes)
	hasOpus := strings.Contains(output, "--enable-libopus")
	hasVpx := strings.Contains(output, "--enable-libvpx")
	hasX264 := strings.Contains(output, "--enable-libx264")
	hasX265 := strings.Contains(output, "--enable-libx265")
	hasWebp := strings.Contains(output, "--enable-libwebp")

	if hasOpus && hasVpx && hasX264 && hasX265 && hasWebp {
		return true
	} else {
		logger.Debugf("found incompatible ffmpeg at %s", ffmpegPath)
		return false
	}
}

func unzip(zipFile string, destDir string) error {
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		filename := f.FileInfo().Name()
		if filename != getFFMpegFilename() && filename != getFFProbeFilename() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		out, err := os.Create(filepath.Join(destDir, filename))
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, rc)
		if err != nil {
			return err
		}
	}

	return nil
}
