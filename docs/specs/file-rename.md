# Spec: Rename media files from a template (F86)

**Status**: Draft
**Phase**: Library management (owner-only)
**Owner**: Project owner
**Date**: 2026-10-27
**Epic**: [HOLODEX-602](https://whoiskevinrich.atlassian.net/browse/HOLODEX-602). The brainstorm of
2026-10-27 is recorded in the epic description.

**Builds on**:
- [Keep a video's identity when its file moves](video-identity-on-move.md) (F79). Its non-goal
  "renaming or moving files from inside Holodex" is this feature. F79's move recognition is also the
  safety net that keeps a video's identity if Holodex stops partway through a rename (P0-9).
- [Filename extraction](metadata-extraction.md) (F48). Every rename template is also read back as a
  filename pattern (P0-11), using F48's token grammar, its `{edition-…}` and `{part-N}` markers and
  its review routing.
- [File maintenance](video-file-maintenance.md). Its held originals block a rename (P0-8).

**Architecture**: not written yet. The rename record, how a rename is queued and the path perimeter
are the architecture gate's to decide.
**Design**: not written yet. The plan, the drift review queue and the name lock are the design
gate's.

**Touches the filesystem with values that come from metadata. A `/security-review` sign-off is
required before merge** (label `needs-security-review`).

---

## Problem Statement

Holodex keeps rich, curated metadata about every video, but the files on disk keep whatever names
they arrived with. Those names are often junk from a download or a rip. They are useless to a person
browsing the share and to another media server reading the same folders, and they say nothing about
which video a file is if Holodex's library were lost. Today the owner renames files by hand outside
Holodex. That is slow, and until F79 it also cost the video its curation.

## Goals

1. **The owner can give every file a name built from its metadata**, from one ordered list of
   templates, without leaving Holodex.
2. **A rename never costs a video anything.** It keeps its identity, its place in "Date added" and
   "Updated", its curation, links, playlist places, provider data, thumbnail and montage.
3. **Nothing is ever overwritten or lost.** A rename that would clash with a file Holodex doesn't
   track is refused. Every rename is recorded and the latest one can be undone.
4. **The same name works on Windows and Linux.** One name, valid on both, everywhere.
5. **Names stay true without churn.** When metadata changes after a file is named, the owner sees
   the stale names and decides when they change. A name the owner fixed by hand is never fought.

## Non-Goals

- **Moving files between folders.** A rename stays in the file's own folder. Folder templates are
  deferred (P2).
- **A link or shortcut "view" tree** beside the real files. Rejected on 2026-10-27: the owner wants
  the real files renamed.
- **Renaming files that travel with a video**, such as subtitle `.srt` files. They keep their old
  names for now (P2).
- **Telling another media server about a rename.** It finds the file on its own next scan (P2).
- **Numbered duplicates such as `(2)`.** A number depends on the order of a run and changes between
  runs. A collision uses the video's own id instead (P0-6).
- **Renaming people's or studios' images**, or any file Holodex created itself.

## Concepts

- **Template.** A filename pattern with tokens, such as `[{studio}] {title} ({year})`. The owner
  keeps an **ordered list** of templates. A template names the extension-less part of the file; the
  extension never changes.
- **Desired name.** The name the first fitting template (P0-2) gives a video from its resolved
  values today.
- **Holodex-written name.** A name that Holodex itself gave the file, as recorded in the rename
  record (P0-10).
- **Out of date (drift).** A video whose current name is Holodex-written but no longer equals its
  desired name, because its metadata has changed since.
- **Name lock.** A per-video switch. A locked video is never renamed by any trigger.

## User Stories

- As the **owner**, I want to preview what every file would be called before anything changes, so
  that I can tune my templates without touching the disk.
- As the **owner**, I want to rename one video, a selection, or everything in the plan, so that I
  can adopt the feature gradually.
- As the **owner**, I want a provider id in the name, so that another media server matches the file
  and the file identifies itself even without Holodex's library.
- As the **owner**, when I correct a title, I want to see which file names are now stale and apply
  them when I choose, or have them follow along when Holodex next writes into the file.
- As the **owner**, I want newly imported junk names cleaned up on their own once their metadata has
  settled, if I turn that on.
- As the **owner**, when I rename a file by hand, I want Holodex to leave that name alone.
- As the **owner**, I want to undo a rename I didn't like.
- As the **owner**, when two videos would get the same name, I want both to be named, predictably,
  without one overwriting the other.
- As a **visitor**, I never see renaming controls, plans, drift or locks.

## Requirements

### Must-Have (P0)

**P0-1. Template tokens.** A template is literal text plus tokens. Every token is one the filename
grammar (F48) can read back, so a name Holodex writes is also a name Holodex can parse:

| Token | Renders | Read back as |
|---|---|---|
| `{title}`, `{studio}` | the resolved value | the same field |
| `{people}` | the cast, joined with the configured people delimiter | the cast |
| `{year}` | the year of the release date, four digits | the release year |
| `{edition}`, `{part}` | the F48 markers `{edition-…}` and `{part-N}` | the same field |
| `{provider-1-id}` (one per provider) | an identity marker, `{provider-1-<id>}` | an identity hint (P0-11) |

- A template with no `{title}` and no identity token is refused when saved, because it could name
  many unrelated videos the same.
- A template that repeats a token, or names a token that doesn't exist, is refused when saved, as
  F48 refuses such a pattern.
- An identity token renders only from an id the video's provider match has confirmed. A video
  without one doesn't fit that template.

**P0-2. The ordered template list.** For each video, the **first template whose every token has a
value** gives its desired name (decided 2026-10-27). A video no template fits is not renamed, and
the plan says it has no fitting template.
- Given templates `[{studio}] {title} ({year})`, `{title} ({year})` and `{title}`, a video with
  studio Acme, title Ada Lovelace and year 2023 is named `[Acme] Ada Lovelace (2023)`. With no
  studio it is named `Ada Lovelace (2023)`. With no studio and no year it is named `Ada Lovelace`.

**P0-3. A name must read back exactly.** Before a name is used, Holodex reads it back with the
template list (P0-11). If the read-back values differ from the values written, the video is
**refused**, keeps its current name, and the plan says why (decided 2026-10-27). This happens when a
value contains the literal text that follows its token, because a token's value ends at the first
occurrence of that text.
- Given template `{title} - {studio}`, a video titled `Ada - Redux` at studio Acme is refused: the
  name would read back as title `Ada` and studio `Redux - Acme`.
- Given template `[{studio}] {title}`, a studio whose name contains `]` is refused.
- An earlier template that would read a later template's name differently also refuses the name.
- A refused video does **not** fall through to the next template. The first fitting template is
  the video's template, so a quirk of spelling never silently changes which template names it.
- Identity, edition and part markers never cause a refusal, because braces are removed from values
  (P0-4).

**P0-4. One name that is valid everywhere.** Holodex builds the strictest name that is valid on both
Windows and Linux, on every platform:
- The characters `< > : " / \ | ? *`, control characters and braces `{ }` are removed from each
  value, and any run of whitespace this leaves becomes one space.
- A name never ends in a dot or a space, and is never a name Windows reserves (`CON`, `PRN`, `AUX`,
  `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, with or without an extension). Such a name is refused.
- The name is in Unicode composed form (NFC).
- The whole file name, extension and any disambiguator included, is at most **255 bytes**. A longer
  name is refused, not cut short.
- A rename that only changes letter case takes effect, including on a filesystem that ignores case.
- The values written, and the read-back check of P0-3, use the sanitised values.

**P0-5. A name stays inside the file's folder.** No value can make a name that leaves the video's
own folder or names a folder at all. `..`, `.`, path separators and an empty name are impossible
after P0-4, and any name that would still resolve outside the folder, or outside the library root,
is refused. Only the owner can rename.

**P0-6. Collisions get the video's id, never a number.** When a desired name is already in use in
the folder:
- **By a file Holodex doesn't track**, the rename is **refused**. That file is never overwritten.
- **By another video in the library**, a disambiguator ` [hx-<video id>]` is added before the
  extension, but only to the video that needs it. The video that already has the name keeps it. When
  several videos want a name no one has yet, the oldest (lowest id) takes it plain and the others
  get the disambiguator.
- A name that another video gives up in the same run counts as free.
- The disambiguator is the same on every run, so applying the same plan twice changes nothing.
- Given two parts of one film with the same title and no `{part}` token, one becomes
  `Ada Lovelace.mkv` and the other `Ada Lovelace [hx-412].mkv`.

**P0-7. Progressive triggers.** The owner chooses how far renaming goes. Each mode includes the ones
before it:

| Mode | What happens |
|---|---|
| **Off** | Nothing. The default. |
| **Plan** | The plan shows every video's current name, its desired name, or why it won't be renamed. No file is touched. |
| **Manual** | The owner applies the plan or the drift queue: one video, a selection, or all. |
| **On writeback** | When Holodex writes metadata into a file whose name is out of date (drift), the file also takes its desired name in the same step, unless it is locked or refused. A file Holodex has never named keeps its name; Manual or Job names it first. |
| **Job** | A sweep renames every unlocked video whose desired name has not changed for the **settle window** (default **7 days**, configurable). It covers new imports and drifted names. |

- Plan mode never changes anything on disk.
- A refused or locked video is never renamed by any mode.

**P0-8. A held original blocks a rename.** While a video has a held original from file maintenance,
it is not renamed by any trigger, and the plan says so (decided 2026-10-27). Once the hold ends it is
eligible again.

**P0-9. A rename keeps the video whole.** After a rename, the video is the same video under its new
name, with everything Goal 2 lists. A rename is **not an update**: its place in the "Updated" sorts
doesn't change.
- A rename never runs at the same time as a writeback or a maintenance job on the same file. A
  queued rename survives a restart.
- A scan that runs during a rename never adds the new name as a new video, and never hides the
  video as missing.
- If Holodex stops between renaming the file and recording it, the next scan recognises the file
  through F79's move recognition. The video keeps its identity, and the rename is then recorded.

**P0-10. Every rename is recorded.** For each rename, Holodex records the old name, the new name,
when it happened, which trigger did it, and the value written for each token. The record is kept for
the life of the video, because P0-11 and P0-12 depend on it.

**P0-11. Reading a Holodex-written name back.** For a video with a rename record, its name is read
back with the rename templates ahead of the owner's other filename patterns. For any other file, the
owner's filename patterns come first and the rename templates are tried only when none of them fits.
- **Echoes are ignored.** A parsed value equal to the value Holodex last wrote for that token is not
  a new filename value. The comparison is with the recorded value, **not** the current resolved
  value, so a drifted name is never mistaken for a human edit.
- A parsed value that differs from the recorded one is a new filename value and is routed exactly as
  F48 routes any filename value (auto-apply or review).
- **Identity tokens are always read**, echo or not, and are offered to providers as identity hints.
  Today Holodex reads one provider's id marker back from a path; this feature reads every identity
  token a template can write.
- The disambiguator ` [hx-<id>]` is removed before a name is parsed and never becomes a value.
- With no rename record (a new import, or a library rebuilt from scratch), parsing behaves exactly as
  it does today for every name an existing filename pattern fits. A name none of them fits is then
  tried against the rename templates, so a rebuilt library still reads its Holodex-written names.

**P0-12. The drift review queue.** The owner sees every out-of-date video with its current name and
its desired name, and can apply any of them (Manual mode and above). A drifted name is also fixed
for free on writeback (P0-7). Locked and refused videos don't appear as drift.

**P0-13. The name lock.** The owner can lock or unlock any video's name.
- **A hand edit locks automatically.** When a scan finds that a video's file name changed from a
  Holodex-written name to something else, the video is locked. Moving the file to another folder
  without changing its name is not a hand edit. Neither is a rename Holodex started and is completing
  after a stop (P0-9): a file that took the name Holodex was writing is not locked.
- A locked video keeps its name through every trigger until the owner unlocks it.

**P0-14. Undo.** The owner can undo a video's **latest** rename while the file still has the name
Holodex wrote (decided 2026-10-27). Undo restores the previous name, is itself recorded, and locks
the name so that drift doesn't redo it. If the previous name is now taken by any file, tracked or
not, undo is refused; it never adds a disambiguator.

**P0-15. The owner sees what a run did.** A manual apply and a job sweep each report in System
activity ([system-activity.md](system-activity.md)) how many videos were renamed, how many were
refused (by reason) and how many were skipped as locked or held.

### Nice-to-Have (P1)

**P1-1. Plan filters.** Filter the plan by reason: will rename, already named, no fitting template,
would not read back, collision with an untracked file, locked, held.

**P1-2. Per-video history.** A video shows its recorded names, oldest to newest.

### Future Considerations (P2)

- **Folder templates.** Moving files into folders by template. Moves are the riskier half (other
  media servers' libraries, empty folders left behind), so they wait.
- **Moving sidecar files with the video**, such as `.srt` subtitles that share its name.
- **Notifying a media server** after a rename.
- **Undoing a whole run** as one step.

## Edge Cases

- **A title such as `../etc` or `CON`.** Sanitised by P0-4 and checked by P0-5. It can never name a
  file outside its folder; a name that ends up reserved is refused.
- **Two videos swap names.** Each gives up the name the other wants, so under P0-6 both renames
  happen in one run and neither gets a disambiguator.
- **The owner changes the template list.** Every Holodex-written name whose desired name changes
  becomes drift. The job applies it only after the settle window; nothing renames immediately.
- **A name is already the desired name.** Nothing happens, and nothing is recorded.
- **A file was renamed outside Holodex to exactly its desired name.** Its read-back has no record to
  compare against, so it parses as today. It isn't locked, because its name was never
  Holodex-written.
- **A file is trashed.** A trashed video is never renamed.
- **The library is rebuilt from scratch.** Identity markers in the names still identify each file to
  providers (P0-11). Locks and records are gone; every name parses as a new import does.

## Success Metrics

- **Leading:** a plan over the whole production library refuses fewer than **2%** of videos for
  P0-3 (would not read back) with the owner's chosen templates. More than that means the template
  grammar needs a quoted token.
- **Leading:** applying a plan of N videos and rescanning produces **0** new arrivals in
  "Date added", **0** hidden-as-missing videos and **N** recorded renames.
- **Lagging:** no owner report of a lost file, an overwritten file, or a video that lost curation
  after a rename.

## Open Questions

- **[owner · blocks On writeback and Job only]** How does the other media server treat a renamed
  file: does it keep its watch state, or see a new item? Test by renaming one file by hand and
  rescanning it there. Plan and Manual don't depend on the answer.

## Timeline Considerations

- Ship the modes in order. Plan alone is useful and touches no file, so it can land first; Manual
  next; On writeback and Job after the open question is answered.
- Architecture, design, testing strategy and security review each run as their own gate after this
  spec.
