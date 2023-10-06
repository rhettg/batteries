package ci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func Accept(ctx context.Context, cmd string) error {
	if cmd == "" {
		return errors.New("empty command")
	}

	f := strings.Fields(cmd)
	switch f[0] {
	case "ping":
		return nil
	case "fwd":
		return doFwd(ctx, f[1:])
	case "ffwd":
		return doFfwd(ctx, f[1:])
	case "bck":
		return doBck(ctx, f[1:])
	case "lt":
		return doLT(ctx, f[1:])
	case "rt":
		return doRT(ctx, f[1:])
	default:
		return errors.New("unknown command")
	}
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

	err = motorAndStop(ctx, -0.75, -0.75, duration)
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

	err = motorAndStop(ctx, -1.0, -1.0, duration)
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

	err = motorAndStop(ctx, 0.75, 0.75, duration)
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
