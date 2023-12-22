package api

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/collate"
)

// listDir gets a list of all the directories contained in the directory specified
// by everything before the final slash in path. Everything after the final slash
// will be used to filter the results.
func listDir(col *collate.Collator, path string) ([]string, string, error) {
	// filter by everything after the final slash
	_, filterName := filepath.Split(path)
	// don't filter by "." or "..", and leave them in dir
	if filterName == "." || filterName == ".." {
		filterName = ""
	}

	// remove the filter suffix from path
	dir := strings.TrimSuffix(path, filterName)

	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", err
	}

	filterName = strings.ToLower(filterName)

	// filter contents by last path fragment
	// and exclude non-directories
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if strings.HasPrefix(strings.ToLower(fileName), filterName) {
			dirs = append(dirs, fileName)
		}
	}

	if col != nil {
		col.SortStrings(dirs)
	}

	return dirs, dir, nil
}

func getParentDir(path string) *string {
	path = filepath.Clean(path)

	// if path ends in a slash after clean, then path is a root dir
	if path[len(path)-1] == filepath.Separator {
		return nil
	}

	dir := filepath.Dir(path)
	return &dir
}
