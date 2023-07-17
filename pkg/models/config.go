package models

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/stashapp/stash/pkg/fsutil"
)

// Stash configuration details
type StashConfigInput struct {
	Path         string `json:"path"`
	ExcludeVideo bool   `json:"excludeVideo"`
	ExcludeImage bool   `json:"excludeImage"`
}

type StashConfig struct {
	Path         string `json:"path"`
	ExcludeVideo bool   `json:"excludeVideo"`
	ExcludeImage bool   `json:"excludeImage"`
}

type StashConfigs []*StashConfig

func (s StashConfigs) GetStashFromPath(path string) *StashConfig {
	for _, f := range s {
		if fsutil.IsPathInDir(f.Path, filepath.Dir(path)) {
			return f
		}
	}
	return nil
}

func (s StashConfigs) GetStashFromDirPath(dirPath string) *StashConfig {
	for _, f := range s {
		if fsutil.IsPathInDir(f.Path, dirPath) {
			return f
		}
	}
	return nil
}

type BlobsStorageType string

const (
	// Database
	BlobStorageTypeDatabase BlobsStorageType = "DATABASE"
	// Filesystem
	BlobStorageTypeFilesystem BlobsStorageType = "FILESYSTEM"
)

var AllBlobStorageType = []BlobsStorageType{
	BlobStorageTypeDatabase,
	BlobStorageTypeFilesystem,
}

func (e BlobsStorageType) IsValid() bool {
	switch e {
	case BlobStorageTypeDatabase, BlobStorageTypeFilesystem:
		return true
	}
	return false
}

func (e BlobsStorageType) String() string {
	return string(e)
}

func (e *BlobsStorageType) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = BlobsStorageType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid BlobStorageType", str)
	}
	return nil
}

func (e BlobsStorageType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type StashBoxInput struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
	Name     string `json:"name"`
}

type ConfigImageLightboxResult struct {
	SlideshowDelay             *int                      `json:"slideshowDelay"`
	DisplayMode                *ImageLightboxDisplayMode `json:"displayMode"`
	ScaleUp                    *bool                     `json:"scaleUp"`
	ResetZoomOnNav             *bool                     `json:"resetZoomOnNav"`
	ScrollMode                 *ImageLightboxScrollMode  `json:"scrollMode"`
	ScrollAttemptsBeforeChange int                       `json:"scrollAttemptsBeforeChange"`
}

type ImageLightboxDisplayMode string

const (
	ImageLightboxDisplayModeOriginal ImageLightboxDisplayMode = "ORIGINAL"
	ImageLightboxDisplayModeFitXy    ImageLightboxDisplayMode = "FIT_XY"
	ImageLightboxDisplayModeFitX     ImageLightboxDisplayMode = "FIT_X"
)

var AllImageLightboxDisplayMode = []ImageLightboxDisplayMode{
	ImageLightboxDisplayModeOriginal,
	ImageLightboxDisplayModeFitXy,
	ImageLightboxDisplayModeFitX,
}

func (e ImageLightboxDisplayMode) IsValid() bool {
	switch e {
	case ImageLightboxDisplayModeOriginal, ImageLightboxDisplayModeFitXy, ImageLightboxDisplayModeFitX:
		return true
	}
	return false
}

func (e ImageLightboxDisplayMode) String() string {
	return string(e)
}

func (e *ImageLightboxDisplayMode) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImageLightboxDisplayMode(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImageLightboxDisplayMode", str)
	}
	return nil
}

func (e ImageLightboxDisplayMode) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ImageLightboxScrollMode string

const (
	ImageLightboxScrollModeZoom ImageLightboxScrollMode = "ZOOM"
	ImageLightboxScrollModePanY ImageLightboxScrollMode = "PAN_Y"
)

var AllImageLightboxScrollMode = []ImageLightboxScrollMode{
	ImageLightboxScrollModeZoom,
	ImageLightboxScrollModePanY,
}

func (e ImageLightboxScrollMode) IsValid() bool {
	switch e {
	case ImageLightboxScrollModeZoom, ImageLightboxScrollModePanY:
		return true
	}
	return false
}

func (e ImageLightboxScrollMode) String() string {
	return string(e)
}

func (e *ImageLightboxScrollMode) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImageLightboxScrollMode(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImageLightboxScrollMode", str)
	}
	return nil
}

func (e ImageLightboxScrollMode) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ConfigDisableDropdownCreate struct {
	Performer bool `json:"performer"`
	Tag       bool `json:"tag"`
	Studio    bool `json:"studio"`
}
