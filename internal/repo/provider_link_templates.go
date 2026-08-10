package repo

import (
	"context"
	"fmt"
	"time"
)

// ProviderLinkTemplate is one persisted provider-advertised outbound-link template
// (HOLODEX-266, ADR-083 D2): a namespace + entity kind resolve to a URL template
// containing exactly one "{id}" placeholder. Already validated
// (enrich.ValidateLinkTemplate) before it reaches here.
type ProviderLinkTemplate struct {
	Namespace  string
	EntityType string
	Template   string
}

// ReplaceProviderLinkTemplates replaces provider's stored link templates with the
// supplied set (HOLODEX-266, ADR-083 D2). Unlike ReplaceProviderFieldHints, the
// table's primary key is (namespace, entity_type) rather than (provider, ...): a
// namespace is a shared identity space across providers (ADR-055 D2), so the row for
// a namespace is owned by whichever provider most recently advertised a template for
// it, not scoped per-provider. The delete only clears rows THIS provider previously
// contributed; the inserts then take over any (namespace, entity_type) row — whether
// previously held by this provider or another — via INSERT OR REPLACE.
func (r *Repo) ReplaceProviderLinkTemplates(ctx context.Context, provider string, templates []ProviderLinkTemplate) error {
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

	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_link_templates WHERE provider = ?`, provider); err != nil {
		return fmt.Errorf("clear provider link templates: %w", err)
	}
	now := time.Now().UTC().Format(timeLayout)
	for _, t := range templates {
		if t.Namespace == "" || t.EntityType == "" || t.Template == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO provider_link_templates (namespace, entity_type, provider, template, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			t.Namespace, t.EntityType, provider, t.Template, now); err != nil {
			return fmt.Errorf("insert provider link template: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider link templates: %w", err)
	}
	return nil
}

// ProviderLinkTemplates returns every stored link template, keyed by namespace then
// entity type (HOLODEX-266, ADR-083 D2) — the read-path lookup the Service's
// in-memory cache loads to build outbound badge links without contacting a provider.
func (r *Repo) ProviderLinkTemplates(ctx context.Context) (map[string]map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT namespace, entity_type, template
		FROM provider_link_templates`)
	if err != nil {
		return nil, fmt.Errorf("provider link templates: %w", err)
	}
	defer rows.Close()

	out := map[string]map[string]string{}
	for rows.Next() {
		var namespace, entityType, template string
		if err := rows.Scan(&namespace, &entityType, &template); err != nil {
			return nil, err
		}
		byKind := out[namespace]
		if byKind == nil {
			byKind = map[string]string{}
			out[namespace] = byKind
		}
		byKind[entityType] = template
	}
	return out, rows.Err()
}
