package writequeue

import (
	"context"

	"holodex/internal/writeback"
)

// SetFileReaders swaps the worker's two exiftool reads for fakes, so the
// ADR-110 tag-key filter can be tested without media files.
func SetFileReaders(q *Queue,
	tagKeys func(ctx context.Context, path, container string) (map[string][]string, error),
	current func(ctx context.Context, path string, mapped []writeback.Mapped) (map[string]string, error),
) {
	q.readTagKeys = tagKeys
	q.readCurrent = current
}
