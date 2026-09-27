package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

type fakeCompletenessStore struct {
	settings map[string]string
	marks    int
	markErr  error
}

func (f *fakeCompletenessStore) GetSetting(_ context.Context, k string) (string, bool, error) {
	v, ok := f.settings[k]
	return v, ok, nil
}

func (f *fakeCompletenessStore) PutSetting(_ context.Context, k, v string) error {
	f.settings[k] = v
	return nil
}

func (f *fakeCompletenessStore) MarkAllCompletenessDirty(context.Context) error {
	f.marks++
	return f.markErr
}

// ADR-112 D1: every score is re-dirtied at boot only when the executable or a
// config file it is scored against changed since the store was computed.
func TestMarkCompletenessDirtyIfInputsChanged(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	exe := write("holodex", "build-1")
	mappings := write("metadata-mappings.yaml", "fields: []\n")
	sources := filepath.Join(dir, "metadata-sources.yaml") // absent: loads as no providers
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	s := &fakeCompletenessStore{settings: map[string]string{}}
	boot := func() { markCompletenessDirtyIfInputsChanged(ctx, s, log, exe, mappings, sources) }

	boot()
	if s.marks != 1 || s.settings[completenessInputsKey] == "" {
		t.Fatalf("first boot: marks=%d fingerprint=%q; want 1 mark and a recorded fingerprint", s.marks, s.settings[completenessInputsKey])
	}
	boot()
	if s.marks != 1 {
		t.Fatalf("unchanged restart marked dirty (marks=%d); want the stored scores kept", s.marks)
	}

	for _, change := range []struct {
		name string
		do   func()
	}{
		{"new build", func() { write("holodex", "build-2") }},
		{"edited mappings", func() { write("metadata-mappings.yaml", "fields: [x]\n") }},
		{"sources file created", func() { write("metadata-sources.yaml", "providers: []\n") }},
		{"sources file deleted", func() { os.Remove(sources) }},
	} {
		before := s.marks
		change.do()
		boot()
		if s.marks != before+1 {
			t.Errorf("%s: marks %d → %d; want one re-dirty", change.name, before, s.marks)
		}
	}
}

// An unfingerprintable build falls back to the old always-dirty boot and
// records nothing; a failed mark records nothing, so the next boot retries.
func TestMarkCompletenessDirtyIfInputsChanged_FailuresReScore(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	s := &fakeCompletenessStore{settings: map[string]string{}}
	markCompletenessDirtyIfInputsChanged(ctx, s, log, "", "")
	markCompletenessDirtyIfInputsChanged(ctx, s, log, "", "")
	if s.marks != 2 || len(s.settings) != 0 {
		t.Errorf("unknown executable: marks=%d settings=%v; want a mark every boot, nothing recorded", s.marks, s.settings)
	}

	exe := filepath.Join(t.TempDir(), "holodex")
	if err := os.WriteFile(exe, []byte("build"), 0o644); err != nil {
		t.Fatal(err)
	}
	s = &fakeCompletenessStore{settings: map[string]string{}, markErr: errors.New("busy")}
	markCompletenessDirtyIfInputsChanged(ctx, s, log, exe)
	if len(s.settings) != 0 {
		t.Errorf("failed mark recorded fingerprint %v; want none so the next boot retries", s.settings)
	}
}
