package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Studio logos (HOLODEX-130, ADR-056). One self-hosted, normalized logo per studio —
// a derived cache of the studio's RESOLVED `logo` field. The bytes live on disk
// (studioimage); this table is the metadata index the serve route and list attach
// read from. RelinkStudioLogo (api) is the sole writer via ReplaceStudioLogo /
// DeleteStudioLogo; writes take writeMu like the rest of the write path, reads are
// lock-free (WAL).

// StudioLogo is one stored logo (the on-disk bytes are at
// studioimage.ImagePath(dir, StudioID, ID)). ID doubles as the ?v= cache-buster: a
// refresh is delete + insert, so a replaced logo gets a new id and the browser
// re-fetches past the immutable cache (ADR-056 §2).
type StudioLogo struct {
	ID        int64
	StudioID  int64
	SourceURL string // the resolved logo URL this cache was derived from (idempotency key)
	Provider  string
	Width     int
	Height    int
	ByteSize  int
	CreatedAt time.Time
}

// StudioLogoInsert is the payload for ReplaceStudioLogo.
type StudioLogoInsert struct {
	StudioID  int64
	SourceURL string
	Provider  string
	Width     int
	Height    int
	ByteSize  int
}

// GetStudioLogo returns the studio's logo index row, or ErrNotFound when none is
// cached (the common case before enrichment, or after a blank-pin).
func (r *Repo) GetStudioLogo(ctx context.Context, studioID int64) (StudioLogo, error) {
	var (
		sl      StudioLogo
		created string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, studio_id, source_url, provider, width, height, byte_size, created_at
		FROM studio_logos WHERE studio_id = ?`, studioID).
		Scan(&sl.ID, &sl.StudioID, &sl.SourceURL, &sl.Provider, &sl.Width, &sl.Height, &sl.ByteSize, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return StudioLogo{}, ErrNotFound
	}
	if err != nil {
		return StudioLogo{}, fmt.Errorf("get studio logo: %w", err)
	}
	sl.CreatedAt, _ = time.Parse(timeLayout, created)
	return sl, nil
}

// ReplaceStudioLogo makes the studio's single logo row be exactly `in` and returns
// the new server-assigned id (the on-disk filename + ?v= stamp). Delete + insert in
// one transaction so the UNIQUE(studio_id) single-slot invariant holds and the id
// always advances on a refresh. The caller stores the bytes at the returned id and
// removes any superseded file (it holds the prior id from GetStudioLogo).
func (r *Repo) ReplaceStudioLogo(ctx context.Context, in StudioLogoInsert) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM studio_logos WHERE studio_id = ?`, in.StudioID); err != nil {
		return 0, fmt.Errorf("clear studio logo: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO studio_logos (studio_id, source_url, provider, width, height, byte_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.StudioID, in.SourceURL, in.Provider, in.Width, in.Height, in.ByteSize,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("insert studio logo: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit studio logo: %w", err)
	}
	return id, nil
}

// DeleteStudioLogo removes the studio's logo index row (the caller removes the file).
// Idempotent — deleting an absent logo is a no-op success.
func (r *Repo) DeleteStudioLogo(ctx context.Context, studioID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx, `DELETE FROM studio_logos WHERE studio_id = ?`, studioID); err != nil {
		return fmt.Errorf("delete studio logo: %w", err)
	}
	return nil
}

// StudioLogoCount returns the number of cached studio logos — the fast-path gate for
// the one-time boot backfill (ADR-056): once any logo exists the backfill is skipped.
func (r *Repo) StudioLogoCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM studio_logos`).Scan(&n)
	return n, err
}
