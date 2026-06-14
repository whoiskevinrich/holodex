package mapping

import (
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/model"
)

func TestResolvePrecedenceAndMulti(t *testing.T) {
	m, err := parse([]byte(`
fields:
  - canonical: studio
    label: Studio
    sources: [Publisher, Label, Studio]
    filterable: true
  - canonical: director
    label: Director
    sources: [Director]
    multi: true
`))
	if err != nil {
		t.Fatal(err)
	}

	res := m.Resolve([]model.ExtraMetadata{
		{SourceKey: "Label", Value: "Acme"},       // present, but lower precedence
		{SourceKey: "Publisher", Value: "Globex"},  // higher precedence → wins
		{SourceKey: "Director", Value: "Ann, Bob"}, // multi → split
	})
	got := map[string][]string{}
	for _, r := range res {
		got[r.Canonical] = r.Values
	}
	if len(got["studio"]) != 1 || got["studio"][0] != "Globex" {
		t.Errorf("studio = %v, want [Globex] (precedence)", got["studio"])
	}
	if len(got["director"]) != 2 || got["director"][0] != "Ann" || got["director"][1] != "Bob" {
		t.Errorf("director = %v, want [Ann Bob] (multi split)", got["director"])
	}

	if fl := m.Filterable(); len(fl) != 1 || fl[0].Canonical != "studio" {
		t.Errorf("filterable = %+v", fl)
	}
	if f, ok := m.ByCanonical("STUDIO"); !ok || f.Label != "Studio" {
		t.Errorf("ByCanonical(STUDIO) = %+v ok=%v", f, ok)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil || len(m.Fields()) != 0 {
		t.Errorf("missing file should be empty: fields=%d err=%v", len(m.Fields()), err)
	}
}

func TestStoreReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.yaml")
	if err := os.WriteFile(path, []byte("fields:\n  - canonical: studio\n    sources: [Publisher]\n    filterable: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Current().Fields()) != 1 {
		t.Fatalf("initial fields = %d, want 1", len(s.Current().Fields()))
	}
	if err := os.WriteFile(path, []byte("fields:\n  - canonical: studio\n    sources: [Publisher]\n  - canonical: collection\n    sources: [Album]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}
	if len(s.Current().Fields()) != 2 {
		t.Errorf("after reload fields = %d, want 2", len(s.Current().Fields()))
	}
}
