package repo

import (
	"strings"

	"holodex/internal/model"
)

// fileLayerColumns are the videos columns that a file's extraction owns.
// UpsertVideo, RepointVideo and the move fold all write exactly this list, so a
// new extracted column is added here once and can't go stale on a moved row.
// file_path, indexed_at, content_fp, file_tags and active each follow their own
// rule and stay at the call site. Keep it in step with fileLayerArgs.
var fileLayerColumns = []string{
	"file_size", "title", "duration_sec", "width", "height",
	"video_codec", "audio_codec", "bitrate_kbps", "container",
	"recorded_at", "file_mtime",
}

// fileLayerArgs returns v's values for fileLayerColumns, in order.
func fileLayerArgs(v *model.Video) []any {
	var recorded any
	if v.RecordedAt != nil {
		recorded = v.RecordedAt.UTC().Format(timeLayout)
	}
	return []any{
		v.FileSize, v.Title, v.Duration, v.Width, v.Height,
		v.VideoCodec, v.AudioCodec, v.BitrateKbps, v.Container,
		recorded, v.FileMtime.UTC().Format(timeLayout),
	}
}

// fileLayerSet renders "col = expr(col)" for every file-layer column.
func fileLayerSet(expr func(col string) string) string {
	parts := make([]string, len(fileLayerColumns))
	for i, c := range fileLayerColumns {
		parts[i] = c + " = " + expr(c)
	}
	return strings.Join(parts, ", ")
}

// fileLayerList is the comma-separated column list, optionally qualified.
func fileLayerList(prefix string) string {
	parts := make([]string, len(fileLayerColumns))
	for i, c := range fileLayerColumns {
		parts[i] = prefix + c
	}
	return strings.Join(parts, ", ")
}
