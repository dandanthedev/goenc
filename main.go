package main

import (
	"goenc/api"
	"goenc/encoder"
	"goenc/player"
	"goenc/storage"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-co-op/gocron/v2"
)

func InitServer() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	api.APIRouter(r)
	api.VideoDataRouter(r)

	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(player.GeneratePlayer(chi.URLParam(r, "id"), r.URL.Query().Get("token"))))
	})

	slog.Info("Server started on :3000")
	if err := http.ListenAndServe(":3000", r); err != nil {
		slog.Error("Server failed", "error", err)
	}
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	storage.InitStorage()

	workerTasks := strings.Split(os.Getenv("TASKS"), ",")
	for _, task := range workerTasks {
		if task == "encode" {
			go encoder.StartTaskProcessor()
		}
		if task == "server" {
			go InitServer()
		}
		if task == "stuckrecovery" {
			s, err := gocron.NewScheduler()
			if err != nil {
				slog.Error("Failed to start scheduler", "error", err)
				os.Exit(1)
			}
			job, err := s.NewJob(
				gocron.CronJob(os.Getenv("STUCKRECOVERY_CRON"), false),
				gocron.NewTask(func() {
					encoder.RecoverStuckProcessingJobs()
				}),
			)
			if err != nil {
				slog.Error("Failed to schedule job", "error", err)
				os.Exit(1)
			}
			s.Start()
			nextRuns, err := job.NextRuns(5)
			if err != nil {
				slog.Error("Failed to get next runs", "error", err)
			} else {
				slog.Info("Stuck job recovery started", "nextruns", nextRuns)
			}

		}
	}

	slog.Info("Running")
	select {}

}
