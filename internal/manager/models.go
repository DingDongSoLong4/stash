package manager

import (
	"github.com/stashapp/stash/internal/manager/config"
)

type SystemStatus struct {
	DatabaseSchema *int             `json:"databaseSchema"`
	DatabasePath   *string          `json:"databasePath"`
	ConfigPath     *string          `json:"configPath"`
	AppSchema      int              `json:"appSchema"`
	Status         SystemStatusEnum `json:"status"`
}

type SetupInput struct {
	// Empty to indicate $HOME/.stash/config.yml default
	ConfigLocation string                     `json:"configLocation"`
	Stashes        []*config.StashConfigInput `json:"stashes"`
	// Empty to indicate default
	DatabaseFile string `json:"databaseFile"`
	// Empty to indicate default
	GeneratedLocation string `json:"generatedLocation"`
	// Empty to indicate default
	CacheLocation string `json:"cacheLocation"`
	// Empty to indicate database storage for blobs
	BlobsLocation string `json:"blobsLocation"`
}

type MigrateInput struct {
	BackupPath string `json:"backupPath"`
}
