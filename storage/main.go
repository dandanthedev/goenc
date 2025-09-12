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

func FileGet(path string, download bool) GetResult {
	switch storageMode {
	case "local":
		data := LocalFileGet(path)
		return GetResult{Data: &data}
	case "s3":
		data := S3FileGet(path, download)
		return data
	default:
		panic("invalid storage mode")
	}
}

func ServeFile(path string, w http.ResponseWriter) {
	file := FileGet(path, false)
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

func FilePut(path string, data []byte) {
	switch storageMode {
	case "local":
		LocalFilePut(path, data)
	case "s3":
		S3FilePut(path, data)
	default:
		panic("invalid storage mode")
	}
}

func DirectoryCreate(path string) {
	switch storageMode {
	case "local":
		LocalDirectoryCreate(path)
	case "s3":
		slog.Debug("S3 bucket does not support directory creation", "path", path)
	default:
		panic("invalid storage mode")
	}
}

func FileDelete(path string) {
	switch storageMode {
	case "local":
		LocalFileDelete(path)
	case "s3":
		S3FileDelete(path)
	default:
		panic("invalid storage mode")
	}
}

func DirectoryDelete(path string) {
	switch storageMode {
	case "local":
		LocalDirectoryDelete(path)
	case "s3":
		slog.Debug("S3 bucket does not support directory deletion", "path", path)
	default:
		panic("invalid storage mode")
	}
}
