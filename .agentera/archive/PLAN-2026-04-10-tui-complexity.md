# Plan: TUI Complexity Reduction

<!-- Level: light | Created: 2026-04-10 | Status: active -->

## What

Split three oversized TUI files flagged by HEALTH.md Audit 3: extract profile field definitions from profile_widget.go (994 lines), extract handler methods from Layout.Update() (366 lines), and extract tab renderers from content.go (945 lines). Pure refactoring — no behavioral changes.

## Why

Complexity grade C+. Layout.Update() grew 25% in 20 cycles. Each new feature compounds the problem. Splitting now makes each piece independently extensible and prevents further degradation.

## Constraints

- All 99 existing TUI state machine tests must pass unchanged
- No behavioral changes — refactoring only
- Services DI pattern preserved
- Same package (internal/tui) — no new packages

## Acceptance Criteria

- GIVEN the TUI test suite WHEN `go test ./internal/tui/...` is run after all refactoring THEN all existing tests pass without modification
- GIVEN Layout.Update() WHEN examined THEN it delegates to named handler methods rather than containing all logic inline, and no single method exceeds 80 lines
- GIVEN profile_widget.go WHEN examined THEN field definitions are in a separate file and the widget model/view logic is under 600 lines
- GIVEN content.go WHEN examined THEN tab rendering logic is extracted to methods or a separate file and the main file is under 700 lines
