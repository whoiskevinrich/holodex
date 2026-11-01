package rename

import (
	"errors"
	"path/filepath"
	"sort"
	"strconv"
)

// ErrClash refuses a rename whose name is held by a file Holodex doesn't track (P0-6).
var ErrClash = errors.New("the name is taken by a file Holodex doesn't track")

// Want is one video asking for a new name in a folder. Names are base names.
type Want struct {
	VideoID int64
	Current string
	Desired string
}

// Folder is what the collision plan knows about one folder: every entry on disk, and which of them
// belong to a video (by fold key).
type Folder struct {
	Listing []string
	Tracked map[string]int64
}

// Outcome is where a Want lands. Name is the final name, disambiguated or not. HolderID is the
// video that keeps the plain name when Name is disambiguated. Err refuses the rename.
type Outcome struct {
	Name     string
	HolderID int64
	Err      error
}

// Disambiguate inserts ` {hx-<id>}` before the extension (P0-6).
func Disambiguate(name string, id int64) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)] + " {hx-" + strconv.FormatInt(id, 10) + "}" + ext
}

// PlanFolder settles every Want in one folder against each other and the folder (P0-6). The video
// that already has a name keeps it; a name another Want gives up counts as free; among Wants for a
// free name the lowest id takes it plain; an untracked holder refuses. The result depends only on
// the inputs, so planning twice gives the same plan.
func PlanFolder(f Folder, wants []Want) map[int64]Outcome {
	untracked := map[string]bool{}
	for _, n := range f.Listing {
		if k := FoldKey(n); f.Tracked[k] == 0 {
			untracked[k] = true
		}
	}
	sorted := append([]Want(nil), wants...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].VideoID < sorted[j].VideoID })

	// moving holds the Wants still expected to leave their current name. A Want that is refused or
	// lands on its own current name keeps that name, which can change another Want's outcome, so
	// the plan repeats until nothing more drops out.
	moving := map[int64]bool{}
	for _, w := range sorted {
		moving[w.VideoID] = true
	}
	for {
		out := planPass(sorted, moving, f.Tracked, untracked)
		changed := false
		for _, w := range sorted {
			o := out[w.VideoID]
			stays := o.Err != nil || o.Name == w.Current
			if moving[w.VideoID] && stays {
				moving[w.VideoID] = false
				changed = true
			}
		}
		if !changed {
			return out
		}
	}
}

func planPass(wants []Want, moving map[int64]bool, tracked map[string]int64, untracked map[string]bool) map[int64]Outcome {
	// A name is held by the video whose current name it is, unless that video is moving off it.
	holder := func(key string, self int64) int64 {
		id := tracked[key]
		if id == 0 || id == self || moving[id] {
			return 0
		}
		return id
	}
	out := map[int64]Outcome{}
	claimed := map[string]int64{}
	for _, w := range wants {
		if FoldKey(w.Desired) == FoldKey(w.Current) {
			out[w.VideoID] = Outcome{Name: w.Desired} // a case-only change is its own name
			continue
		}
		key := FoldKey(w.Desired)
		if untracked[key] {
			out[w.VideoID] = Outcome{Err: ErrClash}
			continue
		}
		h := holder(key, w.VideoID)
		if h == 0 {
			h = claimed[key]
		}
		if h == 0 {
			claimed[key] = w.VideoID
			out[w.VideoID] = Outcome{Name: w.Desired}
			continue
		}
		name := Disambiguate(w.Desired, w.VideoID)
		o := Outcome{Name: name, HolderID: h}
		dk := FoldKey(name)
		if err := CheckName(name); err != nil {
			o = Outcome{Err: err}
		} else if dk != FoldKey(w.Current) && (untracked[dk] || holder(dk, w.VideoID) != 0) {
			o = Outcome{Err: ErrClash}
		}
		out[w.VideoID] = o
	}
	return out
}
