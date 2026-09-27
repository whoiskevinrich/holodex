package api

import (
	"context"
	"strconv"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// stampTagSetSync compares the genres write set (the "genres" row applyGenreWriteback
// left in resolved) with the file's recorded tag set (ADR-111): the row gets per-item
// on_file, file_only and a set-valued in_sync (D3), and each attached tag gets written
// and on_file (D4). Membership goes through repo.TagIdentityKeys — the rule the
// writer's extra-key filter uses (D2). A failure only costs the markers: the row keeps
// in_sync unknown, never "differs".
func (h *Handlers) stampTagSetSync(ctx context.Context, resolved []resolver.ResolvedField, field mapping.Field, v *model.Video) []resolver.ResolvedField {
	fileTags, known, err := h.repo.FileTags(ctx, v.ID)
	if err != nil {
		h.log.Warn("file tags for detail", "id", v.ID, "err", err)
		return resolved
	}
	row, _ := resolvedByCanonical(resolved, "genres")
	names := append([]string{}, fileTags...)
	for _, it := range row.Items {
		names = append(names, it.Value)
	}
	ident, err := h.repo.TagIdentityKeys(ctx, names)
	if err != nil {
		h.log.Warn("tag identities for detail", "id", v.ID, "err", err)
		return resolved
	}
	resolved = applyTagSetSync(resolved, field, fileTags, known, ident)
	row, _ = resolvedByCanonical(resolved, "genres")
	stampTagMembership(v.Tags, row.Items, fileTags, known, ident)
	return resolved
}

// applyTagSetSync is stampTagSetSync's pure half for the genres row (ADR-111 D3). ident
// maps repo.TagNameKey(name) to a tag identity for every item and file name. With the
// file's tags unknown it changes nothing (in_sync stays omitted). A file with tags but an
// empty write set still gets a row: that write deletes Genre (ADR-110 D6), and hiding
// the row would hide it.
func applyTagSetSync(resolved []resolver.ResolvedField, field mapping.Field, fileTags []string, known bool, ident map[string]string) []resolver.ResolvedField {
	if !known {
		return resolved
	}
	idx := -1
	for i := range resolved {
		if strings.EqualFold(resolved[i].Canonical, "genres") {
			idx = i
			break
		}
	}
	if idx < 0 {
		if len(fileTags) == 0 {
			return resolved
		}
		label, display := resolver.LabelAndDisplay(field)
		resolved = append(resolved, resolver.ResolvedField{
			Canonical: "genres", Label: label, Display: display,
			Values: []string{}, Multi: true, WinningSource: "tag:genres",
		})
		idx = len(resolved) - 1
	}
	rf := &resolved[idx]
	onFile := identitySet(fileTags, ident)
	written := make(map[string]bool, len(rf.Items))
	inSync := true
	for i := range rf.Items {
		id := ident[repo.TagNameKey(rf.Items[i].Value)]
		written[id] = true
		on := onFile[id]
		rf.Items[i].OnFile = &on
		inSync = inSync && on
	}
	rf.FileOnly = nil
	seen := map[string]bool{}
	for _, n := range fileTags {
		id := ident[repo.TagNameKey(n)]
		if id == "" || written[id] || seen[id] {
			continue
		}
		seen[id] = true
		rf.FileOnly = append(rf.FileOnly, n)
		inSync = false
	}
	rf.InSync = &inSync
	return resolved
}

// stampTagMembership sets each attached tag's written (in the write set) and on_file
// (in the file's recorded set; left nil while unknown) — ADR-111 D4. An attached tag's
// identity is its own id, the same "id:<n>" TagIdentityKeys resolves its name to.
func stampTagMembership(tags []model.Tag, items []resolver.ResolvedValue, fileTags []string, known bool, ident map[string]string) {
	written := make(map[string]bool, len(items))
	for _, it := range items {
		written[ident[repo.TagNameKey(it.Value)]] = true
	}
	onFile := identitySet(fileTags, ident)
	for i := range tags {
		id := "id:" + strconv.FormatInt(tags[i].ID, 10)
		w := written[id]
		tags[i].Written = &w
		if known {
			on := onFile[id]
			tags[i].OnFile = &on
		}
	}
}

func identitySet(names []string, ident map[string]string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		if id := ident[repo.TagNameKey(n)]; id != "" {
			out[id] = true
		}
	}
	return out
}
