package main

import (
	"encoding/json"
	"goenc/encoder"
	"goenc/player"
	"goenc/storage"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
)

func replyWithJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func resValid(res string) bool {
	for _, sm := range encoder.SizeMapping {
		if sm.Label == res {
			return true
		}
	}
	return false
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	storage.InitStorage()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//if doesnt start with /data, continue
			if !strings.HasPrefix(r.URL.Path, "/data") {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("token")
			id := r.Header.Get("id")
			if token == "" || id == "" {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token or id"})
				return
			}
			claims, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte(os.Getenv("JWT_SECRET")), nil
			})
			if err != nil {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}
			if !claims.Valid {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}

			//id should only include alphanumeric characters
			allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			for _, c := range id {
				if !strings.Contains(allowedChars, string(c)) {
					replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "ID may only contain alphanumeric characters"})
					return
				}
			}

			//check if id is valid
			valid := storage.FileExists(id + "/meta.json")
			if !valid {
				replyWithJSON(w, http.StatusNotFound, map[string]string{"error": "id does not exist"})
				return
			}

			//check if sub is id
			sub, err := claims.Claims.GetSubject()
			if err != nil {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}
			if sub != id {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "token does not match id"})
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api") {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-API-KEY")

			if token == "" {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing x-api-key header"})
				return
			}

			apiKey := os.Getenv("API_KEY")
			if apiKey == "" {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "API_KEY is not set"})
				return
			}

			if token != apiKey {
				replyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "api key is invalid"})
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	r.Post("/api/key", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]any

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "JWT_SECRET is not set"})
			return
		}

		id := data["id"]
		expires := data["expires"]
		attributes := data["attributes"]

		idStr, ok := id.(string)
		if !ok {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id must be a string"})
			return
		}

		expiresStr, ok := expires.(string)
		if !ok {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "expires must be a string"})
			return
		}

		attributesObj, ok := attributes.(map[string]any)
		if !ok {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "attributes must be an object"})
			return
		}

		if expiresStr == "" {
			expiresStr = "1h"
		}
		if idStr == "" {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}

		//check if id is valid
		valid := storage.FileExists(idStr + "/meta.json")
		if !valid {
			replyWithJSON(w, http.StatusNotFound, map[string]string{"error": "id does not exist"})
			return
		}

		duration, err := time.ParseDuration(expiresStr)
		if err != nil {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expires format"})
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
				replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid attribute type", "attribute": k})
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
			replyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
			return
		}

		//add token to search params
		if searchParams == "" {
			searchParams = "?token=" + tokenString
		} else {
			searchParams += "&token=" + tokenString
		}

		playerUrl := "/" + idStr + searchParams

		replyWithJSON(w, http.StatusOK, map[string]string{"token": tokenString, "playerUrl": playerUrl, "expires": strconv.FormatInt(exp, 10)})

	})

	r.Get("/api/queue", func(w http.ResponseWriter, r *http.Request) {
		queue := encoder.GetQueue()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(queue)
	})

	r.Post("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		//get file id from query
		id := r.URL.Query().Get("id")
		if id == "" {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}

		valid := storage.FileExists(id + "/meta.json")
		if valid {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id already exists"})
			return
		}

		//check if id only contains alphanumeric characters
		allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for _, c := range id {
			if !strings.Contains(allowedChars, string(c)) {
				replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "ID may only contain alphanumeric characters"})
				return
			}
		}

		profiles := r.URL.Query().Get("profiles")
		if profiles == "" {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "profiles is required"})
			return
		}

		// Parse the multipart form with a reasonable maxMemory (e.g., 32MB)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse multipart form"})
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not find file"})
			return
		}
		defer file.Close()

		//write file to tmp
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			replyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
			return
		}
		storage.LocalFilePut("tmp/"+id+"/"+"input", fileBytes)

		id = encoder.AddFileToQueue("data/tmp/"+id+"/"+"input", id, profiles)

		replyWithJSON(w, http.StatusOK, map[string]string{"id": id})
	})

	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(player.GeneratePlayer(chi.URLParam(r, "id"), r.URL.Query().Get("token"))))
	})

	r.Get("/data/validate", func(w http.ResponseWriter, r *http.Request) {
		replyWithJSON(w, http.StatusOK, map[string]string{"valid": "true"})
	})

	r.Get("/data/hls", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/master.m3u8", w)
	})
	r.Get("/data/{res}", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")
		res := chi.URLParam(r, "res")

		if !resValid(res) {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid resolution"})
			return
		}

		resIndex := storage.FileGet(id+"/"+res+"/index.m3u8", true).Data
		stringRes := string(*resIndex)
		modified := stringRes
		modified = regexp.MustCompile(`seg_(\d+)\.m4s`).ReplaceAllStringFunc(modified, func(match string) string {
			num := regexp.MustCompile(`\d+`).FindString(match)
			return "/data/" + res + "/" + num
		})

		modified = regexp.MustCompile(`init\.mp4`).ReplaceAllString(modified, "/data/"+res+"/init")

		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(modified))
	})
	r.Get("/data/{res}/{seg}", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")
		res := chi.URLParam(r, "res")
		seg := chi.URLParam(r, "seg")
		if !resValid(res) {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid resolution"})
			return
		}

		//seg should be numeric
		if _, err := strconv.Atoi(seg); err != nil {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "seg must be a number"})
			return
		}

		storage.ServeFile(id+"/"+res+"/seg_"+seg+".m4s", w)
	})
	r.Get("/data/{res}/init", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")
		res := chi.URLParam(r, "res")
		if !resValid(res) {
			replyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid resolution"})
			return
		}
		storage.ServeFile(id+"/"+res+"/init.mp4", w)
	})
	r.Get("/data/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/imgs/thumbnail.jpg", w)
	})
	r.Get("/data/previews", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/imgs/preview.json", w)
	})
	r.Get("/data/previews/{img}", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/imgs/prev-"+chi.URLParam(r, "img")+".jpg", w)
	})

	encoder.StartConsumer()

	slog.Info("Server started on :3000")
	if err := http.ListenAndServe(":3000", r); err != nil {
		slog.Error("Server failed", "error", err)
	}
}
