# Design: TUI status strip, log panel, and export (git-rain)

**Date:** 2026-04-19  
**Status:** Draft → implementation plan next (after maintainer review)  
**Cross-reference:** Behavior goals match **`git-fire`** `docs/superpowers/specs/2026-04-19-tui-status-log-export-design.md`; keep these documents aligned on UX contract. **Implementation order:** `git-fire` first, then port to `git-rain`.

## Problem

When the Bubble Tea terminal UI is active, users want **live status** (symbols/emojis) and an optional **log surface** that can be **viewed and exported**, without weakening **first-class auditability**. IT/compliance and **AI agents** rely on **machine-parseable** session evidence; casual users mostly want a calm TUI.

## Goals (v1)

1. **Status strip** shows **short, live** phase indicators driven **only** by **structured session events** (same information model as persisted JSONL), not by ad hoc “reporting” prose or issue templates.
2. **Log panel** (toggle): shows a **bounded in-memory view** derived from those same structured events (and optionally lines explicitly admitted from subprocess output—default **minimal**: prefer structured lines first).
3. **Persistence:** **git-rain** aligns with **git-fire**’s session contract: JSON lines of the same **`LogEntry`** semantics (`git-fire` `internal/executor/logger.go`), written under **`UserCacheDir()/git-rain/logs/`** (same layout pattern as `git-fire` uses for `git-fire/logs/`). Until that logger exists in rain, adding it is part of this workstream; do not invent a divergent on-disk format.
4. **Export (user-initiated, simple):**
   - **Plain-text** export of the **visible / ring-buffer** view for humans (paste, tickets, chat).
   - **Evidence path:** expose **session JSONL path** (open folder / copy path)—no coupling to GitHub markdown, partner reports, or other “reporting” pipelines.
5. **Parity with git-fire:** same **behavior contract** (keys, panel semantics, export semantics, structured-event-driven status, **TUI-then-CLI** split for real work). Visual theming may differ.

## Non-goals (v1)

- **Headless daemon / cron / interval runner** attached to the same UX (deferred; same JSONL contract should make a later phase easier).
- **New shared Go module** between rain and fire (avoid release coupling); optional **short written contract** (glyph meanings + keys) duplicated or cross-linked in READMEs.
- **git-harness** as mandatory runtime dependency of the TUI (no v1 coupling). Later: optional adapter from JSONL or live events to harness JSON for CI.
- **Second on-disk log format** as canonical (no parallel “truth” besides JSONL).

## Architecture

### Phased UX: TUI first, then CLI work (must preserve)

Current **git-rain** behavior (`runRainTUIStream` in `cmd/root.go`): **Bubble Tea** runs for **repo discovery / selection** (streaming). When the user confirms, the program **exits the TUI**, returns to the **normal terminal buffer**, prints **Selected repositories:**, then calls **`runRainOnRepos`** for sync/fetch work on **plain stdout/stderr** — not inside a second full-screen TUI.

**git-fire** follows the same split (`runFireStream` → planner/runner); see cross-reference above.

**Requirement:** v1 design **must not** move post-selection sync/fetch execution into a mandatory full-screen TUI. In-TUI status strip + log panel apply to the **selection / scan** phase (and any other TUI surfaces we already use); **post-TUI execution stays CLI-shaped** as today so agents, pipes, and IT log collection keep working. A future phase may add optional live progress for the runner; that is **out of v1** unless it remains opt-in and does not replace this path.

**CLI-only / non-interactive flows:** unchanged — no requirement to show the TUI for users who configure or invoke without the selector.

### Single event model, two projections

- **Source of truth on disk:** JSON lines of `LogEntry` (same schema as git-fire).
- **In-memory:** append each `LogEntry` to a **ring buffer** for the TUI log panel, **in lockstep** with the write to the session file.
- **Status strip:** renders **only** from **structured fields** (e.g. latest `level`, `action`, `repo`, truncated `description`, duration on success). **No** separate string channel for status text; glyphs map from `(level, action)` (or a small enum derived at log site).

### Three audiences (explicit)

| Audience | Primary surface |
|----------|-----------------|
| Casual user | Emoji/symbol status strip + minimal noise |
| Operator | Toggle log panel, plain-text export of buffer |
| IT / automation / AI agents | Session JSONL path + stable `LogEntry` JSON fields; sanitization rules unchanged |

### Export vs reporting

- **Export** = evidence and operator convenience (`.txt` from buffer, path to JSONL).
- **Reporting** = anything shaped for an external audience (issues, PR narratives, bots). **Not** generated as the only export path; no hard coupling in v1.

## git-fire

- **Lead implementation** lives in **git-fire**; **git-rain** ports the **interaction contract** (`internal/ui` patterns already parallel Bubble Tea + lipgloss).
- Shared **semantic** table for status glyphs (documented; exact Unicode may differ by theme).

## Testing

- **git-testkit / integration:** assert **session file exists**, **each line is valid JSON**, and **expected actions** appear after a controlled dry run or smoke scenario where applicable.
- **git-harness:** no v1 requirement; optional future mapping from JSONL lines to `cli-protocol.json` ops for headless parity.

## Risks / mitigations

- **Drift** between rain and fire: mitigate with **cross-linked specs** and mirrored tests where cheap.
- **Subprocess noise:** default minimal tee into panel; expand later if needed.
- **Performance:** ring buffer max lines + avoid re-reading whole JSONL file on every keystroke; status strip updates on new events only.

## Open items for implementation plan (not blockers for this design)

- Exact **keybinding** and **default ring size** (product decision during plan).
- Whether **plain-text export** includes a one-line header (version, session path)—recommended for support.

## Approval

Design agreed in session 2026-04-19: minimalist v1, JSONL canonical (aligned with git-fire), status strip **fully driven** by structured log events, log panel + export decoupled from reporting, **TUI-then-CLI** preserved for **git-rain** (`runRainTUIStream` → `runRainOnRepos`).
