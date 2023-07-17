package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/stashapp/stash/internal/static"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
)

type movieRoutes struct {
	routes
	movie models.MovieReader
}

func (rs movieRoutes) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/{movieId}", func(r chi.Router) {
		r.Use(rs.MovieCtx)
		r.Get("/frontimage", rs.FrontImage)
		r.Get("/backimage", rs.BackImage)
	})

	return r
}

func (rs movieRoutes) FrontImage(w http.ResponseWriter, r *http.Request) {
	movie := r.Context().Value(movieKey).(*models.Movie)
	defaultParam := r.URL.Query().Get("default")
	var image []byte
	if defaultParam != "true" {
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			image, err = rs.movie.GetFrontImage(ctx, movie.ID)
			return err
		})
		if errors.Is(readTxnErr, context.Canceled) {
			return
		}
		if readTxnErr != nil {
			logger.Warnf("read transaction error on fetch movie front image: %v", readTxnErr)
		}
	}

	// fallback to default image
	if len(image) == 0 {
		image = static.ReadAll(static.DefaultMovieImage)
	}

	utils.ServeImage(w, r, image)
}

func (rs movieRoutes) BackImage(w http.ResponseWriter, r *http.Request) {
	movie := r.Context().Value(movieKey).(*models.Movie)
	defaultParam := r.URL.Query().Get("default")
	var image []byte
	if defaultParam != "true" {
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			image, err = rs.movie.GetBackImage(ctx, movie.ID)
			return err
		})
		if errors.Is(readTxnErr, context.Canceled) {
			return
		}
		if readTxnErr != nil {
			logger.Warnf("read transaction error on fetch movie back image: %v", readTxnErr)
		}
	}

	// fallback to default image
	if len(image) == 0 {
		image = static.ReadAll(static.DefaultMovieImage)
	}

	utils.ServeImage(w, r, image)
}

func (rs movieRoutes) MovieCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		movieID, err := strconv.Atoi(chi.URLParam(r, "movieId"))
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		var movie *models.Movie
		_ = rs.withReadTxn(r, func(ctx context.Context) error {
			movie, _ = rs.movie.Find(ctx, movieID)
			return nil
		})
		if movie == nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		ctx := context.WithValue(r.Context(), movieKey, movie)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
