package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

func TestDecisions_SetGetClear(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/d.mkv", "Film", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// No decisions initially.
	if rows, err := r.DecisionsForEntity(ctx, model.EnrichEntityVideo, id); err != nil || len(rows) != 0 {
		t.Fatalf("want no decisions, got %v err=%v", rows, err)
	}

	// Set a provider decision, then a manual one on a second field.
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "title", "provider:tmdb", ""); err != nil {
		t.Fatalf("set provider decision: %v", err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "studio", "manual", "Acme Films"); err != nil {
		t.Fatalf("set manual decision: %v", err)
	}

	rows, err := r.DecisionsForEntity(ctx, model.EnrichEntityVideo, id)
	if err != nil || len(rows) != 2 {
		t.Fatalf("want 2 decisions, got %v err=%v", rows, err)
	}
	byField := map[string]repo.DecisionRow{}
	for _, d := range rows {
		byField[d.FieldKey] = d
	}
	if byField["title"].Source != "provider:tmdb" || byField["title"].ManualValue != "" {
		t.Errorf("title decision = %+v", byField["title"])
	}
	if byField["studio"].Source != "manual" || byField["studio"].ManualValue != "Acme Films" {
		t.Errorf("studio decision = %+v", byField["studio"])
	}

	// Upsert the same field flips the source and clears a stale manual literal.
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "studio", "file", "ignored"); err != nil {
		t.Fatalf("re-set studio: %v", err)
	}
	rows, _ = r.DecisionsForEntity(ctx, model.EnrichEntityVideo, id)
	for _, d := range rows {
		if d.FieldKey == "studio" && (d.Source != "file" || d.ManualValue != "") {
			t.Errorf("upsert should flip source and blank manual_value, got %+v", d)
		}
	}

	// Clear removes the row; clearing again is an idempotent no-op.
	n, err := r.ClearDecision(ctx, model.EnrichEntityVideo, id, "title")
	if err != nil || n != 1 {
		t.Fatalf("clear: n=%d err=%v", n, err)
	}
	n, _ = r.ClearDecision(ctx, model.EnrichEntityVideo, id, "title")
	if n != 0 {
		t.Errorf("second clear should affect 0 rows, got %d", n)
	}
}

func TestDecisions_ForVideosBatch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id1, _ := r.UpsertVideo(ctx, sampleVideo("/m/1.mkv", "One", nil, nil), nil)
	id2, _ := r.UpsertVideo(ctx, sampleVideo("/m/2.mkv", "Two", nil, nil), nil)
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id1, "title", "manual", "Custom One"); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := r.DecisionsForVideos(ctx, []int64{id1, id2})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(got[id1]) != 1 || got[id1][0].Source != "manual" {
		t.Errorf("id1 decisions = %v", got[id1])
	}
	if len(got[id2]) != 0 {
		t.Errorf("id2 should have no decisions, got %v", got[id2])
	}
}


// TestDecisions_ForEntitiesBatch covers the F55 list-wide generic batch loader
// (ADR-081 D4): entity-type-parameterized like DecisionsForVideos, but not
// hardcoded to "video" — a person id and a video id sharing the same numeric value
// must not cross-contaminate.
func TestDecisions_ForEntitiesBatch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, pid := seedVideoAndPerson(t, r)

	if err := r.SetDecision(ctx, model.EnrichEntityPerson, pid, "bio", "provider:tmdb", ""); err != nil {
		t.Fatalf("set person decision: %v", err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, vid, "title", "manual", "Custom"); err != nil {
		t.Fatalf("set video decision: %v", err)
	}

	got, err := r.DecisionsForEntities(ctx, model.EnrichEntityPerson, []int64{pid})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(got[pid]) != 1 || got[pid][0].FieldKey != "bio" {
		t.Fatalf("person rows = %v, want one bio row", got[pid])
	}
}

// TestHasManualSource proves F48.3e's precedence check: a manual: decision on
// a field is detectable independent of DecisionsForEntity's full row list.
func TestHasManualSource(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/e.mkv", "Film", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if has, err := r.HasManualSource(ctx, model.EnrichEntityVideo, id, "title"); err != nil || has {
		t.Fatalf("expected no manual source yet, got has=%v err=%v", has, err)
	}

	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "title", "manual", "Curated Title"); err != nil {
		t.Fatalf("set manual decision: %v", err)
	}
	if has, err := r.HasManualSource(ctx, model.EnrichEntityVideo, id, "title"); err != nil || !has {
		t.Fatalf("expected manual source, got has=%v err=%v", has, err)
	}

	// A provider/file decision on a different field is not a manual source.
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, id, "studio", "provider:tmdb", ""); err != nil {
		t.Fatalf("set provider decision: %v", err)
	}
	if has, err := r.HasManualSource(ctx, model.EnrichEntityVideo, id, "studio"); err != nil || has {
		t.Fatalf("expected provider decision to not count as manual, got has=%v err=%v", has, err)
	}
}
