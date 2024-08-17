package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type stream struct {
	Name string
	data chan []byte
}

type streamManager struct {
	streams map[string]*stream
	mu      sync.RWMutex
}

func (sm *streamManager) Get(name string) *stream {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[name]; ok {
		return s
	}

	sm.streams[name] = &stream{
		Name: name,
		data: make(chan []byte),
	}
	return sm.streams[name]
}

func parsePath(path string) string {
	remaining, found := strings.CutPrefix(path, "/stream/")
	if found {
		return remaining
	}
	return ""
}

func handleGet(w http.ResponseWriter, r *http.Request, sm *streamManager) {
	streamName := parsePath(r.URL.Path)
	if streamName == "" {
		http.Error(w, "Invalid stream name", http.StatusBadRequest)
		return
	}

	w.Header().Set("Transfer-Encoding", "chunked")

	fmt.Println("Handling request for stream:", streamName)
	s := sm.Get(streamName)
	for {
		select {
		case data := <-s.data:
			fmt.Println("Sending data:", string(data))
			n, err := w.Write(data)
			if err != nil {
				http.Error(w, "Error writing data", http.StatusInternalServerError)
				return
			}
			fmt.Println("Sent", n, "bytes")
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			fmt.Println("Request complete")
			return
		}
	}
}

func handlePost(w http.ResponseWriter, r *http.Request, sm *streamManager) {
	streamName := parsePath(r.URL.Path)
	if streamName == "" {
		http.Error(w, "Invalid stream name", http.StatusBadRequest)
		return
	}

	fmt.Println("Handling POST request for stream:", streamName)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	s := sm.Get(streamName)
	fmt.Printf("Putting data into stream %s: %s\n", streamName, string(body))
	s.data <- body

	w.WriteHeader(http.StatusOK)
	fmt.Println("Data received successfully")

}

func main() {
	sm := streamManager{
		streams: make(map[string]*stream),
	}

	http.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGet(w, r, &sm)
		case http.MethodPost:
			handlePost(w, r, &sm)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}
