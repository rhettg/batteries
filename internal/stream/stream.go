package stream

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

type streamChan chan []byte

type Stream struct {
	Name    string
	dataIn  streamChan
	dataOut []streamChan

	mu sync.RWMutex
}

func (s *Stream) stream() {
	for data := range s.dataIn {
		s.mu.RLock()
		for _, out := range s.dataOut {
			select {
			case out <- data:
			default:
				slog.Warn("dropping data from stream", "stream", s.Name)
			}
		}
		s.mu.RUnlock()
	}
}

func (s *Stream) NewReader() streamChan {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(streamChan, 8)
	s.dataOut = append(s.dataOut, ch)
	return ch
}

type Manager struct {
	streams map[string]*Stream
	mu      sync.RWMutex
}

func New(name string) *Stream {
	s := Stream{
		Name:    name,
		dataIn:  make(streamChan),
		dataOut: make([]streamChan, 0),
	}

	go s.stream()

	return &s
}

func (sm *Manager) GetWriter(name string) streamChan {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[name]; ok {
		return s.dataIn
	}

	sm.streams[name] = New(name)

	return sm.streams[name].dataIn
}

func (sm *Manager) GetReader(name string) streamChan {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var s *Stream
	if sm.streams[name] != nil {
		s = sm.streams[name]
	} else {
		s = New(name)
		sm.streams[name] = s
	}

	return s.NewReader()
}

func NewManager() *Manager {
	return &Manager{
		streams: make(map[string]*Stream),
	}
}

func StreamOut(ctx context.Context, w io.Writer, streamName string, sm *Manager) error {
	s := sm.GetReader(streamName)
	for {
		select {
		case data, ok := <-s:
			if !ok {
				slog.Info("stream closed", "stream", streamName)
				return nil
			}
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
	s := sm.GetWriter(streamName)
	s <- b
	return nil
}
