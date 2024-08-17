package mw

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type logmwkey int

const key logmwkey = iota

type ResponseWriterWrapper struct {
	w          http.ResponseWriter
	written    int
	statusCode int
}

func (i *ResponseWriterWrapper) Write(buf []byte) (int, error) {
	written, err := i.w.Write(buf)
	i.written += written
	return written, err
}

func (i *ResponseWriterWrapper) WriteHeader(statusCode int) {
	i.statusCode = statusCode
	i.w.WriteHeader(statusCode)
}

func (i *ResponseWriterWrapper) Header() http.Header {
	return i.w.Header()
}

func (i *ResponseWriterWrapper) Flush() {
	if flusher, ok := i.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func NewLoggerMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// set incomplete request fields
			l := slog.With(
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"referrer", r.Referer(),
				"start_time", start,
			)

			// store logger in context
			ctx := context.WithValue(r.Context(), key, l)

			// invoke next handler
			ww := ResponseWriterWrapper{w: w, statusCode: 200}
			next.ServeHTTP(&ww, r.WithContext(ctx))

			// get completed request fields
			l = l.With(
				"duration", time.Since(start),
				"status", ww.statusCode,
				"bytes_written", ww.written,
			)

			logHTTPStatus(l, ww.statusCode)
		})
	}
}

func logHTTPStatus(l *slog.Logger, status int) {
	var msg string
	if msg = http.StatusText(status); msg == "" {
		msg = "unknown status " + strconv.Itoa(status)
	}

	var level slog.Level
	switch {
	case status >= 500:
		level = slog.LevelError
	case status >= 400:
		level = slog.LevelInfo
	case status >= 300:
		level = slog.LevelInfo
	default:
		level = slog.LevelInfo
	}

	l.Log(context.Background(), level, msg)
}

// Extract returns the logger set by mw.
func Extract(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(key).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
