package storage

import (
	"log/slog"
	"net/http"
	"os"
)

var storageMode string = os.Getenv("STORAGE_MODE")

func InitStorage() {
	if storageMode == "" {
		storageMode = "local"
	}

	switch storageMode {
	case "local":
		InitLocalStorage()
	case "s3":
		InitS3Storage()
	default:
		slog.Error("invalid storage mode", "mode", storageMode)
		os.Exit(1)
	}
}

func FileExists(path string) bool {
	switch storageMode {
	case "local":
		return LocalFileExists(path)
	case "s3":
		return S3FileExists(path)
	default:
		panic("invalid storage mode")
	}
}

type GetResult struct {
	Data *[]byte
	URL  *string
}

func FileGet(path string, download bool) (GetResult, error) {
	switch storageMode {
	case "local":
		data, err := LocalFileGet(path)
		if err != nil {
			return GetResult{}, err
		}
		return GetResult{Data: &data}, nil
	case "s3":
		return S3FileGet(path, download)

	default:
		panic("invalid storage mode")
	}
}

func ServeFile(path string, w http.ResponseWriter, download bool) {
	file, err := FileGet(path, download)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if file.URL != nil {
		w.Header().Set("Location", *file.URL)
		w.WriteHeader(http.StatusFound)
		return
	}
	if file.Data != nil {
		w.Write(*file.Data)
		return
	}
	w.WriteHeader(http.StatusNotFound)

}

func FilePut(path string, data []byte) error {
	switch storageMode {
	case "local":
		return LocalFilePut(path, data)
	case "s3":
		return S3FilePut(path, data)
	default:
		panic("invalid storage mode")
	}
}

func DirectoryCreate(path string) error {
	switch storageMode {
	case "local":
		return LocalDirectoryCreate(path)
	case "s3":
		slog.Debug("S3 bucket does not support directory creation", "path", path)
		return nil
	default:
		panic("invalid storage mode")
	}
}

func FileDelete(path string) error {
	switch storageMode {
	case "local":
		return LocalFileDelete(path)
	case "s3":
		return S3FileDelete(path)
	default:
		panic("invalid storage mode")
	}
}

func DirectoryDelete(path string) error {
	switch storageMode {
	case "local":
		return LocalDirectoryDelete(path)
	case "s3":
		files, err := S3DirectoryListing(path, true, false)
		if err != nil {
			return err
		}
		for _, file := range files {
			slog.Debug("Deleting S3 file", "file", file)
			if err := S3FileDelete(file); err != nil {
				return err
			}
		}
		return nil
	default:
		panic("invalid storage mode")
	}
}

func DirectoryListing(path string, recursive bool, includeFolders bool) ([]string, error) {
	switch storageMode {
	case "local":
		return LocalDirectoryListing(path, recursive, includeFolders)
	case "s3":
		return S3DirectoryListing(path, recursive, includeFolders)
	default:
		panic("invalid storage mode")
	}
}
