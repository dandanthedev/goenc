package api

import (
	"goenc/storage"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func VerifyRequest(r *http.Request) bool {
	token := r.Header.Get("token")
	id := r.Header.Get("id")
	if token == "" || id == "" {
		return false
	}
	claims, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return false
	}
	if !claims.Valid {
		return false
	}

	//id should only include alphanumeric characters
	allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, c := range id {
		if !strings.Contains(allowedChars, string(c)) {
			return false
		}
	}

	//check if id is valid
	valid := storage.FileExists(id + "/meta.json")
	if !valid {
		return false
	}

	//check if sub is id
	sub, err := claims.Claims.GetSubject()
	if err != nil {
		return false
	}
	if sub != id {
		return false
	}

	return true
}

func VideoDataRouter(inputRouter chi.Router) {
	r := chi.NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !VerifyRequest(r) {
				ReplyWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token or id"})
				return
			}

			next.ServeHTTP(w, r)
		})
	})
	r.Get("/validate", func(w http.ResponseWriter, r *http.Request) {
		ReplyWithJSON(w, http.StatusOK, map[string]string{"valid": "true"})
	})

	r.Get("/hls", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/master.m3u8", w, false)
	})
	r.Get("/{res}", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")
		res := chi.URLParam(r, "res")

		if !resValid(res) {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid resolution"})
			return
		}

		result, err := storage.FileGet(id+"/"+res+"/index.m3u8", true)
		if err != nil {
			ReplyWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get index.m3u8"})
			return
		}
		stringRes := string(*result.Data)
		modified := stringRes
		modified = regexp.MustCompile(`seg_(\d+)\.m4s`).ReplaceAllStringFunc(modified, func(match string) string {
			num := regexp.MustCompile(`\d+`).FindString(match)
			return "/data/" + res + "/" + num
		})

		modified = regexp.MustCompile(`init\.mp4`).ReplaceAllString(modified, "/data/"+res+"/init")

		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(modified))
	})
	r.Get("/{res}/{seg}", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")
		res := chi.URLParam(r, "res")
		seg := chi.URLParam(r, "seg")
		if !resValid(res) {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid resolution"})
			return
		}

		//seg should be numeric
		if _, err := strconv.Atoi(seg); err != nil {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "seg must be a number"})
			return
		}

		storage.ServeFile(id+"/"+res+"/seg_"+seg+".m4s", w, false)
	})
	r.Get("/{res}/init", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")
		res := chi.URLParam(r, "res")
		if !resValid(res) {
			ReplyWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid resolution"})
			return
		}
		storage.ServeFile(id+"/"+res+"/init.mp4", w, false)
	})
	r.Get("/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/imgs/thumbnail.jpg", w, false)
	})
	r.Get("/previews", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/imgs/preview.json", w, false)
	})
	r.Get("/previews/{img}", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("id")

		storage.ServeFile(id+"/imgs/prev-"+chi.URLParam(r, "img")+".jpg", w, false)
	})

	inputRouter.Mount("/data", r)

}
