---
paths:
  - "internal/db/migrations/**"
---

# Database migrations

- **Append-only with a manual down.** Add the next sequential
  `internal/db/migrations/NNNN_name.up.sql` **and** a matching `.down.sql` (golang-migrate; no
  auto-rollback). Never edit a shipped migration — add a new one.