package player

import (
	"html/template"
	"log"
	"log/slog"
	"strings"
)

var tmpl = template.Must(template.ParseFiles("templates/player.html"))

func GeneratePlayer(id string, token string) string {
	filePrefix := "/data/" + id

	data := struct {
		ID     string
		PREFIX string
		TOKEN  string
	}{
		ID:     id,
		PREFIX: filePrefix,
		TOKEN:  token,
	}

	slog.Debug("Generating player", "id", id, "token", token)

	var player strings.Builder
	if err := tmpl.Execute(&player, data); err != nil {
		log.Fatal(err)
	}
	return player.String()
}
