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

func LocalFileGet(path string) []byte {
	data, err := os.ReadFile("data/" + path)
	if err != nil {
		panic("failed to read file: " + err.Error())
	}
	return data
}

func LocalFilePut(path string, data []byte) {
	// create dir if it doesn't exist. get the directory part of the path
	dir := "data/" + strings.TrimSuffix(path, "/"+filepath.Base(path))
	if dir != "data/" {
		os.MkdirAll(dir, 0755)
	}
	slog.Debug("Writing local file", "path", path)
	err := os.WriteFile("data/"+path, data, 0644)
	if err != nil {
		panic("failed to write file: " + err.Error())
	}
}

func LocalDirectoryCreate(path string) {
	path = "data/" + path
	slog.Debug("Creating local directory", "path", path)
	os.MkdirAll(path, 0755)
}

func LocalDirectoryListing(path string) []string {
	files, err := os.ReadDir("data/" + path)
	if err != nil {
		panic("failed to read directory: " + err.Error())
	}
	var fileNames []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileNames = append(fileNames, file.Name())
	}
	return fileNames
}

func LocalFileDelete(path string) {
	err := os.Remove("data/" + path)
	if err != nil {
		panic("failed to delete file: " + err.Error())
	}
}

func LocalDirectoryDelete(path string) {
	err := os.RemoveAll("data/" + path)
	if err != nil {
		panic("failed to delete directory: " + err.Error())
	}
}
