package encoder

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// Global Redis client
var Redis = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDIS_ADDR"),
	Password: os.Getenv("REDIS_PASSWORD"),
	DB: func() int {
		db, err := strconv.Atoi(os.Getenv("REDIS_DB"))
		if err != nil {
			slog.Error("Failed to parse REDIS_DB", "error", err)
			return 0
		}
		return db
	}(),
})

// Global WorkerID
var WorkerID string

// Call this at startup to assign a unique ID to this worker
func InitWorkerID() {
	WorkerID = uuid.New().String()
}

type Status string

const (
	Waiting    Status = "waiting"
	Processing Status = "processing"
	Done       Status = "done"
	Fail       Status = "fail"
)

type QueueItem struct {
	Id       string `json:"id"`
	Source   string `json:"source"`
	Profiles string `json:"profiles"`
	Status   Status `json:"status"`
	Step     string `json:"step,omitempty"`
	Attempts int    `json:"attempts"`
	WorkerID string `json:"worker_id,omitempty"`
}

func StartHeartbeat(ctx context.Context) {
	go func() {
		for {
			Redis.Set(ctx, "worker:"+WorkerID+":heartbeat", "1", 35*time.Second)
			time.Sleep(10 * time.Second)
		}
	}()
}

func ModifyQueueItem(id string, status Status, attempts int, step string) error {
	ctx := context.Background()
	item, err := Redis.Get(ctx, "queue:"+id).Result()
	if err != nil {
		return err
	}
	var data QueueItem
	err = json.Unmarshal([]byte(item), &data)
	if err != nil {
		return err
	}
	data.Status = status
	if attempts > 0 {
		data.Attempts = attempts
	}
	if step != "" {
		data.Step = step
	}
	switch status {
	case Processing:
		data.WorkerID = WorkerID
	case Waiting, Fail, Done:
		data.WorkerID = ""
	}
	jsonData, _ := json.Marshal(data)
	Redis.Set(ctx, "queue:"+id, string(jsonData), 0)
	return nil
}

func StartTaskProcessor() {
	if os.Getenv("REDIS_ADDR") == "" {
		slog.Error("REDIS_ADDR is not set")
		os.Exit(1)
	}

	ctx := context.Background()
	// Start heartbeat for this worker
	InitWorkerID()
	StartHeartbeat(ctx)

	for {
		slog.Debug("Waiting for next item in queue...")
		item, err := Redis.BLPop(ctx, 0, "queue:all").Result()
		if err != nil {
			continue
		}
		if item == nil {
			continue
		}
		id := item[1]
		// Mark as processing before actual processing
		err = ModifyQueueItem(id, Processing, 0, "")
		if err != nil {
			slog.Error("Failed to set item to processing", "id", id, "error", err)
			continue
		}
		itemData, err := Redis.Get(ctx, "queue:"+id).Result()
		if err != nil {
			slog.Error("Failed to get queue item", "error", err)
			continue
		}
		var data QueueItem
		err = json.Unmarshal([]byte(itemData), &data)
		if err != nil {
			slog.Error("Failed to get queue item", "error", err)
			continue
		}
		slog.Info("Processing item from queue", "id", data.Id, "source", data.Source, "profiles", data.Profiles, "status", data.Status, "worker_id", WorkerID)
		err = EncodeFile(data.Source, data.Id, data.Profiles)
		if err != nil {
			slog.Error("Failed to process file", "id", data.Id, "error", err)
			//if attempts >= 3, set status to fail
			if data.Attempts >= 3 {
				err := ModifyQueueItem(data.Id, Fail, data.Attempts+1, "")
				if err != nil {
					slog.Error("Failed to modify queue item", "id", data.Id, "error", err)
					continue
				}
			} else {
				err := ModifyQueueItem(data.Id, Waiting, data.Attempts+1, "")
				if err != nil {
					slog.Error("Failed to modify queue item", "id", data.Id, "error", err)
					continue
				}
				// Requeue the item for another attempt
				Redis.RPush(ctx, "queue:all", data.Id)
			}
			continue
		}
		// Mark as done
		err = ModifyQueueItem(data.Id, Done, data.Attempts, "")
		if err != nil {
			slog.Error("Failed to set item to done", "id", data.Id, "error", err)
			continue
		}
	}
}

func RecoverStuckProcessingJobs() {
	slog.Info("Recovering stuck processing jobs")
	ctx := context.Background()
	iter := Redis.Scan(ctx, 0, "queue:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if key == "queue:all" {
			continue
		}
		itemStr, err := Redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var data QueueItem
		err = json.Unmarshal([]byte(itemStr), &data)
		if err != nil {
			continue
		}
		if data.Status == Processing && data.WorkerID != "" {
			// Check if worker is alive
			heartbeat, err := Redis.Get(ctx, "worker:"+data.WorkerID+":heartbeat").Result()
			if err == redis.Nil || heartbeat == "" {
				slog.Info("Recovering stuck job", "id", data.Id, "worker_id", data.WorkerID)
				ModifyQueueItem(data.Id, Waiting, data.Attempts+1, "")
				Redis.RPush(ctx, "queue:all", data.Id)
			}
		}
	}
	if err := iter.Err(); err != nil {
		slog.Error("Failed to scan queue keys for recovery", "error", err)
	}
	slog.Info("Recovery done")
}

func RemoveCompletedJobs() {
	ctx := context.Background()
	iter := Redis.Scan(ctx, 0, "queue:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if key == "queue:all" {
			continue
		}
		item, err := Redis.Get(ctx, key).Result()
		if err != nil {
			slog.Error("Failed to get queue item", "key", key, "error", err)
			continue
		}
		var data QueueItem
		err = json.Unmarshal([]byte(item), &data)
		if err != nil {
			slog.Error("Failed to unmarshal queue item", "key", key, "error", err)
			continue
		}
		if data.Status == Done {
			slog.Info("Removing completed job from queue", "id", data.Id)
			Redis.Del(ctx, key)
		}
		if data.Status == Fail {
			slog.Info("Removing failed job from queue", "id", data.Id)
			Redis.Del(ctx, key)
		}

	}
}

func AddFileToQueue(source string, id string, profiles string) string {
	ctx := context.Background()
	//push to redis
	data := QueueItem{
		Id:       id,
		Source:   source,
		Profiles: profiles,
		Status:   Waiting,
		Attempts: 0,
	}
	jsonData, _ := json.Marshal(data)
	// Add to the list for queue order
	Redis.LPush(ctx, "queue:all", id)
	// Also set the item by id for direct access
	Redis.Set(ctx, "queue:"+id, string(jsonData), 0)
	return id
}

// array of queue items
func GetQueue() []QueueItem {
	ctx := context.Background()
	var queue []QueueItem
	// Scan all keys matching queue:*
	iter := Redis.Scan(ctx, 0, "queue:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Skip the queue:all list key
		if key == "queue:all" {
			continue
		}
		item, err := Redis.Get(ctx, key).Result()
		if err != nil {
			slog.Error("Failed to get queue item", "key", key, "error", err)
			continue
		}
		var data QueueItem
		err = json.Unmarshal([]byte(item), &data)
		if err != nil {
			slog.Error("Failed to unmarshal queue item", "key", key, "error", err)
			continue
		}
		queue = append(queue, data)
	}
	if err := iter.Err(); err != nil {
		slog.Error("Failed to scan queue keys", "error", err)
	}
	return queue
}
