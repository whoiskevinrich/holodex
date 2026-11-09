package rename

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanReasons(t *testing.T) {
	list := mustList(t, "{title} - {studio}", "{title}")
	dir := filepath.Join("lib", "films")
	p := func(n string) string { return filepath.Join(dir, n) }
	inputs := []Input{
		{ID: 1, Path: p("junk1.mkv"), Values: Values{"title": {"Ada"}}},
		{ID: 2, Path: p("Bob.mkv"), Values: Values{"title": {"Bob"}}},
		{ID: 3, Path: p("x3.mkv"), Values: Values{"studio": {"Acme"}}},
		{ID: 4, Path: p("x4.mkv"), Values: Values{"title": {"Ada - Redux"}, "studio": {"Acme"}}},
		{ID: 5, Path: p("x5.mkv"), Values: Values{"title": {"Taken"}}},
		{ID: 6, Path: p("x6.mkv"), Values: Values{"title": {"CON"}}},
		{ID: 7, Path: p("x7.mkv"), Values: Values{"title": {"Lock"}}, Locked: true},
		{ID: 8, Path: p("x8.mkv"), Values: Values{"title": {"Held"}}, Held: true},
		{ID: 9, Path: p("Old.mkv"), Values: Values{"title": {"New"}}, Last: &Record{NewName: "Old.mkv", TokenValues: map[string][]string{"title": {"Old"}}}},
		{ID: 10, Path: p("q.mkv"), Values: Values{"title": {"Bob"}}},
	}
	disk := Disk{Paths: map[int64]string{}, ListDir: func(string) []string {
		return []string{"junk1.mkv", "Bob.mkv", "x3.mkv", "x4.mkv", "x5.mkv", "x6.mkv", "x7.mkv", "x8.mkv", "Old.mkv", "q.mkv", "Taken.mkv"}
	}}
	for _, in := range inputs {
		disk.Paths[in.ID] = in.Path
	}
	rows := Plan(list, "", nil, inputs, disk, nil)
	want := map[int64]string{1: ReasonWillRename, 2: ReasonAlreadyNamed, 3: ReasonNoTemplate, 4: ReasonWontReadBack,
		5: ReasonClash, 6: ReasonBadName, 7: ReasonLocked, 8: ReasonHeld, 9: ReasonOutOfDate, 10: ReasonWillRename}
	for _, r := range rows {
		if r.Reason != want[r.VideoID] {
			t.Errorf("video %d: reason %q, want %q", r.VideoID, r.Reason, want[r.VideoID])
		}
	}
	if rows[5].BadName != "reserved" {
		t.Errorf("bad name chip = %q", rows[5].BadName)
	}
	if rows[9].Name != "Bob {hx-10}.mkv" || rows[9].HolderID != 2 {
		t.Errorf("disambiguated row = %+v", rows[9])
	}
	if !reflect.DeepEqual(rows[8].Changes, []Change{{Token: "title", Recorded: []string{"Old"}, Current: []string{"New"}}}) {
		t.Errorf("changes = %+v", rows[8].Changes)
	}
}
