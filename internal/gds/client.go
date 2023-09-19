package gds

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	missionURL string
	httpClient *http.Client
}

func New(missionURL string) *Client {
	return &Client{
		missionURL: missionURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) GetNotes(ctx context.Context) ([]Note, error) {
	url := c.missionURL + "/note_queue"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		// TODO: parse error
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("unexpected content-type: %s", resp.Header.Get("Content-Type"))
	}

	defer resp.Body.Close()

	fileList := make(map[string][]Note)
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&fileList)
	if err != nil {
		return nil, err
	}

	// Unwrap all the notes
	notes := make([]Note, 0)
	for f, nl := range fileList {
		for _, n := range nl {
			n.File = f
			notes = append(notes, n)
		}
	}

	return notes, nil
}
