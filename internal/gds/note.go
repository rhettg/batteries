package gds

import "time"

type Note struct {
	File      string    `json:"file",omitempty`
	Note      string    `json:"note",omitempty`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt",omitempty`
	UpdatedAt time.Time `json:"updatedAt",omitempty`
}
