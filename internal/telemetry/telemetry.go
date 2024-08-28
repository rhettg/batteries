package telemetry

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var (
	// Mutex for concurrent access to the metrics map
	metricsMutex sync.Mutex
	// Map to store dynamic gauges by key
	dynamicMetrics map[string]prometheus.Gauge
)

func init() {
	dynamicMetrics = make(map[string]prometheus.Gauge)
}

// Function to create or update a metric
func setMetric(key, value string) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	gauge, exists := dynamicMetrics[key]
	if !exists {
		slog.Info("establishing new telemetry metric", "name", key)
		// Metric does not exist, create it
		gauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: key,
			Help: "",
		})
		prometheus.MustRegister(gauge)
		dynamicMetrics[key] = gauge
	}

	slog.Debug("setting metric for telemetry", key, value)
	if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
		gauge.Set(floatValue)
	}
}

func Run(ctx context.Context, rdb *redis.Client, streamKey string) error {
	cursor := "$"
	for {
		slog.Debug("reading telemetry stream", "cursor", cursor, "stream", streamKey)
		entries, err := rdb.XRead(ctx, &redis.XReadArgs{
			Streams: []string{streamKey, cursor},
			Count:   10,
			Block:   10 * time.Second,
		}).Result()

		if err != nil && err != redis.Nil {
			return err
		}

		for _, entry := range entries {
			for _, message := range entry.Messages {
				for key, value := range message.Values {
					valueStr, ok := value.(string)
					if ok {
						setMetric(key, valueStr)
					}
				}
				cursor = message.ID
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(100 * time.Millisecond):
		}
	}
}
