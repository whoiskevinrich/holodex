package api

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// Duplicate videos (F76, HOLODEX-521): the Videos group on the owner Duplicates page.
// Pairs are computed on demand (repo.ListVideoPairs); the verbs are file verbs — keep
// one (the other goes to Trash and the owner's work carries over), keep both, or label
// the pair as editions or parts. Owner-gated (mounted in the requireOwner group).

func (h *Handlers) mountVideoDuplicates(r chi.Router) {
	r.Get("/owner/duplicates/videos", h.listVideoDuplicates)
	r.Get("/owner/duplicates/videos/{a}/{b}", h.compareVideoDuplicate)
	r.Post("/owner/duplicates/videos/keep", h.keepOneVideo)
	r.Post("/owner/duplicates/videos/keep-both", h.keepBothVideos)
	r.Post("/owner/duplicates/videos/label", h.labelVideoPair)
}

// videoPairSide is one file in a listed pair: enough for the row's
// "4K · 34:14 ↔ FHD · 34:12" summary.
type videoPairSide struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Duration int    `json:"duration_sec"`
}

type videoPairRow struct {
	A videoPairSide `json:"a"`
	B videoPairSide `json:"b"`
}

func (h *Handlers) listVideoDuplicates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pairs, err := h.repo.ListVideoPairs(ctx)
	if err != nil {
		h.fail(w, "list video duplicates", err)
		return
	}
	out := []videoPairRow{}
	if len(pairs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"pairs": out})
		return
	}
	facts, titles, err := h.videoPairFacts(ctx, pairIDs(pairs))
	if err != nil {
		h.fail(w, "list video duplicates", err)
		return
	}
	side := func(id int64) videoPairSide {
		f := facts[id]
		return videoPairSide{ID: id, Title: titles[id], Width: f.Width, Height: f.Height, Duration: f.Duration}
	}
	for _, p := range pairs {
		out = append(out, videoPairRow{A: side(p.A), B: side(p.B)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pairs": out})
}

func pairIDs(pairs []repo.VideoPair) []int64 {
	seen := map[int64]bool{}
	var ids []int64
	for _, p := range pairs {
		for _, id := range []int64{p.A, p.B} {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// videoPairFacts loads the file facts and the resolved display title (what the media
// page shows — a provider title over the file's) for each id.
func (h *Handlers) videoPairFacts(ctx context.Context, ids []int64) (map[int64]repo.VideoFileFacts, map[int64]string, error) {
	facts, err := h.repo.VideoFacts(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	items := make([]model.Video, 0, len(ids))
	for _, id := range ids {
		items = append(items, model.Video{ID: id, Title: facts[id].Title})
	}
	if h.mappings != nil {
		h.applyBrowseTitles(ctx, items, h.mappings.Current().Fields())
	}
	titles := make(map[int64]string, len(items))
	for _, v := range items {
		titles[v.ID] = v.Title
	}
	return facts, titles, nil
}

// videoCompareSide is one column of the compare panel's fact table.
type videoCompareSide struct {
	repo.VideoFileFacts
	FileName string            `json:"file_name"`
	Folder   string            `json:"folder"`
	Edition  string            `json:"edition,omitempty"`
	Part     string            `json:"part,omitempty"`
	Work     repo.VideoWork    `json:"work"`
	IfKept   repo.CarryPreview `json:"if_kept"` // what moves onto this side if it is kept
}

func (h *Handlers) compareVideoDuplicate(w http.ResponseWriter, r *http.Request) {
	a, ok := urlParamID(w, r, "a")
	if !ok {
		return
	}
	b, ok := urlParamID(w, r, "b")
	if !ok {
		return
	}
	ctx := r.Context()
	live, err := h.repo.VideoPairLive(ctx, a, b)
	if err != nil {
		h.fail(w, "compare video duplicate", err)
		return
	}
	if !live {
		writeError(w, http.StatusNotFound, "not a duplicate pair")
		return
	}
	facts, titles, err := h.videoPairFacts(ctx, []int64{a, b})
	if err != nil {
		h.fail(w, "compare video duplicate", err)
		return
	}
	parts, err := h.partsFor(ctx, []int64{a, b})
	if err != nil {
		h.log.Warn("compare video duplicate: parts", "err", err)
	}
	side := func(id, other int64) (videoCompareSide, error) {
		f := facts[id]
		f.Title = titles[id]
		s := videoCompareSide{
			VideoFileFacts: f,
			FileName:       filepath.Base(f.FilePath),
			Folder:         filepath.Dir(f.FilePath),
			Edition:        h.videoEdition(ctx, id),
			Part:           parts[id],
		}
		work, werr := h.repo.VideoWorkFor(ctx, id)
		if werr != nil {
			return s, werr
		}
		s.Work = work
		ifKept, perr := h.repo.PreviewCarry(ctx, id, other)
		s.IfKept = ifKept
		return s, perr
	}
	sa, err := side(a, b)
	if err != nil {
		h.fail(w, "compare video duplicate", err)
		return
	}
	sb, err := side(b, a)
	if err != nil {
		h.fail(w, "compare video duplicate", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"can_label_editions": h.labelSettable("edition"),
		"can_label_parts":    h.labelSettable("part"),
		"a":                  sa,
		"b":                  sb,
	})
}

// labelSettable reports whether this library's mapping declares fieldKey as a replace
// field — the same condition a manual decision on it needs (replaceField).
func (h *Handlers) labelSettable(fieldKey string) bool {
	if h.mappings == nil {
		return false
	}
	f, ok := h.mappings.Current().ByCanonical(fieldKey)
	return ok && !f.Multi && !f.Merge
}

func (h *Handlers) keepOneVideo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		KeepID  int64 `json:"keep_id"`
		TrashID int64 `json:"trash_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.KeepID <= 0 || body.TrashID <= 0 || body.KeepID == body.TrashID {
		writeError(w, http.StatusBadRequest, "keep_id and trash_id must be two distinct ids")
		return
	}
	carried, err := h.repo.KeepOneVideo(r.Context(), body.KeepID, body.TrashID)
	if errors.Is(err, repo.ErrVideoPairNotLive) {
		writeError(w, http.StatusConflict, "this pair has changed; reload")
		return
	}
	if err != nil {
		h.fail(w, "keep one video", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"carried": carried})
}

func (h *Handlers) keepBothVideos(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDA int64 `json:"id_a"`
		IDB int64 `json:"id_b"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.IDA <= 0 || body.IDB <= 0 || body.IDA == body.IDB {
		writeError(w, http.StatusBadRequest, "id_a and id_b must be two distinct ids")
		return
	}
	if err := h.repo.DismissVideoPair(r.Context(), body.IDA, body.IDB); err != nil {
		h.fail(w, "keep both videos", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type videoLabel struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
}

// labelVideoPair labels a pair as two editions or two parts and resolves it as keep
// both (spec P0-7/8). Editions: at least one label, a blank side is left untouched.
// Parts: both required, positive whole numbers, and different.
func (h *Handlers) labelVideoPair(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Field  string       `json:"field"`
		Labels []videoLabel `json:"labels"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Field != "edition" && body.Field != "part" {
		writeError(w, http.StatusBadRequest, "field must be edition or part")
		return
	}
	if len(body.Labels) != 2 || body.Labels[0].ID <= 0 || body.Labels[1].ID <= 0 ||
		body.Labels[0].ID == body.Labels[1].ID {
		writeError(w, http.StatusBadRequest, "labels must name two distinct videos")
		return
	}
	if !h.labelSettable(body.Field) {
		writeError(w, http.StatusConflict, body.Field+" is not a settable field on this library")
		return
	}
	ctx := r.Context()
	var labels [2]repo.VideoLabel
	for i, l := range body.Labels {
		labels[i] = repo.VideoLabel{ID: l.ID, Value: enrich.SanitizeValue(l.Value)}
	}
	switch body.Field {
	case "edition":
		if labels[0].Value == "" && labels[1].Value == "" {
			writeError(w, http.StatusBadRequest, "give at least one file an edition")
			return
		}
		// A blank side that currently shows an edition was cleared by the owner: store
		// it as cleared so neither the file tag nor the filename brings it back. A blank
		// side with no edition is left untouched.
		for i := range labels {
			if labels[i].Value == "" && h.videoEdition(ctx, labels[i].ID) != "" {
				labels[i].Clear = true
			}
		}
	case "part":
		na, errA := strconv.Atoi(labels[0].Value)
		nb, errB := strconv.Atoi(labels[1].Value)
		if errA != nil || errB != nil || na <= 0 || nb <= 0 {
			writeError(w, http.StatusBadRequest, "give both files a part number")
			return
		}
		if na == nb {
			writeError(w, http.StatusBadRequest, "the two files need different part numbers")
			return
		}
		labels[0].Value, labels[1].Value = strconv.Itoa(na), strconv.Itoa(nb)
	}
	err := h.repo.LabelVideoPair(ctx, body.Field, labels)
	if errors.Is(err, repo.ErrVideoPairNotLive) {
		writeError(w, http.StatusConflict, "this pair has changed; reload")
		return
	}
	if err != nil {
		h.fail(w, "label video pair", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
