package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Provider icons (HOLODEX-134, ADR-059). One self-hosted, normalized brand icon per
// metadata provider — a cache of the URL the provider advertises in its /describe
// `brand_icon`. The bytes live on disk (providericon); this table is the metadata index
// the serve route and the /providers directory read from. RelinkProviderIcon (api) is
// the sole writer via ReplaceProviderIcon / DeleteProviderIcon; writes take writeMu like
// the rest of the write path, reads are lock-free (WAL).
//
// Unlike studio_logos this is keyed by the provider NAME (a metadata-sources.yaml
// registry id), not a foreign key — providers are not a DB table.

// ProviderIcon is one stored provider brand icon (the on-disk bytes are at
// providericon.ImagePath(dir, ID)). ID doubles as the ?v= cache-buster: a refresh is
// delete + insert, so a replaced icon gets a new id and the browser re-fetches past the
// immutable cache (ADR-059 §2).
type ProviderIcon struct {
	ID        int64
	Provider  string
	SourceURL string // the advertised brand_icon URL this cache was derived from (idempotency key)
	Width     int
	Height    int
	ByteSize  int
	CreatedAt time.Time
}

// ProviderIconInsert is the payload for ReplaceProviderIcon.
type ProviderIconInsert struct {
	Provider  string
	SourceURL string
	Width     int
	Height    int
	ByteSize  int
}

// GetProviderIcon returns the provider's icon index row, or ErrNotFound when none is
// cached (the common case for a provider that advertises no brand_icon).
func (r *Repo) GetProviderIcon(ctx context.Context, provider string) (ProviderIcon, error) {
	var (
		pi      ProviderIcon
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, provider, source_url, width, height, byte_size, created_at
		FROM provider_icons WHERE provider = ?`, provider).
		Scan(&pi.ID, &pi.Provider, &pi.SourceURL, &pi.Width, &pi.Height, &pi.ByteSize, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return ProviderIcon{}, ErrNotFound
	}
	if err != nil {
		return ProviderIcon{}, fmt.Errorf("get provider icon: %w", err)
	}
	pi.CreatedAt, _ = time.Parse(timeLayout, created)
	return pi, nil
}

// ListProviderIcons returns every cached icon index row — the source for the
// /providers directory's icon-URL attach and the boot orphan reconcile.
func (r *Repo) ListProviderIcons(ctx context.Context) ([]ProviderIcon, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, provider, source_url, width, height, byte_size, created_at
		FROM provider_icons ORDER BY provider`)
	if err != nil {
		return nil, fmt.Errorf("list provider icons: %w", err)
	}
	defer rows.Close()
	var out []ProviderIcon
	for rows.Next() {
		var (
			pi      ProviderIcon
			created string
		)
		if err := rows.Scan(&pi.ID, &pi.Provider, &pi.SourceURL, &pi.Width, &pi.Height, &pi.ByteSize, &created); err != nil {
			return nil, fmt.Errorf("scan provider icon: %w", err)
		}
		pi.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, pi)
	}
	return out, rows.Err()
}

// ReplaceProviderIcon makes the provider's single icon row be exactly `in` and returns
// the new server-assigned id (the on-disk filename + ?v= stamp). Delete + insert in one
// transaction so the UNIQUE(provider) single-slot invariant holds and the id always
// advances on a refresh. The caller stores the bytes at the returned id and removes any
// superseded file (it holds the prior id from GetProviderIcon).
func (r *Repo) ReplaceProviderIcon(ctx context.Context, in ProviderIconInsert) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_icons WHERE provider = ?`, in.Provider); err != nil {
		return 0, fmt.Errorf("clear provider icon: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO provider_icons (provider, source_url, width, height, byte_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		in.Provider, in.SourceURL, in.Width, in.Height, in.ByteSize,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("insert provider icon: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit provider icon: %w", err)
	}
	return id, nil
}

// DeleteProviderIcon removes the provider's icon index row (the caller removes the
// file). Idempotent — deleting an absent icon is a no-op success.
func (r *Repo) DeleteProviderIcon(ctx context.Context, provider string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx, `DELETE FROM provider_icons WHERE provider = ?`, provider); err != nil {
		return fmt.Errorf("delete provider icon: %w", err)
	}
	return nil
}
