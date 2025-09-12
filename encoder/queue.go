package encoder

import (
	"context"
	"log/slog"

	"github.com/go-redis/redis/v8"
	"github.com/vmihailenco/taskq/v3"
	"github.com/vmihailenco/taskq/v3/redisq"
)

var Redis = redis.NewClient(&redis.Options{
	Addr: ":6379",
})

var QueueFactory = redisq.NewFactory()

// Create a queue.
var MainQueue = QueueFactory.RegisterQueue(&taskq.QueueOptions{
	Name:  "main",
	Redis: Redis, // go-redis client
})

var EncodeTask = taskq.RegisterTask(&taskq.TaskOptions{
	Name: "encode",
	Handler: func(input string, id string, profiles string) error {
		slog.Info("Starting encoding", "file", input, "profiles", profiles)
		EncodeFile(input, id, profiles)
		return nil
	},
})

func StartConsumer() {
	consumer := MainQueue.Consumer()
	context := context.Background()
	consumer.Start(context)
	slog.Info("Consumer started")
}

func AddFileToQueue(source string, id string, profiles string) string {
	ctx := context.Background()
	task := EncodeTask.WithArgs(ctx, source, id, profiles)
	if err := MainQueue.Add(task); err != nil {
		slog.Error("Failed to add task to queue", "error", err, "file", source)
	} else {
		slog.Info("Added to queue", "file", source)
	}

	return task.ID
}

func GetQueue() map[string]any {
	queueItems := MainQueue.Consumer().Stats()
	return map[string]any{
		"active":  queueItems.InFlight,
		"success": queueItems.Processed,
		"fails":   queueItems.Fails,
	}
}
