package env

import (
	"os"
	"path/filepath"
)

// GetBasedir finds the Git repository root directory and use that as base. If one cannot be found,
// uses the current working directory.
func GetBasedir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := getGitRoot(wd)
	if dir != "" {
		return dir, nil
	}
	return wd, nil
}

func getGitRoot(dir string) string {
	currentDir := dir

	for {
		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return ""
		}

		for _, de := range entries {
			if !de.IsDir() {
				continue
			}
			if de.Name() == ".git" {
				return currentDir
			}
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return ""
		}
		currentDir = parentDir
	}
}
