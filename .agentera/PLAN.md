# Plan: Trusted Profile Loop

<!-- Level: full | Created: 2026-04-24 | Status: active -->
<!-- Reviewed: 2026-04-24 | Critic issues: 17 found, 15 addressed, 2 dismissed -->

## What

Make Spela's trusted profile loop visible and verifiable from effective profile resolution through launch preparation, mutation, and cleanup. The plan covers profile explanation semantics, preflight launch summaries, restore confidence, and CLI/TUI/GUI parity.

## Why

The refreshed vision says one per-game profile is the trusted home for DLSS, GPU, CPU, Proton, overlay, and environment intent. Users need to see what is inherited, what is overridden, what will affect launch, and what Spela can restore.

## Constraints

- Steam `%command%` remains the preferred launch path.
- TUI and GUI remain configuration and inspection surfaces, not launchers.
- Live inheritance remains the profile model.
- UI surfaces must not own independent domain workflows.
- Existing profile YAML behavior must not silently change.
- Schema changes require explicit migration behavior.
- Ephemeral launch environment must stay distinct from restorable mutations.
- No new runtime dependency may be added without explicit approval.
- Release pushes remain user-gated.

## Scope

**In**: Effective profile explanations, launch-impact classification, preflight summaries, restore visibility, CLI/TUI/GUI parity, user-facing guidance, release bookkeeping.
**Out**: New launcher surfaces, Vulkan overlay rendering, community profile sharing, AMD or Intel support.
**Deferred**: DLSS recommendation intelligence, in-game tuning controls, and remote release publication.

## Design

Define one behavioral vocabulary for profile source, launch impact, and restore coverage. Use it across CLI, TUI, and GUI without moving domain ownership into UI code. Treat launch environment as planned child-process state, while file and system changes require restore coverage. Keep versioning and freshness work at the end.

## Tasks

### Task 1: Define profile explanation semantics

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN defaults and game overrides exist WHEN Spela explains an effective profile THEN each visible value has a source: default, override, or unset.
▸ GIVEN a value can affect launch WHEN Spela explains it THEN the impact is classified as environment, game file, system state, overlay, or compatibility.
▸ GIVEN a value can mutate persistent state WHEN Spela explains it THEN restore coverage is distinct from ephemeral launch environment.
▸ GIVEN defaults change live inherited values WHEN Spela explains a game profile THEN the user can tell why the value changed.
▸ GIVEN tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per semantic boundary.

### Task 2: Expose CLI and preflight summaries

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN a user asks what a game launch will do WHEN Spela summarizes preparation THEN environment, DLL, hardware, overlay, and compatibility impacts are visible before mutation.
▸ GIVEN no profile-specific changes exist WHEN Spela summarizes preparation THEN the summary says no profile mutation is planned.
▸ GIVEN a direct Steam URI launch cannot be tracked WHEN Spela explains launch options THEN it points to the wrapper path without claiming cleanup coverage.
▸ GIVEN tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per summary behavior.

### Task 3: Strengthen restore confidence

**Depends on**: Task 1, Task 2
**Status**: □ pending
**Acceptance**:
▸ GIVEN preparation partially succeeds WHEN a later step fails THEN prior restorable mutations are restored once and the failure names the affected area.
▸ GIVEN DLL changes are planned WHEN Spela reports restore coverage THEN backup, denylist, and non-writable path outcomes are visible.
▸ GIVEN hardware or game-file changes are applied WHEN cleanup runs THEN restore success or failure is visible without hiding the launch result.
▸ GIVEN an unwrapped launch path is used WHEN restore coverage is shown THEN Spela does not imply lifetime tracking or cleanup guarantees.
▸ GIVEN tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per restore behavior, plus one cleanup ordering edge case because cleanup has multiple branches.

### Task 4: Align TUI profile semantics

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN a game profile is inspected in the TUI WHEN values render THEN source, impact, and restore meanings match the shared semantics.
▸ GIVEN defaults change an inherited value WHEN the TUI reloads the profile THEN inherited state remains clear and no override is implied.
▸ GIVEN a user changes profile intent in the TUI WHEN the game profile is reloaded THEN effective meaning is preserved.
▸ GIVEN UI tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per TUI behavior boundary.

### Task 5: Align GUI profile semantics

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN a game profile is inspected in the GUI WHEN values render THEN source, impact, and restore meanings match the shared semantics.
▸ GIVEN defaults change an inherited value WHEN the GUI reloads the profile THEN inherited state remains clear and no override is implied.
▸ GIVEN a user changes profile intent in the GUI WHEN the game profile is reloaded THEN effective meaning is preserved.
▸ GIVEN GUI tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per GUI behavior boundary.

### Task 6: Update user-facing guidance

**Depends on**: Tasks 2-5
**Status**: □ pending
**Acceptance**:
▸ GIVEN trusted profile behavior has changed WHEN user-facing docs are checked THEN wrapper-first launch, profile sources, and restore coverage are current.
▸ GIVEN docs mention launch preparation WHEN they are reviewed THEN ephemeral environment and restorable mutations are not conflated.
▸ GIVEN documentation coverage is checked WHEN DOCS.md is reviewed THEN touched project documentation is indexed and current.

### Task 7: Version bump per DOCS.md convention

**Depends on**: Tasks 1-6
**Status**: □ pending
**Acceptance**:
▸ GIVEN trusted profile loop work changes user-facing behavior WHEN release state is prepared THEN CHANGELOG.md has a semver-appropriate version entry.
▸ GIVEN release notes are generated WHEN the version bump completes THEN internal agentera bookkeeping is excluded from user-facing notes.
▸ GIVEN release publication is not approved WHEN this task runs THEN local release state is recorded without pushing tags.

### Task 8: Plan-level freshness checkpoint

**Depends on**: Task 7
**Status**: □ pending
**Acceptance**:
▸ GIVEN this plan's user-facing work has shipped WHEN CHANGELOG.md is checked THEN it has plan-level entries covering completed tasks.
▸ GIVEN this plan completes WHEN PROGRESS.md is checked THEN it has a plan summary entry listing produced commits.
▸ GIVEN this plan resolves or creates known profile-loop issues WHEN TODO.md is checked THEN related entries are current and scoped to this plan.
▸ GIVEN documentation coverage is checked WHEN DOCS.md is reviewed THEN the index reflects touched project documentation.

## Overall Acceptance

▸ GIVEN a game has defaults and overrides WHEN Spela explains its effective profile THEN users can see source, launch impact, and restore coverage.
▸ GIVEN a wrapped launch is prepared WHEN Spela mutates state THEN planned changes and restore coverage are visible before or during the session.
▸ GIVEN preparation fails or exits normally WHEN cleanup runs THEN restore behavior is visible and does not hide failures.
▸ GIVEN CLI, TUI, and GUI inspect the same profile WHEN users compare surfaces THEN profile semantics match.
▸ GIVEN direct Steam URI launch is requested WHEN Spela explains safety THEN it does not imply cleanup guarantees.

## Surprises
