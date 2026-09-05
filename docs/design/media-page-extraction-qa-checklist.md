# Manual QA Checklist: Extract from filename on the media detail page (F48.5a)

**Handoff**: [media-page-extraction-handoff.md](media-page-extraction-handoff.md)
**Spec**: [metadata-extraction.md](../specs/metadata-extraction.md) §F48.5a, §F48.6i—§F48.6l
**Jira**: [HOLODEX-194](https://whoiskevinrich.atlassian.net/browse/HOLODEX-194)

Sections are grouped by who verifies them. `[smoke]` = quick pass anyone can run in a minute.
`[agent]` = needs tooling (DB inspection, computed styles, network). `[human]` = needs a person's
judgement about how it looks and reads.

---

## 1. Setup

| # | Step |
|---|---|
| 1.1 | Start the backend against the films testbed and the frontend dev server (`make run`, `cd web && npm run dev`). |
| 1.2 | Sign in as the owner and turn **Admin mode** on — every control below is invisible without it. |
| 1.3 | Confirm `metadata-patterns.yaml` has at least one pattern that matches a file in your library, and note a video whose filename encodes Studio, Title, People, and Year. |
| 1.4 | Confirm extraction auto-apply is **off** (the default). These cases assume it is off; 3.6 covers on. |
| 1.5 | Note the pending row count on `/owner/extraction` before you start — several cases compare against it. |

## 2. Smoke

| # | Check | Expect |
|---|---|---|
| 2.1 | `[smoke]` Open a media detail page as the owner. | An **Extract from filename** button sits in the Metadata actions row, between Refresh and the provider chips. |
| 2.2 | `[smoke]` Click it. | The button shows an in-flight label, then a panel headed **From filename · N to review** appears below the actions row. |
| 2.3 | `[smoke]` Read the panel header. | The source filename is shown beneath the heading in a monospace face. |
| 2.4 | `[smoke]` Stage one field, then click **Review and write N**. | The preview dialog opens showing an old → new diff for exactly the staged fields. |
| 2.5 | `[smoke]` Confirm the write. | The dialog reports success per row, the panel loses the resolved row, and the Metadata list below shows the new value. |
| 2.6 | `[smoke]` Click **Dismiss** on a row. | The row disappears immediately, with no preview dialog and no file write. |
| 2.7 | `[smoke]` Open a video whose filename matches no pattern and click Extract. | The "no pattern matched" line appears. No empty panel, no error styling, no stack trace. |

## 3. Agent

| # | Check | Expect |
|---|---|---|
| 3.1 | `[agent]` Watch the network tab during 2.2. | Exactly one `POST /media/{id}/extract`, then one `GET /owner/extraction-queue?video_id={id}`. The queue request carries the filter — it must not fetch the whole library. |
| 3.2 | `[agent]` Query `metadata_extraction_review` for this video after 2.2. | One pending row per queued field; no duplicates for a `(video_id, field_key)` pair. |
| 3.3 | `[agent]` Click **Re-extract**, then re-query. | Row count for this video is unchanged and `id`s are stable — the partial unique index updated in place rather than inserting. |
| 3.4 | `[agent]` Resolve a row on the media page, then load `/owner/extraction`. | That row is gone from the owner tab too. Both surfaces call the same resolve endpoint. |
| 3.5 | `[agent]` Resolve a row that names a not-yet-existing person, then query the people table. | The person now exists and is linked (ADR-068 post-write re-extract), same as resolving on the owner tab. |
| 3.6 | `[agent]` Turn auto-apply on, re-extract a video with a high-confidence match. | Fields land as `auto_applied`; the panel renders the "nothing needs review" state rather than an empty panel. |
| 3.7 | `[agent]` Load the page as a visitor (Admin mode off, and again signed out). | No extract button, no panel, and neither appears anywhere in the DOM. Not merely hidden by CSS. |
| 3.8 | `[agent]` Call `GET /owner/extraction-queue?video_id=...` without an owner session. | Rejected by `requireOwner`, same as the unfiltered route. |
| 3.9 | `[agent]` Grep the diff for hex colours and raw Tailwind palette classes in the changed Svelte files. | None. Semantic tokens only (ADR-021). |
| 3.10 | `[agent]` Tab through the panel; check computed contrast of the chip outlines and the staged pill against their backgrounds in all three skins. | Focus reaches every row control; contrast holds in Cinémathèque, Broadcast, and Brutalist. |

## 4. Human

Start each of these on a media detail page for a video whose filename encodes several fields —
open the library, pick that video, and make sure Admin mode is on (the toggle in the top bar).
Everything below happens in the **Metadata** block partway down that page.

| # | Check | What looks right |
|---|---|---|
| 4.1 | `[human]` Look at the row of small controls to the right of the word "Metadata", before clicking anything. | The new "Extract from filename" control looks like a peer of "Refresh" next to it — same small size, same muted grey. It should not be a filled or coloured button competing for attention. |
| 4.2 | `[human]` Click it and watch what happens. | Something visibly changes within a moment. You should never be left wondering whether the click registered. |
| 4.3 | `[human]` Read the panel that appears without anyone explaining it. | You can tell what came out of the filename, what the file's own tags currently say, and which of the two you are being asked to choose between. |
| 4.4 | `[human]` Look at the people or studio chips. | It is obvious at a glance which names Holodex already knows about and which ones it would be creating fresh. (Reference: existing entities carry a green outline, new ones amber.) |
| 4.5 | `[human]` Stage a couple of fields, then stop and look at the bottom of the panel. | It is clear how many changes are waiting, and that nothing has been written to the file yet. |
| 4.6 | `[human]` Click "Review and write", then read the dialog before confirming. | You can see exactly what each file value is changing from and to, and you feel safe confirming. Nothing is ambiguous about what is about to be overwritten. |
| 4.7 | `[human]` Cancel out of that dialog instead of confirming. | Your staged choices are still there. Cancelling loses nothing. |
| 4.8 | `[human]` Compare the panel against the Extraction tab under the owner area. | The two feel like the same feature in two places — same row rhythm, same wording, same buttons — not two different designs. |
| 4.9 | `[human]` Switch skins (Cinémathèque, Broadcast, Brutalist) with the panel open. | Text stays readable and the chips stay distinguishable in all three. Nothing washes out or disappears. |
| 4.10 | `[human]` Narrow the browser window to roughly phone width. | Rows wrap rather than overflowing sideways, and the buttons stay reachable. |
