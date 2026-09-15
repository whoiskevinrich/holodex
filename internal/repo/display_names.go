package repo

import (
	"context"
	"fmt"
	"strings"

	"holodex/internal/fieldsource"
)

// DisplayNames returns, for every entity of one kind whose standing decision on
// `name` selects a spelling other than the canonical column (F60 RD9, HOLODEX-378),
// that spelling keyed by entity id: the frozen manual literal, or the decided
// provider's stored spelling under providerKey (`name` for person/studio; `title`
// for film — the sidecar's film title key, ADR-086 §3).
//
// This is a deliberately narrow SQL mirror of resolver.resolveDecided for one
// single-value replace field, because search has to match the display spelling
// per keystroke and cannot resolve every entity to do it. It agrees with the
// resolver on the two cases that matter: a `file` decision resolves to canonical
// (omitted here), and a decided provider with no stored spelling drops the resolved
// field so the page falls back to canonical (omitted here too). The API-level
// search test pins the mirror to the resolved payload.
func (r *Repo) DisplayNames(ctx context.Context, entityType, providerKey string) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.entity_id,
		       CASE WHEN d.source = ? THEN d.manual_value ELSE COALESCE(e.value, '') END
		FROM field_source_decisions d
		LEFT JOIN entity_enrichment e
		       ON e.entity_type = d.entity_type AND e.entity_id = d.entity_id
		      AND e.field_key = ? AND d.source = ? || e.provider
		WHERE d.entity_type = ? AND d.field_key = 'name' AND d.source != ?`,
		fieldsource.Manual, providerKey, fieldsource.ForProvider(""), entityType, fieldsource.File)
	if err != nil {
		return nil, fmt.Errorf("display names %s: %w", entityType, err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var v string
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		if v = strings.TrimSpace(v); v != "" {
			out[id] = v
		}
	}
	return out, rows.Err()
}

// matchesDisplayQuery is the display-name analogue of ftsPrefixQuery: every
// whitespace token of the query must appear in the spelling, case-folded. A
// substring match (not FTS prefix) because the candidate set is the handful of
// entities with a name decision, not an index.
func matchesDisplayQuery(spelling, query string) bool {
	s := strings.ToLower(spelling)
	for _, tok := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(s, tok) {
			return false
		}
	}
	return true
}
