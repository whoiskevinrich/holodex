package repo

import (
	"testing"

	"holodex/internal/model"
)

// fileLayerArgs must supply one value per fileLayerColumns entry; a column
// added to one and not the other would shift every placeholder after it.
func TestFileLayerArgsMatchColumns(t *testing.T) {
	if got, want := len(fileLayerArgs(&model.Video{})), len(fileLayerColumns); got != want {
		t.Fatalf("fileLayerArgs has %d values for %d columns", got, want)
	}
}
