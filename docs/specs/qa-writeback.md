# QA: Metadata Writeback (F28)

**Feature**: Per-field write-back of enrichment values into media file tags via exiftool  
**Related**: [ADR-041](../architecture/ADR-041-metadata-writeback.md), [metadata-plugins spec §F28](metadata-plugins.md#f28-metadata-writeback)

---

## 0. Setup

**0.1** [smoke] `go build ./...` — clean build, no errors  
**0.2** [smoke] `go test ./internal/writeback/... -v` — `TestTagForField` PASS (18 cases), failure/cancel/empty tests PASS, success tests PASS or SKIP with informative message  
**0.3** [smoke] `go test ./...` — full suite passes  
**0.4** [human] Start Holodex with a media library containing at least one MKV or MP4 file; confirm the file appears on the browse page  
**0.5** [human] Open the admin panel and set your owner token; confirm the admin controls appear (the "Enrich" button or trash icon are visible on a media detail page)

---

## 1. Tag mapping — unit (no files, no exiftool)

**1.1** [smoke] `TagForField("title", "Matroska")` → `("Title", true)`  
**1.2** [smoke] `TagForField("genres", "MP4")` → `("QuickTime:Genre", true)`  
**1.3** [smoke] `TagForField("poster_url", "Matroska")` → `("", false)` — image fields intentionally excluded  
**1.4** [smoke] `TagForField("title", "avi")` → `("", false)` — unsupported container  
**1.5** [smoke] `TagForField("nonexistent_field", "MP4")` → `("", false)`

---

## 2. API — auth & validation (no file writes)

**2.1** [smoke] `POST /api/v1/media/{id}/writeback` without `X-Admin-Token` → `403`  
**2.2** [smoke] `POST /api/v1/media/{id}/writeback` with valid token but omitting `field` → `400`  
**2.3** [smoke] `POST /api/v1/media/{id}/writeback` with valid token but omitting `values` → `400`  
**2.4** [smoke] `POST /api/v1/media/{id}/writeback` with a container that has no mapping (e.g. an `.avi` file) → `422`; response body names the field and container  
**2.5** [smoke] `POST /api/v1/media/999999/writeback` (non-existent video) → `404`

---

## 3. File write — atomicity & audit

**3.1** [human] Pick a media file with known current title. Note the file size and current exiftool output: `exiftool -Title <file>`  
**3.2** [human] On the media detail page (owner logged in), find the "Title" field in the Resolved section; confirm a write (pencil) icon appears next to fields with a winning provider  
**3.3** [human] Click the write icon for "Title" — a confirmation dialog appears showing:
- The value to be written (the enrichment value)
- The source/provider name
- The file path
- An accent-colored (not red) confirm button labeled "Write to file"
- A Cancel button  

**3.4** [human] Click Cancel — dialog closes, no write occurs; re-open exiftool and confirm the tag is unchanged  
**3.5** [human] Click the write icon again, then click "Write to file" — dialog shows "Working…", then closes; a "Written ✓" label appears next to the field  
**3.6** [human] Verify the tag was actually written: `exiftool -Title <file>` → shows the new value  
**3.7** [human] No `.holodex-tmp` file exists in the same directory as the media file  
**3.8** [human] Check the database: `SELECT * FROM file_writebacks ORDER BY written_at DESC LIMIT 1;` → row present with correct `video_id`, `field_key`, `tag_name`, `value`, and `source`

---

## 4. Failure path — original untouched

**4.1** [agent] Make the file read-only (`icacls <file> /deny Everyone:W`) before writing; attempt writeback via API → non-2xx response; file unchanged; no audit row inserted; no `.holodex-tmp` remaining. Restore permissions after.  
**4.2** [smoke] `Write()` with blank tag name returns error before any file I/O (`TestWrite_OriginalUnchangedOnExiftoolFailure`)  
**4.3** [smoke] `Write()` with pre-cancelled context leaves original byte-for-byte unchanged (`TestWrite_ContextCancelled`)  
**4.4** [smoke] `Write()` with nil values returns error immediately (`TestWrite_EmptyValues`)

---

## 5. Multi-value field (genres)

**5.1** [human] If the media item has genres from enrichment, click the write icon for "Genres" — confirmation dialog lists all genre values  
**5.2** [human] Confirm write; verify with `exiftool -GENRE <file>` → all genre values present (multiple `GENRE` tags for Matroska/MP4)  
**5.3** [smoke] `InsertWriteback` with multi-value field → `value` column stores values joined with `\n`

---

## 6. 422 container gate

**6.1** [human] Upload or scan an `.avi` file (if available); navigate to its detail page; the write icon should not appear (the frontend checks `winnerProvider` but the backend is the real gate)  
**6.2** [smoke] API call directly against an `.avi` video ID with a known-mappable field like `title` → `422 Unprocessable Entity`

---

## 7. UI — tokens & theming

**7.1** [human] Switch to **Cinémathèque** skin; confirm the write icon is `text-muted`, turns `text-accent` on hover; confirm dialog uses `bg-surface`, `border-rule`, `text-ink`, and the confirm button uses `bg-accent text-accent-ink` (not red/warn)  
**7.2** [human] Switch to **Broadcast** skin; repeat 7.1 — no hardcoded colours visible  
**7.3** [human] Switch to **Brutalist** skin; repeat 7.1 — icon, dialog, and confirm button all correctly themed  
**7.4** [human] "Written ✓" label uses `text-accent` (not `text-warn`) and persists until page reload

---

## 8. Fields without write action

**8.1** [human] `poster_url` / image fields in the resolved section do **not** show a write icon (display type is `image_url`)  
**8.2** [human] Fields where no provider has won (file-only value) do **not** show a write icon (`winnerProvider` is falsy)  
**8.3** [human] When not owner (no admin token), no write icon appears on any field
