package ci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type CommandID string

func Accept(ctx context.Context, rdb *redis.Client, cmd string) (CommandID, error) {
	if cmd == "" {
		return "", errors.New("empty command")
	}

	var err error

	id, err := stream(ctx, rdb, cmd)
	if err != nil {
		return "", err
	}

	return id, nil
}

func stream(ctx context.Context, rdb *redis.Client, cmd string) (CommandID, error) {
	if cmd == "" {
		return "", errors.New("empty command")
	}

	if rdb == nil {
		return "", nil
	}

	f := strings.Fields(cmd)
	result, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "yakapi:ci",
		Values: map[string]interface{}{
			"cmd":  f[0],
			"args": strings.Join(f[1:], " "),
		},
	}).Result()

	if err != nil {
		return "", fmt.Errorf("failed to stream command: %w", err)
	}

	slog.Info("streamed command", "stream", "yakapi:ci", "id", result)

	return CommandID(result), nil
}

func FetchResult(ctx context.Context, rdb *redis.Client, id CommandID) (string, error) {
	if rdb == nil {
		return "", nil
	}

	var messages []redis.XMessage
	var err error

	for len(messages) == 0 {
		time.Sleep(100 * time.Millisecond)
		messages, err = rdb.XRange(ctx, "yakapi:ci:result", string(id), string(id)).Result()
		if err != nil {
			fmt.Println("Error reading specific ID from stream:", err)
			return "", err
		}
	}

	if errStr, ok := messages[0].Values["error"]; ok {
		return "", errors.New(errStr.(string))
	}

	return messages[0].Values["result"].(string), nil
}
