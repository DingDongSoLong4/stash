package api

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/go-chi/chi"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/static"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/generate"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/utils"
)

type imageRoutesConfig interface {
	IsWriteImageThumbnails() bool
}

type imageRoutes struct {
	routes
	image models.ImageReader
	file  models.FileReader

	manager *manager.Manager
	config  imageRoutesConfig
	paths   *paths.Paths
}

func (rs imageRoutes) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/{imageId}", func(r chi.Router) {
		r.Use(rs.ImageCtx)

		r.Get("/image", rs.Image)
		r.Get("/thumbnail", rs.Thumbnail)
		r.Get("/preview", rs.Preview)
	})

	return r
}

// region Handlers

func (rs imageRoutes) Thumbnail(w http.ResponseWriter, r *http.Request) {
	img := r.Context().Value(imageKey).(*models.Image)
	filepath := rs.paths.Generated.GetThumbnailPath(img.Checksum, models.DefaultGthumbWidth)

	// if the thumbnail exists, serve it
	exists, _ := fsutil.FileExists(filepath)
	if exists {
		utils.ServeStaticFile(w, r, filepath)
		return
	}

	// else encode a thumbnail on the fly

	f := img.Files.Primary()
	if f == nil {
		const useDefault = true
		rs.serveImage(w, r, img, useDefault)
		return
	}

	generator := rs.manager.NewGenerator(false)

	data, err := generator.Thumbnail(r.Context(), f, models.DefaultGthumbWidth)
	if err != nil {
		// don't log for unsupported image format
		// don't log for file not found - can optionally be logged in serveImage
		if !errors.Is(err, generate.ErrUnsupportedFormat) && !errors.Is(err, fs.ErrNotExist) {
			logger.Errorf("error generating thumbnail for %s: %v", f.Base().Path, err)

			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				logger.Errorf("stderr: %s", string(exitErr.Stderr))
			}
		}

		// backwards compatibility - fallback to original image instead
		const useDefault = true
		rs.serveImage(w, r, img, useDefault)
		return
	}

	// write the generated thumbnail to disk if enabled
	if rs.config.IsWriteImageThumbnails() {
		logger.Debugf("writing thumbnail to disk: %s", img.Path)
		if err := fsutil.WriteFile(filepath, data); err == nil {
			utils.ServeStaticFile(w, r, filepath)
			return
		}
		logger.Errorf("error writing thumbnail for image %s: %v", img.Path, err)
	}

	utils.ServeStaticContent(w, r, data)
}

func (rs imageRoutes) Preview(w http.ResponseWriter, r *http.Request) {
	img := r.Context().Value(imageKey).(*models.Image)
	filepath := rs.paths.Generated.GetClipPreviewPath(img.Checksum, models.DefaultGthumbWidth)

	// don't check if the preview exists - we'll just return a 404 if it doesn't
	utils.ServeStaticFile(w, r, filepath)
}

func (rs imageRoutes) Image(w http.ResponseWriter, r *http.Request) {
	i := r.Context().Value(imageKey).(*models.Image)

	const useDefault = false
	rs.serveImage(w, r, i, useDefault)
}

func (rs imageRoutes) serveImage(w http.ResponseWriter, r *http.Request, i *models.Image, useDefault bool) {
	if i.Files.Primary() != nil {
		err := i.Files.Primary().Base().Serve(&file.OsFS{}, w, r)
		if err == nil {
			return
		}

		if !useDefault {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// only log in debug since it can get noisy
		logger.Debugf("Error serving %s: %v", i.DisplayName(), err)
	}

	if !useDefault {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// fallback to default image
	image := static.ReadAll(static.DefaultImageImage)
	utils.ServeImage(w, r, image)
}

// endregion

func (rs imageRoutes) ImageCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imageIdentifierQueryParam := chi.URLParam(r, "imageId")
		imageID, _ := strconv.Atoi(imageIdentifierQueryParam)

		var image *models.Image
		_ = rs.withReadTxn(r, func(ctx context.Context) error {
			if imageID == 0 {
				images, _ := rs.image.FindByChecksum(ctx, imageIdentifierQueryParam)
				if len(images) > 0 {
					image = images[0]
				}
			} else {
				image, _ = rs.image.Find(ctx, imageID)
			}

			if image != nil {
				if err := image.LoadPrimaryFile(ctx, rs.file); err != nil {
					if !errors.Is(err, context.Canceled) {
						logger.Errorf("error loading primary file for image %d: %v", imageID, err)
					}
					// set image to nil so that it doesn't try to use the primary file
					image = nil
				}
			}

			return nil
		})
		if image == nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		ctx := context.WithValue(r.Context(), imageKey, image)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
