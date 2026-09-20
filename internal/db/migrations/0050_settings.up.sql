-- Library-owned settings (ADR-102 D2): values the owner sets from the UI that belong to
-- the archive rather than the deployment — they travel with /data and survive a restore.
-- One row per key; the value's domain is validated by the handler that writes it, not
-- here, so the key set grows without a migration. v1 stores exactly one key,
-- 'theme.active' (F67, the instance skin).
CREATE TABLE settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
