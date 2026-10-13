-- HOLODEX-567 — "Date added" drift probe.
--
-- Read-only. Run against the LIVE library, not a dev database:
--
--     sqlite3 -readonly /path/to/holodex.db < scripts/detect_indexed_at_drift.sql
--
-- The Media page's "Date added" sort orders by videos.indexed_at, which UpsertVideo resets to
-- now on every write (changed-file rescan, manual refresh, writeback re-extract). This measures
-- how far that column has drifted from the order files actually arrived in, and compares the two
-- backfill candidates for a first_seen_at column.
--
-- The yardstick is videos.id. It is AUTOINCREMENT, the videos table has never been rebuilt
-- (0001 is its only CREATE), and UpsertVideo keeps the id on a same-path conflict — so id order
-- IS insertion order. That gives every row an upper bound on when it was first seen:
--
--     est_first_seen = MIN(indexed_at) over this row and every row inserted after it
--
-- A row cannot have arrived later than a row inserted after it was. A row whose own indexed_at
-- is later than that bound has been re-indexed since it arrived — "drifted".
--
-- Timestamps are RFC3339 UTC (repo.go timeLayout), so text comparison orders them correctly.
--
-- OUTPUT IS ANONYMIZED (scripts/CLAUDE.md): counts, buckets and aggregates only — no titles,
-- paths or ids. Safe to paste into a session, an issue or a PR as-is.
--
-- Verified 2026-10-12 against a database built by applying all 62 up migrations to an empty
-- file and seeded with eight videos: ids 1–3 one bulk scan of already-old files (no drift;
-- candidate A dates them by file_mtime, before the scan); ids 4–6 arrived together, then id 4
-- was refreshed 40 days later with no file change (drifted, 30–90d) and id 5 written back 3
-- days later (drifted, 1–7d, the one "modified after first seen"); id 6 untouched (no drift,
-- and 4–6 form §4's tie of 3); id 7 soft-deleted and re-indexed after id 8 arrived (drifted,
-- counted under all_rows but not visible); id 8 the newest arrival, untouched. Expected and
-- observed: §2 5/2/1 all-rows, §3 2 drifted of 7, §4 6 / 6 / 1, ties 5 / 3 / 1.
--
-- §3's "visible" is active and not soft-deleted. The Media page also hides full-film videos by
-- default, so its first 50 can differ slightly; the drift counts in §2 are unaffected.

.mode box
.headers on

DROP VIEW IF EXISTS temp.drift;
CREATE TEMP VIEW drift AS
WITH bounded AS (
    SELECT id,
           active,
           deleted_at IS NULL                                        AS live,
           indexed_at,
           file_mtime,
           MIN(indexed_at) OVER (ORDER BY id DESC
                                 ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS est_first_seen
    FROM videos
)
SELECT *,
       indexed_at > est_first_seen                                   AS drifted,
       julianday(indexed_at) - julianday(est_first_seen)             AS lag_days,
       MIN(indexed_at, file_mtime)                                   AS cand_a
FROM bounded;

-- ── §1 Population ───────────────────────────────────────────────────────────────────────
SELECT COUNT(*)                                          AS videos,
       SUM(active = 1 AND live)                          AS visible,
       SUM(active = 0)                                   AS inactive,
       SUM(NOT live)                                     AS soft_deleted
FROM drift;

-- ── §2 How many rows have been re-indexed since they arrived ────────────────────────────
-- `visible` = what the Media page lists (active, not soft-deleted).
SELECT CASE
         WHEN NOT drifted         THEN '0 · not drifted'
         WHEN lag_days < 1        THEN '1 · < 1 day'
         WHEN lag_days < 7        THEN '2 · 1–7 days'
         WHEN lag_days < 30       THEN '3 · 7–30 days'
         WHEN lag_days < 90       THEN '4 · 30–90 days'
         ELSE                          '5 · ≥ 90 days'
       END                                               AS lag_since_first_seen,
       COUNT(*)                                          AS all_rows,
       SUM(active = 1 AND live)                          AS visible
FROM drift
GROUP BY 1
ORDER BY 1;

-- ── §3 What the default sort shows today ────────────────────────────────────────────────
-- The first 50 visible rows by indexed_at DESC (today's "Date added — newest"): how many are
-- drifted, and how many would still be in the first 50 if the sort used insertion order.
WITH visible AS (SELECT * FROM drift WHERE active = 1 AND live),
     by_indexed AS (SELECT id, drifted FROM visible ORDER BY indexed_at DESC, id DESC LIMIT 50),
     by_id      AS (SELECT id FROM visible ORDER BY id DESC LIMIT 50)
SELECT COUNT(*)                                          AS top_50_today,
       SUM(drifted)                                      AS drifted_in_top_50,
       SUM(id IN (SELECT id FROM by_id))                 AS also_top_50_by_insertion
FROM by_indexed;

-- ── §4 Backfill candidates ──────────────────────────────────────────────────────────────
-- A = MIN(indexed_at, file_mtime): the ticket's lean. For a file that was already old when it
--     was imported, file_mtime predates the scan, so A dates it by the FILE, not the import.
-- B = est_first_seen: monotone in id, so it never reorders arrivals; bulk scans tie.
SELECT SUM(cand_a < est_first_seen)                      AS a_earlier_than_import,
       SUM(julianday(est_first_seen) - julianday(cand_a) > 30)
                                                         AS a_earlier_by_30d_plus,
       SUM(file_mtime > est_first_seen)                  AS modified_after_first_seen
FROM drift
WHERE active = 1 AND live;

-- How much B collapses onto shared timestamps (bulk scans). id DESC breaks these ties.
SELECT COUNT(*)                                          AS distinct_b_values,
       MAX(n)                                            AS largest_tie,
       SUM(n > 1)                                        AS tied_groups
FROM (SELECT est_first_seen, COUNT(*) AS n
      FROM drift WHERE active = 1 AND live
      GROUP BY est_first_seen);
