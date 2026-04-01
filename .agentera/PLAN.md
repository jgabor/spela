# Plan: Foundation Hardening

<!-- Level: full | Created: 2026-04-01 | Status: active -->
<!-- Reviewed: 2026-04-01 | Critic issues: 8 found, 6 addressed, 1 dismissed, 1 acknowledged -->

## What

Fix all critical health findings and add test coverage for critical untested packages. Addresses 7 critical issues across architecture, patterns, coupling, complexity, and tests.

## Why

VISION.md principle #1: "Correctness over convenience." The codebase has 7 critical findings — 3 bugs (GUI/TUI launch cleanup bypass, logging bypass, global mutable state), 3 test gaps (cpu, launcher, config), and 1 complexity hotspot (profile widget switch). Building features on this foundation risks compounding debt. Hardening now enables confident development in subsequent cycles.

## Constraints

- No new features — strictly remediation and test coverage
- Overlay IPC protocol and stats collector must not regress
- Privilege model (batched pkexec apply-profile) must not change
- Bubbletea v2 / lipgloss v2 APIs only — no v1 regressions
- Profile YAML format must remain backward-compatible

## Scope

**In**: Critical bugs from TODO.md, critical test gaps from HEALTH.md, profile widget complexity
**Out**: Warning-level findings, dependency updates, overlay CLI commands, Vulkan layer, dead code cleanup
**Deferred**: Transitive dep updates, UI god-package fan-out, dead code audit

## Design

Two-phase launch fix: first register cleanup closures in both GUI and TUI (immediate correctness), then consolidate launch orchestration into the launcher package as a single entry point for all interfaces (structural fix). Overlay collector uses CollectFunc callback pattern — consolidation composes the function at the call site, not the dependencies, so launcher's import graph stays narrow. Logging cleanup is mechanical: replace log.Printf with slog equivalents. TUI styles refactor passes theme config through the bubbletea model tree. Profile field registry replaces the 54-case switch with a declarative field definition list. Tests use filesystem isolation (t.TempDir, mock sysfs paths).

**Risk**: Overlay lifecycle consolidation (Task 2) is the highest-risk task — restructuring code that manages the mmap collector is the primary regression surface. Task 1 ensures cleanup works immediately regardless of Task 2 outcome.

## Tasks

### Task 1: Fix launch cleanup in GUI and TUI

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN a game with GPU profile WHEN launched from GUI THEN cleanup restores hardware state on exit
▸ GIVEN a game with GPU profile WHEN launched from TUI THEN cleanup restores hardware state on exit
▸ GIVEN the CLI launch path WHEN compared to GUI and TUI paths THEN all three register cleanup closures

### Task 2: Consolidate launch orchestration

**Depends on**: Task 1
**Status**: ■ complete
**Acceptance**:
▸ GIVEN launch orchestration WHEN examining the launcher package THEN a single entry point serves all interfaces
▸ GIVEN a game launch WHEN overlay is enabled THEN IPC lifecycle works identically across CLI, TUI, and GUI
▸ GIVEN the launcher package WHEN inspecting its imports THEN it does not import gui or tui packages

### Task 3: Fix competing logging patterns

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN the codebase WHEN searched for direct log package usage THEN no non-test files use log.Printf
▸ GIVEN a game launch with logging WHEN examining output THEN all entries use structured slog format

### Task 4: Fix TUI global mutable state

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN TUI styles WHEN inspecting package-level declarations THEN no mutable global variables exist
▸ GIVEN the theme changed via options WHEN the TUI re-renders THEN all components reflect the new theme without restart

### Task 5: Add config persistence tests

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN a config with all fields set WHEN written and read back THEN all values roundtrip identically
▸ GIVEN a missing config file WHEN loaded THEN defaults are returned without error
▸ GIVEN a malformed YAML file WHEN loaded THEN error identifies the corruption

### Task 6: Add CPU package tests

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN a mock sysfs tree WHEN SetGovernor is called THEN the correct file contains the expected value
▸ GIVEN SMT control WHEN toggled THEN the sysfs control file reflects the new state
▸ GIVEN CPU functions WHEN tested THEN all exported functions have test coverage

### Task 7: Add launcher package tests

**Depends on**: Task 2
**Status**: □ pending
**Acceptance**:
▸ GIVEN a launched process WHEN it exits THEN all cleanup closures execute in reverse registration order
▸ GIVEN a running game WHEN SIGTERM is received THEN the signal is forwarded to the child process
▸ GIVEN overlay enabled WHEN game launches THEN IPC file exists during game lifetime

### Task 8: Extract profile field registry

**Depends on**: Task 4
**Status**: □ pending
**Acceptance**:
▸ GIVEN profile field definitions WHEN counted THEN each field is defined in exactly one location
▸ GIVEN a field change applied via the widget WHEN compared to current behavior THEN outcomes are identical
▸ GIVEN the TUI profile widget WHEN a new field is added to the profile struct THEN only the registry needs updating

## Overall Acceptance

▸ GIVEN the full codebase WHEN audited for critical health findings THEN zero critical issues remain
▸ GIVEN cpu, launcher, and config packages WHEN test coverage is measured THEN each has test files
▸ GIVEN a game WHEN launched from any interface THEN hardware state is correctly restored on exit
▸ GIVEN all log output WHEN inspected THEN uses structured slog format exclusively

## Surprises
