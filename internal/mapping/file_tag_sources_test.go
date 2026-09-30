package mapping

import (
	"slices"
	"testing"
)

// FileTagSources keeps only file tags, in precedence order: bare and file:
// keys stay; the file:Title column alias and every other namespace go
// (ADR-120 D4 deletes exactly these when a field is cleared).
func TestFileTagSources(t *testing.T) {
	f := Field{ParsedSources: parseSources([]string{
		"Publisher", "file:Label", "file:Title", "filename:studio", "tmdb:studio", "Studio",
	})}
	if got, want := f.FileTagSources(), []string{"Publisher", "Label", "Studio"}; !slices.Equal(got, want) {
		t.Fatalf("FileTagSources = %v, want %v", got, want)
	}
	if got := (Field{}).FileTagSources(); got != nil {
		t.Fatalf("empty field = %v, want nil", got)
	}
}
