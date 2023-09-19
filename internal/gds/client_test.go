package gds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetNotes(t *testing.T) {

	t.Run("returns notes on 200", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// return mock response
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := `{"commands.qo": [{"body": "hello there"}]}`
			w.Write([]byte(resp))
		}))

		client := New(server.URL)

		// Act
		notes, err := client.GetNotes(context.Background())
		require.NoError(t, err)

		require.Equal(t, 1, len(notes))

		n := notes[0]
		require.Equal(t, "hello there", n.Body)
		require.Equal(t, "commands.qo", n.File)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
		}))

		client := New(server.URL)

		// Act
		_, err := client.GetNotes(context.Background())

		// Assert
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

}
