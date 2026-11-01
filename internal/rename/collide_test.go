package rename

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func folder(listing []string, tracked map[string]int64) Folder {
	t := map[string]int64{}
	for n, id := range tracked {
		t[FoldKey(n)] = id
	}
	return Folder{Listing: listing, Tracked: t}
}

// N14: an untracked holder refuses.
func TestPlanFolderUntrackedRefuses(t *testing.T) {
	f := folder([]string{"junk.mkv", "Ada.mkv"}, map[string]int64{"junk.mkv": 1})
	out := PlanFolder(f, []Want{{1, "junk.mkv", "Ada.mkv"}})
	if !errors.Is(out[1].Err, ErrClash) {
		t.Fatalf("got %+v, want ErrClash", out[1])
	}
}

// N13 + N14: a tracked holder keeps its name, compared case-folded; the newcomer is disambiguated.
func TestPlanFolderTrackedHolderKeeps(t *testing.T) {
	f := folder([]string{"ada.mkv", "x.mkv"}, map[string]int64{"ada.mkv": 398, "x.mkv": 412})
	out := PlanFolder(f, []Want{{412, "x.mkv", "Ada.mkv"}})
	want := Outcome{Name: "Ada {hx-412}.mkv", HolderID: 398}
	if out[412] != want {
		t.Fatalf("got %+v, want %+v", out[412], want)
	}
}

// N14: several newcomers for a free name — the lowest id takes it plain.
func TestPlanFolderLowestIDTakesFreeName(t *testing.T) {
	f := folder([]string{"b.mkv", "a.mkv"}, map[string]int64{"b.mkv": 9, "a.mkv": 4})
	out := PlanFolder(f, []Want{{9, "b.mkv", "Ada.mkv"}, {4, "a.mkv", "Ada.mkv"}})
	if out[4].Name != "Ada.mkv" || out[9].Name != "Ada {hx-9}.mkv" || out[9].HolderID != 4 {
		t.Fatalf("got %+v", out)
	}
}

// N14 / spec edge case: a two-video swap needs no disambiguator; a name given up counts as free.
func TestPlanFolderSwap(t *testing.T) {
	f := folder([]string{"x.mkv", "y.mkv"}, map[string]int64{"x.mkv": 1, "y.mkv": 2})
	out := PlanFolder(f, []Want{{1, "x.mkv", "y.mkv"}, {2, "y.mkv", "x.mkv"}})
	if out[1] != (Outcome{Name: "y.mkv"}) || out[2] != (Outcome{Name: "x.mkv"}) {
		t.Fatalf("got %+v", out)
	}
}

// A holder whose own rename is refused keeps its name, so the newcomer is disambiguated.
func TestPlanFolderRefusedHolderKeeps(t *testing.T) {
	f := folder([]string{"x.mkv", "y.mkv", "Zed.mkv"}, map[string]int64{"x.mkv": 1, "y.mkv": 2})
	out := PlanFolder(f, []Want{{1, "x.mkv", "y.mkv"}, {2, "y.mkv", "Zed.mkv"}})
	if !errors.Is(out[2].Err, ErrClash) || out[1].Name != "y {hx-1}.mkv" || out[1].HolderID != 2 {
		t.Fatalf("got %+v", out)
	}
}

// P0-6: the disambiguator is stable, so a video already carrying it is settled and planning twice
// gives the same plan.
func TestPlanFolderIdempotent(t *testing.T) {
	f := folder([]string{"Ada.mkv", "Ada {hx-412}.mkv"}, map[string]int64{"Ada.mkv": 398, "Ada {hx-412}.mkv": 412})
	wants := []Want{{412, "Ada {hx-412}.mkv", "Ada.mkv"}}
	a, b := PlanFolder(f, wants), PlanFolder(f, wants)
	if !reflect.DeepEqual(a, b) || a[412].Name != "Ada {hx-412}.mkv" {
		t.Fatalf("got %+v then %+v", a, b)
	}
}

// N15: a case-only change is a rename, not a self-collision.
func TestPlanFolderCaseOnly(t *testing.T) {
	f := folder([]string{"ada.mkv"}, map[string]int64{"ada.mkv": 1})
	out := PlanFolder(f, []Want{{1, "ada.mkv", "Ada.mkv"}})
	if out[1] != (Outcome{Name: "Ada.mkv"}) {
		t.Fatalf("got %+v", out[1])
	}
}

// N11: a name that fits plain but not with the disambiguator is refused when it needs one.
func TestPlanFolderDisambiguatorTooLong(t *testing.T) {
	long := strings.Repeat("a", 251) + ".mkv"
	f := folder([]string{long, "b.mkv"}, map[string]int64{long: 1, "b.mkv": 12345})
	out := PlanFolder(f, []Want{{12345, "b.mkv", long}})
	if !errors.Is(out[12345].Err, ErrTooLong) {
		t.Fatalf("got %+v, want ErrTooLong", out[12345])
	}
}
