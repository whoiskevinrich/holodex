package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Settings are library-owned values the owner sets from the UI (ADR-102 D2): they
// belong to the archive, travel with /data, and survive a restore — unlike deployment
// config, which stays in holodex.yaml/env. The store is a plain key/value table; the
// domain of each value is the writing handler's business, so a new key needs no
// migration.

// GetSetting returns the stored value for key and whether a row exists. A missing key
// is (“”, false, nil), not an error — callers fall back to their built-in default.
func (r *Repo) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// PutSetting upserts key=value, serialized under writeMu like every other write.
func (r *Repo) PutSetting(ctx context.Context, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("empty setting key")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value)
	return err
}
