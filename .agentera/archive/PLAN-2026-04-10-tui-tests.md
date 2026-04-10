# Plan: TUI End-to-End Test Harness

<!-- Level: full · Created: 2026-04-10 · Status: active -->
<!-- Reviewed: 2026-04-10 | Critic issues: 9 found, 9 addressed, 0 dismissed -->

## What

Build a systematic testing harness for the TUI that covers every user-facing action: key bindings, navigation, state transitions, disabled field contracts, multi-step flows (DLL install wizard, confirmation dialogs), modal dialogs (options, DLSS preset), and batch operations. The harness has two layers: a state machine layer that tests Update logic directly (no I/O), and a dependency injection layer that replaces synchronous I/O calls in model methods so tests can construct models without touching the filesystem.

## Why

The TUI is the primary interface for Spela. It has ~30 key bindings across 6 components (Layout, Sidebar, Content, ProfileWidget, OptionsModal, DLSSPresetModal), a 3-step DLL install wizard, batch operations, confirmation dialogs, and disabled profile fields — none of which have automated tests. HEALTH.md Audit 1 grades Tests at C with "14 of 22 packages lack tests." The Elm architecture makes every Update call a pure function of (state, message) — this is designed for testability, but without tests the architecture's promise is unfulfilled. The disabled "Coming soon" fields have no regression guard: a refactor could accidentally enable them or break the navigation-skipping logic.

## Constraints

- Existing TUI utility tests (thermal, sparkline, help — 966 LOC) must continue passing
- No changes to user-visible behavior — this is a test-only plan
- Service interfaces must not change the signatures or behavior of existing domain packages
- Tests must run without hardware (no NVML, no GPU, no Steam library, no filesystem profiles)
- Bubbletea v2 message types and patterns must be used correctly (not v1)

## Scope

**In**: Dependency injection for synchronous I/O in model methods. State machine tests for all 6 TUI models (Layout, Sidebar, Content, ProfileWidget, OptionsModal, DLSSPresetModal). Navigation stack tests. Batch menu tests. Confirmation dialog tests. Disabled field contract tests. DLL install wizard state machine tests. Test factories for constructing models with known state. Cmd-return-type assertions (verify the correct command is returned without executing it).

**Out**: GUI tests (separate package). Launcher integration tests (already covered). Overlay IPC tests (already exist). Real hardware testing. View rendering / golden-file tests (deferred). Testing the I/O inside Cmd closures (these call domain packages directly — testing them would require interfacing every domain function, which is scope creep for a state machine harness).

**Deferred**: teatest golden-file tests for visual regression. Property-based testing. Performance benchmarking.

## Design

Two categories of coupling exist between TUI models and domain packages:

**Synchronous I/O in model methods**: Calls like game loading, config loading, profile existence checks, backup existence checks, and profile loading happen directly in model constructors and setters — not inside tea.Cmd closures. These block pure state machine testing and require injectable fakes. The injection target is a service struct (or set of function fields) passed into model constructors. The existing concrete functions satisfy these implicitly. Production paths remain unchanged.

**Asynchronous I/O in tea.Cmd closures**: Calls like DLL downloading, manifest fetching, GPU metrics, and game launching happen inside closures returned from Update. These do *not* need interfaces — tests verify that the correct Cmd is returned (by executing it against fakes or by type-asserting on the returned message) without needing to replace the I/O.

Test helpers provide model factories (construct any model with a specific game, DLLs, profile, and config without filesystem), a key-sequence sender, and command type assertion.

## Tasks

### Task 1: Dependency injection for synchronous I/O

**Depends on**: none
**Status**: ■ complete
**Acceptance**:

- GIVEN the TUI model constructors and setters that perform synchronous I/O (game loading, config loading, profile loading/existence, backup existence) WHEN examined THEN each accepts its dependencies through an injectable mechanism rather than calling domain packages directly
- GIVEN the production TUI entry point WHEN the TUI is started normally THEN it passes real implementations and behavior is identical to before
- GIVEN the TUI's asynchronous command closures WHEN examined THEN they are NOT wrapped in interfaces (they remain direct calls inside tea.Cmd closures — out of scope for this task)

### Task 2: Test helpers and model factories

**Depends on**: Task 1
**Status**: ■ complete
**Acceptance**:

- GIVEN a test WHEN a Layout model is needed with a specific game selected, specific DLLs, and a specific profile THEN a factory function produces it with that state — without touching the filesystem, network, or hardware
- GIVEN a test WHEN a sequence of keypresses needs to be sent THEN a helper feeds them through Update sequentially and returns the final model and accumulated commands
- GIVEN a test WHEN a returned command needs type-checking THEN a helper executes the command and asserts on the resulting message type

### Task 3: Layout, navigation, and modal state machine tests

**Depends on**: Task 2
**Status**: ■ complete
**Acceptance**:

- GIVEN the Layout model WHEN each global key binding is pressed THEN the correct state transition occurs (focus changes, modal opens, help toggles, density modes switch, navigation stack pushes/pops)
- GIVEN a navigation stack with varying depths WHEN escape or q is pressed THEN the stack pops correctly (including edge cases: single entry returns to sidebar, deep stack pops one level)
- GIVEN a modal is active (OptionsModal or DLSSPresetModal) WHEN any key is pressed THEN the modal intercepts input before the layout handles it
- GIVEN the OptionsModal WHEN navigating, editing values, and saving/canceling THEN the correct state transitions and messages occur
- Proportionality: 1 happy-path + 1 boundary-condition per key binding. Navigation stack gets 3 additional edge cases (empty stack, single entry, deep stack). OptionsModal gets 1 happy-path + 1 boundary per navigation/edit action.

### Task 4: Sidebar state machine tests

**Depends on**: Task 2
**Status**: ■ complete
**Acceptance**:

- GIVEN the sidebar with a game list WHEN navigation keys are pressed THEN the cursor moves correctly and clamps at boundaries
- GIVEN the sidebar WHEN filter keys are pressed (d for DLLs, p for profiles, s for sort) THEN the filter state changes and the visible game list is filtered accordingly (using injected profile existence fakes)
- GIVEN the sidebar in multi-select mode WHEN space toggles selection and enter is pressed THEN the batch action flow is triggered with the selected games
- Proportionality: 1 happy-path + 1 boundary-condition per key binding.

### Task 5: Content and DLL operation state machine tests

**Depends on**: Task 2
**Status**: ■ complete
**Acceptance**:

- GIVEN no game selected WHEN DLL action keys are pressed (i, u, R) THEN no command is returned and no state changes occur
- GIVEN a game with DLLs and updates available WHEN the update key is pressed with confirmation enabled THEN the pending action state is set, and when confirmed THEN a DLL update command is returned
- GIVEN the DLL install wizard WHEN stepping through type selection, version selection, and confirmation THEN each step transitions the wizard state correctly and the final step returns an install command
- GIVEN a game launch is already in progress WHEN the launch key is pressed again THEN no additional command is returned
- GIVEN the batch menu WHEN navigating and pressing enter THEN the correct batch command is returned (tested at Cmd-return level, not at I/O level)
- Proportionality: 1 happy-path + 1 boundary-condition per action. DLL install wizard gets 2 additional per step (cancel mid-flow, back navigation).

### Task 6: Profile widget and disabled field contract tests

**Depends on**: Task 2
**Status**: ■ complete
**Acceptance**:

- GIVEN the profile widget WHEN iterating all field definitions THEN every field marked disabled has a corresponding test verifying: navigation skips it, input is rejected, and the render output contains "Coming soon"
- GIVEN the profile widget with disabled fields WHEN navigating with arrow keys THEN disabled fields are skipped in both directions, including edge cases (all fields disabled in a group, first field disabled, last field disabled)
- GIVEN the profile widget in editing mode WHEN field values are cycled THEN the profile model is updated and a save command is available
- Proportionality: 1 happy-path + 1 boundary-condition per testable unit. Disabled field navigation gets 3 edge cases (all-disabled group, first disabled, last disabled).

### Task 7: Plan-level freshness checkpoint

**Depends on**: all prior tasks
**Status**: ■ complete
**Acceptance**:

- GIVEN this plan's work has shipped WHEN CHANGELOG.md is checked THEN it has Added entries under [Unreleased] covering the test harness and service interfaces
- GIVEN this plan is otherwise complete WHEN PROGRESS.md is checked THEN it has at least one cycle entry whose What field summarizes the plan and whose Commits field lists the commits this plan produced
- GIVEN this plan is otherwise complete WHEN TODO.md is checked THEN the Tests grade C finding has a corresponding Resolved entry noting the TUI test coverage improvement
- GIVEN this plan resolved HEALTH.md test coverage findings WHEN HEALTH.md is read THEN the TUI test gap is noted as resolved in the PROGRESS.md cycle entry's Discovered field

## Overall Acceptance

- GIVEN the TUI test suite WHEN `go test ./internal/tui/...` is run THEN all tests pass including both the existing utility tests and the new state machine tests
- GIVEN any TUI key binding listed in the help overlay WHEN the corresponding test is checked THEN there exists at least one test verifying its state transition
- GIVEN every profile widget field definition marked disabled WHEN contract tests are checked THEN each has tests verifying navigation skipping, input rejection, and "Coming soon" rendering
- GIVEN the service interfaces WHEN the production TUI is launched THEN behavior is identical to before the plan (no regressions)
- GIVEN the asynchronous command closures WHEN the test suite is examined THEN they are tested at the Cmd-return level (correct message type) not at the I/O level

## Surprises
