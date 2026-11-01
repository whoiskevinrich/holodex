package rename

import (
	"path/filepath"
	"reflect"
	"testing"

	"holodex/internal/repo"
)

// N17: with a rename record, the template that wrote the name parses first, ahead of the current
// list; the record's values are the echo baseline (N18). With none, only the current list reads.
func TestNameReaderOrder(t *testing.T) {
	l := newLib(t)
	id := l.video("Ada - Acme.mkv", "Ada")
	fr := repo.FileRename{VideoID: id, OldName: "junk.mkv", NewName: "Ada - Acme.mkv", Template: "{title} - {studio}",
		TokenValues: map[string][]string{"title": {"Ada"}, "studio": {"Acme"}}, Trigger: repo.RenameTriggerManual}
	var err error
	if fr.ID, err = l.r.InsertPendingRename(l.ctx, fr); err != nil {
		t.Fatal(err)
	}
	if err := l.r.CompleteRename(l.ctx, fr, l.dir); err != nil {
		t.Fatal(err)
	}
	current := mustList(t, "{title}")
	nr := &NameReader{Repo: l.r, Current: func() ([]*Template, string, []string) { return current, "", nil }}

	path := filepath.Join(l.dir, "Ada - Acme.mkv")
	fields, recorded, ok, err := nr.Recorded(l.ctx, id, path)
	if err != nil || !ok {
		t.Fatalf("Recorded: %v %v", ok, err)
	}
	want := map[string][]string{"title": {"Ada"}, "studio": {"Acme"}}
	if !reflect.DeepEqual(fields, want) || !reflect.DeepEqual(recorded, want) {
		t.Errorf("fields = %v, recorded = %v", fields, recorded)
	}
	// A hand-changed studio in the name parses as a new value; the title stays an echo.
	fields, _, _, _ = nr.Recorded(l.ctx, id, filepath.Join(l.dir, "Ada - Other {hx-4}.mkv"))
	if !reflect.DeepEqual(fields, map[string][]string{"title": {"Ada"}, "studio": {"Other"}}) {
		t.Errorf("hand-edited fields = %v", fields)
	}

	// No record: the current list only, and the template that once wrote it is not consulted.
	other := l.video("Bob - Acme.mkv", "Bob")
	if _, _, ok, _ := nr.Recorded(l.ctx, other, filepath.Join(l.dir, "Bob - Acme.mkv")); ok {
		t.Error("a video with no record read as recorded")
	}
	if f, ok := nr.Templates(filepath.Join(l.dir, "Bob - Acme.mkv")); !ok || !reflect.DeepEqual(f, map[string][]string{"title": {"Bob - Acme"}}) {
		t.Errorf("Templates = %v %v", f, ok)
	}
}
