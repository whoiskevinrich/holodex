package api

import (
	"context"
	"slices"

	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/resolver"
)

// collapsePersonAliases dedups each person-typed row (registry.EntityKindPerson) by
// person identity, not spelling (HOLODEX-554): the resolver's merge keys values by
// normalized spelling, so a file credit that is an alias of a person ("Johnny D")
// survived beside that person's canonical name ("John Doe") and both reached the
// writeback dialog. A value naming a person (canonical name, then alias —
// LookupEntityIDByName) folds into the first value naming the same person, and any
// value naming a person linked to the video shows their canonical name — the spelling
// the rename/merge propagation writes too. The person analog of genreWritebackItemsFrom's
// tag-identity dedup; like it, API-layer only, since the resolver stays entity-agnostic.
func (h *Handlers) collapsePersonAliases(ctx context.Context, videoID int64, resolved []resolver.ResolvedField) error {
	var linked map[int64]string // person id → canonical name; loaded on first person row
	for i := range resolved {
		rf := &resolved[i]
		if rf.EntityKind != registry.EntityKindPerson || len(rf.Items) == 0 {
			continue
		}
		if linked == nil {
			people, err := h.repo.PeopleForVideos(ctx, []int64{videoID})
			if err != nil {
				return err
			}
			linked = make(map[int64]string, len(people[videoID]))
			for _, p := range people[videoID] {
				linked[p.ID] = p.Name
			}
		}
		at := map[int64]int{} // person id → index of its kept item
		items := make([]resolver.ResolvedValue, 0, len(rf.Items))
		changed := false
		for _, it := range rf.Items {
			id, ok, err := h.repo.LookupEntityIDByName(ctx, model.EnrichEntityPerson, it.Value)
			if err != nil {
				return err
			}
			if !ok {
				items = append(items, it)
				continue
			}
			if name, isLinked := linked[id]; isLinked && name != it.Value {
				it.Value, changed = name, true
			}
			j, seen := at[id]
			if !seen {
				at[id] = len(items)
				items = append(items, it)
				continue
			}
			changed = true
			kept := &items[j]
			for _, s := range it.Sources {
				if !slices.Contains(kept.Sources, s) {
					kept.Sources = append(kept.Sources, s)
				}
			}
			kept.Manual = kept.Manual || it.Manual
			// An owner's no-write on any spelling holds for the person.
			kept.NoWrite = kept.NoWrite || it.NoWrite
		}
		if !changed {
			continue
		}
		rf.Items = items
		rf.Values = make([]string, len(items))
		for k, it := range items {
			rf.Values[k] = it.Value
		}
	}
	return nil
}
