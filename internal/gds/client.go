package gds

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	missionURL string
	httpClient *http.Client
}

type TelemetryLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Telemetry struct {
	SecondsSinceBoot int               `json:"seconds_since_boot"`
	WifiRSSI         int               `json:"wifi_rssi"`
	Heading          float64           `json:"heading"`
	Location         TelemetryLocation `json:"location"`
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

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
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

func (c *Client) SendTelemetry(ctx context.Context, telemetry Telemetry) error {
	url := c.missionURL + "/notes/telemetry.qo"

	data := struct {
		Body Telemetry `json:"body"`
	}{
		Body: telemetry,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		// TODO: parse error
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
