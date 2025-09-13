package storage

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func InitLocalStorage() {
	os.Mkdir("data", 0755)
}

func LocalFileExists(path string) bool {
	_, err := os.Stat("data/" + path)
	return !os.IsNotExist(err)
}

func LocalFileGet(path string) ([]byte, error) {
	data, err := os.ReadFile("data/" + path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func LocalFilePut(path string, data []byte) error {
	// create dir if it doesn't exist. get the directory part of the path
	dir := "data/" + strings.TrimSuffix(path, "/"+filepath.Base(path))
	if dir != "data/" {
		os.MkdirAll(dir, 0755)
	}
	slog.Debug("Writing local file", "path", path)
	err := os.WriteFile("data/"+path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func LocalDirectoryCreate(path string) error {
	path = "data/" + path
	slog.Debug("Creating local directory", "path", path)
	return os.MkdirAll(path, 0755)
}

func LocalDirectoryListing(path string, recursive bool) ([]string, error) {
	if recursive {
		return filepath.Glob("data/" + path + "/*")
	}
	files, err := os.ReadDir("data/" + path)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileNames = append(fileNames, file.Name())
	}
	return fileNames, nil
}

func LocalFileDelete(path string) error {
	err := os.Remove("data/" + path)
	if err != nil {
		return err
	}
	return nil
}

func LocalDirectoryDelete(path string) error {
	err := os.RemoveAll("data/" + path)
	if err != nil {
		return err
	}
	return nil
}
