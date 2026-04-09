# Plan: Ludusavi Removal and Dead Code Sweep

<!-- Level: full · Created: 2026-04-09 · Status: active -->
<!-- Reviewed: skipped (user-approved scope, removal-only plan) -->

## What

Remove the ludusavi save game integration entirely and sweep the codebase for dead code and unused dependencies. Pure removal — no restructuring, no new features.

## Why

The save game feature needs a rethink before reimplementation. Removing it now eliminates dead weight and clears the path for a future approach. The dead code sweep addresses a recurring HEALTH.md finding (16.6% dead functions, Audit 1) and reduces navigation burden.

## Constraints

- Old profile YAML files with ludusavi keys must still load without errors (yaml.v3 ignores unknown keys by default)
- No structural refactors — this plan removes, it does not restructure
- All existing tests must continue to pass after each task
- Do not remove functions that are test-only exports (used in _test.go files)

## Scope

**In**: ludusavi package removal, all ludusavi integration points (profile, launcher, TUI, GUI, docs, packaging), dead function removal across all packages, unused direct dependency removal from go.mod

**Out**: structural refactors (layout decomposition, profile registry, GUI fan-out), test coverage expansion, dependency version bumps, JSON/YAML serialization alignment

**Deferred**: new save game approach (future plan)

## Design

Two passes. First pass removes the ludusavi feature top-to-bottom: package, struct fields, UI widgets, launcher hooks, docs, packaging. Second pass uses static analysis (sentrux, go vet, grep for unused exports) to identify and remove dead code project-wide, then prunes unused direct dependencies from go.mod. Each pass is one realisera cycle.

## Tasks

### Task 1: Remove ludusavi package and all integration points

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN the codebase WHEN searching for "ludusavi" case-insensitively THEN no references exist outside git history, CHANGELOG.md, and .agentera/ operational files
▸ GIVEN a game profile YAML file containing a `ludusavi:` section WHEN loaded by spela THEN the profile loads without error and the ludusavi section is silently ignored
▸ GIVEN the TUI profile editor WHEN viewing any game's profile settings THEN no "Backup settings" group is shown
▸ GIVEN a game launch WHEN the game starts THEN no save backup operations are attempted regardless of old profile content
▸ GIVEN the project WHEN built and tested THEN all tests pass and the binary compiles cleanly

### Task 2: Dead code and unused dependency sweep

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN the codebase WHEN dead code analysis runs THEN the dead function count is measurably lower than the 16.6% baseline from Audit 1
▸ GIVEN go.mod WHEN checked for direct dependencies THEN every direct dependency has at least one import in non-test Go files, or is a build tool dependency
▸ GIVEN the project WHEN built and all tests run THEN everything passes with no regressions
▸ GIVEN removed functions WHEN checked THEN none were called from test files (test-only exports preserved)

### Task 3: Plan-level freshness checkpoint

**Depends on**: Task 1, Task 2
**Status**: □ pending
**Acceptance**:
▸ GIVEN this plan's work has shipped WHEN CHANGELOG.md is checked THEN it has entries under [Unreleased] covering the ludusavi removal and dead code sweep
▸ GIVEN this plan is otherwise complete WHEN PROGRESS.md is checked THEN it has a cycle entry summarizing the plan and listing commits
▸ GIVEN this plan is otherwise complete WHEN TODO.md is checked THEN resolved items are marked and the ludusavi-related annoying item is removed

## Overall Acceptance

▸ GIVEN the completed plan WHEN the codebase is searched for dead code and ludusavi references THEN both are eliminated, all tests pass, and the project builds cleanly
▸ GIVEN operational artifacts WHEN checked THEN CHANGELOG.md, PROGRESS.md, and TODO.md reflect the work done

## Surprises
