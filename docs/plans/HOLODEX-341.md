---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-341                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # none — dev-environment and CI change only, no user-facing behavior (chore is hidden from the changelog by cliff.toml)
---

# HOLODEX-341 · Local dev credentials come from the environment; CI enforces it

Raised as "the TMDB token was checked into code — remove it from the repository." **The premise
turned out to be false, and establishing that changed the whole plan.** `.claude/launch.json` is
untracked and covered by the `.claude/launch.json*` glob in `.gitignore` — a rule whose own comment
already anticipated provider tokens. Verified three independent ways before touching anything:

- `git log --all -- .claude/launch.json` is empty; no commit ever touched the path.
- A scan of **every object in the database** — 4,585 blobs, 1,352 commits, all local and
  remote-tracking refs, with `git fetch --dry-run` confirming parity with `origin` — contains no
  fragment of the token, its JWT payload, the embedded v3 key, or the account id.
- A full-history `gitleaks` run reports no leaks. `graphify-out/`, which **is** git-tracked and was
  the one plausible index-shaped leak path, is clean.

So no history rewrite, no force-push, no disclosure. The rotation was still correct; `ADMIN_TOKEN`
was the literal placeholder `secret` (sha1 `e5e9fa1b`), never a real credential.

**What did need fixing is a different threat model.** Gitignored is not private: a working-tree file
is routinely read into agent context, and from there into transcripts, screenshots and screen
shares. That is almost certainly how the token was noticed, and the gitignore rule does not address
it. Two structural causes made `launch.json` the path of least resistance — the sidecar is barred by
ADR-033 from importing `internal/config` and so cannot reuse `loadDotenv`, and `launch.json` is
per-worktree so every secret is duplicated and drifts (a six-week-old `.bak` still held superseded
values).

**Mis-rotation is the failure the fix invites.** Moving the token into the environment makes rotation
one command, which makes putting the *wrong* TMDB credential in it the next most likely mistake —
the dashboard shows the v3 API key and the Read Access Token together and regenerates them as a
pair, and both are non-empty, so the old emptiness check waves a swap straight through. D5 closes
that.

**Overlaps:** the sidecar boundary is ADR-033/040's; `.env` precedence is ADR-027's; the other CI
security gate is ADR-070's.

## Gates — definition of done

- [~] spec `write-spec` — not applicable; no functional or behavioral change to the product. The one
  spec edit is a correction, not a new requirement: `qa-tmdb-provider.md` step 0.5 told the reader to
  put `TMDB_API_TOKEN` in a `.env` that never contained it and that the sidecar cannot read
- [x] architecture `architecture` —
  `docs/architecture/ADR-094-local-dev-credentials-from-environment.md` (D1–D5), indexed in
  `docs/architecture/README.md`. ADR number claimed via `scripts/adr-claims.mjs --reserve`, not by eye
- [~] design `design-handoff` — not applicable; no user-facing surface
- [x] testing `testing-strategy` — `providers/tmdb/main_test.go` pins the credential shapes (14 cases
  incl. both swap directions, the template placeholder, and a guard that a hex key can never satisfy
  the JWT shape). All five startup paths verified against the built binary, not just the classifier
- [x] security `security-review` — clean, no HIGH/MEDIUM. Cleared GH Actions script injection (no
  `${{ }}` in the new job), the `safe.directory` command-execution path (`.git/config` is
  runner-generated and untrackable, submodules off), and confirmed `--redact` is present so a
  detection never prints plaintext into public CI logs
- [~] three-skin QA — not applicable; no frontend change

## Up next — ordered (position = priority)

1. [x] [investigate] Establish whether the token was ever committed, before proposing any remedy —
   object-DB scan + `gitleaks` history scan + `graphify-out/` check
2. [x] [architecture] ADR-094 with the verification evidence recorded, so the "never committed"
   claim is auditable rather than asserted — `docs/architecture/`
3. [x] [dev-config] Strip all three credentials from `.claude/launch.json`; it now holds paths and
   profile config only. Stale `.claude/launch.json.bak` removed (moved to scratch, not hard-deleted)
4. [x] [dev-config] Committed `.claude/launch.json.example` carrying the shape **and** the rule, with
   a `!` gitignore re-include; `git check-ignore -v` confirms the real file stays ignored
5. [x] [ci] `secrets` job in `ci.yml` — `gitleaks` over full history at `fetch-depth: 0`, inherited by
   `release.yml` via `workflow_call`. Verified green against a clone of the committed branch
6. [x] [review] `/code-review high --fix` — 3 findings; 2 fixed, 1 `no_change_needed`. The load-bearing
   one: the gitleaks image runs as root while runners check out as uid 1001, so git would refuse the
   bind-mount with "dubious ownership" and redden CI on a clean repo. Local Docker Desktop does **not**
   reproduce it (Windows bind mounts synthesize root ownership) — fixed with a `GIT_CONFIG_*` guard
7. [x] [backend] **ADR-094 D5 — fail fast on a swapped credential.** `classifyCredential` classifies by
   shape at startup: an unambiguous swap names the correct variable and exits, an unrecognized shape
   only warns so a future TMDB format degrades to a hint — `providers/tmdb/main.go`
8. [x] [docs] Corrected a factual error of my own: the ADR and `.env.example` called the sidecar "a
   separate module." There is one `go.mod`; it is a separate *binary* in the same module, barred from
   the import by the ADR-033 rule. Now matches `.claude/rules/provider-sidecar.md`
9. [ ] [—] Confirm the new `secrets` job is green on the real PR run — the `safe.directory` fix is
   reasoned and locally exercised, but only a Linux runner proves it
10. [ ] [—] Mark the PR ready once item 9 is green

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-08 (last) · verified the premise, then hardened the mechanism
- skills: code-review (high, --fix), security-review
- **The most useful output was a negative finding.** The token was never committed, and proving that
  across the whole object database — rather than trusting the gitignore rule — is what removed a
  history rewrite, a force-push and a disclosure from the plan. Worth doing before any remediation:
  the cost of checking is minutes, the cost of assuming is either an unnecessary rewrite or a missed
  leak.
- Second: a fix can invite its own failure. Making rotation trivial makes mis-rotation the next
  likely error, and the pre-existing emptiness check could not catch it because both credentials are
  non-empty. D5 exists because of the mechanism D2 introduced, not independently of it.
- Handoff: branch `HOLODEX-341-local-dev-improvements`, Draft PR open, Jira In Progress. Everything is
  green locally (vet clean, 26 packages, no failures; gitleaks exit 0 on a clone of the branch). The
  one open item is whether the `safe.directory` guard behaves on a real ubuntu runner — check the
  `secrets` job on the first PR run before marking ready.
