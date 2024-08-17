package stream

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

type Stream struct {
	Name string
	data chan []byte
}

type Manager struct {
	streams map[string]*Stream
	mu      sync.RWMutex
}

func (sm *Manager) Get(name string) *Stream {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[name]; ok {
		return s
	}

	sm.streams[name] = &Stream{
		Name: name,
		data: make(chan []byte),
	}
	return sm.streams[name]
}

func NewManager() *Manager {
	return &Manager{
		streams: make(map[string]*Stream),
	}
}

func StreamOut(ctx context.Context, w io.Writer, streamName string, sm *Manager) error {
	s := sm.Get(streamName)
	for {
		select {
		case data := <-s.data:
			_, err := w.Write(data)
			if err != nil {
				return errors.New("error writing data")
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			} else {
				slog.Warn("unable to flush")
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func StreamIn(ctx context.Context, streamName string, b []byte, sm *Manager) error {
	s := sm.Get(streamName)
	s.data <- b
	return nil
}
