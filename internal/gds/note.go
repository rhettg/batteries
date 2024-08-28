package gds

import (
	"encoding/json"
	"time"
)

type Note struct {
	File      string          `json:"file,omitempty"`
	Note      string          `json:"note,omitempty"`
	Body      json.RawMessage `json:"body"`
	CreatedAt time.Time       `json:"createdAt,omitempty"`
	UpdatedAt time.Time       `json:"updatedAt,omitempty"`
}
