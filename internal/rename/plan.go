package rename

import (
	"errors"
	"path/filepath"
	"reflect"
	"sort"
)

// Plan reasons (P0-7, P1-1). Each video has exactly one.
const (
	ReasonWillRename   = "will-rename"
	ReasonOutOfDate    = "out-of-date"
	ReasonAlreadyNamed = "already-named"
	ReasonNoTemplate   = "no-template"
	ReasonWontReadBack = "wont-read-back"
	ReasonClash        = "clash"
	ReasonBadName      = "bad-name"
	ReasonLocked       = "locked"
	ReasonHeld         = "held"
)

// Reasons lists every reason in the plan's filter order.
var Reasons = []string{ReasonWillRename, ReasonOutOfDate, ReasonAlreadyNamed, ReasonNoTemplate,
	ReasonWontReadBack, ReasonClash, ReasonBadName, ReasonLocked, ReasonHeld}

// Renames reports whether a reason is one an apply acts on.
func Renames(reason string) bool { return reason == ReasonWillRename || reason == ReasonOutOfDate }

// Record is what the journal says Holodex last wrote for a video.
type Record struct {
	NewName     string
	TokenValues map[string][]string
}

// Input is one video as the plan sees it.
type Input struct {
	ID     int64
	Path   string
	Values Values
	Locked bool
	Held   bool
	Last   *Record // nil when Holodex never named the file
}

// Change is one token whose value differs from the one Holodex last wrote (P0-12).
type Change struct {
	Token    string   `json:"token"`
	Recorded []string `json:"recorded"`
	Current  []string `json:"current"`
}

// Row is one video's line in the plan.
type Row struct {
	VideoID  int64
	Path     string
	Current  string
	Name     string // the final name; empty when there is none
	Template int    // index of the template that names it, -1 for none
	Reason   string
	BadName  string              // the bad-name chip, when Reason is bad-name
	Parsed   map[string][]string // the read-back, when Reason is wont-read-back
	HolderID int64               // the video keeping the plain name, when Name is disambiguated
	Written  map[string][]string
	Changes  []Change
}

// badNameCodes names each bad-name refusal for the plan chip.
var badNameCodes = []struct {
	err  error
	code string
}{
	{ErrReserved, "reserved"}, {ErrTrailing, "trailing"}, {ErrTooLong, "too-long"},
	{ErrInvalidText, "invalid-text"}, {ErrBadChar, "bad-char"}, {ErrEmptyName, "empty"},
	{ErrNotLocal, "not-local"},
}

// BadNameCode returns the chip for a bad-name error, or "" when err isn't one.
func BadNameCode(err error) string {
	for _, b := range badNameCodes {
		if errors.Is(err, b.err) {
			return b.code
		}
	}
	return ""
}

// Disk is what the collision half of the plan reads: every video row's path, whatever its state,
// and the folder listing.
type Disk struct {
	Paths   map[int64]string
	ListDir func(dir string) []string
}

// Plan computes every input's row. inRun reports whether a video that wants a new name is part of
// this run (a name it gives up counts as free); nil means every such video is.
func Plan(list []*Template, delimiter string, inputs []Input, disk Disk, inRun func(id int64) bool) []Row {
	rows := make([]Row, len(inputs))
	wantsByDir := map[string][]Want{}
	idx := map[int64]int{}
	for i, in := range inputs {
		cur := filepath.Base(in.Path)
		row := Row{VideoID: in.ID, Path: in.Path, Current: cur, Template: -1}
		n, err := Build(list, in.Values, delimiter, filepath.Ext(cur))
		row.Written = n.Written
		if n.Name != "" {
			row.Name, row.Template = n.Name, n.Index
		}
		var rb *ReadBackError
		switch {
		case in.Held:
			row.Reason = ReasonHeld
		case in.Locked:
			row.Reason = ReasonLocked
		case errors.Is(err, ErrNoTemplate):
			row.Reason = ReasonNoTemplate
		case errors.As(err, &rb):
			row.Reason, row.Parsed = ReasonWontReadBack, rb.Parsed
		case err != nil:
			row.Reason, row.BadName = ReasonBadName, BadNameCode(err)
		case n.Name == cur:
			row.Reason = ReasonAlreadyNamed
		default:
			if inRun == nil || inRun(in.ID) {
				dir := filepath.Dir(in.Path)
				wantsByDir[dir] = append(wantsByDir[dir], Want{VideoID: in.ID, Current: cur, Desired: n.Name})
			} else {
				row.Reason = ReasonWillRename // outside this run; settled when it runs
			}
		}
		rows[i], idx[in.ID] = row, i
	}

	tracked := map[string]map[string]int64{}
	for id, p := range disk.Paths {
		dir := filepath.Dir(p)
		if wantsByDir[dir] == nil {
			continue
		}
		if tracked[dir] == nil {
			tracked[dir] = map[string]int64{}
		}
		tracked[dir][FoldKey(filepath.Base(p))] = id
	}
	dirs := make([]string, 0, len(wantsByDir))
	for d := range wantsByDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		out := PlanFolder(Folder{Listing: disk.ListDir(dir), Tracked: tracked[dir]}, wantsByDir[dir])
		for id, o := range out {
			row := &rows[idx[id]]
			switch {
			case errors.Is(o.Err, ErrClash):
				row.Reason = ReasonClash
			case o.Err != nil:
				row.Reason, row.BadName = ReasonBadName, BadNameCode(o.Err)
			case o.Name == row.Current:
				row.Name, row.Reason = o.Name, ReasonAlreadyNamed
			default:
				row.Name, row.HolderID = o.Name, o.HolderID
			}
		}
	}

	for i, in := range inputs {
		row := &rows[i]
		if row.Reason != "" && row.Reason != ReasonWillRename {
			continue
		}
		if in.Last != nil && in.Last.NewName == row.Current {
			row.Reason, row.Changes = ReasonOutOfDate, changes(in.Last.TokenValues, row.Written)
		} else {
			row.Reason = ReasonWillRename
		}
	}
	return rows
}

func changes(recorded, current map[string][]string) []Change {
	keys := map[string]bool{}
	for k := range recorded {
		keys[k] = true
	}
	for k := range current {
		keys[k] = true
	}
	var out []Change
	for k := range keys {
		if !reflect.DeepEqual(recorded[k], current[k]) {
			out = append(out, Change{Token: k, Recorded: recorded[k], Current: current[k]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}
