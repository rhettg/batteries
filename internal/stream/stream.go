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
	Name        string
	dataIn      streamChan
	dataOut     []streamChan
	writerCount int

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

// maybeClose checks if the stream can be closed and closes it if so
// requires the stream to be locked
func (s *Stream) maybeClose() bool {
	if s.writerCount == 0 && len(s.dataOut) == 0 {
		slog.Info("closing stream", "stream", s.Name)
		close(s.dataIn)
		return true
	}
	return false
}

func (s *Stream) CloseReader(ch streamChan) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, out := range s.dataOut {
		if out == ch {
			s.dataOut = append(s.dataOut[:i], s.dataOut[i+1:]...)
			break
		}
	}
	close(ch)
	slog.Info("closed reader for stream", "stream", s.Name, "count", len(s.dataOut))
	return s.maybeClose()
}

func (s *Stream) Writer() streamChan {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writerCount++
	return s.dataIn
}

func (s *Stream) CloseWriter() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writerCount--
	slog.Info("closed writer for stream", "stream", s.Name, "count", s.writerCount)
	return s.maybeClose()
}

func New(name string) *Stream {
	s := Stream{
		Name:        name,
		dataIn:      make(streamChan),
		dataOut:     make([]streamChan, 0),
		writerCount: 0,
	}

	go s.stream()

	return &s
}

type Manager struct {
	streams map[string]*Stream
	mu      sync.RWMutex
}

func (sm *Manager) GetWriter(name string) streamChan {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[name]; ok {
		return s.Writer()
	}

	sm.streams[name] = New(name)

	return sm.streams[name].Writer()
}

func (sm *Manager) ReturnWriter(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.streams[name] == nil {
		slog.Warn("stream not found", "stream", name)
		return
	}

	s := sm.streams[name]
	if s.CloseWriter() {
		delete(sm.streams, name)
		slog.Info("stream closed", "stream", name)
		return
	}
}

func (sm *Manager) ReturnReader(name string, ch streamChan) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.streams[name] == nil {
		slog.Warn("stream not found", "stream", name)
		return
	}

	s := sm.streams[name]
	if s.CloseReader(ch) {
		delete(sm.streams, name)
		slog.Info("stream closed", "stream", name)
		return
	}
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

// TODO: Manager.Close()

func NewManager() *Manager {
	return &Manager{
		streams: make(map[string]*Stream),
	}
}

func StreamOut(ctx context.Context, w io.Writer, streamName string, sm *Manager) error {
	s := sm.GetReader(streamName)
	defer sm.ReturnReader(streamName, s)
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
	defer sm.ReturnWriter(streamName)

	s <- b
	return nil
}
