package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"holodex/internal/model"
	"holodex/internal/query"
	"holodex/internal/repo"
)

// `playlist:` references (F89 P0-3, P0-5). A referencing playlist is evaluated with
// each referenced one inlined as it is now, cap included, so the expansion is where the
// perimeter rules live (security-perimeter.md, "Smart playlist rules"): loops (SEC-6),
// the depth, playlist and condition limits (SEC-5), transitive visibility (SEC-2/SEC-3)
// and a reference to a playlist with no fixed contents (RD13). Every rule reports a
// stale ref rather than evaluating, so a definition never quietly widens; a save runs
// the same expansion and refuses on any of them (SEC-11).

// Reference limits (SEC-5).
const (
	maxRefDepth      = 4
	maxRefPlaylists  = 16
	maxRefConditions = 256
)

// Stale kinds the reference rules add to ADR-121's.
const (
	staleLoop     = "loop"     // the definition reaches itself; Value is the id chain
	staleLimit    = "limit"    // past a SEC-5 limit; Key names it
	staleUnstable = "unstable" // a referenced smart playlist has the Random sort and a cap
)

// refExpansion walks one evaluation's references. It doesn't cache: a playlist reached
// twice is expanded twice, under the visibility of each path, and the condition count
// covers both, which bounds the work.
type refExpansion struct {
	stack    []int64 // the playlists being expanded, the one being read first
	distinct map[int64]bool
	conds    int
	refs     map[int64]*repo.PlaylistRef
}

func newRefExpansion(root int64) *refExpansion {
	return &refExpansion{stack: []int64{root}, distinct: map[int64]bool{}, refs: map[int64]*repo.PlaylistRef{}}
}

// countConds adds an expression's conditions to the evaluation's total.
func (x *refExpansion) countConds(n query.Node) *staleRef {
	x.conds += len(query.Conds(n))
	if x.conds > maxRefConditions {
		return &staleRef{Kind: staleLimit, Key: "conditions", Value: strconv.Itoa(maxRefConditions)}
	}
	return nil
}

// expandRefs resolves every playlist an expression names into x.refs. restricted is a
// visitor's read, or any read of a public playlist: then a private reference, or one
// that itself needs the owner, is stale (SEC-2, SEC-3), and so on down the chain.
func (h *Handlers) expandRefs(ctx context.Context, x *refExpansion, n query.Node, restricted bool) ([]staleRef, error) {
	var stale []staleRef
	for _, id := range query.EntityIDs(n)[query.FieldPlaylist] {
		s, err := h.expandRef(ctx, x, id, restricted)
		if err != nil {
			return nil, err
		}
		// Via ends up as the reference in the playlist being read, the one chip the owner
		// can remove, however deep the problem is: each level up overwrites it.
		for i := range s {
			s[i].Via = id
		}
		stale = append(stale, s...)
	}
	return stale, nil
}

func (h *Handlers) expandRef(ctx context.Context, x *refExpansion, id int64, restricted bool) ([]staleRef, error) {
	idStr := strconv.FormatInt(id, 10)
	if i := slices.Index(x.stack, id); i >= 0 {
		chain := make([]string, 0, len(x.stack)-i+1)
		for _, s := range x.stack[i:] {
			chain = append(chain, strconv.FormatInt(s, 10))
		}
		return []staleRef{{Kind: staleLoop, Key: query.FieldPlaylist, Value: strings.Join(append(chain, idStr), ",")}}, nil
	}
	if len(x.stack) > maxRefDepth {
		return []staleRef{{Kind: staleLimit, Key: "depth", Value: strconv.Itoa(maxRefDepth)}}, nil
	}
	x.distinct[id] = true
	if len(x.distinct) > maxRefPlaylists {
		return []staleRef{{Kind: staleLimit, Key: "playlists", Value: strconv.Itoa(maxRefPlaylists)}}, nil
	}
	p, err := h.repo.GetPlaylist(ctx, id, false)
	if errors.Is(err, repo.ErrNotFound) {
		return []staleRef{{Kind: staleMissing, Key: query.FieldPlaylist, Value: idStr}}, nil
	}
	if err != nil {
		return nil, err
	}
	if restricted && p.Visibility != model.PlaylistPublic {
		return []staleRef{{Kind: staleOwnerOnly, Key: query.FieldPlaylist, Value: idStr}}, nil
	}
	if !p.Smart() {
		x.refs[id] = &repo.PlaylistRef{ID: id, Snapshot: true}
		return nil, nil
	}
	if p.Sort == "random" && p.Cap != nil {
		return []staleRef{{Kind: staleUnstable, Key: query.FieldPlaylist, Value: idStr}}, nil
	}
	x.stack = append(x.stack, id)
	f, stale, err := h.smartFilterAt(ctx, p, !restricted, x)
	x.stack = x.stack[:len(x.stack)-1]
	if err != nil {
		return nil, err
	}
	x.refs[id] = &repo.PlaylistRef{ID: id, Filter: f}
	return stale, nil
}

// checkPlaylistDefinition is the save-time half (SEC-11): the definition a create or
// PATCH would leave behind is expanded as a read would be, and any stale ref refuses the
// save with that ref named. It writes 400 and returns false on refusal.
//
// visibilityOnly is a save that only makes the playlist public, whose refusal is worded
// for the visibility control rather than the editor (handoff "Visibility refusal").
func (h *Handlers) checkPlaylistDefinition(ctx context.Context, w http.ResponseWriter, p *model.Playlist, visibilityOnly bool) bool {
	if !p.Smart() {
		return true
	}
	_, all, err := h.smartPlaylistFilter(ctx, p, true)
	if err != nil {
		h.fail(w, "check playlist", err)
		return false
	}
	// Only the reference rules refuse a save. A flat id facet naming a deleted entity
	// stays F75's read-time notice, as before.
	var stale []staleRef
	for _, s := range all {
		if s.Via != 0 || s.Key == query.FieldPlaylist || s.Kind == staleLoop || s.Kind == staleLimit || s.Kind == staleUnstable {
			stale = append(stale, s)
		}
	}
	if len(stale) == 0 {
		return true
	}
	h.nameStaleRefs(ctx, stale)
	sentence, fix := refusalCopy(stale[0], visibilityOnly)
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": sentence, "fix": fix, "stale_refs": stale})
	return false
}

// nameStaleRefs fills in the current names of the playlists each ref involves, for the
// owner's notices and refusals. A visitor's response never carries them.
func (h *Handlers) nameStaleRefs(ctx context.Context, refs []staleRef) {
	name := func(id int64) string {
		if id == 0 {
			return "this playlist"
		}
		if n, ok, err := h.repo.QueryPlaylistName(ctx, id, false); err == nil && ok {
			return n
		}
		return "#" + strconv.FormatInt(id, 10)
	}
	for i := range refs {
		s := &refs[i]
		if s.Via != 0 {
			s.ViaName = name(s.Via)
		}
		switch {
		case s.Kind == staleLoop:
			var names []string
			for _, v := range strings.Split(s.Value, ",") {
				id, _ := strconv.ParseInt(v, 10, 64)
				names = append(names, name(id))
			}
			s.Name = strings.Join(names, " → ")
		case s.Key == query.FieldPlaylist:
			id, _ := strconv.ParseInt(s.Value, 10, 64)
			s.Name = name(id)
		}
	}
}

// refusalCopy words a refused definition for the owner, a sentence and how to fix it
// (handoff "In-editor problem copy"). The limits are the ref's own, never literals here.
// It reads the names nameStaleRefs filled in.
func refusalCopy(s staleRef, visibilityOnly bool) (sentence, fix string) {
	other := s.ViaName
	if other == "" {
		other = s.Name
	}
	removeChip := "Remove “in: " + other + "”"
	const removeRef = "Remove a playlist reference."
	switch s.Kind {
	case staleLoop:
		return "This would make a loop: " + s.Name + ".", removeChip + "."
	case staleLimit:
		switch s.Key {
		case "depth":
			return "Playlists can include each other at most " + s.Value + " deep.", removeRef
		case "playlists":
			return "A definition can reach at most " + s.Value + " playlists.", removeRef
		}
		return "A definition can have at most " + s.Value + " conditions, counting the playlists it includes.",
			"Simplify the query, or remove a playlist reference."
	case staleUnstable:
		return "“" + s.Name + "” is shuffled and capped, so its contents change every time.",
			removeChip + ", or give it another sort or no cap."
	case staleOwnerOnly:
		direct := s.Via == 0 || (s.Key == query.FieldPlaylist && s.Value == strconv.FormatInt(s.Via, 10))
		switch {
		case s.Key != query.FieldPlaylist:
			return "This playlist can’t be public: it includes “" + other + "”, which uses owner-only conditions.",
				removeChip + " from the definition."
		case !direct:
			return "This playlist can’t be public: it includes “" + other + "”, which includes a private playlist.",
				removeChip + " from the definition."
		case visibilityOnly:
			return "This playlist can’t be public: it includes “" + other + "”, which is private.",
				"Make “" + other + "” public first, or remove it from the definition."
		}
		return "A public playlist can’t include a private playlist.",
			removeChip + ", make “" + other + "” public, or make this playlist private."
	case staleMissing:
		if s.Key == query.FieldPlaylist {
			return "This definition refers to a playlist that was deleted.", "Remove “in: #" + s.Value + "”."
		}
		return "This definition refers to something that was deleted.", "Remove that condition."
	}
	return "This definition can’t be saved.", ""
}

// referencedBy lists the smart playlists whose definition names id: the delete
// confirmation names them (P0-3).
func (h *Handlers) referencedBy(ctx context.Context, id int64) ([]map[string]any, error) {
	all, err := h.repo.ListPlaylists(ctx, false)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, p := range all {
		if p.Query == nil || p.ID == id {
			continue
		}
		q, err := url.ParseQuery(*p.Query)
		if err != nil || q.Get("expr") == "" {
			continue
		}
		expr, err := query.Parse(q.Get("expr"))
		if err != nil {
			continue
		}
		if slices.Contains(query.EntityIDs(expr.Root)[query.FieldPlaylist], id) {
			out = append(out, map[string]any{"id": p.ID, "name": p.Name})
		}
	}
	return out, nil
}
