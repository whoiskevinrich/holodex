package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/rename"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Renamer is the slice of the rename queue the API drives (F86 Manual): the wake-up after an
// apply enqueues.
type Renamer interface {
	Kick()
}

// SetRenamer wires the rename queue. Nil leaves the apply routes 503.
func (h *Handlers) SetRenamer(rn Renamer) { h.renamer = rn }

// mountRename registers the rename routes inside the owner group (security-perimeter.md
// "Rename names": every rename, plan, lock, undo, history and template route is owner-only).
func (h *Handlers) mountRename(r chi.Router) {
	r.Get("/rename/plan", h.renamePlan)
	r.Post("/rename/plan/preview", h.renamePlanPreview)
	r.Put("/media/{id}/name-lock", h.setNameLock)
	r.Get("/media/{id}/renames", h.renameHistory)
	h.mountRenameApply(r)
}

// renameProviderRe bounds the provider names a template may name as an identity token: braces,
// separators and anything else that couldn't sit in a marker are never offered.
var renameProviderRe = regexp.MustCompile(`^[A-Za-z0-9_]+(-[A-Za-z0-9_]+)*$`)

// RenameProviders lists the enabled video providers whose ids a template may name.
func (h *Handlers) RenameProviders() []string {
	if h.enrich == nil {
		return nil
	}
	var out []string
	for _, s := range h.enrich.Sources() {
		// A provider named for one of the builder's own markers would forge or lose it.
		if s.Name == "hx" || s.Name == rename.TokEdition || s.Name == rename.TokPart {
			continue
		}
		if s.Supports(model.EnrichEntityVideo) && renameProviderRe.MatchString(s.Name) {
			out = append(out, s.Name)
		}
	}
	return out
}

// renameDelimiter is the people delimiter the owner's filename patterns use (P0-1).
func (h *Handlers) renameDelimiter() string {
	if h.patterns == nil {
		return ""
	}
	_, d := h.patterns.Current()
	return d
}

// renameCharacters is the owner's P0-4 character table. A stored table that no longer validates
// falls back to the default (nil).
func (h *Handlers) renameCharacters() rename.Characters {
	if h.settings == nil {
		return nil
	}
	c, err := rename.DecodeCharacters(h.settings.Get(rename.KeyCharacters))
	if err != nil {
		return nil
	}
	return c
}

// renameTokenFields maps a template token to the canonical field it renders from.
var renameTokenFields = map[string]string{
	rename.TokTitle: "title", rename.TokStudio: "studio", rename.TokPeople: "actors",
	rename.TokYear: "release_date", rename.TokEdition: "edition", rename.TokPart: "part",
}

// renameState is everything the plan reads, loaded once per request.
type renameState struct {
	inputs []rename.Input
	titles map[int64]string
	locks  map[int64]repo.FileNameState
	disk   rename.Disk
}

// loadRenameState resolves every active video's token values (the completeness resolve, batch
// loaded) and reads the journal, the name locks, the held originals and every video row's path.
// only, when non-nil, limits the resolve to those videos; every row's path is still read.
func (h *Handlers) loadRenameState(ctx context.Context, only map[int64]bool) (*renameState, error) {
	videos, err := h.repo.ListAllVideos(ctx, repo.VideoFilter{})
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	if only != nil {
		videos = slices.DeleteFunc(videos, func(v model.Video) bool { return !only[v.ID] })
	}
	ids := make([]int64, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
	}
	extra, err := h.repo.ExtraMetadataForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	enr, err := h.repo.EnrichmentForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	cur, err := h.repo.CurationForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	dec, err := h.repo.DecisionsForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	var films map[int64][]repo.FilmAttachment
	if h.filmsEnabled {
		if films, err = h.filmsForVideos(ctx, ids); err != nil {
			return nil, err
		}
	}
	last, err := h.repo.LatestDoneRenames(ctx)
	if err != nil {
		return nil, err
	}
	locks, err := h.repo.FileNameStates(ctx)
	if err != nil {
		return nil, err
	}
	held, err := h.repo.HeldVideoIDs(ctx)
	if err != nil {
		return nil, err
	}
	paths, err := h.repo.AllVideoPaths(ctx)
	if err != nil {
		return nil, err
	}
	personIDs := map[int64]bool{}
	for _, v := range videos {
		for _, p := range v.People {
			personIDs[p.ID] = true
		}
	}
	aliases, err := h.repo.AliasesForEntities(ctx, model.EnrichEntityPerson, slices.Collect(maps.Keys(personIDs)))
	if err != nil {
		return nil, err
	}

	st := &renameState{titles: map[int64]string{}, locks: locks, disk: rename.Disk{Paths: map[int64]string{}, ListDir: listDirNames}}
	for _, p := range paths {
		st.disk.Paths[p.ID] = p.FilePath
	}
	providers := h.RenameProviders()
	fields := h.mappings.Current().Fields()
	for i := range videos {
		v := &videos[i]
		rows := enr[v.ID]
		fs, _ := h.mergePromotions(ctx, model.EnrichEntityVideo, fields, rows)
		fs = h.mergeClaims(ctx, model.EnrichEntityVideo, fs)
		resolved := resolver.Resolve(v, extra[v.ID], injectFilmSources(enrichmentFromRows(rows), films[v.ID]),
			curationFromRows(cur[v.ID]), fs, h.resolveOptions(decisionsFromRows(dec[v.ID])))
		vals := renameValues(resolved, rows, providers)
		if people, ok := vals[rename.TokPeople]; ok {
			vals[rename.TokPeople] = canonicalPeople(people, v.People, aliases)
		}
		in := rename.Input{ID: v.ID, Path: v.FilePath, Values: vals,
			Locked: locks[v.ID].Locked, Held: held[v.ID]}
		if fr, ok := last[v.ID]; ok {
			in.Last = &rename.Record{NewName: fr.NewName, TokenValues: fr.TokenValues}
		}
		st.inputs = append(st.inputs, in)
		st.titles[v.ID] = firstResolvedValue(resolved, "title")
	}
	return st, nil
}

// renameValues collects a video's raw token values. An identity token takes only a confirmed
// match's id: a stored enrichment row's external id (P0-1); the filename namespace never counts.
func renameValues(resolved []resolver.ResolvedField, rows []repo.EnrichmentRow, providers []string) rename.Values {
	v := rename.Values{}
	for tok, field := range renameTokenFields {
		if vals := resolvedValues(resolved, field); len(vals) > 0 {
			v[tok] = vals
		}
	}
	for _, p := range providers {
		for _, row := range rows {
			if row.Provider == p && row.ExternalID != "" {
				v[rename.IdentityToken(p)] = []string{row.ExternalID}
				break
			}
		}
	}
	return v
}

// canonicalPeople names each of the video's linked people once, by their canonical name. The actors
// merge folds only case and whitespace, so a file tag's alias and a source's name for the same
// person both survive it; the links already fold them to one person. Every link counts, whatever
// its role: a link made before roles existed has none. A value that isn't a linked person's name
// or alias (not yet relinked) passes through. The field's order is kept.
func canonicalPeople(values []string, linked []model.Person, aliases map[int64][]model.EntityAlias) []string {
	key := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	names := map[string]string{}
	for _, p := range linked {
		names[key(p.Name)] = p.Name
		for _, a := range aliases[p.ID] {
			names[key(a.Alias)] = p.Name
		}
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		if n, ok := names[key(v)]; ok {
			v = n
		}
		if !seen[key(v)] {
			seen[key(v)] = true
			out = append(out, v)
		}
	}
	return out
}

// listDirNames lists a folder's entry names. A folder that can't be read lists nothing; the worker
// checks the disk again before any rename.
func listDirNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

// renameTemplates compiles the stored template list.
func (h *Handlers) renameTemplates() ([]string, []*rename.Template) {
	texts, _ := rename.DecodeTemplates(h.settings.Get(rename.KeyTemplates))
	list, _ := rename.ParseList(texts, h.RenameProviders())
	return texts, list
}

// RenameTemplateSet gives the read-back its current templates, delimiter and providers (F86
// P0-11). With no settings wired the list is empty.
func (h *Handlers) RenameTemplateSet() ([]*rename.Template, string, []string) {
	if h.settings == nil {
		return nil, h.renameDelimiter(), nil
	}
	_, list := h.renameTemplates()
	return list, h.renameDelimiter(), h.RenameProviders()
}

// planRowView is one plan row on the wire. Owner-only: it carries file names.
type planRowView struct {
	VideoID  int64               `json:"video_id"`
	Title    string              `json:"title"`
	Current  string              `json:"current"`
	Name     string              `json:"name,omitempty"`
	Template *int                `json:"template"`
	Reason   string              `json:"reason"`
	BadName  string              `json:"bad_name,omitempty"`
	Parsed   map[string][]string `json:"parsed,omitempty"`
	// Written is the value each token put into the name, sent beside Parsed so a won't-read-back
	// row can say which value didn't survive.
	Written  map[string][]string `json:"written,omitempty"`
	HolderID int64               `json:"holder_id,omitempty"`
	Changes  []rename.Change     `json:"changes,omitempty"`
	// LockSource and LockedAt say why and when a locked video was locked (handoff call 7).
	LockSource string     `json:"lock_source,omitempty"`
	LockedAt   *time.Time `json:"locked_at,omitempty"`
}

type templateCount struct {
	Names        int `json:"names"`
	WontReadBack int `json:"wont_read_back"`
}

type planView struct {
	Mode           string          `json:"mode"`
	Templates      []string        `json:"templates"`
	Providers      []string        `json:"providers"`
	Total          int             `json:"total"`
	Counts         map[string]int  `json:"counts"`
	TemplateCounts []templateCount `json:"template_counts"`
	Reason         string          `json:"reason"`
	Filtered       int             `json:"filtered"`
	Rows           []planRowView   `json:"rows"`
}

const (
	planDefaultLimit = 100
	planMaxLimit     = 500
)

func (h *Handlers) renamePlan(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil || h.mappings == nil {
		writeError(w, http.StatusServiceUnavailable, "renaming not configured")
		return
	}
	texts, list := h.renameTemplates()
	h.writePlan(w, r, texts, list, h.renameCharacters())
}

// planPreview is the body of a preview: an unsaved template list (A6) and, when it changed, an
// unsaved character table. Nothing is saved.
type planPreview struct {
	Templates  []string          `json:"templates"`
	Characters map[string]string `json:"characters,omitempty"`
}

func (h *Handlers) renamePlanPreview(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil || h.mappings == nil {
		writeError(w, http.StatusServiceUnavailable, "renaming not configured")
		return
	}
	var body planPreview
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	list, errs := rename.ParseList(body.Templates, h.RenameProviders())
	if list == nil && len(body.Templates) > 0 {
		tle := rename.ListError(errs)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": tle.Error(), "rows": tle.Rows})
		return
	}
	chars := h.renameCharacters()
	if body.Characters != nil {
		c, err := rename.ParseCharacters(body.Characters)
		var cte *rename.CharacterTableError
		if errors.As(err, &cte) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": cte.Error(), "characters": cte.Rows})
			return
		}
		chars = c
	}
	h.writePlan(w, r, body.Templates, list, chars)
}

func (h *Handlers) writePlan(w http.ResponseWriter, r *http.Request, texts []string, list []*rename.Template, chars rename.Characters) {
	q := r.URL.Query()
	reason := q.Get("reason")
	if reason == "" {
		reason = "all"
	}
	if reason != "all" && !validReason(reason) {
		writeError(w, http.StatusBadRequest, "unknown reason")
		return
	}
	limit, offset := planDefaultLimit, 0
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = min(n, planMaxLimit)
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n > 0 {
		offset = n
	}
	var onlyID int64
	if s := q.Get("video_id"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid video_id")
			return
		}
		onlyID = n
	}

	st, err := h.loadRenameState(r.Context(), nil)
	if err != nil {
		h.fail(w, "rename plan", err)
		return
	}
	rows := rename.Plan(list, h.renameDelimiter(), chars, st.inputs, st.disk, nil)

	view := planView{Mode: h.settings.Get(rename.KeyMode), Templates: texts, Providers: h.RenameProviders(),
		Total: len(rows), Reason: reason, Counts: map[string]int{}, TemplateCounts: make([]templateCount, len(list)),
		Rows: []planRowView{}}
	if view.Templates == nil {
		view.Templates = []string{}
	}
	if view.Providers == nil {
		view.Providers = []string{}
	}
	for _, rs := range rename.Reasons {
		view.Counts[rs] = 0
	}
	for _, row := range rows {
		view.Counts[row.Reason]++
		if row.Template >= 0 {
			switch row.Reason {
			case rename.ReasonWillRename, rename.ReasonOutOfDate, rename.ReasonAlreadyNamed:
				view.TemplateCounts[row.Template].Names++
			case rename.ReasonWontReadBack:
				view.TemplateCounts[row.Template].WontReadBack++
			}
		}
		if (reason != "all" && row.Reason != reason) || (onlyID != 0 && row.VideoID != onlyID) {
			continue
		}
		view.Filtered++
		if view.Filtered <= offset || len(view.Rows) >= limit {
			continue
		}
		rv := planRowView{VideoID: row.VideoID, Title: st.titles[row.VideoID], Current: row.Current, Name: row.Name,
			Reason: row.Reason, BadName: row.BadName, Parsed: row.Parsed, HolderID: row.HolderID, Changes: row.Changes}
		if row.Template >= 0 {
			t := row.Template
			rv.Template = &t
		}
		if row.Reason == rename.ReasonWontReadBack {
			rv.Written = row.Written
		}
		if ls := st.locks[row.VideoID]; ls.Locked {
			at := ls.LockedAt
			rv.LockSource, rv.LockedAt = ls.LockSource, &at
		}
		view.Rows = append(view.Rows, rv)
	}
	writeJSON(w, http.StatusOK, view)
}

func validReason(s string) bool {
	for _, r := range rename.Reasons {
		if r == s {
			return true
		}
	}
	return false
}

// nameLockWrite is the body of a lock change.
type nameLockWrite struct {
	Locked bool `json:"locked"`
}

func (h *Handlers) setNameLock(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body nameLockWrite
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, err := h.repo.PathByID(r.Context(), id); errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "video not found")
		return
	} else if err != nil {
		h.fail(w, "name lock", err)
		return
	}
	if err := h.repo.SetNameLock(r.Context(), id, body.Locked, repo.LockOwner, time.Now()); err != nil {
		h.fail(w, "name lock", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"locked": body.Locked})
}

// renameHistoryView is one recorded name, oldest first (P1-2).
type renameHistoryView struct {
	OldName   string    `json:"old_name"`
	NewName   string    `json:"new_name"`
	Trigger   string    `json:"trigger"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handlers) renameHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	hist, err := h.repo.RenameHistory(r.Context(), id)
	if err != nil {
		h.fail(w, "rename history", err)
		return
	}
	out := make([]renameHistoryView, len(hist))
	for i, fr := range hist {
		out[i] = renameHistoryView{OldName: fr.OldName, NewName: fr.NewName, Trigger: fr.Trigger, CreatedAt: fr.CreatedAt}
	}
	writeJSON(w, http.StatusOK, out)
}
