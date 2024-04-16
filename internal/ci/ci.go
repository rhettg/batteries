package ci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func Accept(ctx context.Context, rdb *redis.Client, id, cmd string) error {
	if cmd == "" {
		return errors.New("empty command")
	}

	var err error

	f := strings.Fields(cmd)
	switch f[0] {
	case "ping":
		// Nothing to do.
	case "fwd":
		err = doFwd(ctx, f[1:])
	case "ffwd":
		err = doFfwd(ctx, f[1:])
	case "bck":
		err = doBck(ctx, f[1:])
	case "lt":
		err = doLT(ctx, f[1:])
	case "rt":
		err = doRT(ctx, f[1:])
	default:
		err = errors.New("unknown command")
	}

	if rdb == nil {
		return err
	}

	var result string
	if err != nil {
		result = "error"
	} else {
		result = "ok"
	}

	resultErr := StreamResult(ctx, rdb, id, result)

	if err != nil {
		return err
	}

	return resultErr
}

func Stream(ctx context.Context, rdb *redis.Client, cmd string) (string, error) {
	if cmd == "" {
		return "", errors.New("empty command")
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

	return result, nil
}

func StreamResult(ctx context.Context, rdb *redis.Client, id, cmdResult string) error {
	result, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "yakapi:ci:result",
		ID:     id,
		Values: map[string]interface{}{
			"result": cmdResult,
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to stream command result: %w", err)
	}

	slog.Info("streamed command result", "stream", "yakapi:ci:result", "id", id, "result_id", result, "result", cmdResult)
	return nil
}

func FetchResult(ctx context.Context, rdb *redis.Client, id string) (string, error) {
	messages, err := rdb.XRange(ctx, "yakapi:ci:result", id, id).Result()
	if err != nil {
		fmt.Println("Error reading specific ID from stream:", err)
		return "", err
	}

	if len(messages) == 0 {
		return "", fmt.Errorf("no result found for command %s", id)
	}

	return messages[0].Values["result"].(string), nil
}

func parseDurationArg(arg string) (time.Duration, error) {
	durationArg, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	duration := time.Duration(durationArg) * time.Millisecond * 10

	return duration, nil
}

func parseAngleArg(arg string) (time.Duration, error) {
	angleArg, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	duration := time.Duration(spinDurationSecs*(float64(angleArg)/90.0)) * time.Second
	return duration, nil
}

func doFwd(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseDurationArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, 0.75, 0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

func doFfwd(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseDurationArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, 1.0, 1.0, duration)
	if err != nil {
		return err
	}

	return nil
}

func doBck(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseDurationArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, -0.75, -0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

const spinDurationSecs = 2.0

func doRT(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseAngleArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, -0.75, 0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

func doLT(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseAngleArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, 0.75, -0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

func execMotorAdapter(ctx context.Context, args []string) error {
	name := os.Getenv("YAKAPI_ADAPTER_MOTOR")
	if name == "" {
		return errors.New("motor adapter not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("running motor adapter", name, args)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed running motor adapter: %w", err)
	}

	return nil
}

func motorAndStop(ctx context.Context, throttle1, throttle2 float64, d time.Duration) error {
	err := execMotorAdapter(ctx,
		[]string{
			fmt.Sprintf("motor1:%.2f", throttle1),
			fmt.Sprintf("motor2:%.2f", throttle2)})
	if err != nil {
		return err
	}

	fmt.Printf("sleeping for %.3fs\n", d.Seconds())
	time.Sleep(d)

	err = execMotorAdapter(ctx, []string{"motor1:0.0", "motor2:0.0"})
	if err != nil {
		return err
	}

	return nil
}
