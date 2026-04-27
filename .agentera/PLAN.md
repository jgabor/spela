# Plan: Gova-Inspired GUI Seams

<!-- Level: full | Created: 2026-04-27 | Status: active -->
<!-- Reviewed: 2026-04-27 | Critic issues: 14 found, 13 addressed, 1 dismissed -->

## What

Strengthen Spela's GUI using the transferable parts of Gova without adopting Gova. Keep Wails and Svelte, while making desktop calls, event subscriptions, stateful UI behavior, and frontend tests more explicit and replaceable.

## Why

The Gova analysis showed that Spela does not need a new GUI framework. It needs clearer seams around GUI state and desktop integration. Better seams reduce coupling, improve tests, and protect wrapper-first profile behavior.

## Constraints

- Wails and Svelte remain the GUI stack.
- The GUI remains a configuration and inspection surface.
- Steam wrapper launch remains the trustworthy launch path.
- Frontend code must display backend-provided domain decisions, not re-derive them.
- Existing profile inheritance behavior must not regress.
- No new runtime dependency is allowed without approval.
- Use existing dev test tooling unless a dev-only dependency has explicit rationale.
- Native notifications, path picker redesign, and framework replacement are out of scope.
- If execution adds user-facing feature or fix scope, recheck the version-bump need.

## Scope

**In**: Frontend desktop boundary, event subscription behavior, game list behavior, profile editor behavior, DLL operation feedback, GUI behavior tests, artifact maintenance.
**Out**: Gova/Fyne adoption, Wails replacement, GUI visual redesign, launch lifecycle changes, native notification work, overlay work.
**Deferred**: Go backend hot-reload supervisor, native dialogs for path selection, desktop notification affordances.

## Design

Put a narrow GUI-facing boundary between Svelte components and desktop bindings. Treat desktop calls and event streams as replaceable test inputs. Characterize current behavior before refactoring. Move behavior-heavy list, editor, and operation state into focused frontend units. Keep domain orchestration in the existing GUI application boundary. If event payloads are insufficient, adjust the backend GUI boundary without moving domain workflow into frontend code.

## Tasks

### Task 1: Characterize Current GUI Behavior

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN current GUI flows WHEN tests run THEN game list, profile, DLL, error, and launch guidance behavior is captured before refactor.
▸ GIVEN current keyboard behavior WHEN tests run THEN editable controls and global shortcuts are both covered.
▸ GIVEN tests are added WHEN coverage is reviewed THEN cap at 1 pass and 1 fail per flow, plus keyboard and filter edge cases due to branching.

### Task 2: Establish Desktop Command Boundary

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN the GUI requests desktop data WHEN the desktop source is replaced in tests THEN visible results stay equivalent.
▸ GIVEN the backend reports eligibility, inheritance, or launch policy WHEN the frontend renders THEN it displays that decision without redefining it.
▸ GIVEN desktop actions fail WHEN the frontend handles them THEN the same user-visible failure state appears.
▸ GIVEN tests are added WHEN coverage is reviewed THEN cap at 1 pass and 1 fail per command-boundary behavior.

### Task 3: Establish Desktop Event Boundary

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN desktop progress events fire WHEN an operation is active THEN the current stage is visible to the user.
▸ GIVEN an operation completes or fails WHEN progress ends THEN stale progress is cleared.
▸ GIVEN the listening view unmounts WHEN progress events continue THEN no stale listener updates the view.
▸ GIVEN tests are added WHEN coverage is reviewed THEN cap at 1 pass and 1 fail per event-boundary behavior.

### Task 4: Make Game List Behavior Testable

**Depends on**: Tasks 2-3
**Status**: □ pending
**Acceptance**:
▸ GIVEN games have names, profile state, and DLL state WHEN search, filters, and sorting are applied THEN the visible list matches characterized behavior.
▸ GIVEN selection mode is active WHEN batch DLL update runs THEN selected eligible games are attempted and ineligible games are skipped visibly.
▸ GIVEN batch update finishes WHEN some attempts fail THEN the summary distinguishes successes from failures.
▸ GIVEN tests are added WHEN coverage is reviewed THEN cap at 1 pass and 1 fail per behavior, plus two filter/sort edge cases.

### Task 5: Make Profile And DLL Operation Behavior Testable

**Depends on**: Tasks 2-3
**Status**: □ pending
**Acceptance**:
▸ GIVEN a game or default profile loads WHEN the editor renders THEN effective values and inherited intent match characterized behavior.
▸ GIVEN profile or DLL actions fail WHEN the user acts THEN a persistent error appears until dismissed.
▸ GIVEN direct launch is rejected WHEN the user presses launch THEN wrapper-path guidance appears and no success state appears.
▸ GIVEN tests are added WHEN coverage is reviewed THEN cap at 1 pass and 1 fail per behavior boundary.

### Task 6: Verify App-Level GUI Flows

**Depends on**: Tasks 4-5
**Status**: □ pending
**Acceptance**:
▸ GIVEN the desktop boundary is mocked WHEN the app switches games, defaults, options, and help THEN visible state follows the user's action.
▸ GIVEN keyboard shortcuts are used WHEN focus is inside editable controls THEN text entry is not hijacked.
▸ GIVEN keyboard shortcuts are used WHEN focus is outside editable controls THEN pane switching, help, and quit behavior still work.
▸ GIVEN verification runs WHEN frontend, tagged GUI backend, and standard project tests execute THEN they pass without adding a mandatory browser e2e server.

### Task 7: Plan-Level Freshness Checkpoint

**Depends on**: Tasks 1-6
**Status**: □ pending
**Acceptance**:
▸ GIVEN this plan's GUI seam work completes WHEN CHANGELOG.md is checked THEN it has a plan-level entry for completed work.
▸ GIVEN this plan completes WHEN PROGRESS.md is checked THEN it has a plan summary entry listing produced commits.
▸ GIVEN this plan resolves or creates known GUI-seam issues WHEN TODO.md is checked THEN related entries are current and scoped.
▸ GIVEN prior plan state is checked WHEN active planning resumes THEN completed predecessor work is archived and no longer blocks this plan.

## Overall Acceptance

▸ GIVEN Spela's GUI uses desktop bindings WHEN frontend behavior is tested THEN desktop behavior can be replaced without changing user-visible results.
▸ GIVEN users browse games, edit profiles, and run DLL operations WHEN GUI state changes THEN current behavior remains intact.
▸ GIVEN backend profile and DLL semantics exist WHEN the frontend displays them THEN the frontend does not redefine domain meaning.
▸ GIVEN verification runs WHEN the plan completes THEN frontend tests, tagged GUI backend tests, and standard project tests pass.

## Surprises

[Empty; populated by realisera during execution when reality diverges from plan]
