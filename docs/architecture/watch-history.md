# Watch history

How the owner's plays are stored and queried (F89 P0-7 to P0-9). The perimeter rules are SEC-9 and
SEC-10 in [security-perimeter.md](security-perimeter.md); what counts as a play is the spec's.

## Plays are rows, with a per-video summary written in the same transaction

`video_plays(id, video_id, played_at, progress)` holds one row per counted play, and
`video_watch(video_id, play_count, last_played_at, manual_watched, manual_at)` holds one summary row
per played or marked video. Both reference `videos(id) ON DELETE CASCADE`, so a purge is their only
delete path (SEC-10). A healed move folds the newer video row into the older, and carries its rows
and summary across first. `video_plays` is indexed on `(video_id, played_at)`.

- **One write path.** An owner-only route records a play (SEC-9). The client posts once, when a
  play crosses the counting point, never as a heartbeat. The handler inserts the row and upserts the
  summary in one transaction under `writeMu`, so the two cannot disagree after a crash. The manual
  watched mark writes only `manual_watched` and `manual_at`, and never adds or removes a play row.
- **Reads.** `watched:`, `plays:` and `played:` compile to a lookup on `video_watch` by primary key,
  and a video with no row reads as unwatched with no plays. A condition that needs a time window over plays reads `video_plays` through its
  index. Both are owner-only at compile time (SEC-4).
- **Rebuildable.** The summary can be recomputed from the rows (`play_count`, `last_played_at`),
  and a test asserts that the recorded and rebuilt summaries match. That makes drift detectable and
  repairable.

**Rejected:** counters only (one row per video). It's the cheapest to query, but it keeps no
history, so no time window can be answered, and it can't hold the play row SEC-10 describes.
**Rejected:** recent rows plus folded totals, pruned after a window. The totals stay exact, but
older detail is lost, which RD8's lifetime retention rules out. The volume doesn't justify it
either: one row per counted play stays small for SQLite. **Rejected:** rows only, with every
`watched:` or `plays:` condition aggregating at query time. It works, but every playlist read pays
a `GROUP BY`, which a single summary row avoids. **Rejected:** storing history in the browser. A
smart playlist is evaluated on the server, so the server must be able to read the history.

Decided in: pending squash (HOLODEX-619).
