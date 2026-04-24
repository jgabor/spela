# Plan: Audit 5 Remediation

<!-- Level: full | Created: 2026-04-24 | Status: active -->
<!-- Reviewed: 2026-04-24 | Critic issues: 12 found, 12 addressed, 0 dismissed -->

## What

Resolve the Audit 5 health findings that block safe GUI parity and overlay work. The plan covers launch orchestration, visible profile failures, input validation, UI/domain seams, TUI routing risk, frontend dependency health, release publication, and stale agentera artifacts.

## Why

Spela promises one trusted per-game profile that composes launch, environment, overlay, DLLs, GPU, and CPU controls. Audit 5 shows the primary wrapper path can skip part of that lifecycle, and adjacent seams would multiply the risk if new UI work starts now.

## Constraints

- Steam wrapper launch remains the primary launch path.
- The TUI remains a configuration and inspection console, not a launcher.
- Live inheritance remains the profile model.
- Existing profile YAML behavior must not silently change.
- No new runtime dependency may be added without explicit approval.
- Release pushes remain user-gated.
- Documentation and design artifact changes require explicit approval.

## Scope

**In**: Audit 5 critical and warning findings, plus info findings that protect the same boundaries.
**Out**: GUI parity redesign, Vulkan layer work, overlay feature expansion, AMD or Intel support.
**Deferred**: Full visual identity redesign beyond aligning DESIGN.md with the shipped v0.5.0 UI.

## Design

Converge launch behavior around one preparation lifecycle. Harden profile and privileged boundary inputs so invalid or corrupt state surfaces early. Keep GUI and TUI actions behind narrow application boundaries instead of letting UI views own domain workflows. Reduce TUI routing risk only where behavior is already covered. Upgrade the frontend stack deliberately, then refresh docs and design contracts to match shipped behavior.

## Finding Trace

- Wrapper preparation bypass: Task 1.
- Direct Steam URI lifetime mismatch: Task 1.
- Default profile errors, boolean parsing, CPU governor validation, env tests: Task 2.
- GUI domain coupling, logging, and backend test gap: Task 3.
- DLL resource coupling and field display duplication: Task 4.
- TUI routing complexity: Task 5.
- Frontend audit advisories and pinning policy: Task 6.
- Missing remote v0.5.0 tag and remediation release: Task 7.
- DESIGN.md and DOCS.md drift: Task 8.

## Tasks

### Task 1: Converge launch lifecycle

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN a game starts through the Steam wrapper WHEN overlay or compatibility settings apply THEN the same preparation behavior runs as other supported launches.
▸ GIVEN Steam passes environment into the wrapper WHEN Spela prepares the game THEN the user command environment is preserved.
▸ GIVEN launch preparation fails WHEN cleanup runs THEN previously applied profile state is restored once and the failure is visible.
▸ GIVEN direct Steam URI launch cannot track the real game lifetime WHEN a user requests it THEN Spela avoids claiming the game is safely wrapped.
▸ GIVEN tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per lifecycle behavior, plus one cleanup edge case.

### Task 2: Harden profile and privileged inputs

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN the default profile is missing WHEN effective values load THEN inheritance still falls back safely.
▸ GIVEN the default profile is unreadable or invalid WHEN effective values load THEN Spela surfaces the error instead of using zero values.
▸ GIVEN a boolean profile flag receives invalid text WHEN a user saves it THEN Spela rejects it consistently across subsystems.
▸ GIVEN a CPU governor value is unavailable WHEN it is saved or applied with privileges THEN Spela rejects it before system state changes.
▸ GIVEN environment behavior is tested WHEN coverage is reviewed THEN map isolation and command environment application are verified without exceeding 1 pass and 1 fail test per behavior.

### Task 3: Establish GUI application boundaries

**Depends on**: Task 1, Task 2
**Status**: □ pending
**Acceptance**:
▸ GIVEN the GUI performs profile, DLL, compatibility, or launch actions WHEN those actions run THEN results match non-GUI behavior for the same game and profile state.
▸ GIVEN GUI actions report failures WHEN logs are captured THEN they appear through the repository logging path.
▸ GIVEN GUI behavior is tested WHEN coverage is reviewed THEN each covered use case has at most 1 pass and 1 fail test.
▸ GIVEN GUI parity redesign is deferred WHEN this task completes THEN no new visual redesign scope has been added.

### Task 4: Move TUI resource workflows out of views

**Depends on**: Task 2
**Status**: □ pending
**Acceptance**:
▸ GIVEN stale DLL deployments exist WHEN the DLLs resource updates them THEN each cell reports success or failure without false success messages.
▸ GIVEN DLL operations are simulated WHEN TUI behavior is tested THEN update planning and result rendering stay deterministic.
▸ GIVEN profile fields are shown in Games and Defaults WHEN field support is reviewed THEN every supported field has a label and value display.
▸ GIVEN tests are added WHEN coverage is reviewed THEN do not exceed 1 pass and 1 fail test per DLL workflow or field-display behavior.

### Task 5: Reduce TUI routing hotspots

**Depends on**: Task 4
**Status**: □ pending
**Acceptance**:
▸ GIVEN modal, pending action, profile, DLL, rail, and message flows exist WHEN key handling is exercised THEN each flow behaves as before.
▸ GIVEN a user navigates Games, DLLs, Defaults, and Metrics WHEN resource-specific keys are pressed THEN focus and messages stay scoped to the active resource.
▸ GIVEN routing changes complete WHEN regression tests run THEN existing TUI behavior tests pass without broad snapshot rewrites.
▸ GIVEN tests are added WHEN coverage is reviewed THEN only changed routing boundaries receive new tests.

### Task 6: Resolve frontend dependency health

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN frontend dependencies are audited WHEN the task completes THEN no moderate-or-higher advisories remain, or remaining advisories have explicit rationale.
▸ GIVEN dependency versions are updated WHEN the frontend installs from the lockfile THEN the install is reproducible.
▸ GIVEN dependency policy is reviewed WHEN package metadata is checked THEN npm pinning has an explicit choice consistent with repository discipline.
▸ GIVEN dependency upgrades affect behavior WHEN frontend verification runs THEN existing GUI behavior still passes tests and build.
▸ GIVEN a clean path requires new runtime dependencies WHEN approval is absent THEN the task records the blocker instead of adding them.

### Task 7: Version bump per DOCS.md convention

**Depends on**: Tasks 1-6
**Status**: □ pending
**Acceptance**:
▸ GIVEN the existing v0.5.0 release tag is missing from the remote WHEN the user approves publication THEN the remote tag exists.
▸ GIVEN the user does not approve release publication WHEN this task runs THEN the task records the external block and does not claim release health resolved.
▸ GIVEN remediation fix work is complete WHEN release is cut THEN a semver-appropriate version entry and local tag exist.
▸ GIVEN release notes are checked WHEN the version bump completes THEN Unreleased is reset for future work.

### Task 8: Plan-level freshness checkpoint

**Depends on**: Task 7
**Status**: □ pending
**Acceptance**:
▸ GIVEN this plan's user-facing work has shipped WHEN CHANGELOG.md is checked THEN it has plan-level Added, Changed, or Fixed entries covering completed tasks.
▸ GIVEN this plan is otherwise complete WHEN PROGRESS.md is checked THEN it has a plan summary entry listing produced commits.
▸ GIVEN this plan resolved Audit 5 findings WHEN TODO.md is checked THEN resolved entries or cross-references exist.
▸ GIVEN the user approves documentation updates WHEN DOCS.md and DESIGN.md are checked THEN they no longer describe stale pre-v0.5.0 launch tabs or theme variants.
▸ GIVEN the user does not approve documentation updates WHEN this checkpoint runs THEN the deferral is recorded and the stale-artifact finding remains open.

## Overall Acceptance

▸ GIVEN a Steam wrapper launch with overlay and compatibility settings WHEN Spela starts a game THEN preparation, environment, warnings, overlay IPC, and cleanup behave as one lifecycle.
▸ GIVEN corrupt profiles or invalid user input exist WHEN users configure or launch games THEN Spela fails visibly before mutating launch or system state.
▸ GIVEN GUI and TUI surfaces perform domain actions WHEN behavior changes THEN UI code does not own independent domain workflows.
▸ GIVEN dependency and artifact health are audited after completion WHEN Audit 5 findings are checked THEN no critical finding remains open and warning count is reduced.

## Surprises

[Empty; populated by realisera during execution when reality diverges from plan.]
