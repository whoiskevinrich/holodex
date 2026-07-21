package writequeue_test

import (
	"testing"

	"holodex/internal/writequeue"
)

// MayIntroduceEntity gates the post-writeback re-extract (HOLODEX-196 #4): it
// fires for entity-field writes that can introduce a not-yet-in-DB value
// (filename/manual), and NOT for merge-propagation or revert writes, whose DB
// entities are already current — so a large merge doesn't pay a redundant
// re-extract per affected video.
func TestMayIntroduceEntity(t *testing.T) {
	tests := []struct {
		name   string
		fields []writequeue.JobField
		want   bool
	}{
		{"filename actors", []writequeue.JobField{{Field: "actors", Source: "filename"}}, true},
		{"manual studio", []writequeue.JobField{{Field: "studio", Source: "manual"}}, true},
		{"merge actors excluded", []writequeue.JobField{{Field: "actors", Source: writequeue.SourceMerge}}, false},
		{"revert studio excluded", []writequeue.JobField{{Field: "studio", Source: writequeue.SourceRevert}}, false},
		{"non-entity field ignored", []writequeue.JobField{{Field: "title", Source: "manual"}}, false},
		{"mixed: one entity write is enough", []writequeue.JobField{
			{Field: "title", Source: "manual"},
			{Field: "actors", Source: "filename"},
		}, true},
		{"empty", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := writequeue.MayIntroduceEntity(tc.fields); got != tc.want {
				t.Fatalf("MayIntroduceEntity(%+v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
