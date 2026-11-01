# Jargon

Plain-language meanings for the internal terms that come up when talking about how Holodex works.
Each entry says what the term means first, then why it matters, then where the mechanism lives.

Each term links to the doc that owns the mechanism. If an entry here and that doc ever disagree,
the doc wins — fix the entry. UI words for owner/visitor rendering live in
[ui-vocabulary.md](ui-vocabulary.md), not here.

## Fold ("the fold")

**What it means.** Merging two video rows that turn out to be the same file back into one video.
The older row survives with its id; the newer row's history is moved onto it, and the newer row is
deleted. People say a fold *heals* a move: the video's identity had split in two, and the fold
joins it back up.

**Why it happens.** When you move or rename a file outside Holodex, the scan normally spots it
straight away — a file at a new path with the same content fingerprint (size plus samples from the
start, middle and end) as a video whose file has gone — and simply updates that video's path. The
fold is the fallback for when the scan *couldn't* tell in time, usually because both copies existed
during one scan: a copy that is deleted later, a sync tool that writes the new file before removing
the old one, or a scan that ran mid-move. The new path then became a brand-new video. Once the old
file is gone, a later check pairs the two rows by fingerprint and folds them.

**Example.**
1. Video #42 is `Ada Lovelace (2023).mkv`.
2. A sync tool copies it to a new folder; a scan runs while both copies exist, so the copy becomes
   video #913.
3. The tool deletes the original. The next scan sees #42's file is missing, pairs it with #913 by
   fingerprint, and folds #913 into #42. Decisions, tags, playlists and thumbnails stay on #42, and
   #913 no longer exists.

**Not a fold:** the ordinary case where the scan matches a moved file in the same pass and updates
the existing row's path (a *repoint*). No second row is ever created.

→ [media-ingest.md](../architecture/media-ingest.md) § the fold (`FoldMovedVideo`);
[file-rename.md](../architecture/file-rename.md) for what a fold does to file-name state.
