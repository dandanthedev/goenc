package storage

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var LocalStoragePath string = os.Getenv("LOCAL_STORAGE_PATH")

func InitLocalStorage() {
	if LocalStoragePath == "" {
		slog.Warn("LOCAL_STORAGE_PATH is not set, defaulting to 'data/'")
		LocalStoragePath = "data/"
	}
	os.Mkdir(os.Getenv("LOCAL_STORAGE_PATH"), 0755)
}

func LocalFileExists(path string) bool {
	_, err := os.Stat(LocalStoragePath + path)
	return !os.IsNotExist(err)
}

func LocalFileGet(path string) ([]byte, error) {
	data, err := os.ReadFile(LocalStoragePath + path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func LocalFilePut(path string, data []byte) error {
	// create dir if it doesn't exist. get the directory part of the path
	dir := LocalStoragePath + strings.TrimSuffix(path, "/"+filepath.Base(path))
	if dir != LocalStoragePath {
		os.MkdirAll(dir, 0755)
	}
	slog.Debug("Writing local file", "path", path)
	err := os.WriteFile(LocalStoragePath+path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func LocalDirectoryCreate(path string) error {
	path = LocalStoragePath + path
	slog.Debug("Creating local directory", "path", path)
	return os.MkdirAll(path, 0755)
}

func LocalDirectoryListing(path string, recursive bool, includeFolders bool) ([]string, error) {
	if recursive {
		return filepath.Glob(LocalStoragePath + path + "/*")
	}
	files, err := os.ReadDir(LocalStoragePath + path)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, file := range files {
		if file.IsDir() {
			if includeFolders {
				fileNames = append(fileNames, file.Name())
			} else {
				continue
			}
		}
		fileNames = append(fileNames, file.Name())
	}
	return fileNames, nil
}

func LocalFileDelete(path string) error {
	err := os.Remove(LocalStoragePath + path)
	if err != nil {
		return err
	}
	return nil
}

func LocalDirectoryDelete(path string) error {
	err := os.RemoveAll(LocalStoragePath + path)
	if err != nil {
		return err
	}
	return nil
}
