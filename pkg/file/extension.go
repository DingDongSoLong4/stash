package file

import (
	"context"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/models"
)

type ExtensionMatcher struct {
	CreateImageClipsFromVideos bool
	StashPaths                 models.StashConfigs

	galleryExtensions []string
	videoExtensions   []string
	imageExtensions   []string
}

func (m *ExtensionMatcher) UseAsVideo(pathname string) bool {
	if m.CreateImageClipsFromVideos && m.StashPaths.GetStashFromDirPath(pathname).ExcludeVideo {
		return false
	}
	return m.IsVideo(pathname)
}

func (m *ExtensionMatcher) UseAsImage(pathname string) bool {
	if m.CreateImageClipsFromVideos && m.StashPaths.GetStashFromDirPath(pathname).ExcludeVideo {
		return m.IsImage(pathname) || m.IsVideo(pathname)
	}
	return m.IsImage(pathname)
}

func (m *ExtensionMatcher) IsZip(pathname string) bool {
	return fsutil.MatchExtension(pathname, m.galleryExtensions)
}

func (m *ExtensionMatcher) IsVideo(pathname string) bool {
	return fsutil.MatchExtension(pathname, m.videoExtensions)
}

func (m *ExtensionMatcher) IsImage(pathname string) bool {
	return fsutil.MatchExtension(pathname, m.imageExtensions)
}

func (m *ExtensionMatcher) VideoFilterFunc() FilterFunc {
	return func(ctx context.Context, f models.File) bool {
		return m.UseAsVideo(f.Base().Path)
	}
}

func (m *ExtensionMatcher) ImageFilterFunc() FilterFunc {
	return func(ctx context.Context, f models.File) bool {
		return m.UseAsImage(f.Base().Path)
	}
}

func (m *ExtensionMatcher) GalleryFilterFunc() FilterFunc {
	return func(ctx context.Context, f models.File) bool {
		return m.IsZip(f.Base().Basename)
	}
}
