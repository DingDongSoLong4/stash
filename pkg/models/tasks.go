package models

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/graphql"
)

type ScanMetadataOptions struct {
	// Set name, date, details from metadata (if present)
	// Deprecated: not implemented
	UseFileMetadata bool `json:"useFileMetadata"`
	// Strip file extension from title
	// Deprecated: not implemented
	StripFileExtension bool `json:"stripFileExtension"`
	// Generate scene covers during scan
	ScanGenerateCovers bool `json:"scanGenerateCovers"`
	// Generate previews during scan
	ScanGeneratePreviews bool `json:"scanGeneratePreviews"`
	// Generate image previews during scan
	ScanGenerateImagePreviews bool `json:"scanGenerateImagePreviews"`
	// Generate sprites during scan
	ScanGenerateSprites bool `json:"scanGenerateSprites"`
	// Generate phashes during scan
	ScanGeneratePhashes bool `json:"scanGeneratePhashes"`
	// Generate image thumbnails during scan
	ScanGenerateThumbnails bool `json:"scanGenerateThumbnails"`
	// Generate image thumbnails during scan
	ScanGenerateClipPreviews bool `json:"scanGenerateClipPreviews"`
}

// Filter options for metadata scanning
type ScanMetadataFilterInput struct {
	// If set, files with a modification time before this time point are ignored by the scan
	MinModTime *time.Time `json:"minModTime"`
}

type ScanMetadataInput struct {
	Paths []string `json:"paths"`

	ScanMetadataOptions `mapstructure:",squash"`

	// Filter options for the scan
	Filter *ScanMetadataFilterInput `json:"filter"`
}

type GeneratePreviewOptionsInput struct {
	// Number of segments in a preview file
	PreviewSegments *int `json:"previewSegments"`
	// Preview segment duration, in seconds
	PreviewSegmentDuration *float64 `json:"previewSegmentDuration"`
	// Duration of start of video to exclude when generating previews
	PreviewExcludeStart *string `json:"previewExcludeStart"`
	// Duration of end of video to exclude when generating previews
	PreviewExcludeEnd *string `json:"previewExcludeEnd"`
	// Preset when generating preview
	PreviewPreset *PreviewPreset `json:"previewPreset"`
}

type GenerateMetadataInput struct {
	Covers              bool                         `json:"covers"`
	Sprites             bool                         `json:"sprites"`
	Previews            bool                         `json:"previews"`
	ImagePreviews       bool                         `json:"imagePreviews"`
	PreviewOptions      *GeneratePreviewOptionsInput `json:"previewOptions"`
	Markers             bool                         `json:"markers"`
	MarkerImagePreviews bool                         `json:"markerImagePreviews"`
	MarkerScreenshots   bool                         `json:"markerScreenshots"`
	Transcodes          bool                         `json:"transcodes"`
	// Generate transcodes even if not required
	ForceTranscodes           bool `json:"forceTranscodes"`
	Phashes                   bool `json:"phashes"`
	InteractiveHeatmapsSpeeds bool `json:"interactiveHeatmapsSpeeds"`
	ClipPreviews              bool `json:"clipPreviews"`
	// scene ids to generate for
	SceneIDs []string `json:"sceneIDs"`
	// marker ids to generate for
	MarkerIDs []string `json:"markerIDs"`
	// overwrite existing media
	Overwrite bool `json:"overwrite"`
}

type AutoTagMetadataOptions struct {
	// IDs of performers to tag files with, or "*" for all
	Performers []string `json:"performers"`
	// IDs of studios to tag files with, or "*" for all
	Studios []string `json:"studios"`
	// IDs of tags to tag files with, or "*" for all
	Tags []string `json:"tags"`
}

type AutoTagMetadataInput struct {
	// Paths to tag, null for all files
	Paths []string `json:"paths"`
	// IDs of performers to tag files with, or "*" for all
	Performers []string `json:"performers"`
	// IDs of studios to tag files with, or "*" for all
	Studios []string `json:"studios"`
	// IDs of tags to tag files with, or "*" for all
	Tags []string `json:"tags"`
}

type CleanMetadataInput struct {
	Paths []string `json:"paths"`
	// Do a dry run. Don't delete any files
	DryRun bool `json:"dryRun"`
}

type ImportDuplicateEnum string

const (
	ImportDuplicateEnumIgnore    ImportDuplicateEnum = "IGNORE"
	ImportDuplicateEnumOverwrite ImportDuplicateEnum = "OVERWRITE"
	ImportDuplicateEnumFail      ImportDuplicateEnum = "FAIL"
)

var AllImportDuplicateEnum = []ImportDuplicateEnum{
	ImportDuplicateEnumIgnore,
	ImportDuplicateEnumOverwrite,
	ImportDuplicateEnumFail,
}

func (e ImportDuplicateEnum) IsValid() bool {
	switch e {
	case ImportDuplicateEnumIgnore, ImportDuplicateEnumOverwrite, ImportDuplicateEnumFail:
		return true
	}
	return false
}

func (e ImportDuplicateEnum) String() string {
	return string(e)
}

func (e *ImportDuplicateEnum) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImportDuplicateEnum(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImportDuplicateEnum", str)
	}
	return nil
}

func (e ImportDuplicateEnum) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ImportObjectsInput struct {
	File                graphql.Upload       `json:"file"`
	DuplicateBehaviour  ImportDuplicateEnum  `json:"duplicateBehaviour"`
	MissingRefBehaviour ImportMissingRefEnum `json:"missingRefBehaviour"`
}

type ExportObjectTypeInput struct {
	Ids []string `json:"ids"`
	All *bool    `json:"all"`
}

type ExportObjectsInput struct {
	Scenes              *ExportObjectTypeInput `json:"scenes"`
	Images              *ExportObjectTypeInput `json:"images"`
	Studios             *ExportObjectTypeInput `json:"studios"`
	Performers          *ExportObjectTypeInput `json:"performers"`
	Tags                *ExportObjectTypeInput `json:"tags"`
	Movies              *ExportObjectTypeInput `json:"movies"`
	Galleries           *ExportObjectTypeInput `json:"galleries"`
	IncludeDependencies *bool                  `json:"includeDependencies"`
}

// If neither performer_ids nor performer_names are set, tag all performers
type StashBoxBatchPerformerTagInput struct {
	// Stash endpoint to use for the performer tagging
	Endpoint int `json:"endpoint"`
	// Fields to exclude when executing the performer tagging
	ExcludeFields []string `json:"exclude_fields"`
	// Refresh performers already tagged by StashBox if true. Only tag performers with no StashBox tagging if false
	Refresh bool `json:"refresh"`
	// If set, only tag these performer ids
	PerformerIds []string `json:"performer_ids"`
	// If set, only tag these performer names
	PerformerNames []string `json:"performer_names"`
}
