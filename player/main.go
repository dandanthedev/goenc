package player

import (
	"embed"
	"html/template"
	"log"
	"log/slog"
	"strings"
)

//go:embed templates
var templates embed.FS

var tmpl = template.Must(template.ParseFS(templates, "templates/player.html"))

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
