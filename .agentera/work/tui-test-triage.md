# TUI test triage — Task 3 shell redesign

Scratch artifact produced inside Task 3. Categorizes every TUI test function prior
to the rail rewrite into keep / rewrite / delete. One-line reason each.

Totals (by file):

| File | Count | Keep | Rewrite | Delete |
|------|------:|-----:|--------:|-------:|
| cli_smoke_test.go | 1 | 1 | 0 | 0 |
| content_test.go | 23 | 13 | 3 | 7 |
| help_test.go | 19 | 14 | 2 | 3 |
| layout_test.go | 29 | 10 | 14 | 5 |
| profile_widget_test.go | 11 | 11 | 0 | 0 |
| sidebar_test.go | 19 | 15 | 4 | 0 |
| sparkline_test.go | 22 | 22 | 0 | 0 |
| styles_test.go | 11 | 11 | 0 | 0 |
| testhelpers_test.go | 1 smoke (9 sub) | rewrite testLayoutWithGame helper; TestFactories_Smoke kept | | |
| thermal_test.go | 10 | 10 | 0 | 0 |

Plus new file `rail_test.go` with ≥4 tests for rail router state machine.

## content_test.go

- TestContent_NoGame_DLLKeysIgnored — **rewrite**: drop "L" from the key list; the rest
  (i/u/R) still matter under ResourceGames detail for Task 6.
- TestContent_NoGame_TabSwitchIgnored — **delete**: TabDLLs/TabLaunch concept gone.
- TestContent_TabSwitch — **delete**: tab switching removed.
- TestContent_Launch — **delete**: launch surface removed.
- TestContent_Launch_DuplicatePrevented — **delete**: launch surface removed.
- TestContent_Launch_DefaultProfileIgnored — **delete**: launch surface removed.
- TestContent_LaunchMsg_ClearsLaunching — **delete**: launchGameMsg removed.
- TestContent_Update_WithConfirmation — **keep**: DLL update confirmation still lives in
  the games resource detail for Task 6.
- TestContent_Update_ConfirmationCancelled — **keep**.
- TestContent_Update_WithoutConfirmation — **keep**.
- TestContent_Update_NoUpdatesIgnored — **keep**.
- TestContent_Restore_WithConfirmation — **keep**.
- TestContent_Restore_NoBackupIgnored — **keep**.
- TestContent_InstallWizard_Start — **keep**.
- TestContent_InstallWizard_AlreadyOperating — **keep**.
- TestContent_InstallWizard_TypeSelection — **keep**.
- TestContent_InstallWizard_TypeToVersion — **keep**.
- TestContent_InstallWizard_VersionToDownload — **keep**.
- TestContent_InstallWizard_Cancel — **keep**.
- TestContent_InstallWizard_QCancel — **keep**.
- TestContent_DLLUpdateMsg_ClearsOperating — **keep**.
- TestContent_DLLRestoreMsg_ClearsOperating — **keep**.
- TestContent_DLLUpdatesCheckedMsg — **keep**.

## help_test.go

- TestContextKeys_SidebarFocused — **rewrite**: "sidebar" becomes "rail" + games
  sub-pane.
- TestContextKeys_SearchFocused — **keep**: search still lives under ResourceGames.
- TestContextKeys_SelectMode — **keep**: multi-select still under ResourceGames.
- TestContextKeys_ContentFocusedWithGame — **rewrite**: drop the "L"/"launch"
  assertion; rest (u/R/i) valid.
- TestContextKeys_ContentFocusedNoGame — **keep**.
- TestContextKeys_DisabledRestoreNoBackup — **keep**.
- TestContextKeys_DisabledUpdateNoUpdates — **keep**.
- TestContextKeys_DisabledUpdateBusy — **keep**.
- TestContextKeys_DisabledLaunchLaunching — **delete**: launching state removed.
- TestContextKeys_HintsDisabled — **keep**.
- TestReasonForUpdate — **keep**.
- TestRenderContextBar_EmptyKeys — **keep**.
- TestRenderContextBar_ZeroWidth — **keep**.
- TestRenderContextBar_ContainsGlobalKeys — **keep**.
- TestRenderContextBar_DisabledKeyShowsReason — **keep**.
- TestRenderContextBar_Truncation — **keep**.
- TestRenderContextBar_WideEnoughNoTruncation — **keep**.
- TestRenderContextBar_EnabledVsDisabledStyling — **keep**.
- TestRenderContextBar_GlobalKeysOnly — **keep**.

## layout_test.go

- TestLayout_HelpToggle — **keep**.
- TestLayout_HelpEscCloses — **keep**.
- TestLayout_HelpQCloses — **keep**.
- TestLayout_HelpBlocksOtherKeys — **rewrite**: assert help blocks rail navigation (j/k).
- TestLayout_TabTogglesFocus — **keep**: tab still toggles rail ↔ resource pane.
- TestLayout_CtrlCQuits — **keep**.
- TestLayout_QFromSidebarQuits — **rewrite**: rename field to railFocused; same semantics.
- TestLayout_F5TogglesDensity — **keep**.
- TestLayout_F11TogglesFocused — **keep**.
- TestLayout_CtrlFActivatesSearch — **rewrite**: ctrl+f now focuses the games resource's
  search; rail gets deselected (rebind decision).
- TestLayout_OptionsModal — **keep**: o from rail opens options.
- TestLayout_OptionsFromContentIgnored — **keep**: o from resource pane ignored.
- TestLayout_RescanFromSidebar — **rewrite**: rescan key moves from `r` to `ctrl+r`
  (displaces sidebar-specific `r`; documented in help); test renamed + rekeyed.
- TestLayout_RescanFromContentIgnored — **rewrite**: same key change.
- TestLayout_JumpKeys_WithGame — **rewrite**: 1-4 now select Resource not ContentTab;
  rail focus stays on rail (doesn't jump to content).
- TestLayout_JumpKey1_FocusesSidebar — **rewrite**: 1 selects ResourceGames; rail
  stays focused.
- TestLayout_JumpKey1_IgnoredInFocusedMode — **delete**: densityMode focus still exists
  but 1-4 work as rail hotkeys regardless of density (no ergonomic reason to disable).
- TestLayout_JumpKeys_NoGameIgnored — **delete**: 1-4 no longer require a game — they
  select resources.
- TestLayout_QFromContent_SingleEntry_ReturnsSidebar — **rewrite**: q from resource
  returns to rail.
- TestLayout_EscFromContent_SingleEntry_ReturnsSidebar — **rewrite**: same.
- TestLayout_NavStack_DeepPop — **delete**: no planned sub-stack inside resources for
  Task 3; can be reintroduced in Task 4 if needed.
- TestLayout_NavStack_RootCannotBePoped — **delete**: same as above.
- TestLayout_ModalInterceptsInput — **rewrite**: updated to use rail instead of sidebar.
- TestLayout_ModalClosesOnCancel — **keep**.
- TestLayout_BatchMenu_EscCloses — **keep**: batch menu still reachable from
  ResourceGames multi-select.
- TestLayout_BatchMenu_Navigation — **keep**.
- TestLayout_BatchMenu_EnterExecutes — **keep**.
- TestLayout_BatchMenu_BlocksGlobalKeys — **keep**.
- TestLayout_WindowSize — **keep**.

## profile_widget_test.go

All 11 tests **keep** — profile widget keeps its current shape; Task 5 rewires
rendering but not the widget's state machine.

## sidebar_test.go

- TestSidebar_CursorDown — **keep**.
- TestSidebar_CursorUp_ClampsAtZero — **keep**.
- TestSidebar_CursorDown_ClampsAtEnd — **keep**.
- TestSidebar_JK_Navigation — **keep**.
- TestSidebar_DLLFilter — **keep**.
- TestSidebar_ProfileFilter — **rewrite**: `p` is displaced by Task 5 pin-field;
  profile filter now on `P` (shift+p). One-key change.
- TestSidebar_SortCycles — **keep**.
- TestSidebar_ClearFilters — **keep**.
- TestSidebar_SpaceEntersSelectMode — **rewrite**: no implicit "default profile" item in
  the games sidebar anymore — cursor starts at 0 on a game.
- TestSidebar_SpaceTogglesSelection — **rewrite**: same.
- TestSidebar_SpaceOnDefaultProfileIgnored — **delete**: default profile is no longer in
  the games sidebar; it is a peer resource on the rail.
- TestSidebar_SelectAll_DeselectAll — **keep**.
- TestSidebar_EscExitsSelectMode — **keep**.
- TestSidebar_EnterInSelectMode_TriggersBatch — **keep**.
- TestSidebar_EnterConfirmsGame — **rewrite**: cursor no longer offset by default
  profile item.
- TestSidebar_EnterOnDefaultProfile — **delete**: default profile is its own resource.
- TestSidebar_SlashActivatesSearch — **keep**.
- TestSidebar_SearchEscBlurs — **keep**.
- TestSidebar_SearchEnterBlurs — **keep**.

## sparkline_test.go (22 tests)

All **keep** — pure-function tests, untouched by shell rewrite.

## styles_test.go (11 tests)

All **keep** — Task 1 theme assertions.

## thermal_test.go (10 tests)

All **keep** — pure-function tests, untouched.

## testhelpers_test.go

TestFactories_Smoke (9 subtests) — **keep** with adjusted `testLayoutWithGame` helper
(game selection flow uses the new resource router). One subtest "testLayoutWithGame
selects the game" updated to assert `ResourceGames` selected + game picked.

## New tests for rail router (rail_test.go)

- TestRail_InitialState — rail is focused; ResourceGames is the initial selection;
  cursor at 0.
- TestRail_HotkeySelectsResource (4 subtests: 1/2/3/4) — pressing each hotkey sets
  the active resource; rail stays focused.
- TestRail_JKMovesCursor — j moves down, k moves up, clamps at ends.
- TestRail_EnterConfirmsCursor — after j/k, enter sets active resource to the
  cursor's resource.
- TestRail_HotkeyStaysOnRail — after pressing 2, railFocused remains true.
- TestRail_TabToggleMovesToResourcePane — tab moves focus off rail; tab again back.
