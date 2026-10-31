# File rename

This doc owns how Holodex renames a media file in its own folder: the rename journal and per-video
name state, rename as a write-queue job, the crash ordering between the filesystem rename and the
row, how the scanner stays consistent while a rename is in flight, the no-overwrite rename primitive
and case-only renames, and where templates and the mode are stored. The behaviour the owner sees is
in the [file rename spec](../specs/file-rename.md) (F86). The queue itself is
[writeback.md](writeback.md); move recognition, which is this feature's crash net, is
[media-ingest.md](media-ingest.md#moves-recognised-by-a-sampled-content-hash-matched-after-the-walk);
held originals are [file-maintenance.md](file-maintenance.md); the bound on which names and paths a
rename may produce is [security-perimeter.md](security-perimeter.md#rename-names-metadata-becomes-a-path-component-only-through-one-builder).

## A separate topic from writeback and maintenance, sharing their queue

A rename moves no value into the file and rewrites no stream. It changes one directory entry and
then `videos.file_path`. It shares the durable queue and the one-running-job-per-file rule with tag
writes and re-encodes, and none of their file machinery: no temp copy, no tool invocation, no
read-back of tags.

**Rejected:** a rename "operation" inside `writeback.WriteBatch`. Every tag-write invariant (copy to
a temp, one tool invocation, snapshot before write) would need a rename exception, and the rename
would cost a full-file copy it doesn't need.

Decided in: pending squash (HOLODEX-602).

## The rename journal: `file_renames`, plus per-video name state in `video_file_names`

Both tables key on `videos.id` with `ON DELETE CASCADE`, so a purge takes them with the video.
Neither keys on a path, because a rename changes the path.

- **`file_renames`** is append-only, one row per rename (`id AUTOINCREMENT`): `video_id`,
  `old_name`, `new_name`, `via_name` (set while a two-step rename is half done), `template` (the
  template's text), `token_values` (JSON, token → the sanitised value written), `trigger`
  (`manual|writeback|job|undo`), `undo_of` (the row an undo reverses), `state ∈ pending|done`,
  `created_at`. Names are base names; the folder is the video's own and never changes. A `pending`
  row is written **before** the filesystem rename and is the crash journal (next section).
- **`video_file_names`** holds one row per video that has name state: `locked`, `lock_source ∈
  auto|owner|undo`, and the settle observation `desired_name`, `desired_since`.

What each consumer reads:
- **Holodex-written name and undo:** the latest `done` row. The file still carries the name Holodex
  wrote when its base name equals that row's `new_name`.
- **Echo suppression:** that row's `token_values` and `template`. The current name is read back
  with the template that **wrote** it, ahead of the current list, so editing the template list reads
  as drift and never as a hand edit.
- **Hand-edit lock:** a scan finds a base name different from the latest `done` row's `new_name`
  while no `pending` row exists, and sets `locked` with `lock_source='auto'`.
- **Settle window:** the job sweep compares each video's desired name with `desired_name` and resets
  `desired_since` when it changed. A name is settled when `desired_since` is older than the window.

`FoldMovedVideo` re-keys `file_renames` to the older id with the other per-video history, and the
newer row's `video_file_names` replaces the older one's, because the newer row describes the file
that is on disk.

**Rejected:** columns on `videos` (`last_written_name`, `name_locked`). They hold one rename, so
they can't give the history the spec requires, and per-token values don't fit columns. Lock and
settle state also don't belong in the row that `UpsertVideo` rewrites on every scan.
**Rejected:** reusing `file_writebacks` or `file_writeback_snapshots`. Those are per-field tag-value
ledgers whose revert is a forward tag write; a rename has no field and its undo is another rename.
**Rejected:** a settle clock derived from the inputs' timestamps. A desired name depends on file
tags, provider fetches, decisions, curation and the template list, and no single clock covers them.
Observing the name itself at each sweep is exact.

Decided in: pending squash (HOLODEX-602).

## Rename is a `kind='rename'` queue row, claimed by the tag-write lane

A rename is a `writeback_queue` row with `kind='rename'`. It inherits durability, `running` →
`pending` at boot, Retry and Dismiss, `job_runs` history (`kind=rename`), and the rule that job status
is a property of the video. The claim's existing clause, no pending row whose `video_id` has a
running row of any kind, keeps a rename from overlapping a writeback or a re-encode of the same file.

- **Lane.** The tag-write workers claim `kind IN ('write','rename')`. A rename takes milliseconds,
  so it doesn't need its own worker, and it must not wait behind an hour-long re-encode in the
  maintenance lane.
- **Payload.** The name, template and token values the plan showed. The payload is data: the worker
  re-runs the name builder and the collision checks, and fails the job if the video's desired name
  has changed since the plan was shown.
- **Batches.** An apply enqueues one row per video under a shared `batch_id`. A job whose target is
  held by a tracked video with a pending rename in the same batch goes back to `pending` rather than
  failing, with its `enqueued_at` reset so the claim order puts it behind that video's job. If that occupant is waiting for this job's file (a swap), this job first steps aside to
  its via name, so the cycle frees itself.
- **Held originals.** A rename is refused at enqueue and again whenever a rename step runs,
  including the on-writeback step inside a write job, while a `held_originals` row exists, because `held_path` is derived from `file_path` and a rename would break restore,
  discard and purge.
- **On writeback.** The rename step runs **inside** the write job, after `WriteBatch` and before
  `PostWriteFunc`, so the read-back re-extracts the file at its new name and nothing interleaves
  between the two steps. If the rename is refused, the write still succeeds and the name stays.

**Rejected:** a separate rename worker or table. It would duplicate recovery, retry, dismiss and
`job_runs` wiring, and the page would need two reads for "is anything happening to this file".
**Rejected:** renaming synchronously in the request. The rename wouldn't survive a restart, and it
could race a running write that is copying the old path to its temp.
**Rejected:** a follow-up rename row enqueued by the write job. Another job could run between the
write and the rename.

Decided in: pending squash (HOLODEX-602).

## Crash ordering: journal, then rename, then one transaction

1. Insert the `file_renames` row with `state='pending'`.
2. Rename the file with the no-overwrite primitive (two steps through `via_name` when needed).
3. In one transaction under `writeMu`: `UPDATE videos SET file_path = new WHERE id = ? AND file_path
   IN (old, new)`, set the row to `done`, and update `video_file_names`.

Step 3 is a narrow path update, not `RepointVideo`. A rename changes no bytes, so the file layer,
`content_fp`, `file_mtime` and `indexed_at` all stay as they are, and the next scan sees an unchanged
file at a known path. The condition accepts `new` because a scan may already have repointed the row.

- **Boot recovery** runs before the queue starts, beside `RecoverSwaps`, and replays each `pending`
  row. If the file is at `new_name`, it finishes step 3. If it is at `old_name`, it deletes the row
  and the requeued job starts again. If it is at `via_name`, it leaves the row for the requeued job to
  resume, or moves the file back to `old_name` when no job remains. If the file is at none of the
  names, or at both `old_name` and `new_name`, it deletes the row and fails the job, leaving the file to the next scan's move and missing-file handling.
- **The F79 net.** When recovery couldn't act (the journal was lost with a restored DB, or the mount
  wasn't ready), the scan's hold-then-match repoints the row by fingerprint. A repoint that lands a
  file on a pending row's `new_name` or `via_name` finishes that row instead of locking the video.

**Rejected:** rename first, then record. A stop between the two leaves no trace of intent, so the
next scan sees a changed name, takes it for a hand edit and locks the video.
**Rejected:** a `done` record written before the rename. A crash leaves a record of a rename that
never happened, and undo and echo suppression would read it.

Decided in: pending squash (HOLODEX-602).

## The scanner reads the journal; a rename never takes the scan lock

The scanner holds no lock against renames. It treats the journal as part of its state instead:

- A video with a `pending` `file_renames` row counts as **seen**, so `DeactivateExcept` never hides
  it while its file is between names, or at its via name.
- A walked path equal to a pending row's `new_name` in that folder belongs to that video. It is never
  held or inserted as a new video.
- The via suffix `.holodex-rename` is not a media extension, so the walk never ingests it. The
  queue's orphan sweep removes only `.holodex-tmp` and `.holodex-new` and must never touch it; a test
  pins both, as for `.holodex-orig`.
- The `fsnotify` Create event a rename raises only nudges a debounced full pass, which the rules
  above make harmless.
- A fold is already held back while either side has a pending or running queue row, and a rename is
  one.

**Rejected:** the rename holding `scanMu` around steps 2 and 3. The scanner would need no journal
awareness, but every rename in a batch would wait behind whatever pass is running, which can last
minutes while it extracts.

Decided in: pending squash (HOLODEX-602).

## No-overwrite rename: `RENAME_NOREPLACE` where the mount supports it, every name compared case-folded

Holodex runs in Docker over either a Linux mount or, on Docker Desktop, an NTFS folder reached
through a 9p/drvfs layer. The rename primitive is chosen per call:

- **Linux:** `renameat2(…, RENAME_NOREPLACE)` through `golang.org/x/sys/unix`. On ext4, xfs and
  btrfs this is atomic and fails with `EEXIST` rather than overwrite.
- **Mounts without it** (drvfs, many CIFS and NFS mounts) return `EINVAL` or `ENOSYS`. The rename
  then re-reads the folder listing, compares every entry case-folded with the target, and calls
  `os.Rename` immediately after. Holodex's own
  jobs on a file are serialised by the queue, so only an outside process creating that exact name in
  the gap could be overwritten.
- **Windows (development):** `MoveFileEx` without `MOVEFILE_REPLACE_EXISTING`, through
  `golang.org/x/sys/windows`.
- **Case and composition.** Every collision check compares names in NFC (`golang.org/x/text`) and
  case-folded, on every mount, against both tracked videos and the folder listing.
- **Case-only renames always take two steps**, `old` → `<old>.holodex-rename` → `new`, on every
  mount. On a case-insensitive filesystem the direct rename is either a no-op or a self-collision.
  `via_name` journals the half-done step.

**Rejected:** refusing renames on a mount without `RENAME_NOREPLACE`. That rules out Docker Desktop
on Windows entirely.
**Rejected:** `link` then `unlink` as the fallback. It is atomic where hard links work, but some NAS
shares don't support them, and a crash leaves two names for one file, which F79 would then see as
an ambiguous copy.
**Rejected:** probing each folder for case sensitivity. The same library can be reached through
both kinds of mount, and the spec's "one name valid everywhere" asks for the stricter answer.

Decided in: pending squash (HOLODEX-602).

## One grammar both ways: templates compile with `internal/extract`

A template is compiled by the same F48 pattern compiler that reads it back. It renders by filling
the tokens and is verified by parsing the result. Because the read-back is the identical compiled
regex, P0-3's refusal is a direct comparison, not a second implementation that could drift. Identity
markers are lifted from the stem in the same pass as the edition and part markers, and go to the
provider whose namespace they name as an identity hint
([metadata-providers.md](metadata-providers.md)). The ` [hx-<id>]` disambiguator is stripped first.

**Rejected:** rendering with the provider search-pattern renderer
([metadata-providers.md](metadata-providers.md)). Its optional-token grammar has no inverse, and
here "the first template that fits" already plays the role of optional tokens.

Decided in: pending squash (HOLODEX-602).

## The plan is computed on request; mode, settle window and templates are `settings` keys

- **The plan and drift queue** are computed per request from the resolver, the templates, the
  journal and `video_file_names`, like maintenance preflight. Nothing is materialised.
- **Settings.** The mode, the settle window and the template list (a JSON array in one row) are
  `settings` keys on the owner-settable allowlist, following the promotion pattern in
  [config-and-settings.md](config-and-settings.md#promoting-a-config-value-to-an-owner-editable-runtime-setting).
  A template list is validated by the compiler plus the rename rules before the row is written.
  Filename patterns stay in `metadata-patterns.yaml` until they move too. Read-back takes patterns
  from YAML and templates from the store, and each rename row carries its own template text, so the
  two sources never have to agree on history.

**Rejected:** a materialised plan table. It would need invalidating on every resolver input.
**Rejected:** templates in `metadata-patterns.yaml`. The owner tunes templates against the plan
preview in the app, and they are library-owned values that should survive a `/data` restore.

Decided in: pending squash (HOLODEX-602).
