package api

import (
	"encoding/json"
	"goenc/encoder"
	"net/http"
)

func ReplyWithJSON(w http.ResponseWriter, code int, data any) {
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
