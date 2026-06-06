// Package model holds the core domain types shared across layers.
package model

import "time"

// Video is one indexed media file. file metadata is the source of truth; this
// record is a rebuildable cache of it (ADR-003/004).
type Video struct {
	ID         int64     `json:"id"`
	FilePath   string    `json:"file_path"` // canonical absolute path (ADR-011)
	FileSize   int64     `json:"file_size"`
	Title      string    `json:"title"`
	Duration   int       `json:"duration_sec"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	RecordedAt *time.Time `json:"recorded_at,omitempty"`
	IndexedAt  time.Time `json:"indexed_at"`
	FileMtime  time.Time `json:"file_mtime"`
	Active     bool      `json:"-"`

	People []Person `json:"people,omitempty"`
	Tags   []Tag    `json:"tags,omitempty"`
}

type Person struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count,omitempty"`
}

type Tag struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count,omitempty"`
}

// ExtraMetadata is a captured raw container tag not mapped to a first-class
// field (ADR-013, F2.9).
type ExtraMetadata struct {
	SourceKey string `json:"source_key"`
	Value     string `json:"value"`
}
