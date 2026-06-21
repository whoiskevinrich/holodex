# Feature Spec: Suppress Windows UAC prompts for `holodex.exe` via embedded `asInvoker` manifest

**Status:** Implemented  
**Owner:** Kevin  
**Type:** Developer experience / build tooling  
**Scope:** v1, single phase  
**ADR:** [ADR-042](../architecture/ADR-042-windows-asInvoker-manifest.md)

---

## Problem Statement

Running `holodex.exe` on Windows during development triggers a User Account Control (UAC)
elevation prompt. Because the Go binary ships without an explicit application manifest,
Windows falls back to its installer-detection heuristic and treats the unsigned binary as
requiring elevation, interrupting every run with an "allow this app to make changes?" dialog.
This slows the inner dev loop and risks the binary running with unintended elevated privileges.
Embedding an explicit manifest that declares `asInvoker` tells Windows the process should run
with the caller's existing privileges and no prompt.

---

## Goals

1. `holodex.exe` launches on Windows with no UAC prompt when invoked by a standard (non-elevated) user.
2. The fix is built into the standard `go build` output — no per-developer machine configuration required.
3. The manifest is embedded reproducibly, so every build (local and CI) produces a non-elevating binary.
4. Linux and macOS builds are completely unaffected (the change is Windows-only).
5. A developer cloning the repo fresh can produce a working, non-elevating Windows binary with the documented build steps.

## Non-Goals

1. **Code signing / Authenticode certificates** — out of scope. Signing addresses SmartScreen reputation warnings, which are a separate mechanism from UAC.
2. **Suppressing Windows SmartScreen** ("Windows protected your PC") — different system, not solved by a manifest.
3. **Supporting binaries that genuinely require admin rights** — if a true elevation need is discovered, that is a separate design decision.
4. **Reworking the build system** — the project builds with plain `go build`; this feature fits that, not replacing it.
5. **Auto-elevation / privilege escalation UX** — we are removing the prompt by declaring `asInvoker`, not adding a "run as admin" flow.

---

## Background / Approach

Go does not embed a Windows application manifest natively. The standard approach is to compile
a Windows resource (`.syso`) file containing the manifest and place it in the `main` package
directory; the Go toolchain links any `*_windows_amd64.syso` (or matching GOOS/GOARCH) into
the resulting Windows binary automatically during `go build`.

The manifest declares:

```xml
<requestedExecutionLevel level="asInvoker" uiAccess="false" />
```

`asInvoker` = run with the privileges of the calling process, do not request elevation, do
not trigger installer-detection. Correct for a server application (as opposed to an
installer/updater).

**Toolchain:** [`rsrc`](https://github.com/akavel/rsrc) (manifest-only, minimal) via
`github.com/akavel/rsrc`. Chosen over `goversioninfo` because the project uses plain
`go build` and does not currently embed version metadata or an icon.

**Confirmed safe:** `holodex.exe` performs no admin-only operations — all paths are
user-configurable, no writes to `Program Files` or `HKLM`, no raw sockets or system service
modifications. `asInvoker` will not silently drop any required privilege.

---

## User Stories

- As a holodex developer on Windows, I want to run a freshly built `holodex.exe` without a UAC prompt, so that my dev loop is not interrupted on every launch.
- As a developer cloning the repo, I want the non-elevating behavior to come from the normal build, so that I do not have to configure anything on my machine.
- As a maintainer, I want the manifest source committed and the `.syso` generation reproducible, so that CI and every contributor produce identical non-elevating binaries.
- As a developer on Linux/macOS, I want my builds to be untouched by this change, so that cross-platform builds keep working.

---

## Requirements

### Must-Have (P0)

**P0-1 — Manifest source file committed to the repo.**  
`cmd/holodex/holodex.manifest` declares `requestedExecutionLevel level="asInvoker"`.

- [x] Manifest file exists in the repo and is valid XML conforming to the Windows application manifest schema.
- [x] `requestedExecutionLevel` is set to `asInvoker`, `uiAccess="false"`.
- [x] File includes a comment explaining its purpose and how to regenerate the `.syso`.

**P0-2 — Windows resource (`.syso`) embedded into the build.**  
`cmd/holodex/holodex_windows_amd64.syso` committed; `go build` links it automatically.

- [x] Building on/for Windows (`GOOS=windows go build`) produces a `holodex.exe` that contains the manifest.
- [x] No change to the `go build` invocation is required.
- [x] Generation step documented via `//go:generate` in `cmd/holodex/main.go`.

**P0-3 — No UAC prompt on launch.**  
Verified manually on a Windows development box.

- [x] Given a standard (non-elevated) Windows session, no UAC dialog appears on launch.
- [x] Process runs at medium integrity — verifiable in Task Manager / Process Explorer.

**P0-4 — Cross-platform builds unaffected.**

- [x] `_windows_amd64.syso` suffix causes the file to be ignored on non-Windows targets.
- [x] `GOOS=linux go build ./...` succeeds unaffected.
- [x] `GOOS=darwin go build ./...` succeeds unaffected (tested via CI).

### Nice-to-Have (P1)

**P1-1 — `go:generate` directive.**  
Added to `cmd/holodex/main.go`:
```go
//go:generate rsrc -manifest holodex.manifest -arch amd64 -o holodex_windows_amd64.syso
```

**P1-2 — `.syso` commit policy.**  
`holodex_windows_amd64.syso` is committed to the repo so a fresh clone builds correctly
without installing `rsrc`. The manifest source + generate directive are committed alongside
it for reproducible regeneration.

**P1-3 — Documentation.**  
ADR-042 documents the decision. This spec is the implementation record.

---

## Future Considerations (P2)

- **arm64 Windows:** if `windows/arm64` is ever targeted, add `holodex_windows_arm64.syso` and extend the generate directive.
- **Code signing:** if SmartScreen warnings become a problem for distributed builds, add Authenticode signing as a separate initiative (separate mechanism from UAC).
- **Embedded version info / icon:** if desired later, switch from `rsrc` to `goversioninfo`, which covers manifest + version + icon in one resource.

---

## Success Metrics

- UAC prompt count on launch drops from every run to zero for a standard user.
- `go build` for Windows continues to succeed with no added flags.
- No regressions in Linux/macOS builds after merge.
