package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// TestCurationForEntities_Batch covers the F55 list-wide generic batch loader
// (ADR-081 D4): entity-type-parameterized like CurationForVideos, but not
// hardcoded to "video" — a person id and a video id sharing the same numeric value
// must not cross-contaminate.
func TestCurationForEntities_Batch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, pid := seedVideoAndPerson(t, r)

	if err := r.SetCuration(ctx, model.EnrichEntityPerson, pid, "aliases", "Miyazaki Hayao", "add"); err != nil {
		t.Fatalf("set person curation: %v", err)
	}
	if err := r.SetCuration(ctx, model.EnrichEntityVideo, vid, "genres", "Animation", "add"); err != nil {
		t.Fatalf("set video curation: %v", err)
	}

	got, err := r.CurationForEntities(ctx, model.EnrichEntityPerson, []int64{pid})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(got[pid]) != 1 || got[pid][0].FieldKey != "aliases" {
		t.Fatalf("person rows = %v, want one aliases row", got[pid])
	}
}
