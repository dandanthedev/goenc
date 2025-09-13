package api

import (
	"encoding/json"
	"goenc/encoder"
	"goenc/storage"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func APIRouter(inputRouter chi.Router) {
	r := chi.NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-API-KEY")

			if token == "" {
				ReplyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing x-api-key header"})
				return
			}

			apiKey := os.Getenv("API_KEY")
			if apiKey == "" {
				ReplyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "API_KEY is not set"})
				return
			}

			if token != apiKey {
				ReplyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "api key is invalid"})
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	r.Post("/token", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]any

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "JWT_SECRET is not set"})
			return
		}

		id := data["id"]
		expires := data["expires"]
		attributes := data["attributes"]

		idStr, ok := id.(string)
		if !ok {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id must be a string"})
			return
		}

		expiresStr, ok := expires.(string)
		if !ok {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "expires must be a string"})
			return
		}

		attributesObj, ok := attributes.(map[string]any)
		if !ok {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "attributes must be an object"})
			return
		}

		if expiresStr == "" {
			expiresStr = "1h"
		}
		if idStr == "" {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}

		//check if id is valid
		valid := storage.FileExists(idStr + "/meta.json")
		if !valid {
			ReplyWithJSON(w, http.StatusNotFound, map[string]string{"error": "id does not exist"})
			return
		}

		duration, err := time.ParseDuration(expiresStr)
		if err != nil {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expires format"})
			return
		}

		exp := time.Now().Add(duration).Unix()

		//convert attributes to url params
		searchParams := ""
		for k, v := range attributesObj {
			switch v := v.(type) {
			case string:
				searchParams += "&" + k + "=" + v
			case bool:
				if v {
					searchParams += "&" + k + "=true"
				} else {
					searchParams += "&" + k + "=false"
				}
			case float64:
				searchParams += "&" + k + "=" + strconv.FormatFloat(v, 'f', -1, 64)
			default:
				ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid attribute type", "attribute": k})
				return
			}
		}
		if len(searchParams) > 0 {
			searchParams = "?" + searchParams[1:]
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": id,
			"exp": exp,
		})
		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			ReplyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
			return
		}

		//add token to search params
		if searchParams == "" {
			searchParams = "?token=" + tokenString
		} else {
			searchParams += "&token=" + tokenString
		}

		playerUrl := "/" + idStr + searchParams

		ReplyWithJSON(w, http.StatusOK, map[string]string{"token": tokenString, "playerUrl": playerUrl, "expires": strconv.FormatInt(exp, 10)})

	})

	r.Get("/queue", func(w http.ResponseWriter, r *http.Request) {
		queue := encoder.GetQueue()
		w.Header().Set("Content-Type", "application/json")

		if queue == nil {
			queue = []encoder.QueueItem{}
		}

		ReplyWithJSON(w, http.StatusOK, map[string]any{
			"queue": queue,
		})

	})

	r.Post("/queue/recover", func(w http.ResponseWriter, r *http.Request) {
		encoder.RecoverStuckProcessingJobs()
		w.WriteHeader(http.StatusNoContent)
	})

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		//get file id from query
		id := r.URL.Query().Get("id")
		if id == "" {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}

		valid := storage.FileExists(id + "/meta.json")
		if valid {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id already exists"})
			return
		}

		//check if id only contains alphanumeric characters
		allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for _, c := range id {
			if !strings.Contains(allowedChars, string(c)) {
				ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "ID may only contain alphanumeric characters"})
				return
			}
		}

		profiles := r.URL.Query().Get("profiles")
		if profiles == "" {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "profiles is required"})
			return
		}

		// Parse the multipart form with a reasonable maxMemory (e.g., 32MB)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse multipart form"})
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not find file"})
			return
		}
		defer file.Close()

		//write file to tmp
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			ReplyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
			return
		}
		storage.LocalFilePut("tmp/"+id+"/"+"input", fileBytes)

		id = encoder.AddFileToQueue("data/tmp/"+id+"/"+"input", id, profiles)

		ReplyWithJSON(w, http.StatusOK, map[string]string{"id": id})
	})

	videosRouter(r)

	inputRouter.Mount("/api", r)

}

func IdValid(id string) bool {
	allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, c := range id {
		if !strings.Contains(allowedChars, string(c)) {
			return false
		}
	}
	return storage.FileExists(id + "/meta.json")
}

func videosRouter(inputRouter chi.Router) {
	r := chi.NewRouter()

	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !IdValid(id) {
			ReplyWithJSON(w, http.StatusNotFound, map[string]string{"error": "id does not exist"})
			return
		}

		//we download meta as redirects aren't the best for apis
		w.Header().Set("Content-Type", "application/json")
		storage.ServeFile(id+"/meta.json", w, true)
	})

	r.Delete("/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !IdValid(id) {
			ReplyWithJSON(w, http.StatusNotFound, map[string]string{"error": "id does not exist"})
			return
		}

		//we delete meta first to prevent multiple delete options as much as possible
		err := storage.FileDelete(id + "/meta.json")
		if err != nil {
			ReplyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete meta.json"})
			return
		}

		err = storage.DirectoryDelete(id)
		if err != nil {
			ReplyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete directory"})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	inputRouter.Mount("/videos", r)
}
