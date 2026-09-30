package writequeue_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"holodex/internal/model"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// clearHarness runs one job through a real worker with a recording writer and
// fake file readers (no exiftool), returning what was written and the job_run.
func clearHarness(t *testing.T, container string, sources []string, fields []writequeue.JobField) ([]writeback.FieldWrite, *model.JobRun) {
	t.Helper()
	r := newRepo(t)
	id := seedVideo(t, r, container)

	var mu sync.Mutex
	var written []writeback.FieldWrite
	wrote := false
	write := func(_ context.Context, _ string, fs []writeback.FieldWrite) error {
		mu.Lock()
		defer mu.Unlock()
		written, wrote = fs, true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, write, testLogger(), 1, "")
	writequeue.SetFileReaders(q,
		func(context.Context, string, string) (map[string][]string, error) { return nil, nil },
		func(context.Context, string, []writeback.Mapped) (map[string]string, error) {
			return map[string]string{"studio": "Acme"}, nil
		})
	if sources != nil {
		q.SetClearSources(func(canonical string) []string {
			if canonical == "studio" {
				return sources
			}
			return nil
		})
	}
	q.Start(ctx)
	if _, err := q.Enqueue(ctx, id, fields); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var wb *model.JobRun
	waitFor(t, func() bool {
		runs, _ := r.ListJobRuns(ctx, 30)
		for i := range runs {
			if runs[i].Kind == model.JobKindWriteback {
				wb = &runs[i]
				return true
			}
		}
		return false
	})
	mu.Lock()
	defer mu.Unlock()
	if !wrote {
		written = nil
	}
	return written, wb
}

func deletes(fs []writeback.FieldWrite) []string {
	var out []string
	for _, f := range fs {
		if !f.Delete || len(f.Values) != 0 {
			return []string{"NOT-A-DELETE:" + f.TagName}
		}
		out = append(out, f.TagName)
	}
	return out
}

var studioSrc = []string{"Publisher", "Label", "Studio", "ProductionCompany"}

// A Clear deletes the write target and every mapped source tag, qualified the
// way the target is (ADR-120 D4).
func TestClear_DeletesEveryStudioTag(t *testing.T) {
	written, wb := clearHarness(t, "MP4", studioSrc, []writequeue.JobField{
		{Field: "studio", Clear: true, Source: "manual"},
	})
	want := []string{"QuickTime:Publisher", "QuickTime:Label", "QuickTime:Studio", "QuickTime:ProductionCompany"}
	if got := deletes(written); !slices.Equal(got, want) {
		t.Fatalf("MP4 deletes = %v, want %v", got, want)
	}
	if wb.Status != model.JobStatusOK || wb.Updated != len(want) {
		t.Fatalf("job run = %+v", wb)
	}

	written, _ = clearHarness(t, "Matroska", studioSrc, []writequeue.JobField{
		{Field: "studio", Clear: true, Source: "manual"},
	})
	if got := deletes(written); !slices.Equal(got, studioSrc) {
		t.Fatalf("Matroska deletes = %v, want %v", got, studioSrc)
	}
}

// A hostile mapping source never reaches the writer, whatever the API let
// through: the worker re-checks every name and names the rejects.
func TestClear_WorkerRejectsHostileNames(t *testing.T) {
	written, wb := clearHarness(t, "Matroska", []string{"Label", "all", "-x"}, []writequeue.JobField{
		{Field: "studio", Clear: true, Source: "manual"},
	})
	if got := deletes(written); !slices.Equal(got, []string{"Publisher", "Label"}) {
		t.Fatalf("deletes = %v, want [Publisher Label]", got)
	}
	if !strings.Contains(wb.Detail, "studio:all") || !strings.Contains(wb.Detail, "studio:-x") {
		t.Fatalf("detail should name the rejected names, got %q", wb.Detail)
	}
}

// Without the mapping hook, a Clear still removes the write target.
func TestClear_NoSourcesDeletesTargetOnly(t *testing.T) {
	written, _ := clearHarness(t, "MP4", nil, []writequeue.JobField{
		{Field: "studio", Clear: true, Source: "manual"},
	})
	if got := deletes(written); !slices.Equal(got, []string{"QuickTime:Publisher"}) {
		t.Fatalf("deletes = %v", got)
	}
}

// An empty studio job WITHOUT Clear still writes nothing (today's safe skip),
// and a Clear that carries values is refused rather than half-applied.
func TestClear_RequiresExplicitFlagAndNoValues(t *testing.T) {
	written, wb := clearHarness(t, "MP4", studioSrc, []writequeue.JobField{
		{Field: "studio", Values: nil, Source: "manual"},
	})
	if written != nil || wb.Updated != 0 {
		t.Fatalf("empty non-clear job wrote %v", written)
	}

	written, wb = clearHarness(t, "MP4", studioSrc, []writequeue.JobField{
		{Field: "studio", Clear: true, Values: []string{"Acme"}, Source: "manual"},
	})
	if written != nil || !strings.Contains(wb.Detail, "studio") {
		t.Fatalf("clear with values wrote %v, detail %q", written, wb.Detail)
	}
}
