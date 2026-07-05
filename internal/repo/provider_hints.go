package repo

import (
	"context"
	"fmt"
	"time"
)

// ProviderFieldHint is one persisted per-field render hint for a provider's
// non-canonical field (F39, ADR-056). Label/Render/Group are already sanitized and
// normalized to the internal/registry vocabulary before they reach here.
type ProviderFieldHint struct {
	FieldKey string
	Label    string
	Render   string
	Group    string
	Order    int
}

// ReplaceProviderFieldHints replaces the stored hints for one provider with the
// supplied set, in a single transaction under the write lock (F39). A provider's
// /describe is the source of truth for its hints, so a refresh is delete-then-insert:
// hints the provider no longer advertises are dropped. An empty set clears the
// provider's rows (a provider that stopped advertising hints leaves none behind).
func (r *Repo) ReplaceProviderFieldHints(ctx context.Context, provider string, hints []ProviderFieldHint) error {
	if provider == "" {
		return nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_field_hints WHERE provider = ?`, provider); err != nil {
		return fmt.Errorf("clear provider field hints: %w", err)
	}
	now := time.Now().UTC().Format(timeLayout)
	for _, h := range hints {
		if h.FieldKey == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO provider_field_hints (provider, field_key, label, render, hint_group, ord, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			provider, h.FieldKey, h.Label, h.Render, h.Group, h.Order, now); err != nil {
			return fmt.Errorf("insert provider field hint: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider field hints: %w", err)
	}
	return nil
}

// ProviderFieldHints returns every stored hint, keyed by provider then field key —
// the read-path lookup the detail handlers consult to render auto-registered
// non-canonical fields without contacting a provider (F39). The table is tiny (a
// few keys per provider), so one query per detail request is cheap.
func (r *Repo) ProviderFieldHints(ctx context.Context) (map[string]map[string]ProviderFieldHint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT provider, field_key, label, render, hint_group, ord
		FROM provider_field_hints`)
	if err != nil {
		return nil, fmt.Errorf("provider field hints: %w", err)
	}
	defer rows.Close()

	out := map[string]map[string]ProviderFieldHint{}
	for rows.Next() {
		var provider string
		var h ProviderFieldHint
		if err := rows.Scan(&provider, &h.FieldKey, &h.Label, &h.Render, &h.Group, &h.Order); err != nil {
			return nil, err
		}
		byKey := out[provider]
		if byKey == nil {
			byKey = map[string]ProviderFieldHint{}
			out[provider] = byKey
		}
		byKey[h.FieldKey] = h
	}
	return out, rows.Err()
}
