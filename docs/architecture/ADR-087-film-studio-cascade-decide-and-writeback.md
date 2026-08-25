# ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action

**Status:** Proposed
**Date:** 2026-08-25
**Deciders:** Project owner

**Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (`field_source_decisions`,
the per-video Studio decision this ADR sets N of) · [ADR-052](ADR-052-baseline-source-contract.md)
(the resolver those decisions feed) · [ADR-077](ADR-077-tag-writeback-exclusion.md) (`writequeue.EnqueueMany`
+ `GetWritebackBatchStatus`, the batch-enqueue/progress mechanism this ADR reuses unchanged — see
Context, "why this isn't just ADR-077 again") · [ADR-085](ADR-085-films-entity.md) (`film_videos`,
the attachment table this cascade walks; the derived-union `FilmStudios` read this ADR gives the
owner a write path into, for Studio only). **Relates to:** [ADR-041](ADR-041-metadata-writeback.md)
(copy→write→rename file-safety model, unchanged) · [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
(`propagateMerge`, the earlier "one owner action batch-enqueues writeback across many videos"
precedent — this ADR's decision-set phase has no analogue there, since a merge's affected-video
list is a side effect of a transaction that already happened, not a fresh per-video decision).
**Spec:** [film-studio-cascade-writeback.md](../specs/film-studio-cascade-writeback.md) (HOLODEX-285).

---

## Context

F57's spec resolves a product-level question (owner-view-gated edit affordance, unconditional
cascade, cascade writes back to file) but leaves the mechanism underneath it unspecified. This
ADR is that mechanism.

### Current state (survey, 2026-08-25)

| Seam | Today | File |
|---|---|---|
| Single-video Studio decision | `setFieldDecision`'s Studio branch: resolve proposed names → `FindStudioCollision` (HOLODEX-271) → `SetDecisionChecked` (locked check+write) → `relinkStudiosWithContext` | `internal/api/decisions.go:124-155` |
| Composite-key collision gate | `FindStudioCollision(ctx, videoID, names)` — a video whose post-edit `{title, people, date, studio}` matches another active video's is blocked unless `Override: true` | `internal/repo` (via `decisions.go:130`) |
| Bulk decide-then-writeback across N videos | **Does not exist.** `propagateMerge` batch-enqueues writeback from a *precomputed* name list after a merge transaction already committed; `syncTagWriteback` (ADR-077 D2) *re-pushes* an *already-decided* value, recomputed per video, but never sets a new decision | `internal/api/merge_writeback.go`, `internal/api/tag_writeback_sync.go` |
| Batch enqueue + progress | `writequeue.EnqueueMany(ctx, jobs, batchID)` (one transaction, N videos) + `GetWritebackBatchStatus(ctx, batchID)` (aggregates `writeback_queue`/`job_runs` by `batch_id`) — both field-agnostic, built for exactly this shape of problem | `internal/writequeue/writequeue.go`, `internal/repo` (ADR-077 D2/D3) |
| Film's Studio read | `FilmStudios(ctx, filmID)` — live `SELECT DISTINCT` union over `film_videos JOIN video_studios`, no per-film storage | `internal/repo/films.go:492-508` |

### Why this isn't just ADR-077 again

ADR-077's `syncTagWriteback` looks superficially identical (loop over video IDs, build
`writequeue.BatchJob`s, one shared `batchID`) but solves a narrower problem: every video already
has a `genres` value to recompute — the loop's only job is to re-derive and re-push what the DB
already implies. F57 has no prior decision to recompute: the owner is asserting a **new** Studio
for videos that may currently disagree (that's the point — "make this film's videos consistent").
Setting that decision is not a read, it's a write with its own success/failure/collision outcome
per video (`SetDecisionChecked` already returns exactly that shape for one video). Nothing in the
codebase today runs N of those, sharing one action and one writeback batch.

### Forces

- **The composite-key collision gate (HOLODEX-270/271) must stay live per video during the
  cascade — RD4's "unconditional overwrite" is not the same axis.** The spec is explicit (RD4,
  Non-Goal 3) that the cascade ignores an attached video's *existing* studio decision — that's
  the point of a film-level source of truth. It says nothing about the collision gate, which
  protects against two different video *records* becoming indistinguishable — a real data-
  integrity concern, not a stale-decision concern. Spec P0-4 confirms the gate stays live: "a
  per-video collision... surfaces in the batch result without aborting the remaining videos."
- **A per-video decision-set failure is not the same kind of event as `syncTagWriteback`'s
  read failure, so it does not warrant the same all-or-nothing response.** ADR-077 D2 aborts the
  whole batch on a read error specifically because "a sync trigger has committed nothing yet, so
  there is nothing to reconcile a partial batch against." Here, each video's `SetDecisionChecked`
  call is itself the commit — by the time video 3 of 10 hits a collision, videos 1-2 have already
  durably changed. Aborting videos 4-10 because video 3 collided would leave the film in a
  *more* inconsistent state than proceeding, not less, and directly contradicts spec P0-4's
  acceptance criterion.
- **The decision-set phase is cheap, synchronous DB work; the writeback phase is the part that
  legitimately needs async batch-status polling.** `SetDecisionChecked` for N videos (N is a
  film's scene count — small, not library-scale) is fast enough to run inline in the request
  handler, unlike file I/O. This means the endpoint can return a definitive per-video decision
  outcome synchronously, and only the *writeback* half needs `WritebackBatchDialog`-style polling
  — sharper information than ADR-077's bare `{batch_id, enqueued}`, which only ever reported an
  aggregate count because every video's read had already succeeded by construction (the abort-
  on-first-error design left no room for a partial per-video outcome to report).
- **No second caller exists yet for "bulk decide-then-writeback."** Generalizing the mechanism
  into an entity-agnostic N-video primitive now would be speculative — the spec's own P1-2
  explicitly defers that. Building it Film-Studio-scoped first, and generalizing only when a
  second caller materializes, matches this repo's "no abstractions for single-use code" rule.

---

## Decision

### D1 — Extract the single-video Studio-decide logic into a shared helper; the cascade calls it once per attached video

`setFieldDecision`'s Studio branch (`decisions.go:124-155`) already does exactly the per-video
work the cascade needs. Extract it, unchanged in behavior, into:

```go
// internal/api/decisions.go
// decideStudioForVideo runs the Studio composite-key collision gate (HOLODEX-271) and
// SetDecisionChecked for one video, then relinks video_studios on success. Shared by
// setFieldDecision's Studio branch (single video, override honored) and the film-studio
// cascade (internal/api/film_studio_cascade.go, override always false — RD4's unconditional
// overwrite applies to a video's prior decision, not to this safety gate).
func (h *Handlers) decideStudioForVideo(ctx context.Context, videoID int64, field mapping.Field, source, manualValue string, override bool) (names []string, collision *repo.VideoCollision, err error) {
	rc, names, err := h.resolveProposedStudioNames(ctx, videoID, field, source, manualValue)
	if err != nil {
		return nil, nil, err
	}
	check := func() (*repo.VideoCollision, error) { return h.repo.FindStudioCollision(ctx, videoID, names) }
	if override {
		check = nil
	} else if collision, err = check(); err != nil {
		return nil, nil, err
	} else if collision != nil {
		return names, collision, nil
	}
	if collision, err = h.repo.SetDecisionChecked(ctx, model.EnrichEntityVideo, videoID, field.Canonical, source, manualValue, check); err != nil {
		return nil, nil, err
	}
	if collision != nil {
		return names, collision, nil
	}
	h.relinkStudiosWithContext(ctx, videoID, rc, names)
	return names, nil, nil
}
```

`setFieldDecision`'s Studio branch becomes a thin wrapper: call this with `override: body.Override`,
translate `(names, collision, err)` into the existing HTTP responses. No behavior change for the
single-video path — this is extraction, not new logic.

### D2 — `CascadeFilmStudio`: per-video decide (best-effort), then one shared-batch enqueue for every video that succeeded

```go
// internal/api/film_studio_cascade.go (new file, sibling of film_fields.go and
// tag_writeback_sync.go)

type filmStudioCascadeResult struct {
	VideoID int64             `json:"video_id"`
	Status  string            `json:"status"` // "enqueued" | "collision" | "error"
	Conflict *repo.VideoCollision `json:"conflict,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func (h *Handlers) cascadeFilmStudio(ctx context.Context, filmID int64, source, manualValue string) (batchID string, results []filmStudioCascadeResult, err error) {
	field, _ := h.mappings.Current().ByCanonical("studio") // always present — replaceField already guards this shape elsewhere
	videoIDs, err := h.repo.VideoIDsForFilm(ctx, filmID)   // new: SELECT video_id FROM film_videos WHERE film_id = ?
	if err != nil {
		return "", nil, err
	}

	jobs := make([]writequeue.BatchJob, 0, len(videoIDs))
	results = make([]filmStudioCascadeResult, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		names, collision, decErr := h.decideStudioForVideo(ctx, videoID, field, source, manualValue, false)
		switch {
		case decErr != nil:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "error", Error: decErr.Error()})
		case collision != nil:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "collision", Conflict: collision})
		default:
			results = append(results, filmStudioCascadeResult{VideoID: videoID, Status: "enqueued"})
			jobs = append(jobs, writequeue.BatchJob{
				VideoID: videoID,
				Fields:  []writequeue.JobField{{Field: "studio", Values: names, Source: fieldsource.Manual}},
			})
		}
	}

	if len(jobs) == 0 {
		return "", results, nil // every video collided or errored; nothing to enqueue
	}
	batchID = fmt.Sprintf("film-studio-cascade-%d", time.Now().UnixNano())
	if _, err := h.writeQueue.EnqueueMany(ctx, jobs, batchID); err != nil {
		return "", nil, err
	}
	return batchID, results, nil
}
```

Each video's decision-set is independent and already durably committed the moment
`decideStudioForVideo` returns success — unlike `syncTagWriteback`, there is no "abort before
anything is committed" option available even if it were desired (see Forces). A video's collision
or error excludes only that video from the enqueue set; every other video proceeds. `names` — the
resolved studio name(s) `decideStudioForVideo` already computed while setting the decision — is
reused directly as the writeback job's values, the same "the value being written is exactly what
was just decided, not re-derived a second time" shape `syncTagWriteback` achieves by recomputation
(there the decision already existed so recomputation was the only option; here the decision was
just freshly computed, so reuse is strictly cheaper and cannot drift from what was actually set).

### D3 — Endpoint: `POST /films/{id}/studio/cascade`, Film-scoped, owner-gated; per-video results returned synchronously, writeback progress polled via the existing batch-status endpoint

```go
// internal/api/film_studio_cascade.go
func (h *Handlers) mountFilmStudioCascade(r chi.Router) {
	r.Post("/films/{id}/studio/cascade", h.cascadeFilmStudioHandler)
}

func (h *Handlers) cascadeFilmStudioHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body decisionBody // reuses the existing {source, manual_value} shape — no Override field meaning here (D2 always runs the gate)
	if !decodeJSON(w, r, &body) {
		return
	}
	batchID, results, err := h.cascadeFilmStudio(r.Context(), id, body.Source, body.ManualValue)
	if err != nil {
		h.fail(w, "cascade film studio", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"batch_id": batchID, "results": results})
}
```

No new status-polling endpoint: once jobs are enqueued, the SPA polls the existing
`GET /writeback/batches/{batchID}/status` (ADR-077 D3) exactly as `WritebackBatchDialog` already
does for Tag sync — that endpoint aggregates `writeback_queue`/`job_runs` by `batch_id`, which is
field- and caller-agnostic. `batch_id` is `""` when every video collided/errored (no jobs
enqueued); the frontend treats an empty `batch_id` as "nothing to poll," showing the synchronous
`results` array alone.

This is deliberately **Film-scoped, not a general N-video "bulk decide+writeback" primitive**
(Forces, spec P1-2) — `cascadeFilmStudio` takes a `filmID`, not a caller-supplied video list. If a
second caller needing the same shape emerges, generalizing `cascadeFilmStudio` into an
entity-agnostic helper (film ID → video IDs is the only Film-specific line) is a small, low-risk
refactor at that point — not worth doing speculatively now.

---

## Options Considered

### D1 — where the per-video Studio-decide logic lives

**A — Extract into a shared helper, called by both the single-video handler and the cascade
(chosen).** Pros: zero behavior change for the existing single-video path (still exercised by its
existing tests), the cascade gets the exact same collision-gate/relink correctness for free, one
place to fix a Studio-decide bug in the future. Cons: none identified — this is a pure extraction
of already-correct logic.

**B — Duplicate the resolve/collision/SetDecisionChecked/relink sequence in the cascade.** Pros:
no change to `decisions.go`. Cons: two copies of a subtle, lock-sensitive sequence
(`SetDecisionChecked`'s TOCTOU-closing re-check, documented at length in the existing code) that
must be kept in sync by hand — exactly the kind of duplication this repo's simplify pass exists
to catch. Rejected.

### D2 — failure-boundary semantics (the ADR's central open question from the spec)

**A — Best-effort per video; a collision/error excludes only that video (chosen).** Pros: matches
spec P0-4's explicit acceptance criterion; each video's decision is already independently
committed by the time a later video might fail, so an abort-the-rest posture cannot actually undo
what already happened — it can only leave more videos out of sync, the opposite of the feature's
goal. Cons: the owner may end up with a partially-cascaded film (some videos updated, one flagged
for manual attention) — but this is exactly the transparency spec P0-4 asks for, not a defect.

**B — Abort the whole batch on the first collision/error, mirroring `syncTagWriteback`'s
posture.** Pros: simpler mental model ("all or nothing"), consistent with the closest existing
precedent (ADR-077 D2). Cons: `syncTagWriteback`'s justification for aborting — "nothing is
committed yet" — does not hold here; a decision-set is a commit, so aborting after video 3 of 10
already succeeded doesn't prevent partial application, it just also skips videos 4-10 that would
have succeeded, actively contradicting the feature's purpose. Also directly conflicts with spec
P0-4. Rejected.

**C — Two-phase: dry-run every video's collision check first (no writes), only proceed to
writing if zero collisions.** Pros: true all-or-nothing, avoids ever leaving a film
partially-cascaded. Cons: contradicts the spec's own resolved decision (RD4/Non-Goal 3 — no
review-before-cascade gate in v1; that's explicitly deferred as a P1/P2), and doubles the
collision-check work (once to dry-run, once for real, since `SetDecisionChecked`'s own re-check
inside the lock is what closes the TOCTOU gap — a separate dry-run pass can't replace it).
Rejected for v1; the spec's own P1-1 ("pre-cascade summary, informational only") is the lighter-
weight version of this idea already on the roadmap.

### D3 — endpoint shape

**A — Film-scoped `POST /films/{id}/studio/cascade`, synchronous per-video results + existing
batch-status polling for writeback progress (chosen).** Pros: matches spec P1-2's "defer
generalizing until a second caller exists," reuses ADR-077 D3's batch-status endpoint with zero
changes, gives the frontend the richer per-video collision/error detail P0-4 needs without
inventing new polling shape. Cons: none identified relative to the alternatives below.

**B — A general `POST /videos/decide-and-writeback` primitive taking a caller-supplied video list
and field, with Film as its first caller.** Pros: would already be in place if a second caller
(e.g. a future bulk Tag-decide feature) shows up. Cons: no second caller exists today — this is
exactly the speculative abstraction this repo's working agreements warn against ("no
abstractions for single-use code... don't design for hypothetical future requirements"). Rejected
for v1; D3's chosen shape is a strict subset that generalizes cleanly later if needed.

**C — Reuse ADR-077's `runTagWritebackSync` shape verbatim (bare `{batch_id, enqueued}`, no
per-video detail).** Pros: maximum consistency with the existing Tag-sync response shape. Cons:
loses the per-video collision/error detail that spec P0-4 explicitly requires — ADR-077's shape
was sufficient there only because every video's read had already succeeded by construction (no
per-video failure mode to report). Rejected — the response needs to carry information ADR-077's
mechanism never had to.

---

## Trade-off Analysis

**Extraction (D1) vs. duplication.** No real trade-off — extraction is strictly better once a
second call site for the same logic exists, and the risk (accidentally changing single-video
behavior during extraction) is fully covered by the existing single-video test suite continuing
to pass unchanged.

**Best-effort (D2-A) vs. abort-on-first-failure (D2-B).** This is the ADR's one genuine trade-off,
and it resolves cleanly once the "is this a read or a commit" distinction is made explicit:
ADR-077's abort posture is correct *for a read failure before any commit*; it is actively wrong
*for a per-video commit that has already happened*, which is what every prior video in this
cascade represents by the time a later one fails. Best-effort is not a weaker safety posture here
— it's the only posture consistent with what "already committed" means once decision-setting
itself is the operation, not a precondition check for it.

**Film-scoped (D3-A) vs. general primitive (D3-B).** Building the general form now would cost
more (a video-list-shaped API instead of a film-ID-shaped one, plus deciding how a generic caller
supplies its own collision-gate semantics) for a need that does not exist yet. The Film-scoped
form is a strict subset of the general one — nothing about D3-A's implementation forecloses
generalizing `cascadeFilmStudio` later; the `filmID → videoIDs` line is the only Film-specific
part.

---

## Consequences

**What becomes easier**
- `decideStudioForVideo` becomes the one place Studio's collision-gate + decision-write +
  relink sequence lives — the single-video endpoint and the Film cascade can never drift apart on
  this logic, and a future third caller (if one ever needs single-video Studio-decide semantics)
  has an obvious extension point.
- The Film cascade inherits `GetWritebackBatchStatus` and `POST /writeback/batches/{batchID}/revert`
  for free, exactly as Tag sync did (ADR-077's stated "what becomes easier") — a bad cascade is
  revertible via the same mechanism, no new work.
- A future second bulk-decide-then-writeback caller (if one materializes) has a working,
  battle-tested reference implementation to generalize from, rather than a green-field design.

**What becomes harder**
- The Film cascade's best-effort-per-video posture (D2-A) is a deliberate departure from
  `syncTagWriteback`'s all-or-nothing posture (ADR-077 D2) for a reason that is easy to miss on a
  surface read (both loop over video IDs and call `EnqueueMany`) — a future maintainer reasoning
  by analogy to `syncTagWriteback` should not assume the same abort-on-first-error behavior here;
  the doc comment on `cascadeFilmStudio` (and this ADR) is the record of why they differ.
- The cascade endpoint's response shape (`results` array with per-video status) is richer than
  every existing batch-trigger endpoint's `{batch_id, enqueued}` — a future generalization (D3-B,
  if it happens) needs to decide whether to widen the existing shape or keep two shapes, since
  Film's per-video detail need may not generalize to every future bulk-writeback caller.

**What we'll need to revisit**
- **Generalizing `cascadeFilmStudio` into an entity-agnostic bulk decide-then-writeback primitive**
  (D3-B) — only once a second real caller exists; no evidence of one yet (spec P1-2 explicitly
  defers this).
- **A pre-cascade dry-run summary (D2-C)** — spec P1-1 already earmarks this as a lighter-weight
  follow-up ("informational only, no opt-out checklist") if the unconditional-overwrite-with-
  after-the-fact-reporting posture proves too blunt in practice.

---

## Action Items

1. [ ] `internal/api/decisions.go`: extract `decideStudioForVideo` (D1); `setFieldDecision`'s
   Studio branch becomes a thin caller with `override: body.Override`. Existing Studio-decision
   tests must pass unchanged.
2. [ ] `internal/repo.VideoIDsForFilm(ctx, filmID)`: active/non-deleted video IDs attached to a
   film via `film_videos` (mirrors `VideoIDsForTag`'s shape).
3. [ ] `internal/api/film_studio_cascade.go`: `cascadeFilmStudio` (D2), `mountFilmStudioCascade`
   + `cascadeFilmStudioHandler` (D3), mounted in the `requireOwner` group alongside
   `mountFilmFields`.
4. [ ] No backend change needed for progress polling — confirm the frontend's
   `WritebackBatchDialog` (or equivalent) can be driven by this endpoint's `batch_id` against the
   existing `GET /writeback/batches/{batchID}/status` unchanged.
5. [ ] `/design-handoff`: the Film-side trigger UI, the synchronous per-video collision/error
   result presentation (new — no existing precedent renders a mixed enqueued/collision/error
   list from one action), 3-skin QA. Out of this ADR's scope.
6. [ ] `/testing-strategy`: cover D1's extraction (single-video Studio-decide behavior unchanged),
   D2's best-effort posture (one colliding video among N does not block the others' enqueue), D2's
   value reuse (the writeback job's values match exactly what `decideStudioForVideo` just set, no
   redundant recomputation), and D3's empty-batch case (every video collides → `batch_id: ""`, no
   enqueue attempted).
7. [ ] `/security-review` before merge — new owner-gated bulk-mutation endpoint touching N videos'
   decisions and file writes in one action; confirm `requireOwner` gating, parameterized
   `VideoIDsForFilm` query, and that a film with zero attached videos is a clean no-op (not an
   error).
