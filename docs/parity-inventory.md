# spela parity inventory

Date: 2026-10-25. Written by research agents that read the code and docs read only, then assembled without edits. Labels follow mejl's method: (verified: path:line), (inferred), (unverified). Paths without a repo prefix are relative to the repo the section covers. This is the contract for the Rust rewrite: what the Go CLI and TUI do today, which parts are contracts, which are bugs, and which are GUI only. Four readers covered every non-GUI Go file. A completeness critic then checked their coverage and spot-checked 30 claims, of which 26 held.

## Corrections from the completeness check

These override the sections below where they disagree.

- spela-cli-launch 3.1: scan prints `Games with DLSS/FSR/XeSS: <M>` when any game has DLSS, FSR or XeSS. 3.7: `dll list` without an argument lists games with DLSS, FSR or XeSS. **Correction:** Both call db.GamesWithDLSS(), which keeps only games with a dlss, dlssg or dlssd DLL and drops tool names. A game whose only DLLs are XeSS or FSR is never counted or listed, although the printed label says DLSS/FSR/XeSS. With a game argument, `dll list <game>` prints all of that game's DLLs, whatever their type. `Found N games` counts db.Games, tools included. Evidence: internal/game/database.go:173-184 (HasDLSS || HasDLSSG || HasDLSSD); cmd/spela/commands/scan.go:44-46 and 104-118; cmd/spela/commands/dll.go:58-70. The dll-overlay report states it correctly at its section 2.11.
- spela-dll-overlay 4.6: the position value is not validated by the CLI flag, though the profile field lists allowed values. **Correction:** `overlay set --position` goes through profileChanges.set, which calls profile.Set, and Set rejects any value outside the descriptor's allowed list. An invalid position fails with `overlay.position: unsupported value "x" (valid: , top-left, top-right, bottom-left, bottom-right)` (exact text inferred), and nothing is written. Evidence: cmd/spela/commands/overlay.go:97-99; cmd/spela/commands/inheritance_render.go:151-175; internal/profile/fields.go:94 (allowed list), 192 and 200-205 (check and wrapping).
- spela-cli-launch 3.9: `--shader-cache-path` with `default` clears it, while `--power-mizer default` storing an empty override is flagged as a bug. **Correction:** The two flags behave the same way. Both map `default` to "" and call changes.set, which writes an empty value and marks an override. Neither resets the field, so the game stops following All games for that field. Either both are bugs or both are intended. Only `cpu set --governor default` and `--smt default` call changes.reset. Evidence: cmd/spela/commands/gpu_set.go:100-106 and 116-122; cmd/spela/commands/cpu_set.go:72-81, 89-90; cmd/spela/commands/inheritance_render.go:151-161.
- spela-cli-launch 3.13: `denylist show` prints `Denied games (anti-cheat):`, its entries, then `User-denied games:` with that section's entries. **Correction:** `User-denied games:` is printed only when overrides.denied is non-empty, preceded by a blank line. LoadOverrides errors are ignored. `[FORCE-ALLOWED]` appears only on deny-list entries. Evidence: cmd/spela/commands/denylist.go:71-96 (the `if overrides != nil && len(overrides.Denied) > 0` guard at line 88).
- spela-dll-overlay 3: the built-in list includes 'PUBG 578080', and each entry has a reason naming the anti-cheat. **Correction:** The entry name is `PUBG: Battlegrounds`, and its reason is the generic `Anti-Cheat`, not a named anti-cheat. The other reasons are Easy Anti-Cheat, RICOCHET Anti-Cheat or BattlEye. The names matter because `denylist show` prints them. Evidence: internal/denylist/denylist.go:29-42 (PUBG line).
- spela-cli-launch 3.8: `--sr-preset` accepts default, auto, A-M (quoting the flag help). **Correction:** The help text says A-M, but the allowed list is "", default, auto, A-F, J-M. G, H and I are rejected by profile.Set. `--rr-preset` passes NormalizeSRPreset, so `auto` is accepted by the flag and then rejected, because rr_preset has no auto. This is a mismatch between help text and code, not something to copy. Evidence: cmd/spela/commands/dlss.go:52, 55, 126-145; internal/profile/fields.go:47, 50, 204.

## Contents

1. CLI surface and launch path
2. Domain packages
3. DLL subsystem, denylist, overlay and release check
4. TUI
5. Additions from the completeness check

## Spela CLI surface and launch path: parity contract for the Rust rewrite

### Scope and method

**Scope**
- All of `cmd/spela/**`, with GUI parts noted but excluded.
- The packages `internal/launcher`, `internal/privilege`, `internal/lock`, `internal/logging`, `internal/xdg` and `internal/config`.
- `data/polkit`, `magefile.go` and `tests/contracts`.
- The docs `docs/releasing.md`, `docs/configuration.md`, `docs/coverage.md`, README, CHANGELOG (skimmed) and AGENTS.md.

**Supporting files**
- Read: internal/profile/apply.go, internal/gpu/nvidia.go, internal/cpu/cpu.go, internal/env/env.go, internal/overlay/setup.go, internal/game/database.go, pkg/aur, .github/workflows and scripts/release-notes.sh.
- Skimmed: internal/display/vrr.go and internal/steam/refresh.go.
- Read for cobra behaviour: the cobra v1.10.2 source in the Go module cache.
- Cross-checked: open TODOs in TODO.md and `.agentera/entities/todo`.

**Labels**
- (verified: path:line) means the code or doc at that line was read.
- (inferred) means it follows from the code but was not run.
- (unverified) means it was not checked.
- Nothing was built or run.

**Classification tags**
- **[contract]**: preserve in Rust.
- **[bug]**: current bug; do not port as is.
- **[gui]**: GUI only, excluded.
- **[intent]**: port the intent, replace the implementation.

---

### 1. Findings that change the Rust design

1. **Wrapper detection pre-empts subcommands [bug].** It runs on the raw argv before cobra (verified: cmd/spela/main.go:121-128). See section 5. The TODO entry is cridhyzjcj.
2. **Direct `launch` is not usable for a real launch [bug].** See section 3.4.
3. **Wrapper game detection can choose the wrong game [bug].** It falls back to an InstallDir substring match over an unordered map (verified: internal/launcher/launcher.go:374-380).
4. **Exit codes are 0 or 1 only [contract for the 0/1 split; bug for child status].** The wrapped game's exit status is lost (verified: main.go:122-126).
5. **Privileged restore is lossy, and a failed apply has no restore [bug].** TODO entries iynrkonane and wzpctclhaa. See section 7.
6. **The apply-profile protocol is internal to one binary.** The contract is the polkit action id, the /usr/bin/spela path and the single prompt per apply. The flag form can be redesigned, but see open question 6 about restores across the binary switch.
7. **Live hardware commands are not batched.** A typo in `cpu smt` disables SMT [bug]. See section 3.10.
8. **Signal coverage has gaps [bug].** See section 6.7.

---

### 2. Process entry, dispatch, errors and exit codes

#### 2.1 main() order [contract for order of effects]
- `release.Version = version` (verified: main.go:116). `version` defaults to "dev" and is set by the linker flag `-X main.version=` (verified: main.go:21, magefile.go:35-41).
- `config.ApplyLogLevel()` loads config.yaml and sets the slog level. A load error keeps the default level (verified: internal/config/loglevel.go:16-22).
- `gpu.Init()` initialises NVML for every invocation, including wrapper mode and `config show`. `defer gpu.Shutdown()` is skipped on `os.Exit` (verified: main.go:118-119, 125, 131).
- Wrapper check: `launcher.IsWrapperMode(os.Args[1:])` runs `runWrapperMode`. On error it prints the error to stderr with no `Error:` prefix and exits 1 (verified: main.go:121-127).
- Otherwise `rootCmd.Execute()` runs, and any error exits 1 (verified: main.go:130-132).

#### 2.2 Root command [contract]
- `Use: "spela [command]"`, `Short: "Linux gaming optimization tool"`, `Long: "Spela is a Linux gaming optimization tool that combines DLSS/DLL management with comprehensive gaming environment setup."` (verified: main.go:23-29).
- Registered commands: scan, list, show, launch, profile, config, dll, dlss, gpu, cpu, proton, overlay, tui, gui, denylist and the hidden apply-profile (verified: main.go:31-48).
- With no arguments, the root help goes to stdout with exit 0. The test pins "Linux gaming optimization tool", "Available Commands:", "profile" and "config" (verified: main_test.go:120-135; main.go:50-53).
- Cobra adds `--version`/`-v`, which prints `spela version <v>` (verified: cobra@v1.10.2/command.go:1252, 2064-2072). It also adds `help` and `completion` (inferred: nothing disables the default completion command; grep found no CompletionOptions).
- Help lists commands in alphabetical order (inferred: cobra default sorting).
- A non-runnable parent such as `spela dll` or `spela dll bogus` prints help and exits 0 (verified: cobra command.go:955-956 returns flag.ErrHelp).
- The `runRoot` wrapper branch (main.go:55-57) is unreachable. Cobra's legacyArgs returns `unknown command "x" for "spela"` before RunE runs (verified: cobra args.go:28-39) [bug: dead code].

#### 2.3 Error format [contract]
- No command sets SilenceUsage or SilenceErrors (verified: grep in cmd/ and internal/ found none).
- On a RunE error, cobra prints `Error: <err>` through PrintErrln to stderr, then the failing command's usage through `c.Println`, which writes to `OutOrStderr()`. Both reach **stderr** in production (verified: cobra command.go:1160-1166, 1435-1442).
- main_test.go:99-115 asserts that "Usage:" appears on stdout only because the test calls `rootCmd.SetOut(os.Stdout)`. That is test wiring, not production behaviour (see the conflicts).
- Pinned error strings:
  - `unknown config key: not_a_key` (main_test.go:115; contract_test.go:28).
  - `unknown config key: theme` (contract_test.go:32).
  - `unsupported log level: verbose` (contract_test.go:36).
  - `accepts 1 arg(s), received 0` (contract_test.go:88).
  - `game not found: missing` (contract_test.go:81).
- Exit codes: 0 on success and on help, 1 on any error (verified: main.go:126, 131). There are no other codes.
- `tui` with a malformed config prints a recovery message and returns nil, so it exits 0 (verified: commands/tui.go:34-38; supported_commands_test.go:82-104).

---

### 3. Command inventory

The common shapes below repeat across commands:
- **Game argument:** `<game>` resolves through `db.FindGame` (section 4).
- **Set commands:** each prints `No changes specified. Use --help to see available options.` when no flag is given.
- **Reset commands:** each prints one of three messages (section 3.16).

#### 3.1 `scan` [contract]
- **Flags:** `--json` bool, default false (verified: commands/scan.go:24-26).
- **Behaviour:** loads config, runs `steam.Rescan(cfg)` and saves the database (verified: scan.go:29-37).
- **Errors:** `failed to load config: %w` and `scan failed: %w`.
- **Text output:**
  - `Found <N> games`.
  - If any game has DLSS, FSR or XeSS: `Games with DLSS/FSR/XeSS: <M>`, then `  - <Name> (<AppID>)` for each, and `      <dll>: <version|unknown>` per DLL (verified: scan.go:48-62).
  - Games appear in map order, which is unsorted (inferred).
- **JSON:** `db.Games` is an object keyed by the AppID as a decimal string, holding Game objects. Steam tools are not filtered (verified: scan.go:40; supported_commands_test.go:127 pins `"42"`).

#### 3.2 `list` [contract]
- **Flags:** `--with-dlls` bool and `--json` bool (verified: list.go:30-33).
- **Behaviour:** LoadDatabase, then `steam.RefreshIfNeeded`. That rescans when the database is empty **or** `rescan_on_startup` is true, which is the default (verified: list.go:36-48; internal/steam/refresh.go:31-39; config.go:54). Tool names are filtered (verified: internal/game/database.go:162-171), and games are sorted by Name.
- **Text output:** `<Name> (<AppID>)`. With `--with-dlls`, also `  <dll>: <version|unknown>` (verified: list.go:69-80).
- **Empty database:** prints `No games found. Run 'spela scan' first.`. It does this **even with --json** [bug] (verified: list.go:51-54).
- **JSON:** an array of Game objects sorted by Name. A test pins `"AppID": 7` (verified: error_contracts_test.go:77).
- **Errors:** `failed to load game database: %w`, `failed to load config: %w` and `failed to refresh game database: %w`.

#### 3.3 `show <game>` [contract]
- **Arguments:** ExactArgs(1). **Flags:** `--json` bool (verified: show.go:18-28).
- **Behaviour:** LoadDatabase, then RefreshIfNeeded (verified: show.go:31-43).
- **Error:** `game not found: <q>`.
- **Text output:**
  - `Name:  <Name>`, `App ID: <id>`, `Install:  <dir>`, and `Prefix:  <path>` when set.
  - `Detected DLLs:` with `  <name>: <version|unknown>` and `    <path>`.
  - `Profile: yes|no` (verified: show.go:59-83; display_and_dll_boundaries_test.go:30-36).
- **JSON:** a single Game object.

#### 3.4 `launch <game>` [contract for dry-run and rejection; bug for direct launch]
- **Arguments:** MinimumNArgs(1). **Flags:** `--game-id` uint64, default 0, "Launch by Steam App ID"; `--dry-run` bool (verified: launch.go:27-38).
- **Lookup:** `GetGame(--game-id)` when the flag is non-zero, otherwise `FindGame(args[0])`. The error is a bare `game not found` (verified: launch.go:48-56).
- **Profiles:** `LoadLaunchProfiles` returns the raw profile, the defaults and the effective profile. Errors are fatal here: `failed to load profile: %w` (verified: launch.go:58-61, 93-106).
- **`--dry-run`:** `PrintLaunchSummary` only, with no mutation (verified: launch.go:63-65, 87-90). The output format is in section 6.9.
- **Rejection:** non-dry-run with `--game-id` or exactly one argument fails with:
  `direct Steam URI launch cannot track the game lifetime or cleanup coverage; set the game's Steam launch options to `spela %command%` instead` (verified: launch.go:67-69). Tests pin "spela %command%" and "cannot track the game lifetime or cleanup coverage" (verified: launch_test.go:115-136).
- **Two or more arguments without --game-id [bug]:**
  - `l.Launch(args)` executes `args[0]`, the game query, as the program (verified: launch.go:83).
  - The summary prints `strings.Join(args, " ")` as the command (verified: launch.go:113-115).
  - So `spela launch 1091500 /path/game` tries to exec `1091500` (inferred).
  - It does not take the lock, does not refresh the database, and sets no wrapper env.
- **Messages:** `Launching <Name> with profile...` when an effective profile exists, which includes defaults only; otherwise `Launching <Name> (no profile)...` (verified: launch.go:78-82).

#### 3.5 `profile` [contract]
- **`list`:**
  - Prints `No profiles found.` when there are none (verified: profile.go:78-81).
  - Otherwise each line is `<game name or AppID> (<AppID>): <profile Name or "custom">`, sorted by name (verified: profile.go:83-110).
- **`create <game>`:**
  - Prints `Created profile for <Name>` (verified: profile.go:115-136).
  - Errors: `profile already exists for <Name>` and `failed to save profile: %w`.
  - The new profile's Name is the game name.
- **`show <game>` (`--json` bool):**
  - **Text:** `Profile for <Name>:` then a blank line. Without a profile it adds `No profile for <Name>; every value follows All games or the system default.` (verified: profile.go:156-165; inheritance_render.go:52-56).
  - **Sections:** Proton, DLSS, GPU, CPU, Overlay, in that order (verified: inheritance_render.go:58-67).
  - **Rows:** `  <label>  <value>  <source>`, with aligned columns. The source is dim, or accented when the game sets the value itself (verified: inheritance_render.go:72-107).
  - **Source words pinned by tests:** `this game · All games: Off`, `this game · same as All games`, `from All games`, `system default`, `100 MHz`, `60%` (verified: profile_vocabulary_test.go:63-76). Every descriptor label is printed, and no retired word appears (verified: profile_vocabulary_test.go:58-82).
  - **`--json`:** prints the **raw stored** profile with keys `Name`, `DLSS`, `GPU`, `CPU`, `Proton`, `Overlay` and `Overrides`. It errors with `no profile for <Name>` when no profile exists (verified: profile.go:144-153; contract_test.go:63-78).
  - Show never creates a profile (verified: profile_vocabulary_test.go:99-101).
- **`reset <game>`:** clears every override and keeps the file and its name (verified: profile.go:169-205; profile_vocabulary_test.go:150-189). It prints one of:
  - `No profile for <Name>; it already follows All games for all settings.`
  - `<Name> already follows All games for all settings.`
  - `<Name> now follows All games for all settings (<n> cleared).`
- **`delete <game>`:** prints `Deleted profile for <Name>`, even when no profile existed [bug] (verified: profile.go:218-222; internal/profile/storage.go:237-245).

#### 3.6 `config` [contract]
- **`show`:** prints `yaml.Marshal(cfg)` of the defaults merged with the file, including `theme` (verified: commands/config.go:36-49). Load errors pass through unwrapped.
- **`set <key> <value>`:**
  - ExactArgs(2).
  - The key must exist and be CLI-visible, otherwise the error is `unknown config key: <key>` (verified: config.go:57-61).
  - The value goes through `option.Set`, which validates it, then the whole config is saved.
  - Prints `Set <key> = <value as typed>` (verified: config.go:62-71; contract_test.go:20).
  - CLI-visible keys: `log_level`, `shader_cache` and `check_updates` only (verified: internal/config/options.go:86-88; options_test.go:19-33). See section 11.

#### 3.7 `dll` [contract]
- **`list [game]`:**
  - Without an argument it lists games with DLSS, FSR or XeSS, unsorted. With one it lists that game.
  - Prints `No games with DLSS/FSR/XeSS DLLs found.` when there are none.
  - Lines are `<Name> (<AppID>)`, `  <dll>: <ver|unknown>` and `    <path>`, with no colour (verified: dll.go:54-90).
- **`check-updates`:**
  - Calls `dll.GetManifest(false, "")`, which may use the network.
  - For each game with a newer version it prints `<Name> (<AppID>)` and `  <dll>: <cur|unknown> -> <latest>`. Otherwise it prints `All DLLs are up to date.`.
  - Error: `failed to fetch manifest: %w` (verified: dll.go:92-145; supported_commands_test.go:263 pins `3.8.10 -> 3.9.0`).
- **`update <game> <dll-type>`:**
  - ExactArgs(2). The type is lowercased.
  - Progress: `Downloading <type> <version>...`, then `\rDownloading: <x.x>%` or `\rDownloading: <n> bytes`.
  - Results:
    - `Failed <path>: <err>` per failure.
    - `DLL update result: <u> updated, <n> unchanged, <f> failed`.
    - `<type> DLLs are already at the latest version`.
    - `Updated <n> <type> DLLs to version <v>` or `Updated <type> DLL to version <v>`.
  - Errors: `game does not have a <type> DLL` and `<f> DLL update(s) failed: <joined>` (verified: dll.go:147-210).
  - README lists the types dlss, dlssg, dlssd, xess and fsr (verified: README.md:126-127; internal/game/game.go:11-15).
- **`restore <game>`:** prints `Restored original DLLs for <Name>`. A missing backup errors with text containing "no backup" (verified: dll.go:212-231; display_and_dll_boundaries_test.go:98).

#### 3.8 `dlss` [contract]
- **`show <game>` (`--json`):**
  - Text: `DLSS configuration for <Name>:`, a blank line, the no-profile note, then rows (verified: dlss.go:100-105).
  - JSON: the **resolved** DLSS struct. `"SRMode": "quality"` is pinned (verified: dlss.go:91-97; supported_commands_test.go:133).
- **`set <game>` flags** (verified: dlss.go:51-61):
  - `--sr-mode` string.
  - `--sr-preset` string (default, auto, A-M).
  - `--sr-model-preset` string, deprecated. It prints `warning: --sr-model-preset is deprecated; use --sr-preset instead` to stderr (verified: dlss.go:131-135).
  - `--rr-mode` string.
  - `--rr-preset` string, normalised by NormalizeSRPreset (verified: dlss.go:143).
  - `--rr-override` bool-string.
  - `--fg` bool-string.
  - `--fg-override` bool-string.
  - `--fg-indicator` bool, applied only when Changed.
  - `--multi-frame` int, default -1 as a sentinel; values of 0 or more apply.
  - `--indicator` bool, applied only when Changed.
- **Side effects:** sr-mode, sr-preset and sr-model-preset also set `sr_override=true`. rr-mode and rr-preset set `rr_override=true`. `--fg` and `--multi-frame` set `fg_override=true` (verified: dlss.go:121-175).
- **Messages:** `Updated DLSS configuration for <Name>`. A bad bool gives `--rr-override: invalid bool "x" (use true/false)`.
- **`reset <game> <field>`:** fields per the help are sr_mode, sr_preset, sr_override, rr_mode, rr_preset, rr_override, fg_enabled, fg_override, multi_frame, indicator and fg_indicator (verified: dlss.go:70-83).

#### 3.9 `gpu` [contract, with bugs noted]
- **`info`:** prints these lines (verified: commands/gpu.go:39-79; hardware_read_commands_test.go:53-75):
  - `GPU:  <name>`, `Driver:  <v>`, `VRAM:  <mem>`, `Temp:  <t>`.
  - `Power:  <draw> / <limit> limit (<range> range)`, or the nvidia-smi fallback `Power:  <p>`.
  - `Core:`, `Mem:`, `Fan:` and `Load:` when present.
  - Error: `failed to get GPU info: %w`.
- **`reset`:** live. It calls `gpu.ResetClocks`, which is NVML when root, otherwise two pkexec nvidia-smi calls (`-rgc` and `-rmc`). Prints `GPU clocks reset to default`. Error: `failed to reset clocks: %w` (verified: gpu.go:81-87; internal/gpu/nvidia.go:46-57).
- **`set <game>` flags** (verified: gpu_set.go:41-48):
  - `--clock-offset` int, "GPU core clock offset in MHz".
  - `--memory-offset` int.
  - `--power-limit` int, watts.
  - `--fan-speed` int, "(0 to clear)".
  - `--power-mizer` string (adaptive, max).
  - `--shader-cache` bool-string.
  - `--shader-cache-path` string; `default` clears it.
  - `--threaded-opt` bool-string.
- **How values are stored:**
  - The integer flags apply when Changed, so a value of 0 is stored as an override of 0, against the "0 to clear" help [bug] (TODO rgddfoguos, bdnaynbvan) (verified: gpu_set.go:84-98).
  - `--power-mizer default` stores an empty override instead of resetting [bug] (verified: gpu_set.go:100-106).
  - Prints `Updated GPU profile for <Name>`.
  - There is no `--vrr` flag. gpu.vrr can only be set through the TUI or YAML, but profile-reset accepts it (inferred: resolveProfileField at inheritance_render.go:127-144).
- **`show <game>` (`--json`):** prints `GPU profile for <Name>:`. JSON is the resolved GPU struct, pinned with `"ClockOffset": 150`, and it works without a profile (verified: gpu_set.go:145-166; profile_vocabulary_test.go:104-119). KDE VRR shows inheritance and `unset` (verified: vrr_test.go:10-40).
- **`profile-reset <game> <field>`:** the help lists clock_offset, memory_offset, power_limit, fan_speed, power_mizer, shader_cache, shader_cache_path and threaded_optimization (verified: gpu_set.go:57-69).

#### 3.10 `cpu` [contract, with bugs noted]
- **`info`:** prints these lines (verified: commands/cpu.go:71-125; hardware_read_commands_test.go:31):
  - `CPU:  <model>`, `Cores:  <n>`.
  - `Governor:  <g> (<a, b> available)`.
  - `SMT:  enabled|disabled`.
  - `Freq:  <n> MHz avg`, `Load:  <n>%`, `RAM:  <used> / <total> MB`.
  - `SCX:  active|inactive`, shown when scx_loader is on PATH.
- **`governor [governor]`:**
  - With no argument: `Current governor: <g>` and `Available: [performance powersave]`. The second line is Go's `%v` slice format, pinned by a test (verified: cpu.go:131-144; hardware_read_commands_test.go:32).
  - With an argument it sets the governor live. As root it writes sysfs; otherwise it runs `pkexec tee` once per CPU (verified: internal/cpu/cpu.go:84-100).
  - Messages: `Governor set to <g>` and `failed to set governor (may need root): %w`.
- **`smt [on|off]`:**
  - With no argument: `SMT: true|false`.
  - Setting it enables SMT only for `on`, `true` or `1`; **any other value disables it** [bug] (verified: cpu.go:157-178).
  - Messages: `SMT enabled|disabled` and `failed to set SMT (may need root): %w`.
- **`set <game>`:**
  - `--governor` is checked against sysfs scaling_available_governors before saving; `default` resets it (verified: cpu_set.go:72-81; cpu_validation.go:11-19; cpu_validation_test.go:21-60).
  - `--smt` accepts on, true, off, false or default, else `invalid SMT value: <v> (use on, off, or default)` (verified: cpu_set.go:83-94).
  - Prints `Updated CPU profile for <Name>`.
- **`show <game>` (`--json`):** prints `CPU profile for <Name>:`. JSON is resolved and pins `"Governor": "performance"`.
- **`profile-reset <game> <field>`:** the help lists governor, smt and affinity. Affinity has no setter, and fields.go marks it "Not used yet" (verified: cpu_set.go:46-57; internal/profile/fields.go:91).

#### 3.11 `proton` [contract]
- **`set <game>`:** flags `--hdr`, `--wayland`, `--ngx-updater` and `--vkd3d-heap`, all bool-strings. Prints `Updated Proton profile for <Name>`. Errors name the flag, for example `--vkd3d-heap: invalid bool "maybe" (use true/false)` (verified: proton.go:75-147; proton_test.go:76-98).
- **`show <game>` (`--json`):**
  - Text: `Proton profile for <Name>:` then rows.
  - When the resolved vkd3d_heap is true, it also prints the compatibility notice, indented 4 spaces and dim, for example `⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: ...)` (verified: proton.go:153-180; proton_test.go:194-259).
  - JSON is resolved and pins `"EnableHDR": true`.
- **`reset <game> <field>`:** fields hdr, wayland, ngx_updater and vkd3d_heap. An unknown field errors with the suffix ` (valid: hdr, wayland, ngx_updater, vkd3d_heap)` (verified: proton.go:87-89).

#### 3.12 `overlay` [contract; most fields are stored only]
- **`set <game>` flags:**
  - Bool-strings: `--enabled`, `--show-fps`, `--show-frametime`, `--show-cpu`, `--show-gpu`, `--show-vram`.
  - Strings: `--position` and `--toggle-key`.
  - Prints `Updated overlay profile for <Name>` (verified: overlay.go:47-155).
- **`show <game>` (`--json`):** prints `Overlay profile for <Name>:`. JSON is resolved and pins `"Position": "bottom-right"`.
- **`reset <game> <field>`:** fields enabled, position, show_fps, show_frametime, show_cpu, show_gpu, show_vram and toggle_key.
- **Effect at launch:** only `enabled` and `position` are used, for IPC setup (verified: launcher.go:198-217). show_* and toggle_key reach no consumer outside the profile, TUI, GUI and command code (verified: grep, see coverage).

#### 3.13 `denylist` [contract]
- **`show`:** prints `Denied games (anti-cheat):`, then `  <Name> (<id>): <reason>` with ` [FORCE-ALLOWED]` where it applies. It then prints `User-denied games:` with that section's entries (verified: denylist.go:59-84).
- **`check <game>`:** prints either:
  - `<name> is DENIED: <reason>` and `Use 'spela denylist allow' to force-allow (not recommended)`, or
  - `<name> is allowed for DLL swapping` (verified: denylist.go:86-115).
- **`allow <game>`:** prints `Force-allowed <name> (<id>)` and `WARNING: Swapping DLLs in anti-cheat games may result in bans!` (verified: denylist.go:117-144).
- **`deny <game>` (`--reason` string, default `user-specified`):** prints `Denied <name> (<id>): <reason>` (verified: denylist.go:146-177).
- **Game argument:** FindGame first. Then an unknown **numeric** ID is accepted, with the name being the numeric text. Otherwise `game not found: <q>` (verified: denylist.go:92-104; error_contracts_test.go:89-95).

#### 3.14 `tui` [contract]
- Takes the lock first; an error is `spela is already running (PID: N)`.
- Then LoadDatabase and config.Load. On a config load error it prints this to stderr and returns nil, so it exits 0 without usage (verified: commands/tui.go:23-44; supported_commands_test.go:82-104):
  `Cannot load Spela configuration <path>: <err>` and `Fix the YAML or remove the file to restore defaults.`
- Then RefreshIfNeeded and `tui.Run(db)`.

#### 3.15 `gui` [gui]
- The `gui` command takes the lock and runs `gui.Run()` (verified: commands/gui.go:10-24).
- main_wails.go and main_wails_test.go are the `wails` build-tag entry point; its exit contract is error to stderr with code 1 (verified: cmd/spela/main_wails.go:14-27; main_wails_test.go:11-20).
- wails.json is GUI build configuration.
- Excluded. The Rust binary needs a decision on what `spela gui` does (open question).

#### 3.16 Shared behaviour for set and reset [contract]
- **Bool-strings (profile setters):** true, True, TRUE, 1, yes, on / false, False, FALSE, 0, no, off. Anything else gives `invalid bool "<raw>" (use true/false)` (verified: commands/bool_flag.go:7-15).
- **Field names:** `-` becomes `_`. Aliases: proton.hdr to enable_hdr, proton.wayland to enable_wayland, proton.ngx_updater to enable_ngx_updater, gpu.threaded_opt to threaded_optimization, dlss.fg to fg_enabled. The field must belong to the named subsystem (verified: inheritance_render.go:127-144).
- **Reset messages:** an unknown field gives `unknown <SUB> field "<f>"`. Otherwise one of (verified: inheritance_render.go:177-212; error_contracts_test.go:24-63):
  - `No profile for <Name>; <Label> already follows All games.`
  - `<Name>: <Label> already follows All games.`
  - `<Name>: <Label> now follows All games.`
- **Writes:** setters go through `profile.Mutate` and fill `Name` with the game name when it is empty (verified: inheritance_render.go:163-175).
- **Flag state:** flags are package globals. Only tests are affected.

#### 3.17 `apply-profile` (hidden)
See section 7.

---

### 4. AppID versus name resolution [contract]
- **`FindGame(query)`:**
  - If the query parses as a uint64 and that AppID is in the database, that game is returned.
  - Otherwise the first game whose Name equals the query exactly, case-sensitive, is returned. Map iteration makes duplicate names non-deterministic (inferred).
  - There is no fuzzy, prefix or case-insensitive matching. Tool entries are not filtered (verified: internal/game/database.go:147-160).
- **`launch --game-id`** uses `GetGame` directly (verified: launch.go:48-49).
- **The denylist** accepts unknown numeric IDs (section 3.13).
- **Error text is inconsistent:**
  - Most commands: `game not found: <q>`.
  - launch: bare `game not found` (verified: launch.go:55).
  - Database load errors are wrapped as `failed to load game database: %w` in some commands and passed through raw in others (verified: inheritance_render.go:25-28; dlss.go:109-112).
- **Database source:** games.yaml only. Only list, show and tui refresh it (section 3.2).
- TODO klrtivfcke ("support both names and IDs") is stale, because FindGame already does both (see conflicts).

---

### 5. Wrapper mode (`spela %command%`)

#### 5.1 Detection, before cobra [contract for intent; bug for pre-emption]
- `ParseWrapperArguments(args)` takes leading `KEY=VALUE` tokens as environment (verified: launcher.go:297-319):
  - A key must match `[A-Za-z_][A-Za-z0-9_]*` (verified: launcher.go:321-353; wrapper_boundaries_test.go:38-42).
  - A bare `--` ends the environment section.
  - The rest of the arguments is the command.
- `IsWrapperMode` is true when the command is non-empty, its first element does not start with `-`, and one of these holds (verified: launcher.go:266-295; wrapper_boundaries_test.go:22-37):
  - it is an absolute path that exists;
  - it contains `/`;
  - LookPath finds it and its extension is empty, `.exe` or `.sh`.
- **[bug]:** a subcommand name that is also on PATH is taken as a wrapped program (TODO cridhyzjcj: `overlay` from mesa-demos ran instead of spela's subcommand).

#### 5.2 `runWrapperMode` [contract]
- LoadDatabase. An error aborts the launch with `failed to load game database: %w`, exit 1, and the game does not start (verified: main.go:63-66).
- An empty command gives `no command provided for wrapper mode`. This is unreachable after IsWrapperMode (verified: main.go:68-71).
- **`DetectGameFromCommand`** tries, in order (verified: launcher.go:355-401; launcher_test.go:381-436; wrapper_boundaries_test.go:48-53):
  1. Env `SteamAppId`, then env `SteamGameId`. Each is trimmed, must parse as uint64, and must be in the database.
  2. An argv token `SteamAppId=<id>`.
  3. Any argument containing a game's `InstallDir` as a substring, over map order. [bug] An empty InstallDir matches every argument, and prefix collisions such as `common/Portal` against `common/Portal 2` are possible (inferred).
- If no game is found: `Warning: could not detect game for wrapper invocation` on stderr, and the launch continues with no profile (verified: main.go:73-76).
- **Profile load:** errors are non-fatal. It prints `Warning: failed to load profile for <Name>: <err>` to stderr and continues with **no profile and no defaults**, because `LoadLaunchProfiles` returns all nil on error (verified: main.go:78-85; launch.go:93-100; main_test.go:13-46).
- **Environment:** a new env gets the argv assignments, then Prepare runs, then the assignments are applied **again**, so the user's argv values beat profile values (verified: main.go:87-103; main_test.go:48-88 pins `SPELA_USER_SETTING=keep`).
- When a game is found, the summary is printed (section 6.9) (verified: main.go:95-97).
- **Messages** (verified: main.go:105-111; main_test.go:34-41, 72-74):
  - `Launching <Name> with profile...`
  - `Launching <Name> (no profile)...`
  - `Launching command...`
- **Exit:** on success the code is 0 whatever the game returned. If the game exits non-zero, Go's `exit status N` text goes to stderr and the code is 1 (verified: main.go:122-126; launcher.go:247-251).
- **No lock** is taken (verified: main.go:62-113).

---

### 6. Launch flow

#### 6.1 Sequence [contract unless marked]
1. Load the database, detect the game, load the profiles (above).
2. **`Prepare()`** does nothing without a profile (verified: launcher.go:77-92). With one:
   1. **`Profile.Apply(env)`** (verified: internal/profile/apply.go:38-61):
      - Proton env: `PROTON_ENABLE_WAYLAND=1`, `PROTON_ENABLE_HDR=1`, `PROTON_ENABLE_NGX_UPDATER=1`, and `VKD3D_CONFIG=descriptor_heap` (verified: apply.go:152-166; internal/env/env.go:34-55).
      - DLSS env: `DXVK_NVAPI_DRS_NGX_DLSS_{SR,RR,FG}_OVERRIDE=on`, the matching `*_MODE` and `*_RENDER_PRESET_SELECTION=render_preset_<x>`, and `DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT`. `DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS` is built from `DLSSIndicator=1024` and `DLSSGIndicator=2` (verified: apply.go:168-208).
      - The SR preset `auto` maps to L for ultra_performance, M for performance, and K otherwise (verified: apply.go:224-240).
      - GPU env: `__GL_SHADER_DISK_CACHE=1` and `__GL_SHADER_DISK_CACHE_PATH`, where an empty path uses the config shader_cache or the XDG default. `__GL_THREADED_OPTIMIZATION=1|0` is **always** set (verified: apply.go:210-222; env.go:57-68).
      - **KDE VRR** runs through `display.ApplyVRR` in the user session, not under pkexec. Errors log `failed to apply KDE VRR policy`, and a cleanup is kept even when an error is returned (verified: apply.go:45-52; internal/display/vrr.go:15-68).
      - **Hardware** goes through one pkexec round trip (section 7). A failure logs `failed to apply hardware settings` and the launch continues (verified: apply.go:54-58).
   2. `applyVKD3DHeapEnv`: when vkd3d_heap is set, it resolves the game's Proton build. `PROTON_VKD3D_HEAP=1` is added when resolution fails or the build is a legacy 10.x (verified: launcher.go:103-127; launcher_test.go:98-142).
   3. `vkd3dPreflight` only logs (section 9) (verified: launcher.go:129-177; preflight_test.go:53-275).
   4. `setupOverlay`: when overlay is enabled, it creates `$RUNTIME/spela/overlay-<appid>.dat`, sets `SPELA_OVERLAY_IPC` and starts a collector every 500 ms. A failure runs every cleanup registered so far, once, and returns `prepare overlay: setup overlay collector: ...`. The wrapper then fails with `failed to prepare launch: ...` and the game does not start (verified: launcher.go:85-88, 198-217; internal/overlay/setup.go:13-34; launcher_test.go:283-314).
3. Print the launch message, then run `Launch(command)` (section 6.7).

#### 6.2 Environment build [contract; Rust needs explicit precedence]
- `BuildEnv` is `os.Environ()` followed by the spela variables, which can produce duplicate keys (verified: env.go:22-28). Go exec keeps the last duplicate (inferred, stdlib behaviour).
- Effective order: parent env, then profile env, then argv assignments (inferred from main.go:87-103).

#### 6.3 Preflight [contract]
See section 9 for the pinned log messages. The check is non-blocking and runs only when vkd3d_heap is true.

#### 6.4 Privileged apply
See section 7.

#### 6.5 KDE VRR [contract]
- Supported policies come from `validPolicy`. `""` and `unset` do nothing.
- It needs XDG_SESSION_TYPE=wayland and XDG_CURRENT_DESKTOP containing KDE.
- The primary output (priority 1) must be unique, enabled and connected.
- `kscreen-doctor` runs with a 10 s timeout and `LC_ALL=C`.
- Restore finds the output by UUID (verified: vrr.go:18-80).
- The command sequence is pinned: `-o`, `output.<uuid>.vrrpolicy.always`, `-o`, `output.<uuid>.vrrpolicy.never` (verified: internal/launcher/vrr_test.go:41).
- Dry-run never calls the doctor (verified: vrr_test.go:80-87).

#### 6.6 Overlay [contract for env and file; feature is incomplete]
`SPELA_OVERLAY_IPC` points to the IPC file, and cleanup removes it (verified: launcher_test.go:253-281; main_test.go:81-87).

#### 6.7 Child process, signals, tracking
- `exec.Command(args[0], args[1:]...)` runs with stdin, stdout and stderr inherited and the built env (verified: launcher.go:224-228).
- `signal.Notify(SIGINT, SIGTERM)` is registered **only now**, after Prepare's pkexec and VRR steps (verified: launcher.go:230-231).
- If Start fails, cleanup runs and the error is returned (verified: launcher.go:233-237; vrr_test.go:48-63).
- On the first signal, that signal is forwarded to the direct child and spela waits. Notify is never stopped, so later signals are swallowed (inferred).
- There is no process group, no reaper and no SIGHUP handling (verified: launcher.go:219-252). Grandchildren are not tracked; Steam's own reaper sits in `%command%` (inferred).
- **[bug]:** a signal during the pkexec prompt or the VRR apply kills spela with no cleanup (inferred).
- Tests pin that cleanup runs after SIGTERM (verified: launcher_test.go:217-251).

#### 6.8 Cleanup [contract]
- Steps carry an area label; an empty label becomes `cleanup` (verified: launcher.go:61-73; wrapper_boundaries_test.go:56-61).
- Steps run in **reverse** registration order. Registration order is KDE VRR, then hardware (from Apply), then overlay, so execution order is **overlay, then hardware, then KDE VRR** (verified: apply.go:46-58; launcher.go:79-82, 212, 254-264; launcher_test.go:144-162).
- Each step logs `restore succeeded area=<a>` at INFO, or `restore failed area=<a> error=<e>` at WARN, and the remaining steps continue (verified: launcher.go:257-261; launcher_test.go:180-215).
- The list is cleared afterwards, so cleanup runs once (verified: launcher.go:263; launcher_test.go:283-314).
- The launch error is returned unchanged; a cleanup failure does not hide it (verified: launcher_test.go:198-215).

#### 6.9 Dry-run and launch summary format [contract, with bugs marked]
`PrintLaunchSummary` prints to stdout, coloured, in this order (verified: launch.go:109-136):

1. `Game: <Name> (AppID <id>)` and a blank line.
2. `Command: steam steam://rungameid/<id>`, or the joined arguments when there is more than one.
3. With exactly one argument, also: `Launch option: direct Steam URI launch cannot track cleanup coverage; use Steam launch options: spela %command%`.
4. `Profile: no profile-specific mutation planned`, or `Profile: <n> profile-specific override(s) planned`, where n counts true overrides.
5. `No profile or defaults will affect launch preparation` when there is no effective profile.
6. **`Profile impacts`** (verified: launch.go:240-277):
   - Groups in order: `compatibility:`, `environment:`, `game_file:`, `system_state:`, `overlay:`.
   - Each group shows `none`, or items `    <field>=<value> source=<default|override|unset> restore=<not_applicable|ephemeral_launch_environment|restorable_mutation>` (verified: internal/profile/explanation.go:11-35).
   - Items appear when overridden or when the value is non-zero. Empty values print as `unset`.
7. **`Environment`:** sorted `KEY=VALUE` lines from `ApplyEnv`, or `no launch environment variables planned`. **[bug]** It omits PROTON_VKD3D_HEAP and SPELA_OVERLAY_IPC (verified: launch.go:138-154; apply.go:30-36).
8. **`DLL`** (verified: launch.go:199-238; launch_test.go:57-80):
   - `no launch-time DLL file mutation planned`.
   - `denylist: denied (<reason>)` or `denylist: allowed`.
   - `backup: available` or `backup: not created yet`.
   - Per DLL, sorted by type then name: `<type>: <name> <version|none>; path write: write bit present|no write bit|unknown (path not accessible)`.
9. **`Hardware`** (verified: launch.go:156-187; launch_boundaries_test.go:13-44):
   - Either `no privileged hardware mutation planned`, or `privileged changes apply via pkexec and restore on cleanup` followed by:
     - `GPU clock offset:  N MHz`
     - `GPU memory offset:  N MHz`
     - `GPU power limit:  N W`
     - `GPU fan speed:  N%`
     - `CPU governor:  g`
     - `CPU SMT:  true|false`
   - **[bug]** It disagrees with Profile impacts for zero values (TODO bdnaynbvan) and never mentions VRR.
10. **`Overlay`:** `collector IPC will be created before launch` and `Position:  <pos|none>`, or `overlay collector not planned` (verified: launch.go:189-197).

#### 6.10 Rejection of untracked direct launches [contract]
See section 3.4. README states the same rule (verified: README.md:177-181).

---

### 7. Privilege model and the hidden `apply-profile`

#### 7.1 pkexec wrapper [contract for behaviour; implementation free]
- `Exec(cmd, args)`: as root, the command runs directly. Without pkexec on PATH the error is `pkexec not available: install polkit for privileged operations`. Otherwise it runs `pkexec <cmd> <args>` (verified: internal/privilege/privilege.go:33-61).
- A non-zero exit becomes an `AuthError` (`authentication dismissed by user`) when the output contains `authentication cancelled`, `canceled` or `dismissed`, or the exit code is 126. Otherwise the error is the trimmed stderr (verified: privilege.go:53-60, 132-138; privilege_test.go:20-63, 75-130).
- `ExecWithInput` does the same with stdin and is used by the live CPU writes through `tee` (verified: privilege.go:63-80; cpu.go:94, 129).
- `ExecSelf` resolves `os.Executable()` through `EvalSymlinks` and runs `pkexec <that path> <args>` (verified: privilege.go:120-130).

#### 7.2 apply-profile protocol [internal; the Rust protocol may change]
- **Caller:** `applyHardwareWithOperations` (verified: apply.go:86-150).
  - It runs only when the profile has a non-zero clock offset or memory offset, a power limit above 0, a fan speed above 0, a governor, or an SMT value (verified: apply.go:65-72).
  - Prior state is captured **unprivileged** before the apply: governor from cpu0 scaling_governor, SMT from smt/active, power limit from NVML or nvidia-smi metrics. **Read errors are ignored**, so the zero value is used (verified: apply.go:92-94).
- **Apply call:** `apply-profile` with only the non-zero options (verified: apply.go:96-121):
  - `--gpu-clock-offset=N`
  - `--gpu-memory-offset=N`
  - `--gpu-power-limit=N`
  - `--gpu-fan-speed=N`
  - `--cpu-governor=g`
  - `--cpu-smt=on|off`
- **Data channel:** no stdin, and stdout is not used. Results come back only as the exit code and stderr.
- **Child (as root)** (verified: commands/apply_profile.go:44-171):
  - Flags: four int flags defaulting to 0, `--cpu-governor` and `--cpu-smt` strings, and `--reset` bool.
  - Without root it fails with `apply-profile must be run with root privileges`.
  - Validation: only the governor is checked, and that happens before any change (verified: cpu_validation_test.go:62-96). There are no range checks.
  - **Apply order:** graphics, memory, power, fan (only when above 0), governor, SMT (only `on` or `off`). It stops at the first failure, leaving earlier changes in place. Errors read `set GPU clock offset: %w` and so on (verified: apply_profile.go:78-120; apply_profile_supported_test.go:13-41, 60-138).
  - **Reset order:** resetClocks (graphics and memory, always), power (when the flag is above 0), fan (0 resets to auto, more than 0 sets it), governor, SMT. Errors read `reset GPU clocks`, `restore GPU power`, `reset GPU fan`, `restore GPU fan`, `restore CPU governor`, `restore CPU SMT` (verified: apply_profile.go:122-162; apply_profile_supported_test.go:42-57).
- **Setters** (verified: internal/gpu/nvidia.go:15-92; internal/cpu/cpu.go:84-142):
  - LockGraphicsClocks locks the range 0 to (2100 + offset) MHz.
  - LockMemoryClocks locks 0 to offset. That is an absolute maximum, and negative values wrap through uint32.
  - SetPowerLimit sets the power management limit.
  - SetFanSpeed applies to every fan and needs NVML.
  - ResetClocks resets both the GPU and memory locked clocks.
  - When NVML is not available as root, the code falls back to nvidia-smi `-lgc`, `-lmc`, `-pl`, `-rgc` and `-rmc`.
  - The governor is written to cpu0..NumCPU-1 scaling_governor, and SMT to smt/control.
- **Restore closure** (area `hardware`) runs `apply-profile --reset` as a **second pkexec call** at game exit (verified: apply.go:125-147):
  - `--gpu-power-limit=<prev>` only when the profile set a limit and the previous value is above 0.
  - `--gpu-fan-speed=0` (auto) whenever the profile set a fan speed.
  - `--cpu-governor=<prev>` only when it was read.
  - `--cpu-smt=<prev>`, where an unread value counts as false.
- **Bugs** (TODO iynrkonane, wzpctclhaa; inferred where not run):
  - Clocks always reset to the default, not to the prior value.
  - A prior manual fan speed is lost.
  - A failed SMT read restores SMT to off.
  - A failed or partial apply registers no restore (verified: apply.go:121-123).
  - The child's stderr includes cobra's `Error:` line and usage, and becomes the parent's error string (inferred from section 2.3 and privilege.go:58).

#### 7.3 Polkit policy [contract]
- File `data/polkit/io.github.spela.policy` (verified: lines 1-19):
  - vendor `Spela`, vendor_url `https://github.com/jgabor/spela`.
  - Action `io.github.spela.apply-profile`.
  - Description `Apply game performance profile`.
  - Message `Authentication is required to apply GPU/CPU performance settings`.
  - Defaults: allow_any `auth_admin`, allow_inactive `auth_admin`, allow_active `auth_admin_keep`.
  - Annotation `org.freedesktop.policykit.exec.path` = `/usr/bin/spela`.
- Installed by the PKGBUILD to `/usr/share/polkit-1/actions/io.github.spela.policy` (verified: pkg/aur/PKGBUILD:35). **Not** installed by `mage install` (verified: magefile.go:277-294).
- **Tension:** the action is not scoped by argv, so `pkexec /usr/bin/spela <anything>` matches, including wrapper mode running any program (inferred). A binary not at /usr/bin/spela falls back to the generic pkexec action (inferred).
- **Install location:** `mage install` copies to /usr/bin because Steam runs launches with `PATH=/usr/bin:/bin` (verified: magefile.go:273-276). That location is a [contract].

---

### 8. Lock [contract]
- **Path:** `$XDG_RUNTIME_DIR/spela/spela.pid`, or `/tmp/runtime-$USER/spela/spela.pid` (verified: internal/lock/lock.go:15, 31-33; internal/xdg/xdg.go:31-40).
- **Acquire:**
  - Creates the directory with mode 0700, then creates the file with O_EXCL and mode 0600, and writes the PID with no newline.
  - If the file exists, it reads the PID. A bad PID means remove and retry. A live PID (kill 0 succeeds) gives `spela is already running (PID: N)`, which matches `ErrAlreadyRunning`. A dead PID means remove and retry (verified: lock.go:35-65; lock_test.go:11-58).
- **Release** removes the file without checking the owner. A second Release returns not-exist (verified: lock.go:67-69).
- **Users:** tui and gui only (verified: commands/tui.go:24-27; gui.go:18-21). launch, wrapper and set commands do not lock.
- **Tensions (inferred):**
  - EPERM from kill 0 on another user's PID counts as dead, so the lock is stolen.
  - PID reuse gives false "already running" results.
  - If Remove fails, Acquire recurses without bound.
- **Related lock:** the game database uses a separate flock on `$RUNTIME/spela/games.lock` (verified: internal/game/database.go:52-68).

---

### 9. Logging [contract for messages; no file]
- **Output:** one slog TextHandler writing to the `os.Stderr` captured at package init, with a LevelVar defaulting to INFO. There is **no log file** (verified: internal/logging/logging.go:13-17).
- **Format** (inferred: slog text): `time=<RFC3339> level=WARN msg="..." key=value ...`.
- **Levels:** `SetLevel` accepts debug, info, warn or error; an unknown name gives `unknown log level "<n>"` and the level stays (verified: logging.go:22-38; logging_test.go:26-60). It is applied at startup from config `log_level`; an invalid stored value logs `ignoring configured log level` (verified: config/loglevel.go:7-22).
- `SetHandler` exists for tests (verified: logging.go:73-83).
- **Messages pinned by tests:**
  - `restore succeeded` with `area="game files"`, and `restore failed` with `area=hardware error=...` (launcher_test.go:180-215).
  - `vkd3d_heap: Proton build does not support descriptor_heap` with `detected=` and `minimum=` (preflight_test.go:53-79).
  - `vkd3d_heap: NVIDIA driver below minimum` (preflight_test.go:106-132).
  - `vkd3d_heap: could not resolve Proton for appID; skipping Proton compatibility check` at INFO (preflight_test.go:196-215).
  - `vkd3d_heap: NVIDIA driver not detected; skipping driver compatibility check` (launcher.go:151-157).
  - `failed to apply KDE VRR policy` (vrr_test.go:89-105).
  - `failed to apply hardware settings` (apply.go:55).
- **Note:** under Steam, stderr goes to Steam's log, so restore results are effectively invisible to users (inferred). CHANGELOG mentions an XDG state directory for logs, but no code uses one (see conflicts).

---

### 10. XDG resolution and files [contract]
- **Base directories** (verified: xdg.go:10-62; xdg_test.go:8-22):
  - ConfigHome is `$XDG_CONFIG_HOME/spela`, or `$HOME/.config/spela`.
  - DataHome is `$XDG_DATA_HOME/spela`, or `$HOME/.local/share/spela`.
  - CacheHome is `$XDG_CACHE_HOME/spela`, or `$HOME/.cache/spela`.
  - RuntimeDir is `$XDG_RUNTIME_DIR/spela`, or `/tmp/runtime-<$USER or unknown>/spela`.
  - There is no XDG_STATE_HOME. Env values are not checked to be absolute (inferred).
- **Directory modes:** config and data 0755; runtime 0700 (verified: xdg.go:42-50; lock.go:38; overlay/setup.go:14).
- **Files:**
  - `config.yaml`: config.go:67, 94-95. Written non-atomically with mode 0644.
  - `profiles/<appid>.yaml` and `profiles/default.yaml`: atomic temp file and rename (storage.go:21-31, 87-110).
  - `games.yaml` in data: atomic (database.go:106-136).
  - `backups/<appid>/backup.json` in data (docs/configuration.md:21).
  - `dlls/`, `nvidia/`, `manifest.json` and `update-check.json` in cache (config/paths.go:15-37; docs/configuration.md:15, 52).
  - `spela.pid`, `games.lock` and `overlay-<appid>.dat` in runtime.

---

### 11. Configuration keys [contract]
- **Loading:** the file is read over `Default()`. Missing keys keep their defaults. Unknown and retired keys (`show_hints`, `preferred_dll_source`) are ignored and dropped on save. The file is **not validated** at load (verified: config.go:66-82; yaml_contract_test.go:93-139).
- **Saving:** writes the whole struct in declaration order, as pinned by a test (verified: yaml_contract_test.go:27-46):
  - The expected output uses yaml.v3's **4-space** list indent (`    - /games`).
  - Explicit zero values are kept, an empty list is omitted, and the empty list reloads as nil (verified: yaml_contract_test.go:52-89).

| Key | Default | Accepted | CLI `set` | Notes |
|---|---|---|---|---|
| log_level | info | debug, info, warn, error; else `unsupported log level: x` | yes | |
| shader_cache | absolute `$XDG_CACHE_HOME/spela/nvidia` computed at Default() | absolute folder or empty; else `shader_cache must be an absolute path: x` | yes | Always written, because the tag has no omitempty (config.go:24, 51) |
| check_updates | true | true, false, 1, 0; else `invalid boolean for check_updates: x` | yes | |
| rescan_on_startup | true | bool | no | Changes what `list`, `show` and `tui` do |
| auto_update_dlls | false | bool | no | |
| steam_path | empty, omitempty | any string | no | |
| additional_library_paths | nil, omitempty | colon text or JSON list | no | |
| dll_cache_path, backup_path | empty | absolute or empty | no | Relative values fall back to the default at use (config/paths.go:42-51) |
| dll_manifest_url | empty | https, or http to loopback; else `dll_manifest_url must be an https URL` | no | config/paths.go:63-77 |
| auto_refresh_manifest | true | bool | no | |
| manifest_refresh_hours | 24 | any integer; else `invalid integer for manifest_refresh_hours: x` | no | Choices are not enforced; 0 or less is treated as 24 per docs |
| theme | default, omitempty | default, dark, light | no | **[gui]**; GUI-only visibility (options.go:98) |
| compact_mode | false | bool | no | |
| confirm_destructive | true | bool | no | |

Sources: verified at config.go:22-64, options.go:85-101, 165-266, options_test.go:11-104 and docs/configuration.md:11-27. docs_contract_test.go:49-117 ties the CLI, TUI and GUI columns and the parity table to the catalog.

**GUI parts [gui]:** the Config `json:` tags, VisibilityGUI and VisibilityWails, `OptionByJSONKey`, and `internal/config/wails.go` (`WailsCatalog`).

---

### 12. Usage text, colours and logo
- The CLI uses cobra's default templates with no custom usage template (verified: grep found no SetUsageTemplate).
- **No logo in CLI help** [contract today]. TODO vwsuxxnada would add one. The ASCII logo is 6 lines of box drawing in the TUI header (verified: internal/tui/header.go:17-24).
- **Colours:**
  - The CLI helpers CLIPrimary, CLIAccent, CLISecondary, CLIDim and CLISuccess map to magenta `#ff5fd2`, cyan `#5ff0ff`, FgMuted and `#7ce38b` (verified: internal/tui/styles.go:285-317).
  - Output is written with fmt.Printf. There is no NO_COLOR or isatty handling, and tests strip ANSI from piped output (verified: profile_vocabulary_test.go:11-13). So colour is probably always on (inferred).

---

### 13. Build, release, AUR and contract tests

#### 13.1 Mage targets
- **Build (default)** [intent]: FrontendBuild, then `go build -buildvcs=false -trimpath -tags embed_assets,production,webkit2_41 -ldflags "-s -w -X main.version=<v>" -o spela ./cmd/spela` (verified: magefile.go:22, 44-52).
  - `<v>` is `SPELA_VERSION`, else `git describe --tags --always --dirty`, else `dev` (verified: magefile.go:24-33). The describe call has no `--match`, so `dll-*` tags can be picked up (inferred; see conflicts).
- **Test:** `go test -v ./...` plus the GUI tags (verified: magefile.go:157-162).
- **Coverage and CoverageCheck:** five build-tag profiles plus the frontend, with a 90% minimum (verified: magefile.go:171-255; docs/coverage.md).
- **TestTUIE2E:** `go test -tags e2e ./tests/e2e/...`. **TestE2E:** that plus Playwright.
- **Lint:** golangci-lint.
- **Install:** `go install`, then `sudo install -m755` to /usr/bin/spela.
- **Clean.**
- **[gui]:** GenNavJS, FrontendBindings, FrontendBuild, TestFrontend, Dev, DevStop, and the frontend and Wails parts of Coverage.

#### 13.2 Release [intent]
- **Trigger:** a pushed `v*` tag. It is validated against `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` in its own job, and the tag passes through env, not shell interpolation (verified: .github/workflows/release.yml:3-26).
- **Build job:** `go tool mage build`, `mv spela spela-linux-amd64`, `sha256sum spela-linux-amd64 > checksums.txt`, then `sh scripts/release-notes.sh "$RELEASE_TAG" > release_notes.md` and softprops/action-gh-release with those two files (verified: release.yml:28-76).
- **Release notes:** the exact body of the CHANGELOG section `## [X.Y.Z]` (verified: scripts/release-notes.sh:4-16).
- The release workflow has no AUR job (verified: release_contract_test.go:91-93).
- **Operator steps:** git-cliff, review, annotated tag, explicit push. GitHub Actions is the only publisher (verified: docs/releasing.md:3-64).

#### 13.3 AUR [intent]
- `spela-git` is a VCS package. pkgver comes from `git describe --long --abbrev=7 --tags --match 'v[0-9]*'` (verified: pkg/aur/PKGBUILD:2-24).
- Build: `SPELA_VERSION="$pkgver" CGO_ENABLED=1 go tool mage build` (verified: PKGBUILD:26-30).
- Package: installs `/usr/bin/spela`, the polkit policy and LICENSE (verified: PKGBUILD:32-37).
- Dependencies: nvidia-utils, plus webkit2gtk-4.1 and gtk3 [gui]. Make dependencies: git, go, and bun [gui]. Optional: polkit, nvidia-settings (verified: PKGBUILD:10-15).
- `scripts/aur-submit.sh` does a dry run by default, has `--publish`, and guards the identity with `Welcome to AUR, jgabor!` (verified: docs/releasing.md:66-100; release_contract_test.go:324-327).

#### 13.4 Contract tests in tests/contracts [intent]
- `TestReleaseWorkflowArtifactContract` (release_contract_test.go:40-94): pins the trigger, permission, build commands, checksum, release files, notes command, and no `aur-publish` job.
- `TestReleaseTagIsValidatedBeforeUse` and `TestReleaseTagValidationRejectsShellSyntax` (release_contract_test.go:96-214): tag injection safety.
- `TestWorkflowSyntax` (release_contract_test.go:229-252).
- `TestReleaseBuildInputsContract` (release_contract_test.go:254-328): pins mage v1.17.2, wails v2.16.0 [gui] and bun 1.3.14 [gui]; no `@latest`; frozen bun lock; no local release publisher in the magefile; PKGBUILD is spela-git, uses `--match 'v[0-9]*'` and `go tool mage build`, and sets SPELA_VERSION.
- `TestReleaseNotesAreExactChangelogSection` (release_contract_test.go:330-360).
- `TestReleaseArtifactContractBuildsFromCleanArchiveAndVerifiesChecksum` (release_contract_test.go:385-491): builds from `git archive` with the overlay.
- `TestAURSubmit` (aur_submit_test.go:13-205): seven scenarios with fake ssh, git and makepkg.
- **For Rust:** keep the intents. These are the tag gate, a single binary artifact `spela-linux-amd64` with checksums, notes taken from the changelog, the AUR VCS package that installs the binary and the polkit policy to the same paths, and dry-run-first submission. The Go, mage, Wails and bun pins are replaced.

---

### 14. Classification summary

| Item | Class |
|---|---|
| Command names, args, flags, text messages and error strings listed above | contract |
| Exit 0/1 and `Error:` plus usage on stderr | contract (open question on child exit code) |
| `--json` shapes (Go default PascalCase, resolved vs raw as described) | contract (open question on stability) |
| Wrapper env parsing, Steam env detection, argv env precedence | contract |
| Wrapper pre-emption of subcommands; InstallDir substring fallback | bug |
| Direct `launch <game> <cmd...>` executes the game query | bug |
| Dry-run format | contract; env omissions and Hardware/impacts disagreement are bugs |
| Cleanup order and restore log messages | contract |
| Late signal registration; no SIGHUP; single forward | bug |
| Lossy and missing hardware restore; offset semantics | bug (TODO iynrkonane, wzpctclhaa) |
| `cpu smt` typo disables SMT; per-CPU pkexec for live governor | bug |
| `gpu set --fan-speed 0` and `--power-mizer default` storing overrides | bug (TODO rgddfoguos, bdnaynbvan) |
| `list --json` empty-db text; `profile delete` false success | bug |
| Polkit action id, message, defaults, /usr/bin/spela path | contract |
| apply-profile flag protocol | internal; Rust may redesign (transition question) |
| PID lock for TUI | contract; recursion and EPERM edge cases are bugs |
| stderr slog logging and pinned messages | contract |
| XDG paths and file names | contract |
| Config keys, defaults, validation, CLI visibility | contract; theme is gui |
| YAML byte layout | open question |
| gui command, main_wails.go, wails.json, config/wails.go, json tags, theme, frontend targets, webkit/gtk/bun deps | gui |
| Mage, Go and bun build specifics, release workflow mechanics | intent |

### Conflicts and mismatches

- **Usage output stream on error.** The test sees usage on stdout only because it redirects cobra's output. In production, both the Error line and the usage go to stderr, because cobra's Print writes to OutOrStderr and main never calls SetOut. Evidence: cmd/spela/main_test.go:99-100,115 vs cobra@v1.10.2/command.go:1160-1166,1435-1442; cmd/spela/main.go:130
- **Wrapper test asserts empty stderr.** The test swaps os.Stderr after the slog handler has already bound the original stderr at init. Restore logs such as 'restore succeeded area=overlay' therefore bypass the capture. In production they do appear on stderr. Evidence: cmd/spela/main_test.go:69-71 vs internal/logging/logging.go:16; internal/launcher/launcher.go:257-261
- **README on --json.** README says --json prints stored values only, and also calls dlss show --json 'effective'. In the code, subsystem shows print resolved values, while only profile show --json prints the raw profile. Evidence: README.md:77,141 vs cmd/spela/commands/dlss.go:92, gpu_set.go:152, cpu_set.go:116, overlay.go:165, proton.go:160, profile.go:148
- **Restore of prior state.** AGENTS.md and README describe restore as returning to the previous state. The code resets clocks to the driver default and the fan to auto. Power limit, governor and SMT are restored only when they were read, and SMT becomes off when the read failed. A failed apply has no restore. Tracked as TODO iynrkonane. Evidence: AGENTS.md:51; README.md:171-175 vs internal/profile/apply.go:92-94,121-147; cmd/spela/commands/apply_profile.go:122-162
- **AGENTS.md says NVML setters directly.** When NVML is unavailable, even as root, the code falls back to nvidia-smi. Fan control needs NVML. Live CPU and GPU commands use pkexec tee or nvidia-smi rather than the batched path. Evidence: AGENTS.md:46-51 vs internal/gpu/nvidia.go:15-57,61-63,204-207; internal/cpu/cpu.go:84-100
- **Clock and memory offset naming.** The flags and docs call these offsets. Clock offset actually locks the maximum clock at 2100 plus the value. Memory offset is an absolute maximum clock, and negative values wrap. The apply-profile help says 'GPU memory clock (MHz)' while gpu set says 'GPU memory clock offset in MHz'. Tracked as TODO wzpctclhaa. Evidence: internal/gpu/nvidia.go:15-28; cmd/spela/commands/apply_profile.go:44-45; gpu_set.go:41-42
- **gpu set --fan-speed help says '0 to clear'.** A value of 0 is stored as a game override of 0, not cleared. --power-mizer default stores an empty override, while cpu set default resets the field. Tracked as TODOs rgddfoguos and bdnaynbvan. Evidence: cmd/spela/commands/gpu_set.go:44,96-106 vs cpu_set.go:76-90
- **launch accepts a command but executes the game query.** The help says the game can be given by name or ID. With more than one argument, args[0] (the game query) is executed as the program and printed as Command. There is no working non-dry-run direct form. Evidence: cmd/spela/commands/launch.go:28-31,67-69,83,113-115
- **Dry-run Hardware block vs Profile impacts.** Zero values show as override impacts while Hardware says no mutation planned. The Hardware block never lists VRR. Tracked as TODO bdnaynbvan. Evidence: cmd/spela/commands/launch.go:156-187,240-277; .agentera/entities/todo/todo_item/bdnaynbvan.yaml
- **Dry-run environment is incomplete.** The dry-run uses ApplyEnv. Prepare also sets PROTON_VKD3D_HEAP (for legacy or unresolved Proton) and SPELA_OVERLAY_IPC, and neither appears in the summary. Evidence: cmd/spela/commands/launch.go:138-154; internal/profile/apply.go:30-36 vs internal/launcher/launcher.go:103-127,211
- **Two source vocabularies.** The dry-run prints source=default|override|unset. The show commands and the CHANGELOG 'one vocabulary' entry use 'from All games', 'this game · ...' and 'system default'. Tracked as TODO wuaaeuetff. Evidence: cmd/spela/commands/launch.go:274; internal/profile/explanation.go:11-15 vs CHANGELOG.md:29; profile_vocabulary_test.go:63-76
- **Inconsistent bool vocabularies.** Config booleans accept true/false/1/0. Profile flags also accept yes/no/on/off and capitalised forms. cpu set --smt accepts on/true/off/false/default only. The live cpu smt command treats anything else as off. Evidence: internal/config/options.go:170-176; cmd/spela/commands/bool_flag.go:8-13; cpu_set.go:84-93; cpu.go:167; docs/configuration.md:15
- **cpu profile-reset lists affinity, and the CLI has no gpu.vrr setter.** Affinity can be reset but not set, and fields.go marks it 'Not used yet'. gpu.vrr can be reset through field resolution but has no gpu set flag, although the CHANGELOG describes VRR editing in the TUI and GUI only. Evidence: cmd/spela/commands/cpu_set.go:49-50; internal/profile/fields.go:91; gpu_set.go:41-48,60-62; inheritance_render.go:127-144; CHANGELOG.md:17
- **Fields stored but never applied.** gpu.power_mizer, cpu.affinity, and the overlay show_* and toggle_key fields have setters and are displayed, but nothing outside the profile, TUI, GUI and command code consumes them. Tracked as TODO xuyalcslma. Evidence: grep of PowerMizer, .Affinity, ShowFPS and ToggleKey outside those packages found no hits; internal/profile/apply.go:38-222; internal/launcher/launcher.go:198-217
- **Stale TODO klrtivfcke.** The TODO asks to support both names and Steam IDs. FindGame already resolves numeric AppIDs first and exact names second, and every game command uses it. Evidence: TODO.md:29 vs internal/game/database.go:147-160
- **CHANGELOG XDG state directory for logs.** The CHANGELOG fixed list says 'Use XDG state directory instead of ~/logs'. There is no XDG state support, and spela's own logs go only to stderr. Evidence: CHANGELOG.md (0.5.0/0.3.0 fixed list, 'Use XDG state directory instead of ~/logs') vs internal/xdg/xdg.go:10-62; internal/logging/logging.go:16
- **docs/coverage.md on AUR publications.** coverage.md says the workflow test pins both stable and development AUR publications. The test asserts there is no aur-publish job, and releasing.md describes only spela-git. Evidence: docs/coverage.md:110-114 vs tests/contracts/release_contract_test.go:91-93; docs/releasing.md:7-8
- **Terminal Control version.** AGENTS.md requires Terminal Control 1.2.1 or newer. CI installs 0.3.1. Evidence: AGENTS.md:65 vs .github/workflows/ci.yml:97
- **AGENTS.md project structure is incomplete.** internal/display, internal/nav, internal/proton and internal/release exist but are not listed. Evidence: AGENTS.md:15-37 vs /bin/ls internal
- **mage install does not install the polkit policy.** README presents mage install as the source install path. Only the PKGBUILD installs the policy, so a source install gets the generic pkexec action without auth_admin_keep. Evidence: magefile.go:277-294; README.md:39 vs pkg/aur/PKGBUILD:35; data/polkit/io.github.spela.policy:13-17
- **Version tag selection differs.** The magefile uses git describe --tags without --match, so dll-* tags can be chosen. The PKGBUILD restricts to v[0-9]*. Evidence: magefile.go:28; CHANGELOG.md has dll-dlssg-v310.5.3 and dll-dlssd-v310.5.3 sections; pkg/aur/PKGBUILD:23
- **Polkit action not scoped to apply-profile.** The action is named apply-profile but annotates only exec.path. It therefore matches any pkexec /usr/bin/spela invocation, including wrapper mode running an arbitrary program. Evidence: data/polkit/io.github.spela.policy:9-17; internal/privilege/privilege.go:120-130 (inferred)
- **Wrapper detection pre-empts subcommands.** IsWrapperMode runs before cobra, so subcommand names found on PATH are executed as programs. runRoot's wrapper branch is dead code. Tracked as TODO cridhyzjcj. Evidence: cmd/spela/main.go:55-57,121-128; internal/launcher/launcher.go:266-295; cobra@v1.10.2/args.go:35-37
- **Profile load error handling differs by path.** In launch, a corrupt game profile is fatal. In wrapper mode it only warns, but it also drops valid defaults. A corrupt games.yaml aborts the wrapper launch entirely. Evidence: cmd/spela/commands/launch.go:58-61,93-100 vs cmd/spela/main.go:63-66,78-85; main_test.go:13-46
- **'with profile' wording.** The message says 'Launching X with profile...' even when only All games defaults apply and the game has no profile. Evidence: cmd/spela/commands/launch.go:102-105,78-82; cmd/spela/main.go:105-106
- **JSON and text inconsistencies.** list --json on an empty database prints plain text. scan --json includes Steam tools while list filters them. profile show --json errors without a profile while the subsystem show --json commands work. profile delete reports success when no profile existed. Evidence: cmd/spela/commands/list.go:51-54; scan.go:40; internal/game/database.go:162-171; profile.go:145-147,218-222; profile_vocabulary_test.go:104-119; internal/profile/storage.go:237-245
- **'game not found' message forms.** launch returns a bare 'game not found'. Other commands return 'game not found: <q>'. Database load errors are wrapped in some commands and not in others. Evidence: cmd/spela/commands/launch.go:55 vs show.go:47, profile.go:123; inheritance_render.go:25-28; dlss.go:109-112
- **Config write atomicity.** config.yaml is written directly with os.WriteFile. Profiles and games.yaml use a temp file and rename. Evidence: internal/config/config.go:94-95 vs internal/profile/storage.go:100-110; internal/game/database.go:119-136
- **CLI config visibility vs behaviour.** rescan_on_startup defaults to true and makes list and show rescan Steam on every run, but config set rejects it as an unknown key. Only three keys can be set from the CLI. Evidence: internal/config/options.go:86-89; internal/steam/refresh.go:35-38; cmd/spela/commands/list.go:45

### Open questions

- Should Rust keep any non-dry-run `spela launch <game> ...` form? The current one executes the game query as the program, so it cannot work. The alternative is to make `launch` dry-run only and point to `spela %command%`.
- How should wrapper detection change so subcommands always win (TODO cridhyzjcj)? Options: detect only after subcommand matching, only for paths and existing files, or only after an explicit `--`. Which current `spela <name-on-PATH>` uses must keep working?
- Should the wrapper exit with the game's own exit status (and 128+signal), instead of always 1 with 'exit status N' on stderr?
- Is the exact YAML byte layout a contract (yaml.v3 4-space list indent, key order, explicit zeros, shader_cache always written)? Rust YAML serializers format differently, and yaml_contract_test.go pins the bytes.
- Is the --json output a stable contract? It uses Go default PascalCase keys, null for empty slices, RFC3339Nano times and HTML escaping. Or may Rust define a new, versioned schema?
- Transition safety: a game launched under the Go binary registers a restore that re-executes the resolved binary path with Go's `apply-profile --reset ...` flags. If the package switches to Rust mid-session, must Rust accept the Go flag protocol?
- What should GPU 'clock offset' and 'memory offset' actually do: real VF offsets or locked clocks with a hard-coded 2100 MHz base (TODO wzpctclhaa)? This needs hardware confirmation before porting.
- Is a second polkit prompt at game exit acceptable, given that auth_admin_keep has probably expired after a session? What happens with no polkit agent (gamescope or Big Picture)? Should restore use a longer-lived privileged helper, or a design that always restores?
- Should the polkit action be scoped to the apply-profile argument (for example with the exec.argv1 annotation), and should a source install also install the policy?
- Should the Rust CLI honor NO_COLOR and drop colour when stdout is not a terminal? The Go CLI appears to always emit ANSI.
- Should `config set` expose every TUI-visible key (rescan_on_startup, steam_path and others), or keep only log_level, shader_cache and check_updates?
- Must the GUI-only `theme` key be kept on load and save so that an existing config.yaml is not changed, now that the GUI is gone?
- What should `spela gui` do in the Rust binary: be removed (unknown command) or print a message saying the GUI was dropped?
- Should the PID lock also cover launch and wrapper runs, so two concurrent launches cannot interleave hardware apply and restore?
- Should logging gain a file (for example under XDG_STATE_HOME) so that restore results under Steam are visible? mejl's pattern may apply.
- Should `list` and `show` keep rescanning Steam on every run by default (rescan_on_startup defaults to true)?
- Should cobra's auto-generated `completion` and `help` commands and the `-v/--version` output (`spela version <v>`) be reproduced exactly?
- Is the overlay IPC collector (SPELA_OVERLAY_IPC, 500 ms mmap writes) in parity scope, given that no consumer exists yet and most overlay fields are stored only?
- Is the live `cpu governor` path, with one pkexec tee per CPU when not root, a contract, or should live commands also batch through apply-profile?

## Spela domain packages: inventory for the Rust rewrite

### Scope, method and labels

This section covers these paths:

- internal/profile, internal/env, internal/game, internal/steam, internal/proton, internal/display, internal/gpu, internal/cpu and internal/nav.
- docs/profile-fields.md, docs/kde-vrr.md, docs/NVAPI.md, docs/gamemode.sh and docs/configuration.md.

How the work was done:

- Every in-scope file was read in full.
- Some files outside scope were read only where they define behaviour the domain depends on. They are marked "outside scope, read" where cited: cmd/spela/commands/apply_profile.go, internal/launcher/launcher.go:50-290, cmd/spela/commands/launch.go:90-150 and 225-300, cmd/spela/main.go:55-150, internal/privilege/privilege.go:40-140, internal/xdg/xdg.go, internal/config/paths.go:35-58, internal/tui/layout.go:205-340, TODO.md and AGENTS.md.

How claims are labelled:

- "verified" means I read the cited lines.
- "inferred" means I reasoned from verified code but did not run it.
- "unverified" means only a live test or an outside source can settle it.

Nothing was built or run.

### 1. Profile schema (35 fields)

#### Shape

- Profile has a Name and five groups: dlss, gpu, cpu, proton and overlay. It also has `overrides: map[string]bool` with no omitempty (verified: internal/profile/profile.go:31-49).
- Every leaf except Name carries a yaml omitempty tag. cpu.smt is a `*bool` (verified: profile.go:51-99).
- The descriptor table is the single source for key, subsystem, kind, label, allowed values, impact, restore class, description, editor kind, unit and display range (verified: fields.go:43-101).
- A test pins the count at 35 and checks that every struct leaf has exactly one descriptor (verified: fields_test.go:9-55; yaml_contract_test.go:141-161).
- The editor kind is derived when it is not set: bool gives toggle, int gives integer, a field with allowed values gives choice, and anything else gives text. gpu.vrr sets choice explicitly (verified: fields.go:136-150 and :80).
- The default for every field is its zero value: false, 0, "" or nil SMT. "Default" in the user's sense means "inherit from All games, else zero". No other built-in default exists (verified: profile.go has no default constructor, and inheritance.go:217-245 starts from `&Profile{}`).

#### Field table

Order is descriptor order. "Allowed" lists the descriptor's allowed values; `""` always means unset. "Effect" is what launch does with the resolved value.

| Key | Kind | Allowed | Effect at launch | Status |
|---|---|---|---|---|
| proton.enable_hdr | bool | any | `PROTON_ENABLE_HDR=1` when true; unset when false (apply.go:156-158, env.go:38-40) | live |
| proton.enable_wayland | bool | any | `PROTON_ENABLE_WAYLAND=1` (apply.go:153-155, env.go:34-36) | live |
| proton.enable_ngx_updater | bool | any | `PROTON_ENABLE_NGX_UPDATER=1` (apply.go:159-161, env.go:42-44) | live |
| proton.vkd3d_heap | bool | any | `VKD3D_CONFIG=descriptor_heap` (apply.go:162-164, env.go:50-55). The launcher adds `PROTON_VKD3D_HEAP=1` when Proton resolution fails or the build uses the legacy gate (launcher.go:103-127, outside scope, read) | live |
| dlss.sr_mode | string | "", off, ultra_performance, performance, balanced, quality, dlaa | `DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE=<mode>`, only when sr_override is on and the mode is non-empty (apply.go:169-173) | live |
| dlss.sr_preset | string | "", default, auto, A-F, J-M | `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_<x>`, only with sr_override. `auto` maps to L for ultra_performance, M for performance, and K for everything else including an empty mode. `default` and unknown values give `render_preset_default` (apply.go:174-176, 224-249; apply_test.go:122-157) | live |
| dlss.sr_override | bool | any | `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on` (apply.go:169-170) | live |
| dlss.rr_mode | string | same as sr_mode | `DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE`, only with rr_override (apply.go:179-183) | live |
| dlss.rr_preset | string | "", default, A-F, J-M (no auto) | `..._RR_OVERRIDE_RENDER_PRESET_SELECTION` (apply.go:184-186; fields_test.go:157-183) | live |
| dlss.rr_override | bool | any | `DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE=on` | live |
| dlss.fg_enabled | bool | any | With fg_override on: `DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT=<multi_frame>`, which may be 0 (apply.go:189-194) | live |
| dlss.fg_override | bool | any | `DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE=on` | live |
| dlss.multi_frame | int | 0-4 | The value of the variable above | live |
| dlss.indicator | bool | any | `DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS` gets `DLSSIndicator=1024` (apply.go:196-205) | live |
| dlss.fg_indicator | bool | any | The same variable gets `DLSSGIndicator=2`. Parts are joined with "," | live |
| gpu.clock_offset | int, MHz | unbounded | Privileged when != 0. Locks graphics clocks to 0..(2100+N) MHz (apply.go:98-100; nvidia.go:15-21) | live, misnamed |
| gpu.vrr | string | "", unset, automatic, always, never | KDE primary-display VRR through kscreen-doctor. "" and unset do nothing (display/vrr.go:36-38) | live |
| gpu.memory_offset | int, MHz | unbounded | Privileged when != 0. Locks memory clocks to 0..N MHz (nvidia.go:23-28) | live, misnamed |
| gpu.power_limit | int, W | unbounded | Privileged when > 0 (apply.go:104-106; nvidia.go:39-44) | live |
| gpu.fan_speed | int, % | display range 0-100, not enforced | Privileged when > 0. Sets every fan through NVML (nvidia.go:61-75) | live |
| gpu.power_mizer | string | "", adaptive, max, prefer_max_performance, prefer_maximum_performance | None. Its description says so (fields.go:84) | no-op |
| gpu.shader_cache | bool | any | `__GL_SHADER_DISK_CACHE=1` and `__GL_SHADER_DISK_CACHE_PATH=<path>` (apply.go:211-217; env.go:57-60) | live |
| gpu.shader_cache_path | string | unbounded | The path above when shader_cache is on. Otherwise the config path, which must be absolute, else `$XDG_CACHE_HOME/spela/nvidia` (config/paths.go:35-51, outside scope, read; shader_cache_test.go:12-53) | live |
| gpu.threaded_optimization | bool | any | `__GL_THREADED_OPTIMIZATION=1`, or `0` when false. It is always written when a profile applies (apply.go:219; env.go:62-68) | live |
| cpu.governor | string | "", performance, powersave, schedutil, ondemand | Privileged `--cpu-governor` (apply.go:110-112) | live |
| cpu.smt | optional bool | "", true, false | Privileged `--cpu-smt=on|off` (apply.go:113-119) | live |
| cpu.affinity | string | unbounded | None (fields.go:91) | no-op |
| overlay.enabled | bool | any | The launcher starts the overlay feed and sets `SPELA_OVERLAY_IPC` (launcher.go:198-217, outside scope, read) | live |
| overlay.position | string | "", top-left, top-right, bottom-left, bottom-right | Passed to the overlay collector (launcher.go:203) | live |
| overlay.show_fps, show_frametime, show_cpu, show_gpu, show_vram | bool | any | None (fields.go:95-99) | no-op |
| overlay.toggle_key | string | unbounded | None (fields.go:100) | no-op |

Additional facts:

- A grep of production code outside internal/profile and internal/gui found no reader of ShowFPS, ShowFrametime, ShowCPU, ShowGPU, ShowVRAM, ToggleKey, PowerMizer or Affinity (verified). That confirms the eight no-op fields, which TODO.md:10 tracks (`xuyalcslma`).
- Impact classes: environment, game_file (unused by any field), system_state, overlay and compatibility. Restore classes: not_applicable, ephemeral_launch_environment and restorable_mutation (explanation.go:20-35).
- Units and ranges: clock and memory use "MHz", power uses "W", fan uses "%". Only fan has a range (0-100), and it is display only (fields.go:79-83; vocabulary_test.go:211-254).

#### Typed write rules

These are the `WriteField` and `Set` rules (fields.go:178-273):

- Allowed values are checked against `fmt.Sprint(input)`, so the int 4 passes as "4" and nil passes as "".
- Bool fields need a real bool; the string "true" is rejected.
- Int fields accept int or a whole float64; strings are rejected.
- String fields need a string.
- Optional bool accepts nil, "", a bool, or a string that parses as a bool.
- The tests pin the rejections (fields_test.go:117-142).
- `Set` writes and marks the override. False, 0, "" and nil SMT are explicit values (fields_test.go:100-115).

### 2. Inheritance

#### Model

- default.yaml is "All games" (AllGamesName, vocabulary.go:15).
- A game profile stores raw leaves plus `overrides`. A leaf counts only when `overrides[key]` is true; a raw value without the marker is ignored (inheritance.go:126-131; inheritance_test.go:43-49).
- `ResolveForApply(defaults)` copies each leaf from the game when it is overridden, and from defaults otherwise. With nil defaults, inherited leaves stay at zero. SMT is deep-copied (inheritance.go:217-262).
- When no game profile file exists, launch applies the raw defaults profile directly (launch.go:94-99, outside scope, read).
- `Reset(field)` zeroes the leaf and deletes the marker. `ResetAll` zeroes all 35 leaves, sets overrides to nil and keeps Name (inheritance.go:183-208).
- `PinField` copies the effective defaults value onto the game and marks it. It does nothing if the field is already overridden, and with nil defaults it pins zero (inheritance.go:159-178; inheritance_test.go:363-441).
- In production only the GUI calls PinField (grep count: tui 0, cmd 0, gui 1).
- `MergeChanges(current, before, after)` re-applies only the fields that changed between before and after. A field that lost its override is reset; a changed field is Set (fields.go:277-304). The TUI uses it once (grep).
- On the All games profile itself, `setsField` is true when the field is marked, or when its value is non-zero. The non-zero rule covers legacy defaults without markers (explanation.go:129-138; vocabulary_test.go:138-147).

#### Explanation and origin

`ExplainField` returns Value, DefaultValue, AllGamesSets, Source (`default`, `override` or `unset`), Origin, Impact and Restore (explanation.go:58-197).

| Origin | When |
|---|---|
| `this_game` | The game overrides the field and its value differs from the All games value |
| `this_game_same` | The game overrides the field and its value equals the All games value. If All games does not set the field, the game's value must be zero |
| `all_games` | The game inherits and All games sets the field |
| `system_default` | Nothing sets the field |
| `set_here` | All games scope only: the field is set there (ExplainAllGamesField, explanation.go:101-125) |

`Source` is kept for older consumers. The strings `default`, `override` and `unset` are a pinned contract (vocabulary_test.go:107-134).

#### Exact vocabulary strings

All strings are verified from vocabulary.go and pinned by tests.

- Source text (vocabulary.go:110-123):
  - `this game · All games: <value>`. `<value>` is DisplayValue of the All games value. An int that All games does not set reads `none`.
  - `this game · same as All games`
  - `from All games`
  - `set here`
  - `system default`
  - The separator is U+00B7 with a space on each side.
- Value words: `On`, `Off`, `none` (vocabulary.go:29-33).
- DisplayValue (vocabulary.go:53-97):
  - Units read `150 MHz`, `250 W` and `60%`. There is no space before %.
  - Optional bool nil reads `none`.
  - Bool nil reads `Off`.
- ValueText: an int whose origin is `system_default` reads `none`. An explicit 0 reads `0 MHz` (vocabulary.go:101-106; vocabulary_test.go:256-276).
- Action words (vocabulary.go:18-26):
  - `Follow All games`
  - `Use system default`
  - `Follow All games for all settings`
  - `Use system default for all settings`
  - `Apply to games with their own value`
  - `Set for this game`
  - `Set here`
- ClearAllBody (vocabulary.go:150-163; vocabulary_shared_test.go:9-28):
  - `Nothing to clear: every value already follows All games.` The All games scope ends with `the system default.`
  - `This clears the 1 setting that this game sets itself. It follows All games afterwards. It is saved at once.`
  - `This clears the N settings that this game sets itself. They follow All games afterwards. It is saved at once.`
  - On All games the owner reads `All games sets` and the target reads `the system default`.
- OwnValueSuffix: `1 game has its own value` and `N games have their own value` (vocabulary.go:208-217).
- GamesText: `1 game` and `N games`.
- ApplyToOwnValueBody: `<label>: 1 game has their own value: A. It will follow All games (<value>). It is saved at once and cannot be undone.` It lists 6 names, then `and N more`. The `(<value>)` part is omitted when the value text is empty (vocabulary.go:228-248; vocabulary_shared_test.go:58-71).
- FollowOutcomeText: `<label>: 1 game follows All games` and `<label>: N games follow All games`. A failure prefix reads `Not saved for Beta (disk full). ` (vocabulary.go:252-273; vocabulary_shared_test.go:73-92).
- NumberHint: `Whole number in MHz.` and `Whole number in %, shown range 0 to 100.` It is empty for non-free numbers (vocabulary.go:277-289).
- RetiredWords: `(default)`, `Override for this game`, `Inherited from defaults`, `Default for all games`, `defaults profile`, `root profile`, `This change does not reach`, `Effect:` (vocabulary.go:37-48).
- Nav breadcrumbs use the separator ` › `, which is U+203A (nav.go:291).

### 3. Storage

#### Paths

The xdg package defines these paths (outside scope, read: xdg.go:10-62):

- Config is `$XDG_CONFIG_HOME/spela`, else `$HOME/.config/spela`. Data is `$XDG_DATA_HOME/spela`, else `~/.local/share/spela`. Cache is `$XDG_CACHE_HOME/spela`, else `~/.cache/spela`.
- Runtime is `$XDG_RUNTIME_DIR/spela`, else `/tmp/runtime-$USER/spela`.

Files used by the domain packages:

| File | Format, lock, mode |
|---|---|
| `<config>/profiles/<appid>.yaml` | Game profile (storage.go:21-31) |
| `<config>/profiles/default.yaml` | All games profile |
| `<config>/profiles/.mutation.lock` | flock(LOCK_EX), mode 0600 (storage.go:221-235) |
| `<data>/games.yaml` | Game database (database.go:84-137) |
| `<runtime>/games.lock` | flock(LOCK_EX), mode 0600; directory 0700. The process also holds an in-process mutex (database.go:47-82) |

#### Write path

The write path is the same for profiles (storage.go:87-118) and games.yaml (database.go:106-137):

1. Create a temp file in the same directory: `.profile-*.tmp` or `.games-*`.
2. chmod it to 0644.
3. Write the data, then close the file.
4. Rename it over the target.
5. Remove the temp file in a defer.

There is no fsync (verified: no Sync call in either file). The profiles directory is created with mode 0755 (storage.go:33-35), and the data home with 0755 (xdg.go:47-50).

#### Transactions

- `Mutate(appID, cb(current, defaults))` runs under the lock.
  - It loads the game (missing becomes an empty profile) and loads fresh defaults.
  - It clones the profile and runs the callback. A callback error leaves the file unchanged (storage_transaction_test.go:43-53).
  - An existing profile that the callback left deep-equal is not rewritten. This stops an in-memory legacy migration from being frozen (storage.go:133-162; reset_persistence_test.go:293-318).
- `MutateExisting` does not create a missing profile (reset_persistence_test.go:336-342).
- `MutateDefault` always saves. It creates default.yaml when missing (storage.go:207-219).
- `Create` writes only if no profile exists, under the lock (storage.go:248-258; storage_transaction_test.go:56-80).
- `Delete` takes the lock and treats a missing file as success (storage.go:237-245; storage_transaction_test.go:83-111).
- `FollowAllGamesIn(entries, fields)`:
  - It writes one locked MutateExisting per game. It only touches games that override one of the fields, and only those fields.
  - It is not atomic across games. The outcome holds Followed and Failed (both sorted) and Errors keyed by name (storage.go:177-204; storage_transaction_test.go:114-184).
- Concurrent Mutate calls on different fields both land (storage_transaction_test.go:13-38).
- `List()` reads `*.yaml` and `*.yml` with a numeric stem. It skips files that fail to load or parse, and skips directories (storage.go:260-297; storage_boundaries_test.go:27-52).

#### Loading and validation

- `Load` returns nil, nil for a missing file. A directory in place of the file is an error (storage.go:37-65; storage_boundaries_test.go:10-25).
- Legacy migration runs only when the overrides key is absent. It needs `LoadDefault`, so a broken default.yaml makes the legacy `Load` fail (storage.go:56-62).
- Only gpu.vrr is validated, on both load and save. The error names `gpu.vrr` (migrate_sr_preset.go:40-42; storage.go:88-92; vrr_test.go:67-76). No other enum is validated on load (verified by reading unmarshalProfileYAML).

#### YAML contract

These rules are pinned byte for byte (yaml_contract_test.go:31-108):

- Indentation is 4 spaces.
- Top-level order follows the struct: `name`, `dlss`, `gpu`, `cpu`, `proton`, `overlay`, `overrides`.
- Leaves within each section follow struct order. This is not descriptor order: for example, gpu starts with `vrr`, and proton starts with `enable_wayland`.
- Override keys are sorted.
- `smt: false` is written when the pointer is non-nil. False bools, zero ints and empty strings are omitted, so an explicit zero is carried by the override marker alone (yaml_contract_test.go:114-139).
- A nil or empty overrides map is written as `overrides: {}`, and loads back as a non-nil empty map. An absent key loads as nil, which means legacy (reset_persistence_test.go:151-229).
- Fixture files with 2-space indentation must still load (reset_persistence_test.go:178).

#### Migrations

- Legacy inheritance, run on load in memory only (inheritance.go:275-325):
  - A field equal to the defaults value is zeroed and treated as inherited.
  - A field that differs is marked overridden. Zero versus non-zero counts as differing, so a legacy file without `enable_hdr` pins HDR Off when default.yaml has it On.
  - With nil defaults, every non-zero field is pinned.
  - gpu.vrr is special: any non-empty literal, including `unset`, is pinned (vrr_test.go:47-58).
  - The first changed save freezes the result (reset_persistence_test.go:345-364). docs/profile-fields.md:142-147 documents this as accepted residual risk.
- `sr_model_preset`, run on every load (migrate_sr_preset.go:9-130):
  - The legacy `dlss.sr_model_preset` key is mapped: auto, k, l and m become auto, K, L and M. The legacy value wins over sr_preset.
  - sr_preset is then normalized: default and auto are lowercased, a single letter is uppercased.
  - An override key `dlss.sr_model_preset` moves to `dlss.sr_preset`.
  - Tests: migrate_sr_preset_test.go:5-86.
- Unknown override keys and `false` entries survive load and save, because the map passes through untouched (inferred from migrate_sr_preset.go:49 and storage.go:96).

### 4. Apply and cleanup

#### Sequence in `Profile.Apply`

1. Environment: proton, DLSS, then GPU env (apply.go:41-43).
2. KDE VRR through `display.ApplyVRR`. It runs in the user session, never under pkexec. An error only logs a warning, but a returned restore is kept even when there is an error (apply.go:45-52; vrr.go:15-17 and :85-86).
3. Hardware, only when needed. "Needed" means clock_offset != 0, memory_offset != 0, power_limit > 0, fan_speed > 0, governor != "" or SMT != nil (apply.go:65-72).
   - Before elevating, it reads the current governor (cpu0), SMT (`smt/active`) and power limit (NVML or smi). Read errors are ignored (apply.go:92-94).
   - It then runs one `privilege.ExecSelf("apply-profile", ...)`. The flags are `--gpu-clock-offset=N`, `--gpu-memory-offset=N`, `--gpu-power-limit=N`, `--gpu-fan-speed=N`, `--cpu-governor=X` and `--cpu-smt=on|off`, in that order (apply.go:96-123; apply_hardware_test.go:64-94).
   - On error, it returns no cleanup (apply.go:121-123).
4. The cleanup is `apply-profile --reset`, followed by:
   - `--gpu-power-limit=<prev>` if power was set and prev > 0.
   - `--gpu-fan-speed=0` if fan was set.
   - `--cpu-governor=<prev>` if the governor was set and prev was read.
   - `--cpu-smt=<prev>` if SMT was set. prev is false when the read failed (apply.go:125-147).

The legacy dry-run path, `ApplyEnv`, does the env part only (apply.go:32-36).

#### Privileged half

`apply-profile` is a hidden command (outside scope, read: apply_profile.go:36-171).

- It refuses to run when not root.
- Apply steps, in order: lock graphics clocks, lock memory clocks, power limit, fan (only if > 0), governor (validated against `scaling_available_governors`), then SMT.
- It returns at the first error and does not roll back the steps that already ran.
- The `--reset` steps:
  - It always resets graphics and memory locked clocks first.
  - It then restores the power limit if one was passed and is > 0.
  - Fan 0 means reset fans to automatic.
  - It then sets the governor, then SMT.
  - It also returns at the first error.
- `gpu.Init()` runs in `main()` before any command, so the root process uses NVML when it is available (main.go, outside scope, read).
- ExecSelf runs `pkexec <EvalSymlinks(os.Executable())> apply-profile ...`. When the process is already root it runs the binary directly.
  - Exit code 126, or output containing "authentication cancelled", "canceled" or "dismissed", becomes an AuthError.
  - Any other non-zero exit becomes an error carrying stderr (privilege.go:44-61 and :120-131).

#### Cleanup order and failure handling

- The launcher registers Apply's cleanups in the order KDE VRR, then hardware. It registers the overlay cleanup after both (launcher.go:79-87, 198-217).
- `runCleanup` runs in reverse: overlay, hardware, then KDE VRR. A failure is logged and the next step still runs (launcher.go:254-266).
- The game still launches after a VRR or hardware apply failure; both are warnings only (apply.go:47-58).
- TODO.md:11 (`iynrkonane`) tracks the missing restore after a failed apply.

### 5. Environment builder

- `Environment` is a map. `BuildEnv` returns `os.Environ()` followed by the map entries, with no dedupe (env.go:8-28).
- Go's exec runtime keeps the last duplicate (inferred from Go's documented behaviour; unverified here). The test helpers read the last match (apply_test.go:159-167).
- `ApplyToCmd` does not change the process environment (env_test.go:26-44).
- Setters: `PROTON_ENABLE_WAYLAND=1`, `PROTON_ENABLE_HDR=1` and `PROTON_ENABLE_NGX_UPDATER=1`.
- `EnableVKD3DHeap(legacy)` sets `VKD3D_CONFIG=descriptor_heap`, and `PROTON_VKD3D_HEAP=1` only when legacy is true. Profile code always passes false (apply.go:163). The launcher sets the variable itself.
- `SetShaderCache(path)` sets `__GL_SHADER_DISK_CACHE=1` and `__GL_SHADER_DISK_CACHE_PATH`.
- `SetThreadedOptimization` sets `1` or `0` (env.go:34-68).
- DLSS variables are set through `Set` (section 1).
- The full set of variables these packages can emit: the 8 above, plus `DXVK_NVAPI_DRS_NGX_DLSS_{SR,RR,FG}_OVERRIDE`, `..._DLSS_{SR,RR}_MODE`, `..._DLSS_{SR,RR}_OVERRIDE_RENDER_PRESET_SELECTION`, `DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT` and `DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS`.
- Outside scope: `SPELA_OVERLAY_IPC`, set by the launcher. Wrapper-supplied environment is applied after Prepare, so it wins over profile values (main.go, outside scope, read).

### 6. Game database and running detection

- games.yaml has top-level `games` (a map from uint64 app id to a game) and `updated_at`.
  - Each game has `app_id`, `name`, `install_dir`, `library_path`, `prefix_path` (omitempty), `dlls` (omitempty) and `scanned_at`.
  - Each DLL has `path`, `name`, `type` and `version` (omitempty).
  - DLL types: dlss, dlssg, dlssd, xess and fsr (game.go:8-33; database.go:42-45).
- `Transaction(update)`:
  - It takes the in-process mutex and the flock, then loads the database. A missing file becomes an empty map.
  - It saves only when the callback reports a change (database.go:52-82).
  - The cross-process behaviour is tested (database_transaction_test.go:28-72).
- `FindGame(query)`: a numeric query is tried as an app id first, then the name is matched exactly. The map is unordered, so duplicate names resolve unpredictably (inferred) (database.go:148-160).
- `List` and `GamesWithDLSS` drop tool names. GamesWithDLSS keeps games with dlss, dlssg or dlssd (database.go:162-184).
- Tool name patterns, case-insensitive: `^proton(\s|$)`, `^steam\s+linux\s+runtime`, `^steamworks`, `redistributable` and `^steam\s+controller` (database.go:23-40; game_test.go:31-53).
- `RunningGames` (running.go:11-100; running_test.go:9-60):
  - It scans `/proc/<pid>` for every pid except its own, reading the `cwd` and `exe` links and the NUL-split `cmdline`.
  - A game is running when an argument equals `AppId=<id>`, or when cwd, exe or any argument equals the install folder or sits under `folder/`. The symlink-resolved folder also counts.
  - Unreadable processes are ignored.
  - The only production caller is `dll.AutoUpdate` in the TUI and GUI (grep: tui/services.go:65).

### 7. Steam

- Root detection (library.go:18-32, 127-132):
  - The configured `steam_path` wins.
  - Otherwise it takes the first existing directory of `$HOME/.steam/steam`, `$HOME/.local/share/Steam` and `$HOME/.var/app/com.valvesoftware.Steam/.steam/steam`.
- Libraries (library.go:34-79, 163-200):
  - They come from `<root>/steamapps/libraryfolders.vdf`, node `libraryfolders`.
  - Each child node gives `path`, `label` and `apps`; entries with an empty path are skipped.
  - Old-format string children are ignored (inferred: library.go:53-57).
  - Extra paths from config are added after these, with label "additional", deduplicated by exact string.
  - A missing or unparsable vdf when a root exists is an error for the whole scan.
- Scanning a library (library.go:81-124):
  - It globs `steamapps/appmanifest_*.acf` and skips parse failures and nil manifests.
  - It keeps a manifest only when StateFlags == 4.
  - It drops tools: a known app id (36 entries, filter.go:11-51), a `toolmanifest.vdf` in the install dir, or a tool name.
  - The install dir is `<library>/steamapps/common/<installdir>`.
  - The prefix is `<library>/steamapps/compatdata/<appid>`, valid when `pfx/drive_c` is a directory (prefix.go:16-29).
  - DLLs come from `dll.ScanDirectory` (out of scope).
- Manifest parsing (appmanifest.go:20-62):
  - `AppState` gives appid, name, installdir, StateFlags, LastUpdated and SizeOnDisk.
  - An appid that is 0 or not a number gives nil, nil.
  - The library path is the manifest's grandparent directory.
- VDF parser (vdf.go:13-104):
  - It works line by line. A line that is exactly `{` opens a node under the last single-token key. A line that is exactly `}` closes it. `//` lines are comments.
  - A line with 2 tokens is a key-value pair; any other token count is ignored.
  - Quotes are not escaped, and a later duplicate key wins.
  - An unbalanced `}` is the only error.
- Rescan (refresh.go:10-46):
  - Inside the game Transaction, it scans and then replaces the whole database. Games that disappear are dropped.
  - No libraries gives the error `could not find Steam installation`.
  - `RefreshIfNeeded` rescans when the database is empty or `rescan_on_startup` is on (refresh_test.go:10-26).
  - The DLL updater waits for the rescan lock (refresh_concurrency_test.go:17-114).

### 8. Proton

- Resolver (resolver.go:48-179):
  - It needs a non-empty Steam root.
  - It reads `<root>/config/config.vdf` at `InstallConfigStore > Software > Valve > Steam > CompatToolMapping`. It tries `<appid>.name` first, then `"0".name`.
  - It then looks for the build directory in `<root>/compatibilitytools.d/<name>`, then `<root>/steamapps/common/<name>`.
  - Every failure wraps `ErrProtonNotResolved`.
- Legacy gate: the `<build>/proton` script contains the bytes `PROTON_VKD3D_HEAP`. A missing script means false with no error (resolver.go:88-101).
- `SupportsVKD3DHeap`: true when the build uses the legacy gate, or when its CachyOS tag is at or above `11.0-20260520` (resolver.go:75-84; requirements.go:35-39).
- Build tag: the case-insensitive pattern `cachyos[-_]?(\d+)\.(\d+)-(\d{8})`, compared by major, minor, then date (build_version.go:8-65).
- Driver version parsing (driver.go:44-137):
  - Input is trimmed. Empty input gives `ErrDriverUnavailable`.
  - It accepts 1 to 4 dot-separated integers and ignores the 4th.
  - Comparison uses major, minor, then patch. An unavailable version sorts lowest.
  - `String()` gives `M.m.p`, or `unavailable`.
  - The minimum is `580.94.16` (requirements.go:27).
- Notices (notice.go:106-187; notice_test.go):
  - `⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ (detected: <name>)`
  - `⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: <M.m.p>)`
  - `⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ and NVIDIA driver 580.94.16+ (detected: <name>, driver <M.m.p>)`
  - The info prefix is `ⓘ `. Info lines:
    - `could not resolve active Proton for this game; skipping compatibility check`
    - `Proton resolution failed; skipping compatibility check`
    - `Proton marker probe failed; skipping compatibility check`
    - `NVIDIA driver probe failed; skipping driver compatibility check`
    - `NVIDIA driver not detected; skipping driver compatibility check`
    - `NVIDIA driver version unparsable; skipping driver compatibility check`
  - A hard failure on one axis wins over an info skip on the other.
  - Nil dependencies give an empty notice.
- Callers: the CLI `proton show`, the TUI services, and the launcher preflight, which uses `CheckCompatibility` for slog lines (grep: commands/proton.go:20-33, tui/services.go:111, launcher.go:181-196).

### 9. Display (KDE VRR)

- `ApplyVRR(policy)` reads `XDG_CURRENT_DESKTOP` and `XDG_SESSION_TYPE` (vrr.go:18-20).
- "" and `unset` do nothing. Any policy other than automatic, always or never is an error. The session must be `wayland` and the desktop list (split on `:`) must contain `KDE`, compared case-insensitively (vrr.go:36-45, 102-113).
- Command: `kscreen-doctor` with a 10-second context timeout, env `LC_ALL=C` appended, and stdout only (vrr.go:24-34). There is no D-Bus use anywhere in scope (verified by grep).
- Discovery uses `kscreen-doctor -o`. Parsing (vrr.go:129-206):
  - ANSI colours are stripped first.
  - `Output: <id> <name> <uuid>` must have exactly 4 fields and a valid, non-null, unique UUID.
  - Each output needs exactly one each of enabled/disabled, connected/disconnected, `priority N` and `Vrr: <policy>`. The policy is lowercased.
  - Anything missing or duplicated fails closed.
- Selection: exactly one output must have priority 1, and it must be enabled and connected. Its policy must be valid; `incapable` fails. If it already matches, nothing changes and no restore is returned (vrr.go:50-68).
- Set: `kscreen-doctor output.<uuid>.vrrpolicy.<policy>`. Stdout containing `applying config failed!`, in any case, is treated as an error (vrr.go:89-100).
- Restore runs discovery again and finds the original UUID. It requires that output to be connected, enabled and still capable, then sets the original policy. It never re-targets the current primary (vrr.go:69-84; vrr_test.go:21-62, 117-168).
- docs/kde-vrr.md matches the code.

### 10. GPU

- NVML initialisation: `Init`, `DeviceGetCount` (must be greater than 0) and `DeviceGetHandleByIndex(0)`. Only GPU 0 is used (nvml.go:16-37).
- Metrics through NVML (nvml.go:44-90):
  - Temperature (GPU sensor), power usage (mW to W), power management limit, GPU utilization, memory used and total (bytes to MiB), graphics clock, memory clock and fan speed.
  - Throttle reasons: HwThermalSlowdown, SwThermalSlowdown, SwPowerCap and HwPowerBrakeSlowdown.
- Info through NVML (nvml.go:170-218):
  - Keys: name, driver, memory (`N MB`), temperature (`N°C`), power_draw (`N.N W`), power_limit (`N W`), power_range (`min–max W`, with an en dash), graphics_clock, memory_clock, fan_speed (`N%`) and utilization.
- NVML setters, all root-only (nvml.go:92-168):
  - `SetGpuLockedClocks(min,max)` and `SetMemoryLockedClocks(min,max)`.
  - `SetPowerManagementLimit(W*1000)`.
  - `ResetGpuLockedClocks` and `ResetMemoryLockedClocks`.
  - `SetFanSpeed_v2(i,speed)` and `SetDefaultFanSpeed_v2(i)` for every fan from `GetNumFans`, or fan 0 when that call fails (nvidia.go:61-92).
- Policy (nvidia.go:15-57):
  - When root and NVML is available, the code uses NVML.
  - Otherwise it uses `pkexec nvidia-smi`: `-lgc 0,<2100+N>`, `-lmc 0,<N>`, `-pl <W>`, and `-rgc` then `-rmc`.
  - Fan control has no smi fallback and errors out (nvml_test.go:85-90).
- smi readers run `nvidia-smi` with stdout and stderr combined and no timeout (nvidia.go:195-202):
  - Info: `--query-gpu=name,driver_version,memory.total,temperature.gpu,power.draw --format=csv,noheader,nounits`. Keys: name, driver, memory, temperature and power (nvidia.go:114-132).
  - Metrics: `--query-gpu=temperature.gpu,power.draw,power.limit,utilization.gpu,memory.used,memory.total,clocks.gr,clocks.mem --format=csv,noheader,nounits`. Fan and throttle are not filled (nvidia.go:168-193).
  - Driver: `--query-gpu=driver_version --format=csv,noheader,nounits` (driver_version.go:54).
  - Output is split on `", "`.
- `GetCurrentPowerLimit` truncates the metrics power limit to whole watts (nvidia.go:31-37).
- `DriverVersionString` returns "", nil when no driver is found (driver_version.go:45-62).

### 11. CPU

- sysfs paths (cpu.go:39-142):
  - Read `/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor` and `.../scaling_available_governors`.
  - Write `/sys/devices/system/cpu/cpu<i>/cpufreq/scaling_governor` for i from 0 to `runtime.NumCPU()-1`.
  - Read `/sys/devices/system/cpu/smt/active`, where "1" means on.
  - Write `/sys/devices/system/cpu/smt/control` with `on` or `off`.
- Validation: the governor must appear in `scaling_available_governors`. The error lists the available governors, or `none` (cpu.go:60-82; cpu_test.go:86-104).
- When not root:
  - The governor is written with `pkexec tee <path>`, once per CPU, with the value on stdin.
  - SMT uses one `pkexec tee`.
  - The non-root governor path ignores the test root (cpu.go:84-100, 121-134).
- Read-only helpers:
  - `GetCPUInfo` returns model (from /proc/cpuinfo), cores, governor and smt.
  - `GetCPUMetrics` returns scaling_cur_freq per CPU (kHz to MHz), average, governor, SMT, and RAM from /proc/meminfo. RAM used is total minus available.
  - Its "utilization" is load1 / NumCPU * 100, capped at 100 (cpu.go:144-257).
  - `SCXIsAvailable` looks up `scx_loader` on PATH. `SCXStatus` runs `systemctl is-active scx.service` (cpu.go:176-185).
- Affinity: nothing exists, although AGENTS.md:18 says it does.

### 12. Nav (TUI-relevant part)

- Enums and labels (nav.go:9-170):
  - Destination: Library, DLL Catalog, Monitor, Settings. Hotkeys are "1" to "4".
  - ScopeKind: global or game.
  - Aspect: Overview, Profile, DLLs.
  - DLLCatalogSection: Library, Deployment.
  - MonitorSection: GPU, CPU, Alerts.
  - SettingsSection: Display, Startup, Paths, DLL policy, Logging.
  - ProfileSubsystem: Proton, DLSS, GPU, CPU, Overlay, with YAML keys.
- `DefaultState`: Library, global scope, Profile, Proton.
- Transitions (nav.go:200-249; nav_test.go):
  - SelectDestination resets the section. For Library, global scope forces Profile, and game scope keeps DLLs, else Profile.
  - SelectScope: global forces Profile; a game keeps DLLs, else Profile.
  - SelectAspect blocks Overview and DLLs at global scope.
- Breadcrumb: `Spela › Library › All games|<game> › <Aspect> › <Subsystem>`. The subsystem only appears with Profile (nav.go:258-292).
- What the TUI actually uses (grep):
  - nav.State, DefaultState, Scope, the enums, SelectScope (3 uses), SelectAspect (5 uses), MonitorSectionLabels and DLLCatalogSectionLabels.
  - It sets `Destination` directly (tui/layout.go:213-221), not through SelectDestination.
  - It never sets ProfileSubsystem.
  - `Breadcrumb()` is called only from `renderBreadcrumbs`, which has no production caller; a test calls it (tui/layout.go:311-327; tui/shell_contract_test.go:93).
- gui.go is GUI-only (Wails JSON state and contract). It drops AppID on round trip (gui.go:75-89). Nothing in it is needed for CLI or TUI.

### 13. External commands run from these packages

| Command | Arguments | Where |
|---|---|---|
| kscreen-doctor | `-o` | display/vrr.go:130 |
| kscreen-doctor | `output.<uuid>.vrrpolicy.<never|automatic|always>` | display/vrr.go:90 |
| nvidia-smi | `--query-gpu=name,driver_version,memory.total,temperature.gpu,power.draw --format=csv,noheader,nounits` | gpu/nvidia.go:115 |
| nvidia-smi | `--query-gpu=temperature.gpu,power.draw,power.limit,utilization.gpu,memory.used,memory.total,clocks.gr,clocks.mem --format=csv,noheader,nounits` | gpu/nvidia.go:169-172 |
| nvidia-smi | `--query-gpu=driver_version --format=csv,noheader,nounits` | gpu/driver_version.go:54 |
| pkexec nvidia-smi, or nvidia-smi when root without NVML | `-lgc 0,<2100+N>`, `-lmc 0,<N>`, `-pl <W>`, `-rgc`, `-rmc` | gpu/nvidia.go:20,27,43,53,56 |
| pkexec tee, or a direct write when root | `/sys/devices/system/cpu/cpu<i>/cpufreq/scaling_governor` (stdin is the value) | cpu/cpu.go:92-98 |
| pkexec tee | `/sys/devices/system/cpu/smt/control` | cpu/cpu.go:129 |
| systemctl | `is-active scx.service` | cpu/cpu.go:182 |
| pkexec `<spela path>` | `apply-profile [--reset] --gpu-...=N --cpu-...=X` | profile/apply.go:96-147 via privilege.ExecSelf |

Lookups only: `exec.LookPath("scx_loader")` (cpu.go:177) and `pkexec` (privilege.go:34, outside scope).

### 14. In-scope docs

- **profile-fields.md.** It mostly matches the code, and a test enforces the table key, kind and label order (vocabulary_test.go:280-314). The mismatches are listed under conflicts: retired words, the pin rule for defaults, and the cpu.governor unset value.
- **configuration.md.** These rows matter for the domain:
  - `shader_cache` must be absolute or empty, and a relative value falls back to the default. The profile's shader_cache_path has no such check.
  - `steam_path` and `additional_library_paths` feed the scan.
  - `rescan_on_startup` gates RefreshIfNeeded.
  - `auto_update_dlls` uses RunningGames.
- **kde-vrr.md.** It matches the code.
- **NVAPI.md.** This is a copy of the upstream dxvk-nvapi wiki. Several Spela values fall outside its lists; see the conflicts.
- **gamemode.sh.** This is the user's pre-Spela script. Spela never runs it, and the docs backlog marks it for removal (beans-source/spela-dop3 note). It explains where some fields came from:
  - Real nvidia-settings offsets: `GPUGraphicsClockOffset`, `GPUMemoryTransferRateOffset` and `GpuPowerMizerMode`.
  - `taskset -c` for affinity.
  - SMT through `smt/control`.
  - `__GL_SHADER_DISK_CACHE_PATH=$HOME/.cache/nvidia`.

### 15. Notes for the rewrite

- Keep one descriptor table as the source of words, kinds and allowed values. The 35-field count and the doc table test are good guards to carry over.
- Decide whether the YAML must stay byte compatible. The Go tests pin 4-space indentation and struct order. serde_yaml and serde_norway emit 2-space indentation by default (unverified for the chosen crate). Loading must accept both. The overrides map must serialize `{}` for an empty or absent map, never `null`, and the "absent key" state must survive load as a separate case.
- Keep the cross-process flock semantics. Go's flock on separate opens is exclusive within one process as well (storage_transaction_test.go:13-38), so the Rust lock must open a new descriptor for each transaction, or add an in-process mutex.
- Environment variables need last-wins dedupe before spawning, to match Go's exec behaviour (inferred).
- The privileged argv contract (`apply-profile` flags) is internal. It can change in the rewrite, but it is the place to fix the restore holes.
- GUI-only APIs that the CLI and TUI do not need: nav/gui.go, PinField (unless the TUI adopts it), OwnValueSuffix and the GUI JSON contract.

### Conflicts and mismatches

- **YAML byte contract versus Rust emitters.** The Go tests pin exact bytes for the saved profile. Indentation is 4 spaces. Sections follow struct order (dlss, gpu, cpu, proton, overlay), and leaves follow struct order within each section (gpu starts with vrr, proton with enable_wayland). Override keys are sorted. Descriptor order, used in the docs, nav and UI, is different: proton, dlss, gpu, cpu, overlay, with hdr and clock_offset first. Common Rust YAML emitters default to 2-space indentation. 2-space fixtures must still load. Evidence: internal/profile/yaml_contract_test.go:31-108; profile.go:31-99; fields.go:61-101; reset_persistence_test.go:178; docs/profile-fields.md:3-6
- **Only gpu.vrr is validated on load and save.** Load and save reject an invalid gpu.vrr and name the field. Every other bounded field (sr_mode, presets, governor, position, power_mizer, multi_frame) round-trips any stored value without validation. Only Set and WriteField validate. Also, a bad vrr in default.yaml makes LoadDefault fail, and that breaks every Mutate and every legacy Load. Evidence: internal/profile/migrate_sr_preset.go:35-42; storage.go:87-92, 56-62, 144-147; vrr_test.go:67-76
- **Validation error text starts with an empty value.** The allowed-values list includes "", so the error message reads 'unsupported value "x" (valid: , unset, automatic, always, never)', with a leading empty entry. Evidence: internal/profile/fields.go:203-204, 80
- **SMT restore defaults to off when the prior state is unreadable.** The error from smtStatus is discarded, so prevSMT stays false. The cleanup then passes --cpu-smt=off, which turns SMT off at game exit on a machine where it was on but smt/active could not be read. No test covers the read-error path. Evidence: internal/profile/apply.go:93, 136-141; apply_hardware_test.go:64-123
- **Reset always resets clocks first and aborts at the first error.** apply-profile --reset calls resetClocks before anything else, even when the profile set only the governor or SMT. An error returns immediately, so a GPU that rejects the locked-clock reset blocks the restore of power limit, fan, governor and SMT. Evidence: cmd/spela/commands/apply_profile.go:122-162 (outside scope, read); profile/apply.go:125-147
- **Failed privileged apply leaves partial state with no restore.** The root apply returns at the first failing setter and does not roll back the steps before it. The user side then returns no cleanup on error, so changes that did apply are never restored. VRR is the opposite: it keeps the restore even when apply errors. TODO.md tracks this as an open bug. Evidence: cmd/spela/commands/apply_profile.go:78-120; internal/profile/apply.go:121-123 vs display/vrr.go:85-86 and profile/apply.go:46-52; TODO.md:11
- **Clock and memory 'offsets' are absolute clock locks.** The fields are labelled 'offset' and the apply-profile flags are named offsets. The code locks graphics clocks to 0..(2100+N) MHz and memory clocks to 0..N MHz. The descriptions state this honestly, but the labels, the flag names and the docs table ('unbounded, MHz') do not. gamemode.sh shows the original intent was real nvidia-settings offsets. TODO.md tracks this. Evidence: internal/gpu/nvidia.go:15-28; fields.go:79, 81; cmd/spela/commands/apply_profile.go:44-45; docs/gamemode.sh:31-33; docs/profile-fields.md:30,32; TODO.md:10
- **Negative memory offset is not guarded.** needsHardwareApply passes any non-zero memory_offset. A negative value is cast to uint32 for NVML, which wraps to a huge value, or is sent to nvidia-smi as '0,-N'. A negative clock_offset below -2100 wraps the same way. There is no range check because the range is unbounded by design. Evidence: internal/gpu/nvidia.go:15-28; profile/apply.go:65-72, 98-103 (inferred effect)
- **Threaded optimization is forced to 0 whenever any profile applies.** SetThreadedOptimization always writes 1 or 0, so a launch with any game or default profile exports __GL_THREADED_OPTIMIZATION=0 and overrides the driver default. When the field's origin is 'system default', the source text reads 'system default' even though the value really applied is 0. The description admits that 'Off is applied'. Evidence: internal/profile/apply.go:219; env.go:62-68; fields.go:87; vocabulary.go:110-123
- **DLSS mode 'off' is not a documented NGX mode.** sr_mode and rr_mode allow 'off' and export it as DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE=off. The dxvk-nvapi values in docs/NVAPI.md are PERFORMANCE, BALANCED, QUALITY, SNIPPET_CONTROLLED, DLAA, ULTRA_PERFORMANCE, CUSTOM and DEFAULT. Evidence: internal/profile/fields.go:67,70; apply.go:171-173; docs/NVAPI.md:35-44, 116-125
- **Preset 'default' maps to an undocumented value.** dlssPresetToEnv turns 'default', and any unknown value, into 'render_preset_default'. NVAPI.md lists 'DEFAULT' and 'OFF', not RENDER_PRESET_DEFAULT. It also lists presets G, H, I, N, O and RENDER_PRESET_LATEST, which Spela does not offer. Evidence: internal/profile/apply.go:242-249; fields.go:68,71; docs/NVAPI.md:89-108, 170-189
- **Multi-frame allows 4, docs allow 0 to 3.** The descriptor allows 0 to 4 ('Extra frames to generate, 0 to 4'). NVAPI.md says the valid values are 0 (application-controlled), 1 (2x), 2 (3x) and 3 (4x). So 0 does not mean 'zero extra frames'. Evidence: internal/profile/fields.go:75; apply.go:191-193; docs/NVAPI.md:195-208
- **DLSS indicators are classed as ephemeral, but the tweak is persistent.** The indicator fields are restore class 'ephemeral launch'. NVAPI.md says DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS permanently modifies the registry. Spela only writes the 'on' values and never writes DLSSIndicator=0 when the field is Off, so turning it off in Spela likely does not hide an indicator that was set before. Evidence: internal/profile/fields.go:76-77; apply.go:196-205; docs/NVAPI.md:260
- **Dry-run environment omits PROTON_VKD3D_HEAP.** ApplyEnv, used by the launch summary, never sets PROTON_VKD3D_HEAP. The launcher adds it at Prepare whenever Proton resolution fails or the build uses the legacy gate. So the printed environment can differ from the real one. Evidence: internal/profile/apply.go:32-36, 162-164; internal/launcher/launcher.go:103-127; cmd/spela/commands/launch.go:139-150 (outside scope, read)
- **VKD3D heap description overstates 11.0 support.** The description says 'Needs Proton-CachyOS 10.0-20260321+ or 11.0+'. The code requires the integrated build tag 11.0-20260520 or later; an 11.0 build dated earlier counts as unsupported. The notice text names only the 10.0 minimum. Evidence: internal/profile/fields.go:65; proton/requirements.go:33-39; proton/resolver.go:75-84; proton/notice.go:130-138
- **GPU info map keys differ between NVML and nvidia-smi.** NVML returns power_draw, power_limit, power_range, clocks, fan_speed and utilization. The smi fallback returns 'power' instead of 'power_draw' and lacks the other keys. A test locks in the smi key 'power'. Evidence: internal/gpu/nvml.go:189-215; nvidia.go:125-131; nvml_test.go:61-64
- **smi metrics lack fan speed and throttle data.** The NVML metrics fill FanSpeed and ThrottleReasons. The nvidia-smi query has no fan.speed column, so FanSpeed is 0 and ThrottleReasons is nil on the fallback. Evidence: internal/gpu/nvml.go:76-87; nvidia.go:168-193
- **Single GPU and single-line smi assumptions.** NVML always uses device index 0. The smi parsers split all output on ', ', so a multi-GPU line set would corrupt a field, and DriverVersionString would return several lines that ParseDriverVersion rejects. smi also runs with stderr merged and no timeout. Evidence: internal/gpu/nvml.go:28; nvidia.go:120,177,195-202; driver_version.go:54-61 (multi-GPU effect inferred)
- **Installed check uses StateFlags equal to 4.** IsFullyInstalled is StateFlags == 4. StateFlags is a bitfield in Steam, so an installed game with other flags set (for example, an update pending) would be dropped from the library. The tests cover only 4 and 2. Evidence: internal/steam/appmanifest.go:60-62; appmanifest_contract_test.go:31-37; library_test.go:108 (bitfield semantics unverified)
- **VDF parser limits, and appmanifest tests that hit the wrong branch.** Braces must sit on their own lines, and there is no escape handling. The subtests 'missing app ID' and 'invalid app ID' use inline braces on one line. That line tokenizes to 5 tokens, is ignored, and fails at the missing AppState check, so the appid parse branch (appmanifest.go:37-40) is not exercised by those cases. Evidence: internal/steam/vdf.go:25-46, 56-86; appmanifest_contract_test.go:39-54; appmanifest.go:32-40
- **A broken libraryfolders.vdf aborts scans of extra libraries.** When a Steam root exists, a GetLibraries error fails librariesToScan, so configured additional_library_paths are not scanned either. Evidence: internal/steam/library.go:163-171; refresh.go:15-28
- **Library order and duplicate app ids are not deterministic.** Libraries come from ranging over a Go map, so their order is random. If the same appid is in two libraries, AddGame keeps whichever comes last. FindGame by name also ranges over a map. Evidence: internal/steam/library.go:53-76, 155-157; game/database.go:154-158 (effect inferred)
- **List accepts .yml but Load reads only .yaml.** List() accepts *.yml with a numeric stem, then calls Load(appID), which reads only <id>.yaml. So a .yml profile is silently skipped. Evidence: internal/profile/storage.go:278-291, 29-31
- **Inconsistent 'own value' grammar for one game.** OwnValueSuffix(1) says '1 game has its own value'. ApplyToOwnValueBody with one name says '1 game has their own value: Alpha. It will follow...', and a test locks that in. Evidence: internal/profile/vocabulary.go:208-217, 234-248; vocabulary_shared_test.go:43-48, 63
- **Retired words list differs between code and doc.** The code retires 8 phrases, including 'This change does not reach' and 'Effect:'. docs/profile-fields.md lists 6. Evidence: internal/profile/vocabulary.go:37-48; docs/profile-fields.md:67-70
- **Pin rule for All games is documented but not enforced.** The doc says pin 'is invalid for the defaults root'. PinField has no such guard. In production only the GUI calls it. Evidence: docs/profile-fields.md:95-96; internal/profile/inheritance.go:159-178; grep: PinField callers tui 0, cmd 0, gui 1
- **Governor value sets disagree.** The profile allows performance, powersave, schedutil and ondemand. The cpu package constants are performance, powersave, ondemand and conservative, with no schedutil. The real check is the sysfs available list at apply time. The doc table also omits the unset value that the code allows. Evidence: internal/profile/fields.go:89; cpu/cpu.go:28-33, 60-71; docs/profile-fields.md:39
- **Governor writes assume contiguous online CPUs.** The governor is written to cpu0..runtime.NumCPU()-1. NumCPU is affinity-limited and counts online CPUs only. The previous governor is read from cpu0 only. Restore sets the governor before turning SMT back on, so re-onlined siblings may not get it, and writing to an offline CPU may fail and abort the rest of the reset. Evidence: internal/cpu/cpu.go:39-45, 91-110; cmd/spela/commands/apply_profile.go:149-159 (effects inferred, need a live test)
- **CPU 'utilization' is load average.** The comment says the value comes from /proc/stat. The code reads /proc/loadavg and divides load1 by NumCPU. Evidence: internal/cpu/cpu.go:235-257
- **AGENTS.md claims CPU affinity support.** AGENTS.md says internal/cpu handles 'affinity via sysfs'. No affinity code exists, and the cpu.affinity field is a no-op. Evidence: AGENTS.md:18; internal/cpu/cpu.go (no affinity); fields.go:91
- **Per-CPU pkexec outside apply-profile.** The privilege model in AGENTS.md describes a single batched pkexec round trip. The direct CLI paths (cpu governor and smt commands, and gpu reset when not root) call pkexec once per CPU for the governor, and twice for a clock reset. Evidence: internal/cpu/cpu.go:91-98; gpu/nvidia.go:53-56; cmd/spela/commands/cpu.go:146,168; AGENTS.md privilege model section
- **nav transitions bypassed and breadcrumb unused in TUI.** The TUI assigns Destination directly instead of calling SelectDestination, so the section and aspect reset rules do not run. It never sets ProfileSubsystem. Breadcrumb() is used only by renderBreadcrumbs, which has no production caller. The nav transition API is exercised mainly by the GUI and the tests. Evidence: internal/tui/layout.go:213-221, 311-327; tui/shell_contract_test.go:93; nav/nav.go:201-292
- **Driver version component count.** The doc comment says 1 to 3 components. The code accepts up to 4 and ignores the fourth. Evidence: internal/proton/driver.go:39-41, 50-53, 75-76
- **Asymmetric unchanged-save rule.** Mutate skips rewriting an existing profile that is unchanged. MutateDefault always rewrites default.yaml. The skip depends on reflect.DeepEqual telling a nil overrides map from an empty one. Evidence: internal/profile/storage.go:153-158, 207-219; reset_persistence_test.go:293-318
- **No fsync on atomic writes.** Profiles and games.yaml use temp file, then rename, without fsync of the file or the directory. A crash can leave an empty or old file. This is a durability gap, not a doc mismatch. Evidence: internal/profile/storage.go:100-117; game/database.go:119-136
- **Override map keeps unknown keys and false entries.** Unmarshal passes any override key through, and saving writes it back. A 'key: false' entry persists but counts as not overridden. countProfileOverrides counts only true entries. Evidence: internal/profile/migrate_sr_preset.go:17, 49; storage.go:96; cmd/spela/commands/launch.go:283-294 (outside scope)
- **Profile shader cache path is not checked like the Settings path.** configuration.md says folder options reject relative paths and that a relative config value falls back to the default. The profile's gpu.shader_cache_path is exported as-is, even when relative. Evidence: docs/configuration.md:48-50; internal/config/paths.go:42-51; internal/profile/apply.go:211-217
- **Legacy migration pins zero values against non-zero defaults.** For a file without the overrides key, any field that is zero in the game but non-zero in default.yaml becomes a pinned zero override, as well as fields that genuinely differ. This is intended to preserve legacy apply behaviour and is documented as residual risk. The legacy test does not assert the side-effect fields. Evidence: internal/profile/inheritance.go:313-323; inheritance_test.go:211-285; reset_persistence_test.go:170-196; docs/profile-fields.md:142-147

### Open questions

- Must the Rust rewrite read and write the existing files in place, byte compatible (4-space YAML, struct order, 'overrides: {}'), or is a one-time migration to a new layout acceptable?
- Should the 8 no-op fields (gpu.power_mizer, cpu.affinity, overlay.show_*, overlay.toggle_key) be kept as stored-only for parity, implemented, or dropped? TODO.md:9 (xuyalcslma) is open.
- Should gpu.clock_offset and gpu.memory_offset keep the locked-clock behaviour (0..2100+N, 0..N) for parity, or become real offsets? Is the 2100 MHz base intended for the user's GPU? TODO.md:10 (wzpctclhaa) is open.
- Should the rewrite fix the restore holes: no cleanup after a partial privileged apply, reset aborting at the first error, SMT restored to off when unreadable, unconditional clock reset? Or should it reproduce today's behaviour first? TODO.md:11 (iynrkonane) is open.
- Live test: on the user's GPU and driver, do NVML SetGpuLockedClocks, SetMemoryLockedClocks and SetFanSpeed_v2 succeed? Does the nvidia-smi fallback ever matter in practice?
- Live test: after SMT off then on, do the re-onlined CPUs get the restored governor? Does writing scaling_governor for an offline CPU fail on this machine?
- Live test or upstream check: does dxvk-nvapi accept mode 'off', 'render_preset_default' and DLSSG multi-frame 4? Should presets G, H, I, N, O and 'latest' be offered?
- Should Off for the DLSS indicators write DLSSIndicator=0 and DLSSGIndicator=0 to undo the persistent registry change?
- Unverified: which Steam StateFlags values does an installed game have when an update is pending or running? Should 'installed' be a bit test instead of equality to 4?
- Unverified: does the resolver find the user's Proton? CachyOS packages may install compat tools under /usr/share/steam/compatibilitytools.d, which the resolver never checks. Built-in tool names such as 'proton_experimental' may not match their steamapps/common directory names. The real config.vdf key casing also needs checking.
- Is multi-GPU support (device index other than 0) in scope for parity, or is GPU 0 only acceptable?
- Should VDF parsing stay line based without escapes for parity, or should it become a full tokenizer (inline braces, escaped quotes)?
- Should the TUI keep using nav's breadcrumb and ProfileSubsystem (currently unused in production), or should those be dropped together with gui.go?
- Should __GL_THREADED_OPTIMIZATION=0 keep being exported whenever any profile applies, or only when a profile sets the field?

## DLL subsystem, denylist, overlay and release check

This section covers the Go spela code for the Rust rewrite. Its scope is internal/dll, internal/denylist, internal/overlay, internal/release, data/manifest.json, tools/dlss-updater and tools/coverage-inventory. It also covers docs/design/dll-operations.md, docs/design/overlay.md, docs/NVAPI.md and the domain side of the CLI commands dll, denylist, overlay and dlss. The tests are treated as the spec.

How claims are labelled:
- **(verified: path:line)** means I read it in the file.
- **(inferred)** means a conclusion from the code that I did not run.
- **(unverified)** means it needs a live check.

"The doc says" is kept apart from "the code does". All paths are relative to /home/jgabor/git/spela unless they are absolute.

### 1. External contracts a rewrite can break

#### 1.1 The live manifest URL
- `DefaultManifestURL = "https://raw.githubusercontent.com/jgabor/spela/main/data/manifest.json"` (verified: internal/dll/manifest.go:30).
- The daily workflow `.github/workflows/dll-manifest.yml` commits to `data/manifest.json` on `main` (verified: .github/workflows/dll-manifest.yml:139-191). It runs on a 06:00 cron, on manual dispatch and on every push to main (verified: dll-manifest.yml:3-9).
- Every installed Go client fetches that exact path. If the rewrite restructures the repo and moves or reshapes the file, old clients stop getting DLL updates (inferred). The JSON schema is in section 2.4.

#### 1.2 Release tags
- The update check reads `https://api.github.com/repos/jgabor/spela/tags?per_page=100` (verified: internal/release/release.go:28).
- It accepts only tags matching `^v?\d+\.\d+\.\d+$` (verified: release.go:41).
- Current tags run up to v0.6.2. There are also three `dll-*-v310.5.3` tags in the same repo (verified with `git tag -l`).
- Release builds get their version from `-X main.version` (verified: magefile.go:40, cmd/spela/main.go:21, cmd/spela/main.go:116).
- A Rust release must keep publishing plain vX.Y.Z tags newer than the Go releases, or Go users never see the notice (inferred).

#### 1.3 On-disk state the Rust binary should read
All paths use the app name `spela` under the XDG directories.

| State | Path | Format | Evidence |
|---|---|---|---|
| Backup index | `$XDG_DATA_HOME/spela/backups/<appid>/backup.json`. It never moves, even when `backup_path` changes. | JSON with `app_id`, `game_name`, `created_at`, `backup_path`, and `files[]` of `original_path`, `backup_path`, `dll_name`, `version`. Paths are absolute. | verified: internal/dll/swap.go:20-33, 44-46; internal/config/paths.go:28-30 |
| Backup files | `<backup_path or $XDG_DATA_HOME/spela/backups>/<appid>/<sha256 hex of cleaned original path>-<basename>` | raw DLL copy | verified: swap.go:37-39, 304-307; config/paths.go:23-25 |
| Legacy backup files | `<backup dir>/<appid>/<basename>`, referenced from the metadata | raw DLL | verified: swap_test.go:65-87 |
| DLL cache | `<dll_cache_path or $XDG_CACHE_HOME/spela/dlls>/<manifest key>/<version>.dll` | raw DLL | verified: download.go:18-24; config/paths.go:15-17; paths_test.go:22-47 |
| Manifest cache | `$XDG_CACHE_HOME/spela/manifest.json` | The manifest JSON plus `fetched_at` (omitzero) and `source_url` (omitempty). An old cache without them counts as fetched from the default URL at the file's mtime. | verified: manifest.go:31, 55-59, 83-89, 212-225; manifest_policy_test.go:99-125; docs/configuration.md:52-54 |
| Update check state | `$XDG_CACHE_HOME/spela/update-check.json` | JSON `{"checked_at": RFC3339, "latest": "vX.Y.Z"}` | verified: release.go:34, 58-61, 124 |
| Deny list (optional user file) | `$XDG_CONFIG_HOME/spela/lists/denylist.yaml` | YAML `entries: [{app_id, name, reason}]` | verified: denylist.go:14-22, 48-50 |
| Deny overrides | `$XDG_CONFIG_HOME/spela/lists/overrides.yaml` | YAML `allowed: [appid]`, `denied: [{app_id, name, reason}]` | verified: denylist.go:24-27, 52-54 |
| Detected DLL versions | inside `games.yaml` (`dlls[].version`) | strings from `formatVersion` (section 2.2) | verified: internal/game/game.go:28-33; detect.go:154-162 |

Backups are the safety-critical item. A user who swapped DLLs with Go spela can only get the original files back if the Rust binary reads this index and both backup filename schemes (inferred).

#### 1.4 Overlay IPC is not an external contract today
See section 4.8. No consumer exists, so the layout can change freely in the rewrite unless the user wants to keep compatibility with a planned layer.

### 2. DLL subsystem (internal/dll)

#### 2.1 DLL families and detection
- **Filename map, matched case-insensitively** after lowercasing the entry name (verified: detect.go:17-24, 69-70):
  - `nvngx_dlss.dll` maps to dlss.
  - `nvngx_dlssg.dll` maps to dlssg.
  - `nvngx_dlssd.dll` maps to dlssd.
  - `libxess.dll` maps to xess.
  - `amd_fidelityfx_vk.dll` and `amd_fidelityfx_dx12.dll` both map to fsr.
- The stored `Name` keeps the on-disk case (verified: detect.go:72-76).
- **Ordered catalogue.** dlss "DLSS", dlssg "DLSS-G", dlssd "DLSS-D", xess "XeSS", fsr "FSR". The fsr filename is `amd_fidelityfx_vk.dll` only. The order is load-bearing for the TUI deployment table (verified: detect.go:36-44). `KnownDLLTypes()` returns a copy (verified: detect.go:48-52; test manifest_download_detect_test.go:183-190).
- **DLLType string values** are `dlss`, `dlssg`, `dlssd`, `xess` and `fsr`. They double as manifest keys (verified: internal/game/game.go:10-16).
- **Scan.** `ScanDirectory` walks the whole install directory with `filepath.WalkDir`, which does not follow symlinks (verified: detect.go:54-83).
  - Any walk error, even in a nested folder, aborts the scan and returns the error. Results gathered so far are returned too (verified: detect.go:61-64; test manifest_download_detect_test.go:209-217).
  - A missing directory is an error (verified: manifest_download_detect_test.go:180-182).
- **Not detected (inferred):** FSR 4 file names such as `amd_fidelityfx_upscaler_dx12.dll`, and XeSS frame generation files (`libxess_fg.dll`, `libxell.dll`). The FSR 4 names appear in the backlog (TODO item onlsraejfs).

#### 2.2 Version parsing from a DLL file
- `GetDLLVersion` opens the file as a PE with Go's `debug/pe` (verified: detect.go:85-106):
  - It takes the first `.rsrc` section.
  - It finds the UTF-16LE string `VS_VERSION_INFO`.
  - From there it finds the `VS_FIXEDFILEINFO` signature bytes `BD 04 EF FE`.
  - It reads `dwFileVersionMS` and `dwFileVersionLS` at signature+8 and signature+12, little-endian (verified: detect.go:108-145).
- This is FileVersion, not ProductVersion (verified: detect.go:128-132).
- **Formatting** (verified: detect.go:147-162; test manifest_download_detect_test.go:192-206):
  - major.minor is 0.0: empty string.
  - revision 0 and build 0: `major.minor`.
  - revision 0: `major.minor.build`.
  - otherwise all four parts.
- Errors are swallowed during a scan. A non-PE file gets an empty version, shown as "unknown" (verified: detect.go:71; cmd/spela/commands/dll.go:80-83).
- I did not check that NVIDIA's current DLL FileVersion (for example 310.5.3.0) formats to the manifest string `310.5.3` (unverified, needs a real DLL).

#### 2.3 Version comparison
- `CompareVersions` (verified: version.go:11-47; tests version_test.go:5-56):
  - strips one leading `v`;
  - splits on `.`;
  - treats a non-numeric part as 0, so `3-beta` becomes 0;
  - pads the shorter version with zeros.
- `IsNewer(installed, available)` is true when installed < available (verified: version.go:50-52).
- The release check reuses this function (verified: release.go:84, 114). It is shared logic, so it belongs in a common module (inferred).

#### 2.4 Manifest format, URL, cache and policy
**Schema** (verified: manifest.go:35-50; data/manifest.json:1-68):
- Top level: `version` (string, "1"), `updated_at` (RFC 3339), `repository` (string) and `dlls`, a map from manifest key to a list ordered newest first.
- Each entry has:
  - `version`
  - `filename` (the in-game file name used by install)
  - `url`
  - `sha256` (64 hex characters)
  - `size`
  - `release_date` (RFC 3339)
  - `notes`

**What the bundled data contains:**
- dlss, dlssg and dlssd each have 310.5.3 and 310.5.0. xess and fsr are empty lists (verified: data/manifest.json:6-67).
- `repository` is `jgabor/spela-dlls` (verified: data/manifest.json:4).
- The 310.5.3 URLs point at `github.com/jgabor/spela/releases/...` (verified: data/manifest.json:10, 30, 50). The 310.5.0 URLs point at `jgabor/spela-dlls` (verified: data/manifest.json:19, 39, 59).
- No code outside the GUI reads `repository`, `size`, `release_date`, `notes` or `updated_at`. Size is never checked against the download (verified by grep over internal and cmd, excluding the GUI).

**Lookups:**
- `GetLatestDLL(key)` returns `dlls[key][0]`, so the first entry, not the highest version (verified: manifest.go:227-233).
- `GetDLLVersion(key, v)` matches an exact version string (verified: manifest.go:235-248).
- `ListDLLNames` returns keys sorted alphabetically (verified: manifest.go:250-257).

**Fetch:**
- GET with a shared client: 30 s response-header timeout and a 2 min whole-request timeout (verified: manifest.go:19-27, 113-139).
- Non-200 returns an error "HTTP <code>" (verified: manifest_download_detect_test.go:85-87).
- The body is read with no size cap (verified: manifest.go:128).

**Policy, re-read on every call** (verified: manifest.go:157-225):
- **URL:** the configured `dll_manifest_url` is used only if `ValidateManifestURL` accepts it. That means https, or http to localhost or a loopback IP (verified: config/paths.go:60-77). Otherwise the default URL applies, with no warning (verified: manifest.go:175-180; manifest_policy_test.go:127-140). An explicit argument overrides both (verified: manifest.go:181-183).
- **Refresh switch:** `auto_refresh_manifest` (default true).
- **Max age:** `manifest_refresh_hours`, default 24. Zero or negative means 24 (verified: manifest.go:171-174; config/config.go:57-58; manifest_policy_test.go:94-96).
- **GetManifest(force, url):**
  - When not forced, it uses the cache if the cache's source URL equals the policy URL and either auto refresh is off or the cache is younger than max age.
  - Otherwise it fetches and saves.
  - A failed fetch falls back to the cache when one exists (verified: manifest.go:191-210; manifest_policy_test.go:52-97).
  - A changed source is fetched even with auto refresh off (verified: manifest_policy_test.go:87-88).
- **Cache writes are atomic** through a temp file and rename (verified: manifest.go:99-111; swap.go:276-302).
- Nothing in CLI or TUI code passes `force=true`. `dll check-updates` calls `GetManifest(false, "")` (verified: cmd/spela/commands/dll.go:93; internal/tui/content.go:339, 373, 394).

#### 2.5 Download and DLL cache
`acquireDLL(entry, type, allowDownload, progress)` works like this (verified: download.go:43-86):
1. If the cached file's SHA-256 matches the manifest entry (case-insensitive), it uses the cache.
2. If `allowDownload` is false (cached-only), it errors on a read error, a checksum mismatch ("cached DLL checksum mismatch") or a missing file ("cached DLL not found").
3. Otherwise it downloads `entry.URL`. Non-200 returns "download DLL: HTTP <code>". The body streams through a hasher into an atomic temp file. A checksum mismatch aborts before the rename. Progress reports (downloaded, Content-Length).

Further points:
- A corrupt cache is re-downloaded (verified: swap_test.go:205-245). A valid cache makes no request (verified: swap_test.go:247-274).
- A stalled body times out (verified: manifest_download_detect_test.go:144-167).
- **Format:** the payload is the raw DLL. There is no archive and no unpacking on the client. The zip only exists in CI (verified: dll-manifest.yml:78-90).
- `ListCachedVersions(key)` lists `*.dll` files in the type folder and strips the suffix. A missing folder gives nil with no error (verified: download.go:26-41; manifest_download_detect_test.go:127-133).
- The cache folder follows `dll_cache_path` when it is absolute. A relative value falls back to the default (verified: config/paths.go:39-51; paths_test.go:43-46).

#### 2.6 Operations: install, update, batch update and restore
**Types** (verified: operations.go:14-82):
- Outcome is `changed` or `no-op`.
- Stage is `resolving`, `downloading`, `installing`, `updating`, `restoring`, `scanning` or `saving`.
- `ProgressEvent{Stage, DLLType, Version, Filename, Downloaded, Total}`.
- `Result{Outcome, FilesChanged, MetadataPersisted, Game}`.
- `PartialFailure{Result, Stage, Err}` with the message "DLL files changed but metadata did not fully persist during <stage>: ..." (verified: operations.go:57-59).
- `UpdateRequest{AppID, DLLType, Version, InstalledPath, CachedOnly}`.
- `BatchResult{Items, Updated, Unchanged, Failed}`.

**Serialization:**
- Every mutation runs inside `game.Transaction`. It takes an in-process mutex plus a cross-process lock at `$XDG_RUNTIME_DIR/spela/games.lock`, loads a fresh `games.yaml`, and saves atomically only when changed (verified: operations.go:128-143, 169-185; docs/design/dll-operations.md:71-88).
- The lock is held through the download, which can take up to 2 min per DLL (verified: dll-operations.md:75-77).
- A blocked runtime dir or a lock path that is a directory fails the operation (verified: operations_test.go:633-651).
- A game removed before the lock is not brought back (verified: operations_test.go:568-581).

**Install(appID, type, version)** (verified: operations.go:216-250):
1. Requires a type.
2. Resolves "" or "latest" to `GetLatestDLL`, otherwise to the exact version.
3. Validates the entry. The version must be non-empty. The filename must be a bare base name with no separator or traversal. The SHA-256 must be 32 bytes of hex (verified: operations.go:354-377).
4. Picks the target. A detected DLL whose `Name == filename` exactly wins, and more than one match is an error. Otherwise the target is `<InstallDir>/<filename>`, and it must stay inside InstallDir (verified: operations.go:379-416).
5. Applies the denylist guard, acquires the payload (download allowed), backs up, copies atomically, then rescans.
6. Install can write any version, including an older one (inferred).
7. A bad filename is rejected before any request or file change (verified: operations_test.go:520-566).

**Update(request)** (verified: operations.go:252-293):
1. Requires an exact detected path of that type inside InstallDir. It never picks by basename (verified: operations.go:334-345; operations_test.go:506-511).
2. Cached-only needs an explicit version.
3. Resolves the entry and validates the filename.
4. **No-op rule:** if the installed version is known and the target is not newer, it returns `no-op` with `MetadataPersisted` true and no download, scan or save (verified: operations.go:274-277). So update never downgrades, and an unknown installed version always swaps (inferred).
5. Validates the checksum, applies the denylist guard, acquires the payload, backs up, swaps the bytes at the installed path, then rescans.

**UpdateGames(appIDs, type)** resolves the requests inside the transaction from each game's current detected DLLs. An empty type means every family (verified: operations.go:102-123). The CLI's `dll update` uses `UpdateGame` (verified: cmd/spela/commands/dll.go:163).

**BatchUpdate:**
- One transaction runs each request independently and continues after errors (verified: operations.go:125-163; operations_test.go:388-420, 583-616).
- It saves once if anything changed. A failed save turns every changed item into a `PartialFailure` and leaves no-op items successful (verified: operations_test.go:422-474).
- An item counts as updated only after both the rescan and the save succeed.

**Restore(appID)** (verified: operations.go:198-214; swap.go:168-201):
1. Loads the backup index. A missing index gives "no backup found for app N".
2. Preflights every file: the backup is a regular file, the target's parent exists, and the target is not a directory.
3. Replaces each file atomically.
4. If a later replace fails after earlier ones succeeded, it returns `PartialFailure{Stage: restoring}` and skips the rescan and save (verified: operations_test.go:164-209).
- Restore has **no denylist guard**.
- Restore does **not** delete the backup files or the index.
- Restore does **not** remove a DLL that install created where none existed before (verified: swap.go:168-201, 228-255).

**Partial failures:**
- A scan failure after a replace keeps the earlier in-memory metadata (verified: operations_test.go:324-350).
- A save failure keeps the rescanned snapshot in `Result.Game` (verified: operations_test.go:351-374; dll-operations.md:40-48).

**Progress stages emitted:**
- Install: resolving, downloading (repeatedly), installing, scanning, saving (verified: operations_test.go:52-54).
- Update: resolving, downloading, updating, scanning, saving.
- Restore: restoring, scanning, saving.

#### 2.7 Backups
`CreateBackup(appID, name, dlls)` (verified: swap.go:95-166):
- Loads the existing index. A malformed or unreadable index fails closed, so no mutation happens (verified: operations_test.go:275-321).
- Preflights existing backup files and new sources (verified: swap_test.go:322-341).
- Skips any original path that already has an entry. **The first backup of a path is kept forever** (verified: swap.go:118-128, 135-138; operations_test.go:65).
- Refuses to overwrite an orphan backup file that has no metadata (verified: swap.go:139-144; swap_test.go:342-358).
- Removes the files it created if a later step fails.
- Saves the index atomically.
- Backs up **every detected DLL of the game** on the first swap, not only the target (verified: swap.go:210-226).

`LoadBackup` rejects an index whose app_id does not match, whose file list is empty, or that has an empty path (verified: swap.go:48-75; swap_test.go:437-458). Changing `backup_path` puts new files in the new folder while old entries stay restorable (verified: paths_test.go:49-129).

#### 2.8 Atomic writes
`writeAtomically` writes the file like this (verified: swap.go:283-302):
1. Creates a temp file `.spela-write-*` in the target folder.
2. Sets the mode with chmod.
3. Writes the data.
4. Closes the file.
5. Renames it over the target.

It does not fsync (inferred). `copyFile` keeps the destination's existing mode, otherwise the source's (verified: swap.go:257-274).

#### 2.9 Startup auto-update
`AutoUpdate(running)` (verified: autoupdate.go:24-76; tests autoupdate_test.go:13-96):
- Uses the manifest from the normal policy.
- Skips:
  - deny-listed games;
  - DLLs of unknown version;
  - DLLs that are not older than the manifest's latest.
- Calls the `running` predicate only for candidate games. Production uses `game.RunningGames`, a /proc scan (verified: internal/tui/services.go:65; internal/game/running.go:14-16).
- Then runs `BatchUpdate` with explicit latest versions.

`Notice()` text (verified: autoupdate.go:80-103; autoupdate_test.go:50, 64, 98-106):
- Empty when nothing happened.
- `DLL auto-update: updated N DLL(s); K failed (<first error>); skipped A, B while running`.
- `DLL auto-update skipped: <err>` when the manifest is missing.

It runs only from the TUI (and the GUI). It is gated by `auto_update_dlls` (default false), which is visible in the TUI and GUI only. In the TUI it waits for the startup rescan (verified: config/options.go:90; internal/tui/layout.go:121, 138; internal/tui/auto_update.go:13-42).

#### 2.10 What "update available" means on each surface
| Surface | Unknown installed version | Compared against | Evidence |
|---|---|---|---|
| CLI `dll check-updates` | counted as an update ("unknown -> X") | manifest latest | verified: cmd/spela/commands/dll.go:121-129 |
| TUI per-game targets | counted as an update | manifest latest | verified: internal/tui/dll_mutation.go:44 (latestDLLMutationTargets) |
| `update()` domain | always swaps | requested or latest | verified: operations.go:274 |
| AutoUpdate | skipped | manifest latest | verified: autoupdate.go:70 |
| TUI DLL catalogue "stale" | not stale | newest **cached** version | verified: internal/tui/dlls_resource.go isStale (excerpt around the isStale function) |

#### 2.11 CLI surface (`spela dll`)
- **`dll list [game]`.** Without a game it lists `db.GamesWithDLSS()`, which counts only dlss, dlssg and dlssd (verified: cmd/spela/commands/dll.go:54-90; internal/game/database.go:173-184).
  - Output: `<name> (<appid>)`, then `  <file>: <version|unknown>`, then `    <path>`.
  - Games come in Go map order, which is not deterministic (inferred).
  - Empty: "No games with DLSS/FSR/XeSS DLLs found."
- **`dll check-updates`.** Prints `  <file>: <cur> -> <latest>` per game, or "All DLLs are up to date." (verified: dll.go:92-145; test supported_commands_test.go:263).
- **`dll update <game> <type>`** (verified: dll.go:147-210; tests display_and_dll_boundaries_test.go:82-93, supported_commands_test.go:266):
  - Always targets latest.
  - Progress shows as "Downloading <type> <ver>..." and then `\rDownloading: NN.N%`.
  - Results:
    - "game does not have a <type> DLL" when nothing matches.
    - "Updated <type> DLL to version X".
    - "<type> DLLs are already at the latest version".
    - A final line "DLL update result: U updated, N unchanged, F failed".
    - A non-zero exit on any failure.
- **`dll restore <game>`** prints "Restored original DLLs for <name>" or the error, for example "no backup" (verified: dll.go:212-231; display_and_dll_boundaries_test.go:95-99).
- Game lookup is by numeric app ID, then by exact name (verified: internal/game/database.go:148-160).
- **Missing from the CLI, present in the TUI (verified: internal/tui/services.go:56-64):** install, a specific or older version, cached-only update, update all games and the cache listing.
- `launch --dry-run` prints denylist status, whether a backup exists and per-DLL write state (verified: cmd/spela/commands/launch.go:201-225).

#### 2.12 Domain API the TUI uses
`KnownDLLTypes`, `GetManifest`, `LoadManifest`, `ListCachedVersions`, `Install`, `BatchUpdate`, `UpdateGames(appIDs, "")`, `Restore`, `LoadBackup`, `BackupExists`, `IsNewer`, `CompareVersions` and `AutoUpdate` (verified: internal/tui/services.go:56-65; content.go:339-394; layout.go:452; dlls_resource.go:167-213). The design doc assigns confirmation, busy state and message rendering to the adapters and requires them to preserve `PartialFailure` (verified: dll-operations.md:55-69).

### 3. Denylist (internal/denylist)

**Data and source:**
- A built-in list of 10 Steam app IDs (verified: denylist.go:29-42):
  - Apex Legends 1172470
  - Call of Duty: HQ 1938090
  - Battlefield 2042 1517290
  - New World 1063730
  - Lost Ark 1599340
  - Hunt: Showdown 594650
  - PUBG 578080
  - Destiny 2 1085660
  - Rust 252490
  - Escape from Tarkov 1623660
- Each entry has a reason naming the anti-cheat.
- If `lists/denylist.yaml` exists, it **replaces** the built-in list and is not merged with it (verified: denylist.go:60-76; test denylist_test.go:172-195).
- Overrides live in `lists/overrides.yaml` (section 1.3).

**Matching** is by app ID only (verified: denylist.go:109-132):
1. An app in `overrides.allowed` is allowed. This beats a user deny: the CLI test runs deny, then allow, then check and expects "is allowed" (verified: cmd/spela/commands/supported_commands_test.go:279-293).
2. Then `overrides.denied`.
3. Then the deny list.

**Load errors are ignored.** A malformed `denylist.yaml` or `overrides.yaml` makes `IsDenied` return not denied, so it fails open (verified: denylist.go:110, 122).

**What it blocks:**
- install and update, via `guardSwap` before acquiring and again inside the swap and install helpers (verified: operations.go:238, 281; swap.go:203-208, 210-213, 228-231);
- auto-update, which skips the game (verified: autoupdate.go:38-40).

It does **not** block restore or launch. Launch dry-run only reports the status (verified: launch.go:202-206).

**Writes:**
- `Allow` is idempotent. `Deny` updates the name and reason in place (verified: denylist.go:134-169; denylist_test.go:103-159).
- `SaveOverrides` uses a plain `os.WriteFile`. It is not atomic and takes no lock (verified: denylist.go:96-107).
- `RemoveAllow` and `RemoveDeny` exist and are tested (denylist_test.go:161-170), but no CLI or TUI command calls them (verified by grep). So an allow cannot be undone except by editing the file.

**CLI** (verified: cmd/spela/commands/denylist.go:14-177):
- `denylist show` lists "Denied games (anti-cheat):" with a `[FORCE-ALLOWED]` marker, then "User-denied games:".
- `denylist check <game|appid>` prints "<name> is DENIED: <reason>" plus a hint, or "is allowed for DLL swapping".
- `denylist allow <game|appid>` prints a ban warning.
- `denylist deny <game|appid> [--reason]` defaults the reason to "user-specified".
- A raw numeric app ID works even when the game is not in the database.

**TUI:** there is no denylist surface. No match for deny or anti-cheat in non-test TUI files (verified by grep). Users see the domain error "DLL swap denied for app N: <reason>" (verified: swap.go:205).

### 4. Overlay (internal/overlay)

#### 4.1 What it is today
- When `overlay.enabled` is true in the resolved profile, the launcher (the spela wrapper process) creates an mmap file and starts a collector (verified: internal/launcher/launcher.go:198-217).
- The collector writes GPU metrics and an alert summary every 500 ms.
- The launcher exports `SPELA_OVERLAY_IPC=<path>` to the game. It stops the collector and deletes the file at cleanup (verified: setup.go:13-34; launcher_test.go TestPrepareOverlayCreatesIPC; cmd/spela/main_test.go wrapper test around line 114).
- **Nothing reads the file.** It feeds a planned Vulkan layer that does not exist.
- The profile field text says: "Nothing is drawn unless an overlay layer reads it" (verified: internal/profile/fields.go:93).
- A failed overlay setup fails `Prepare` with an error naming the overlay (verified: launcher_test.go TestPrepareFailureRunsCleanupOnce).

#### 4.2 File path and lifecycle
- **Path:** `$XDG_RUNTIME_DIR/spela/overlay-<appid>.dat`, or `/tmp/runtime-$USER/spela/...` when XDG_RUNTIME_DIR is unset (verified: setup.go:18; internal/xdg/xdg.go:31-40).
- **Directory:** created with mode 0700 when missing.
- **File:** created with `O_CREATE|O_RDWR|O_TRUNC`, mode 0600, truncated to 80 bytes, mapped `MAP_SHARED` read-write (verified: ipc.go:72-94).
- **Reader side:** `OpenIPC` requires size >= 80, magic and version to match. It does not check the state offset or state size (verified: ipc.go:96-126, 230-240).

#### 4.3 Exact binary layout, version 1, little-endian, 80 bytes
| File offset | Size | Type | Field | Notes |
|---|---|---|---|---|
| 0 | 4 | u32 | magic | 0x5350454C. The comment calls it "SPEL", but written little-endian the bytes on disk are 4C 45 50 53, which read "LEPS" (verified: ipc.go:14, 223) |
| 4 | 2 | u16 | version | 1 |
| 6 | 2 | u16 | reserved | 0 |
| 8 | 4 | u32 | state offset | 16 |
| 12 | 4 | u32 | state size | 64 |
| 16 | 4 | u32, atomic | seqlock | odd while writing, even when stable |
| 20 | 4 | i32 | temperature | degrees C, can be negative (ipc_test.go:168-186) |
| 24 | 4 | u32 | power draw | milliwatts: watts x 1000, truncated |
| 28 | 4 | u32 | power limit | milliwatts |
| 32 | 4 | u32 | utilization | percent |
| 36 | 4 | u32 | VRAM used | MB |
| 40 | 4 | u32 | VRAM total | MB |
| 44 | 4 | u32 | graphics clock | MHz |
| 48 | 4 | u32 | memory clock | MHz |
| 52 | 4 | u32 | fan speed | percent |
| 56 | 1 | u8 | alert active | 0 or 1 |
| 57 | 1 | u8 | alert severity | 0 info, 1 warning, 2 critical |
| 58 | 1 | u8 | visible | the collector always writes 1 |
| 59 | 1 | u8 | position | 0 top-left, 1 top-right, 2 bottom-left, 3 bottom-right |
| 60 | 20 | bytes | reserved | |

Sources (verified): ipc.go:12-46, 131-167, 172-206; round trip in ipc_test.go:9-85.

#### 4.4 Seqlock semantics
**Writer** (verified: ipc.go:131-167):
1. Keeps a process-local counter.
2. Stores the counter + 1 (odd) atomically.
3. Writes the fields with plain stores.
4. Stores the counter + 1 again (even).

The header is written once at create time, outside the seqlock (verified: ipc.go:91-92, 222-228).

**Reader** (verified: ipc.go:172-206):
1. Loads the sequence and gives up if it is odd.
2. Reads the fields.
3. Loads the sequence again and fails on any change.

There is a single attempt and no retry loop.

**For Rust (inferred):**
- Use the standard seqlock pattern: a Relaxed odd store, a Release fence, the data stores, then a Release even store. The reader does an Acquire load, the data loads, an Acquire fence, then a Relaxed reload.
- Read the data with atomics or volatile reads. Plain reads of memory another process writes concurrently are undefined behaviour in Rust.

#### 4.5 Collector, alerts and thresholds
**Collector** (verified: collector.go:42-61; tests collector_test.go):
- `run()` writes once right away, then on every tick until stopped.
- `Stop()` blocks until the goroutine ends.

**`BuildGPUCollector(position)`** (verified: gpu_collector.go:8-59):
- Reads `gpu.GetGPUMetrics()`. On error it writes only Visible=1 and the position, with zeroed metrics.
- Otherwise it fills the metrics.
- Throttle flags: thermal is HW thermal or SW thermal; power is power cap or power brake.
- If any alert fires, it sets AlertActive and the highest severity.

**`Evaluate(input, thresholds)`**, a pure function (verified: alerts.go:95-155; tests alerts_test.go:5-201):
- **Thermal:**
  - NVML thermal throttle flag: critical "GPU thermal throttling at T C".
  - Otherwise temp >= 85: critical with the same message.
  - Otherwise temp >= 80: warning "GPU temperature high at T C", suggesting a power cut of max(10% of draw, 10 W).
- **Power:**
  - NVML power flag: warning "GPU power-limited at D W/L W", suggesting "Increase power limit to (L x 1.1) W".
  - Otherwise, when limit > 0 and draw/limit >= 95%, the same warning.
- **Fan:** fan >= 95%: info "GPU fans at N%".
- **Defaults:** thermal warning 80, critical 85, power margin 95%, fan max 95 (verified: alerts.go:66-73; alerts_test.go:203-217).
- **Enum strings:** severity info, warning, critical, unknown. Type thermal_throttle, power_limit, fan_maximum, unknown (verified: alerts.go:14-47).
- `AlertInput.GraphicsClock` is filled in but never used (verified: alerts.go:80, 95-155).

**Second consumer: the TUI.** The header and Monitor build the same `AlertInput` from NVML metrics and call `Evaluate` with the defaults (verified: internal/tui/header.go:85-104). The labels are "High temp", "Throttling" (critical thermal), "Power limited" and "Fan max" (verified: header.go alertLabel; header_contract_test.go:182-189). The alert evaluator is a TUI parity requirement whatever happens to the IPC.

#### 4.6 Position and profile fields
- **Position mapping:** `ParsePosition` maps "top-right" to 1, "bottom-left" to 2, "bottom-right" to 3 and anything else to 0 (verified: position.go:9-20; position_test.go).
- **Profile overlay fields:** enabled, position, show_fps, show_frametime, show_cpu, show_gpu, show_vram and toggle_key (verified: internal/profile/profile.go:51-60).
- **Inert fields:** only enabled and position are used. The other six are labelled "Not used yet" (verified: internal/profile/fields.go:95-100).
- **CLI** (verified: cmd/spela/commands/overlay.go:15-179):
  - `overlay set <game>` with --enabled, --position, --show-fps, --show-frametime, --show-cpu, --show-gpu, --show-vram and --toggle-key.
  - `overlay show <game> [--json]`.
  - `overlay reset <game> <field>`.
  - The position value is not validated by the CLI flag, though the profile field lists allowed values (fields.go:94).
- **Open bug:** `spela overlay ...` runs `/usr/bin/overlay` when that program exists, because wrapper-mode detection runs before the command parser (verified: TODO.md:12; .agentera/entities/todo/todo_item/cridhyzjcj.yaml).

#### 4.7 Designed but not built (docs/design/overlay.md and TODO.md)
The doc describes:
- a Zig Vulkan implicit layer `VK_LAYER_SPELA_overlay` with ImGui, MSDF fonts and custom charts (overlay.md:15-117);
- Go components for stats, a UI state machine, XDG GlobalShortcuts input and themes (overlay.md:119-144);
- a single mmap file with a 64-byte header, about 4 KB of state, and event, command and data rings with SPSC atomics (overlay.md:146-205);
- a watchdog and kill switch `SPELA_OVERLAY_DISABLE=1` (overlay.md:243-245);
- `~/.config/spela/overlay.yaml` colors (overlay.md:265-282);
- an alert table including "GPU clock below boost" and 10 s auto-dismiss (overlay.md:286-297);
- polkit session tuning (overlay.md:312-318) and MangoHud coexistence (overlay.md:320-327);
- `SPELA_OVERLAY=1` opt-in through `enable_environment` (overlay.md:337-342);
- mage building Zig, with install to /usr/lib/spela (overlay.md:344-362).

None of this exists:
- There is no `overlay/` directory and no Zig code (verified: repo root listing).
- No `config.go`, `stats.go`, `input.go`, `uistate.go` or `theme.go` exists under internal/overlay (verified: file list).
- No spela layer is installed on this machine (verified: /usr/share/vulkan/implicit_layer.d/ lists other layers only; /usr/lib/spela is absent).
- The backlog has the epic plus Phase 0 to 5 items, all open (verified: TODO.md:26, 30, 32-34, 36-37; agentera items eggbfxzfxp, koenbmhuri, lmqqegkizu).
- The Phase 1 item itself plans the ring-buffer IPC (verified: lmqqegkizu.yaml requirements), which shows the current 80-byte format is an interim.

#### 4.8 Is the layout a contract?
Not today (inferred from sections 4.1 and 4.7). The only producer is spela and the only consumers are Go tests. The rewrite can:
- **(a)** keep layout v1 byte for byte, if a future layer is meant to target it;
- **(b)** drop the writer until a layer exists;
- **(c)** implement the ring design.

This is a user decision.

### 5. Release check (internal/release)

**Endpoint and parsing:**
- GET on `TagsURL` with `Accept: application/vnd.github+json`, a 5 s timeout and a 1 MiB body cap. It decodes `[{name}]` and picks the highest tag that matches vX.Y.Z (verified: release.go:28, 33-37, 90-122).
- Errors: "look up releases: HTTP <code>", "read releases: ...", and "read releases: no release tags".
- There is no pagination beyond `per_page=100`. No User-Agent is set explicitly; Go sends its default (verified: release.go:93-98).
- A Rust HTTP client such as reqwest sends no default User-Agent, and GitHub's REST API rejects requests without one (unverified, check live).

**Caching:**
- `Check(ctx, current, now)` returns nil for non-release versions such as dev, git describe output or AUR -git (verified: release.go:67-70; release_test.go:55-65).
- It asks again when the last attempt is zero, at least 24 h old, or in the future after a clock change (verified: release.go:73).
- Every attempt sets `checked_at`. Only a success updates `latest` (verified: release.go:74-83; release_test.go:67-86).
- An unreadable state file counts as never checked (verified: release.go:71).
- It returns a notice when the stored latest is a release and newer than current, together with any lookup error (verified: release.go:84-87).
- The state file is written atomically (verified: release.go:141-163).

**Version comparison** is `dll.IsNewer` (verified: release.go:18, 84, 114).

**Notice text:** "Spela vX is available (you have vY)", with a `v` added when missing (verified: release.go:53-56, 165-170; release_test.go:39-41).

**TUI:**
- Init runs `checkForRelease(release.Version)` when `check_updates` is on (default true). Failures stay silent (verified: internal/tui/layout.go:135-137; release_check.go:15-26).
- The notice goes to the header (verified: layout_handlers.go:145-146).
- In the full header it is line 6, level with the last logo row. In the compact header it trails line 2 and is cut first. Both use the Warning color (verified: header.go:275-280, 384-386).
- The CLI never checks, though `check_updates` can be set from the CLI (verified: config/options.go:88; docs/configuration.md:15).

**Test seam:** under the `e2e` build tag, `SPELA_E2E_RELEASES_URL` replaces TagsURL when it points at loopback (verified: release_e2e.go:1-22).

### 6. The dlss command and docs/NVAPI.md
**`dlss` CLI** (verified: cmd/spela/commands/dlss.go:16-196):
- `dlss show <game> [--json]`.
- `dlss set <game>` with:
  - `--sr-mode`, `--sr-preset` and a deprecated `--sr-model-preset`;
  - `--rr-mode`, `--rr-preset`, `--rr-override`;
  - `--fg`, `--fg-override`, `--fg-indicator`;
  - `--multi-frame (0-4)`;
  - `--indicator`.
- `dlss reset <game> <field>`.
- Setting a mode or preset also turns its override on. Setting `--fg` or `--multi-frame` turns the FG override on.

These are profile fields, not DLL operations. They become environment variables in `internal/profile/apply.go:168-208`:
- `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on`, plus `..._SR_MODE` and `..._SR_OVERRIDE_RENDER_PRESET_SELECTION`;
- the same set for RR;
- `..._FG_OVERRIDE=on` and `DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT`;
- `DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS=DLSSIndicator=1024,DLSSGIndicator=2`.

`PROTON_ENABLE_NGX_UPDATER=1` is set by the proton option (verified: internal/env/env.go:43; fields.go:64).

**docs/NVAPI.md** is a copy of the dxvk-nvapi "passing driver settings" reference: keys, values and examples (verified: NVAPI.md:1-264). It is the source for the mapping above. Its mismatches with the code are in the conflicts list (multi-frame range, indicator persistence, SR mode "off").

### 7. tools/dlss-updater and the DLL workflows
**What the tool does** (verified: tools/dlss-updater/main.go:38-262; tests main_test.go:15-210):
- Scrapes the TechPowerUp download pages for dlss, dlssg and dlssd with a Firefox User-Agent.
- Gets the version from the `<title>` regex and the file id from a hidden input.
- POSTs `id` and `server_id=27` and reads the 302 `Location` header as the download URL.
- Compares against `dlls[type][0].version` in data/manifest.json, with a leading v trimmed.
- `IsNew` is true when the manifest is empty **or the strings differ** (verified: main.go:112).
- Prints text or JSON (`type`, `version`, `download_url`, `filename`, `is_new`, `source`).
- Exit codes: 0 new, 2 up to date, 1 error, 2 for a flag parse error, 0 for -h (verified: main.go:82-138; main_test.go:152-210).

**Who runs it:** `.github/workflows/dll-manifest.yml` (verified: dll-manifest.yml:1-191):
1. Runs daily, per DLSS family.
2. Builds the tool with Go 1.22.
3. When the result is new, downloads the TechPowerUp **zip**, unzips it and finds the file by name.
4. Computes SHA-256 and size.
5. Creates the tag `dll-<type>-v<ver>` and a GitHub release **in this repo** (`github.repository`).
6. Inserts the entry at index 0 and commits the manifest to main, retrying the push up to 5 times.

**Manual alternative:** `.github/workflows/dll-release.yml` releases to `jgabor/spela-dlls`. It writes `filename: "<type>.dll"` and a matching URL, and inserts at index 0 (verified: dll-release.yml:58-115).

**History:**
- The last manifest commits are by github-actions[bot] on 2026-01-31, 2026-02-01 and 2026-02-03.
- Before those, commit 49feb7b on 2026-01-17 says "move DLL releases to separate spela-dlls repository" (verified: `git log -- data/manifest.json`).
- No manifest commit has come in more than eight months, so the scraper may be failing (unverified).

**Does the Rust rewrite need it?**
- Not in the shipped binary. It is CI infrastructure that keeps the manifest current (inferred).
- The rewrite still needs some feeder for the manifest at the URL in 1.1, or users get no new DLLs.
- Options: keep the Go tool and workflow untouched beside the Rust crate, port it to a small Rust binary, or turn it into a script. This is a user decision.

### 8. tools/coverage-inventory (brief)
- A Go-only CI gate. It parses every non-test `.go` file under cmd, internal and tools, and fails if any file with a non-empty function body is missing from the given Go coverprofiles (verified: tools/coverage-inventory/main.go:1-178).
- Called from the magefile coverage target (verified: magefile.go:228).
- Not relevant to a Rust crate except as the precedent of "every source file is covered". cargo-llvm-cov or similar would replace it (inferred).

### 9. Tests that pin behaviour (the spec to port)
- **internal/dll:**
  - install/update/no-op/restore with exact bytes (operations_test.go:20-84);
  - checksum and denylist failures before any mutation (86-117);
  - cached-only verification (119-162);
  - partial restore (164-209);
  - concurrent exact-path updates with duplicate basenames (211-273);
  - fail closed on a bad backup index (275-321);
  - partial failures for scan and save (323-386);
  - batch semantics (388-474, 583-616);
  - validation errors (476-518);
  - path escape (520-566);
  - no resurrection of a removed game (568-581);
  - lock and setup failures (618-661).
- Also in internal/dll:
  - backup and restore primitives (swap_test.go);
  - configured cache and backup paths (paths_test.go);
  - manifest fetch, cache, download, detection and the version resource (manifest_download_detect_test.go);
  - refresh policy (manifest_policy_test.go);
  - auto-update and its notices (autoupdate_test.go);
  - compare table (version_test.go).
- **internal/denylist:** denylist_test.go (defaults, overrides, idempotency, the file replacing the list).
- **internal/overlay:**
  - IPC round trip, multiple writes, bad magic, a file that is too small, the seqlock, negative temperatures (ipc_test.go);
  - the Evaluate table (alerts_test.go);
  - collector timing (collector_test.go);
  - positions (position_test.go).
- **internal/release:** release_test.go (once a day, dev builds skipped, failure recorded, unusable tag lists).
- **CLI contracts:**
  - cmd/spela/commands/display_and_dll_boundaries_test.go:59-99 (outputs "No games", "game not found", "does not have", "already at the latest", "no backup");
  - supported_commands_test.go:241-293 (dll list, check-updates "3.8.10 -> 3.9.0", update "Updated dlss DLL to version 3.9.0", restore, then denylist deny, check, show, allow, check);
  - error_contracts_test.go:38 ("unknown overlay field").
- **Launcher overlay:** launcher_test.go TestPrepareOverlayCreatesIPC, TestPrepareFailureRunsCleanupOnce, TestPrepareNoOverlayWithoutProfile; cmd/spela/main_test.go wrapper test (IPC created and cleaned up).
- **E2E:** tests/e2e/mock_helper.go:172-268 writes a mock manifest and turns auto-refresh off, so tests never touch the network.

### 10. Notes for the Rust design (inferred, not decisions)
- **Shared crate code:** put version parsing and comparison in one shared module used by the DLL, release and TUI code.
- **Domain/adapter boundary:** keep the dll-operations.md contract. The domain owns resolve, acquire, back up, replace, rescan and save, emits progress events, and returns a typed partial failure. Adapters own confirmation and rendering. This fits a ratatui architecture where effects run off the UI thread and post messages back.
- **Atomic writes:** keep same-directory temp files, rename, and the leftover-temp handling. Consider fsync of the file and directory.
- **Lock:** keep the cross-process lock path `$XDG_RUNTIME_DIR/spela/games.lock`, so the Go and Rust binaries cannot corrupt `games.yaml` if both are installed during a transition.
- **Manifest parsing:** treat the time fields as optional RFC 3339. A strict parser that rejects a missing `updated_at` would break the policy tests' fixtures (manifest_policy_test.go:27).

### Conflicts and mismatches

- **Overlay IPC file path.** The doc says the file is $XDG_RUNTIME_DIR/spela-overlay-<pid>.dat. The code writes $XDG_RUNTIME_DIR/spela/overlay-<appid>.dat, keyed by Steam app ID inside a spela subfolder. Evidence: docs/design/overlay.md:148 vs internal/overlay/setup.go:18 and internal/xdg/xdg.go:31-39
- **Overlay IPC layout.** The doc describes a 64-byte header with flags and three ring offsets, about 4 KB of state with display strings and UI state, plus event, command and data rings. The code has a 16-byte header (magic, version, reserved, state offset, state size) and a 64-byte state of numeric fields, 80 bytes in total. Evidence: docs/design/overlay.md:152-205 vs internal/overlay/ipc.go:12-46
- **Overlay Go package structure.** The doc lists config.go, stats.go, ipc.go, alerts.go, input.go, uistate.go and theme.go. The code has alerts.go, collector.go, gpu_collector.go, ipc.go, position.go and setup.go. There is no input, UI state, theme or config code, and no overlay/ Zig directory. Evidence: docs/design/overlay.md:69-144 vs the internal/overlay file list and the repo root listing
- **Overlay opt-in environment variable.** The doc says Spela sets SPELA_OVERLAY=1 to enable the layer through enable_environment. The code only sets SPELA_OVERLAY_IPC=<path>. SPELA_OVERLAY_DISABLE is not read anywhere. Evidence: docs/design/overlay.md:245, 339-340 vs internal/launcher/launcher.go:211
- **Overlay alert table vs Evaluate.** The doc's alert table differs from Evaluate:
- The doc lists a 'GPU clock below boost' alert. It is not implemented, and AlertInput.GraphicsClock is never read.
- The doc fires the fan alert at 100%. The code fires at 95% or more.
- The doc's power message promises '+{estimated}% FPS'. The code suggests limit x 1.1 W.
- The doc says alerts auto-dismiss after 10 s. No such timer exists in the overlay package. Evidence: docs/design/overlay.md:290-297 vs internal/overlay/alerts.go:80, 95-155
- **Alert precedence comment vs behaviour.** The comment says NVML throttle reasons take precedence and that thresholds are used only when all flags are false, as with the nvidia-smi fallback. The code falls back to thresholds whenever a flag is false, even with NVML available. So 86 C with no NVML throttle still says 'GPU thermal throttling', and the TUI labels it 'Throttling'. Evidence: internal/overlay/alerts.go:83-85, 93-94, 99-112; internal/tui/header.go alertLabel; header_contract_test.go:185-186
- **Inert overlay profile fields.** The CLI help describes show-fps, show-frametime, show-cpu, show-gpu, show-vram and toggle-key as working settings. The field descriptors say 'Not used yet', and nothing reads them. Visible is always 1 and no toggle exists. Evidence: cmd/spela/commands/overlay.go:49-54 vs internal/profile/fields.go:95-100 and internal/overlay/gpu_collector.go:13-16
- **IPC magic byte order.** The comments say the magic is 'SPEL'. It is written as the little-endian u32 0x5350454C, so the bytes on disk read 'LEPS'. A non-Go reader that compares bytes would fail. Evidence: internal/overlay/ipc.go:14, 24, 223
- **Seqlock reader retry.** The doc says the layer retries a read while the seqlock is odd or changed. The only reader, in Go, tries once and returns false. Evidence: docs/design/overlay.md:203 vs internal/overlay/ipc.go:172-206
- **Overlay doc archive claim.** The migrated backlog says the original design was archived to overlay-v0.md. No such file exists in docs/design. Evidence: docs/migrations/beans-source/spela-bsbq--rewrite-overlay-design-document.md:14 vs docs/design listing
- **Restore and the denylist.** The doc says every workflow uses the denylist. Restore never calls guardSwap. That is probably the intended behaviour, but the doc overstates it. Evidence: docs/design/dll-operations.md:3-5 vs internal/dll/operations.go:198-214 and swap.go:168-201
- **Manifest repository field vs entry URLs and history.** The repository field says jgabor/spela-dlls. The 310.5.3 URLs point at jgabor/spela releases, while the 310.5.0 URLs point at spela-dlls. Commit 49feb7b (2026-01-17) moved releases to spela-dlls, but the bot workflow still uses github.repository, so later entries went back to jgabor/spela. No client code reads the field. Evidence: data/manifest.json:4, 10, 19, 30, 39, 50, 59; .github/workflows/dll-manifest.yml:149; .github/workflows/dll-release.yml:71, 94; git log -- data/manifest.json
- **dll-release.yml filename.** The manual workflow writes filename '<type>.dll', for example dlss.dll, with a matching URL. Install uses the manifest filename to pick or create the in-game file, so it would write dlss.dll into the game folder instead of nvngx_dlss.dll. Update only needs a base name, so it is unaffected. Evidence: .github/workflows/dll-release.yml:74, 93-94 vs internal/dll/operations.go:234, 379-400
- **Meaning of 'latest' in the manifest.** GetLatestDLL returns the first list entry without sorting by version. Both workflows insert new entries at index 0, and dlss-updater flags any string difference as new. A manual or scraped older version would therefore become 'latest', and install would deploy it. Evidence: internal/dll/manifest.go:227-233; dll-manifest.yml:159-160; dll-release.yml:105-108; tools/dlss-updater/main.go:112
- **Unvalidated manifest version in a filesystem path.** The filename is fully validated, but the version is only checked for being non-empty before it is joined into the cache path <cache>/<type>/<version>.dll. A version containing path parts could escape the cache folder. Evidence: internal/dll/download.go:18-19; internal/dll/operations.go:354-377
- **DLL list and check-updates skip XeSS and FSR.** The messages say 'No games with DLSS/FSR/XeSS DLLs found', but GamesWithDLSS counts only dlss, dlssg and dlssd. XeSS-only and FSR-only games never appear in 'dll list' without an argument or in 'dll check-updates'. Evidence: cmd/spela/commands/dll.go:69, 73, 103-105 vs internal/game/database.go:173-184
- **FSR maps two files to one manifest key.** Both amd_fidelityfx_vk.dll and amd_fidelityfx_dx12.dll are detected as fsr, but the catalogue and manifest have a single fsr entry, with filename vk. An update swaps the one fsr payload into whichever file was detected, so a dx12 file could receive vk bytes. This is latent while the fsr list is empty. Evidence: internal/dll/detect.go:22-23, 43; operations.go:289; data/manifest.json:67
- **Case handling: detection vs install.** Detection lowercases names and keeps the original case. installTarget compares Name == manifest filename exactly. A game with NVNGX_DLSS.DLL would get a second, lowercase file at the install root on a case-sensitive filesystem. Evidence: internal/dll/detect.go:69-76 vs internal/dll/operations.go:382
- **Five meanings of 'update available'.** Each surface treats an unknown installed version differently:
- CLI check-updates and TUI targets count it as an update.
- update() always swaps it.
- AutoUpdate skips it.
- The TUI catalogue 'stale' marker ignores it and compares against the newest cached version instead of the manifest. Evidence: cmd/spela/commands/dll.go:121; internal/tui/dll_mutation.go:44; internal/dll/operations.go:274; internal/dll/autoupdate.go:70; internal/tui/dlls_resource.go isStale
- **README says check-updates fetches.** The README comment says 'fetch/check available versions'. The command uses the cached manifest under the refresh policy and never forces a fetch. Evidence: README.md:119 vs cmd/spela/commands/dll.go:93 and internal/dll/manifest.go:191-210
- **A test assertion that cannot fail.** TestCopyFileLeavesDestinationIntactOnSourceMissing checks that no '<dst>.tmp' file is left. Temp files are named .spela-write-*, so the check passes whatever happens. Evidence: internal/dll/swap_test.go:200-202 vs internal/dll/swap.go:284
- **Denylist fails open.** The README presents the denylist as blocking known unsafe swaps. A malformed or unreadable denylist.yaml or overrides.yaml is ignored by IsDenied, so nothing is denied and anti-cheat games can be swapped. A user denylist.yaml also replaces the built-in entries instead of merging with them. Evidence: internal/denylist/denylist.go:60-76, 109-132; README.md:111-114
- **Denylist removal not wired.** RemoveAllow and RemoveDeny exist and are tested, but no CLI, TUI or GUI command calls them. An 'allow' cannot be undone without editing overrides.yaml. Allow also beats a user deny, so a later 'deny' has no effect. Evidence: internal/denylist/denylist.go:112-119, 171-201; denylist_test.go:161-170; cmd/spela/commands/supported_commands_test.go:279-293
- **Overrides file writes not atomic.** The DLL code writes atomically everywhere, but SaveOverrides uses os.WriteFile with no temp file, rename or lock. Evidence: internal/denylist/denylist.go:96-107 vs internal/dll/swap.go:283-302
- **Multi-frame range.** NVAPI.md lists 0 (app-controlled), 1 (2x), 2 (3x) and 3 (4x). The CLI help and field descriptor allow 0 to 4 and describe the value as 'extra frames to generate', which differs in both range and meaning. Evidence: docs/NVAPI.md:197-208 vs cmd/spela/commands/dlss.go:60 and internal/profile/fields.go:75
- **DLSS indicator persistence.** NVAPI.md says DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS permanently modifies the registry. The field is labelled as lasting only for the launch, and turning it off just leaves the variable unset (it never writes DLSSIndicator=0). The indicator probably stays on in the prefix. Evidence: docs/NVAPI.md:260 vs internal/profile/fields.go:76 and internal/profile/apply.go:196-205
- **SR mode 'off'.** 'off' is an accepted SR mode and is passed straight to DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE. NVAPI.md does not list OFF as a value for that key. Evidence: internal/profile/profile.go:6; cmd/spela/commands/dlss.go:51; internal/profile/apply.go:171-173 vs docs/NVAPI.md:35-44
- **NVAPI.md example error.** The DLSS-SR section's custom-scale example uses the RR keys (NGX_DLSS_RR_MODE and NGX_DLSS_RR_OVERRIDE_SCALING_RATIO). The error is copied from the upstream wiki. Evidence: docs/NVAPI.md:50
- **CI Go version vs go.mod.** dll-manifest.yml installs Go 1.22, but go.mod requires go 1.25.5. Whether toolchain auto-download saves the build is unverified. The manifest's last bot commit is 2026-02-03, so the job may be failing. Evidence: .github/workflows/dll-manifest.yml:33-40; go.mod:3; git log -- data/manifest.json
- **Release check pagination and shared tag namespace.** The check reads only the first page of 100 tags. The DLL workflow pushes dll-<type>-v<ver> tags into the same repo, so over time the newest vX.Y.Z could fall off page one. Evidence: internal/release/release.go:28, 106-121; .github/workflows/dll-manifest.yml:120-124
- **check_updates visible in the CLI but the CLI never checks.** The option can be set from the CLI. Only the TUI and GUI run release.Check. The configuration doc says 'When the TUI or GUI starts', so the doc agrees with the code, but the CLI's visibility is odd. Evidence: internal/config/options.go:88; docs/configuration.md:15; internal/tui/layout.go:135-137
- **Manifest fetch has no body cap.** The release check caps responses at 1 MiB. The manifest fetch reads an unbounded body into memory. Evidence: internal/dll/manifest.go:128 vs internal/release/release.go:36, 109
- **Restore and backup refresh semantics.** The index keeps the first original of each path forever. Restore deletes neither the backup nor files that install created where none existed before. If Steam later updates a game's DLL, restore rolls back to the older original. An install into a game with no DLLs creates no backup, so restore then fails with 'no backup found' and cannot remove the installed file. Evidence: internal/dll/swap.go:118-138, 168-201, 228-255
- **Power alert suggestion ignores the max limit.** The suggestion always proposes limit x 1.1 W without checking the GPU's maximum power limit, so the advice can be impossible to follow. Evidence: internal/overlay/alerts.go:123-141
- **Wrapper detection captures 'spela overlay'.** An open bug: IsWrapperMode runs before the command parser, so 'spela overlay show <id>' ran /usr/bin/overlay (mesa-demos). Evidence: TODO.md:12; .agentera/entities/todo/todo_item/cridhyzjcj.yaml
- **CLI and TUI feature split.** DLL install, version pinning, cached-only updates and update-all exist only in the TUI. Denylist management exists only in the CLI, and the TUI shows no denylist state. Full parity for CLI and TUI means the union, and the rewrite must decide whether to equalise. Evidence: cmd/spela/commands/dll.go:47-52; internal/tui/services.go:56-64; grep for deny in internal/tui (none)

### Open questions

- Must the Rust binary read Go-era on-disk state as is? Backups matter most (backups/<appid>/backup.json, hash-prefixed and legacy basename files). The same applies to the manifest cache, update-check.json, lists/*.yaml and games.yaml version strings. Should both binaries share the same XDG 'spela' folders and the games.lock path during the transition?
- Will data/manifest.json stay at raw.githubusercontent.com/jgabor/spela/main/data/manifest.json after the rewrite? Existing Go installs and the daily CI job depend on that path.
- Should tools/dlss-updater and the two DLL workflows stay in Go beside the Rust crate, be ported to Rust, or become a script? And is the daily scraper actually succeeding? The last bot commit to the manifest is 2026-02-03, and CI pins Go 1.22 against go.mod 1.25.5.
- Overlay: keep the 80-byte v1 IPC writer byte for byte, drop it until a layer exists, or implement the ring-buffer design in docs/design/overlay.md? Is the Zig layer plan still current?
- Should the inert overlay fields (show_fps, show_frametime, show_cpu, show_gpu, show_vram, toggle_key) and their CLI flags be carried over for parity even though they do nothing?
- For each hazard, fix it or keep it exactly? The list:
- GetLatestDLL takes the first entry, not the highest version.
- The denylist fails open on a malformed file.
- The manifest version is unvalidated in the cache path.
- FSR maps two files to one manifest key.
- Install matches filenames by exact case.
- The XeSS and FSR omission in 'dll list' and 'dll check-updates'.
- dll-release.yml writes a broken '<type>.dll' filename.
- Restore semantics: should restore delete DLLs that install created where none existed before? Should backups refresh after Steam updates a game's DLL, or after a successful restore?
- Should the rewrite make the CLI and TUI equal for DLL install, version pinning, cached-only and update-all (TUI only today), and for denylist management including un-allow and un-deny (CLI only today, removal not wired anywhere)?
- Should 'denylist allow' keep beating a user 'deny', or should the most recent action win?
- Live check: does the PE FileVersion of current NVIDIA DLLs (for example nvngx_dlss.dll 310.5.3) format to exactly the manifest string '310.5.3'?
- Live check: which multi_frame values does the current dxvk-nvapi accept (0-3 per NVAPI.md vs 0-4 in spela)? Does the DLSS indicator stay on in the prefix after the profile turns it off?
- Live check: does PROTON_ENABLE_NGX_UPDATER=1 override a DLL that spela swapped at launch time? If so, the DLL management and the proton option interact.
- Release check in Rust: confirm that GitHub's tags API rejects requests without a User-Agent (reqwest sends none by default). Decide how the Rust build embeds its version so the 'only plain X.Y.Z builds check' rule holds.
- Should the CLI also show the newer-release notice and the DLL auto-update, or stay TUI only as today?
- /home/jgabor/git/spela.old is an older Rust 'spela' (authored as Mutker Telag, codeberg) with no DLL code seen in its README. Is it related to this rewrite, or should it be ignored?

## Spela TUI parity contract (scope: internal/tui, tests/e2e, TUI design docs)

Conventions:
- Paths without a prefix are relative to `/home/jgabor/git/spela`. Paths starting with `mejl/` are relative to `/home/jgabor/git/mejl`.
- Each claim has a label:
  - (verified: path:line) means I read it there.
  - (inferred) means I reasoned it from the code without running anything.
  - (unverified) means I did not check it.
- "Doc says" and "code does" are kept separate. Nothing was built or run.

### 1. Authority map: which documents describe the shipped TUI

| Document | Status | Evidence |
| --- | --- | --- |
| docs/design/tui-navigation.md | Current behavioral contract. It matches the code in almost every point (exceptions are in section 23). | "There is no draft, no Save command, and no Discard command" (verified: docs/design/tui-navigation.md:118). Autosave is in the code (verified: internal/tui/pane_actions.go:33-39). |
| docs/tui-testing.md | Current testing procedure. | Says "Profile changes are saved as they are made" (verified: docs/tui-testing.md:8). |
| AGENTS.md TUI testing section | Current. It is a short form of tui-testing.md. | (verified: AGENTS.md:62-120) |
| PLAN.md | Historical. Phase 3 describes drafts, a dirty state and Save/Cancel. Phase 2 opens a game on Overview. Both are now superseded. Its non-goals still bind. | Drafts (verified: PLAN.md:72-96). "Open a selected game on Overview" (verified: PLAN.md:57). Non-goals (verified: PLAN.md:139-149). |
| docs/design/tui-basic-key-navigation-plan.md | Historical. It still describes Ctrl+S, Save and Discard, Overview first, draft guards and Show hints. | Ctrl+S (verified: docs/design/tui-basic-key-navigation-plan.md:13-14,75). Drafts (verified: :154-198). Show hints (verified: :218-219,247,282). |
| docs/design/tui-basic-key-acceptance.md | Historical acceptance of the draft model. | Input/Apply/Cancel/Save and Ctrl+S (verified: docs/design/tui-basic-key-acceptance.md:19). |
| docs/design/tui-space-and-dll-controls-acceptance.md | Mixed. The Space cycle `(default) → true → false → (default)` is retired. The direct DLL controls still hold. | Space cycle (verified: docs/design/tui-space-and-dll-controls-acceptance.md:11). DLL controls (verified: :17-22). |
| docs/design/tui-profile-settings-ux-review.md | Research that led to the current model (alternative B). Its status line is stale. | "awaiting your choice ... Nothing has been implemented" (verified: docs/design/tui-profile-settings-ux-review.md:3-5). B is in the code (verified: internal/tui/profile_header.go:20-59, internal/tui/detail.go:542-580). |
| docs/design/tui-profile-settings-ux-review-plan.md | Historical plan. It records the user's direction "Basic keys only stays the rule". | (verified: docs/design/tui-profile-settings-ux-review-plan.md:110-118) |
| docs/design/tui-profile-settings-ux-review-mocks.md | Non-binding drawings. They diverge from the code. | The B-allgames mock says "2 games keep their own (above)" (verified: mocks.md:420). A test forbids "keep their own value" (verified: internal/tui/profile_vocabulary_test.go:202-206). The B-settings single page was never built (verified: mocks.md:445-489 versus internal/tui/options_modal.go:358-386). |
| TODO.md | Open TUI bugs still affect what parity means. | The field editor fix is open (verified: TODO.md:7). The Actions menu item is open (verified: TODO.md:44). The redesign task is still open although the code has it (verified: TODO.md:40). |

### 2. Shell model and state

- The shell owns these fields (verified: internal/tui/layout.go:39-76):
  - the destination (`navState`);
  - pane focus (`focus`, List or Detail);
  - `inputMode`;
  - overlay flags (`actions`, `decision`, `showHelp`, `showBatchMenu`);
  - the search session;
  - batch state;
  - rescan bookkeeping;
  - density;
  - size.
- The destination bar is never focusable. Digits 1 to 4 switch destination from either pane and always return focus to List (verified: internal/tui/layout.go:213-221; test internal/tui/shell_contract_test.go:116-130).
- Each destination keeps its own cursor, group and filters. This works because the TUI sets `Destination` directly and does not call `nav.State.SelectDestination`, which would reset sub-sections (verified: internal/tui/layout.go:213-221 versus internal/nav/nav.go:200-218). Library multi-selection also survives a destination change (verified: internal/tui/routing_contract_test.go:189-202).
- The effective input mode is computed, not stored (verified: internal/tui/layout_actions.go:84-94):
  - Edit when a pane editor is active;
  - Search when a search session exists;
  - Overlay when any shell overlay or an in-pane DLL flow is open.
- `nav.AspectOverview` still exists (verified: internal/nav/nav.go:66-72), but the TUI maps it to Profile. A game opens on Profile, or on DLLs if DLLs was the current aspect (verified: internal/tui/resource_view.go:351-358; internal/nav/nav.go:233-240).

### 3. Destinations

| Destination | List owns | Detail shows | Default entry | Empty / loading / error / unavailable |
| --- | --- | --- | --- | --- |
| 1 Library | Find row, pinned "All games", games, search, DLL and profile filters, 4 sorts, multi-select (verified: internal/tui/sidebar.go:49-66,290-295) | Profile page (game or All games), or the game's DLLs section (verified: internal/tui/resource_view.go:130-156) | First game selected (cursor 1), List focus (verified: internal/tui/sidebar.go:83-86; internal/tui/layout.go:109,124-128) | Empty: "No games found" plus "Use Actions to rescan or Settings for paths" (verified: internal/tui/sidebar.go:345-349). No match: `No games match "<q>"` plus "Use Clear search or Cancel below" (verified: :351-355). Filtered out: "No games match filters / Clear filters in Actions" (verified: :356-358). No scope: "Select a game from the scope list" (verified: internal/tui/resource_view.go:136-138). |
| 2 DLL Catalog | Two groups, Library and Deployment, switched with Left/Right (verified: internal/nav/nav.go:89; internal/tui/layout_actions.go:297-299). Rows: known DLL types or deployment games (verified: internal/tui/dlls_resource.go:81-114) | Library: type, file, latest from manifest, cached versions. Deployment: per-game DLL rows with "current/stale/unknown" (verified: internal/tui/dlls_resource.go:571-621) | Group Library on first visit, then kept | "DLL catalog unavailable", "No deployed DLLs", "No deployed DLL selected" (verified: internal/tui/dlls_resource.go:92,106,574). "Latest: unavailable", "Cached: unavailable" (verified: :609-619). |
| 3 Monitor | GPU, CPU, Alerts rows (verified: internal/nav/nav.go:107; internal/tui/list_pane.go:111-135) | Boxed section "› GPU/CPU/Alerts" with sparklines and gauges (verified: internal/tui/metrics_resource.go:90-134) | GPU | "Loading metrics…", "Unsupported on this system", "Metrics failed to load", "Stale metrics" + em dash + "refresh failed" (verified: internal/tui/metrics_resource.go:44-49). "No active alerts" (verified: :243-246). Header "GPU: N/A" and similar (verified: internal/tui/header.go:224,239,257,272). |
| 4 Settings | Non-empty option groups (Display, Startup, Paths, DLL policy, Logging), Left/Right to change group, Up/Down for an option. The List shows labels only, no values (verified: internal/tui/options_modal.go:76-84,358-386) | One option: title, value (wraps at "/"), save status, description (verified: internal/tui/options_modal.go:390-440) | The first group in the list, which is Display (verified: tests/e2e/tui_journeys_test.go:542-543) | "No settings", "No setting selected" (verified: internal/tui/options_modal.go:360,393) |

Settings inventory and order (verified: internal/tui/options_modal_test.go:48; internal/config/options.go:146-152):
- Display: compact_mode, confirm_destructive.
- Startup: rescan_on_startup, auto_update_dlls, check_updates.
- Paths: steam_path, additional_library_paths, dll_cache_path, backup_path, shader_cache.
- DLL policy: auto_refresh_manifest, manifest_refresh_hours, dll_manifest_url.
- Logging: log_level.

Bool, enum and int-with-choices options cycle with Space. Path and string-list options open an editor (verified: internal/tui/active_state_contract_test.go:76-96).

### 4. Layout contract

- Constants: minimum 80x24, list pane width 32, status 2 rows, message 1 row, full header 7 rows, compact header 3 rows (verified: internal/tui/layout.go:27-35).
- The list width is `min(32, max(width/3, 24))` and Detail takes the rest (verified: internal/tui/layout.go:244-245).
- Single pane applies when `width < 100 || height < 30 || Focused` (verified: internal/tui/layout_render.go:12-14). The header is compact under the same thresholds or in Compact mode, and absent in Focused mode (verified: :16-24). In single pane the box title is always "▸ List" or "▸ Detail", and Tab reveals the other pane (verified: :61-66).
- The workspace height is `height - header - 1 (destination bar) - 2 (footer) - 1 (message)`, with a floor of 3 (verified: internal/tui/layout_render.go:26-32). `boundedBox` reserves borders and pads every row to the exact width. Children cannot grow the frame (verified: :36-53).
- Frame order: header, destination bar (digit labels only in Browse), workspace, message line (truncated), 2 footer rows (verified: internal/tui/layout_render.go:72-78).
- Shell overlays (Actions, decision, Help, batch) replace header and workspace. They draw the destination bar without digits, a blank line, then a box of `(width-4) x (height-3)` indented by 2 (verified: internal/tui/layout.go:252-262; internal/tui/layout_render.go:81-88).
- In-pane overlays (DLL chooser, confirmation, progress, result) render inside Detail. The footer then shows their local hint (verified: internal/tui/resource_view.go:131-133; internal/tui/layout_render.go:94-103).
- Below the minimum, the screen shows "Resize terminal", "Spela needs at least 80x24" and "Current size: WxH", cut to fit (verified: internal/tui/layout.go:269-282). Keys are ignored except Ctrl+C. Messages still reach their owners (verified: internal/tui/layout.go:166-177; tests internal/tui/viewport_contract_test.go:170-195, internal/tui/routing_contract_test.go:237-258).
- The renderer is Bubble Tea v2 with the alt screen (verified: internal/tui/layout.go:264-266; go.mod:11-13). An upstream Ultraviolet resize fix is pinned (doc says: docs/tui-testing.md:50-53).
- E2E row positions: the destination bar is at row index 7 at 120 columns and at index 3 at 80 columns (verified: tests/e2e/tui_journeys_test.go:89-91).

### 5. Panes and views

#### Header
- Full header: a 6-line logo, then metrics on the right (verified: internal/tui/header.go:17-24,148-318):
  - Line 1: "GPU: temp°C" (thermal colour 30 to 95), a sparkline (20, 10 or 0 cells depending on space), util %, power W (coloured by ratio to the limit), and the highest alert icon and label.
  - Line 2: "VRAM: [gauge] used/total GB".
  - Line 3: "CPU: util%" with a sparkline and MHz.
  - Line 4: "RAM: [gauge]".
  - Line 6: the release notice, if any.
  - Metrics are truncated to 60 columns (verified: :26,288).
- Compact header: 2 lines. Line 1 is "GPU temp util power  CPU util MHz". Line 2 is "VRAM x/yG  RAM x/yG", then the alert, then the notice, which is cut first (verified: internal/tui/header.go:321-404).
- Alert labels: "High temp", "Throttling", "Power limited", "Fan max". Icons: ✗ critical, ⚠ warning (verified: internal/tui/header.go:200-220,433-447).
- Cadence: one fetch at Init. After each `metricsMsg`, a 2 s tick, then the next fetch. The period is therefore 2 s plus fetch latency (verified: internal/tui/header.go:76-84,112-131).
- Four 20-sample ring buffers: temp, util, power, CPU (verified: internal/tui/header.go:56-66; internal/tui/metricsbuffer.go:7-73).
- Metric state machine: Loading, then Unsupported, Failed or Stale (verified: internal/tui/header.go:135-146).
- Alerts come from `overlay.Evaluate` (verified: internal/tui/header.go:87-106).
- The Monitor reuses the header's buffers. There is no second polling loop (verified: internal/tui/layout.go:158-162; internal/tui/resource_view.go:103-113).
- Release check: async if `check_updates` is set. The notice is "Spela vX is available (you have vY)". Failures stay silent (verified: internal/tui/layout.go:135-137; internal/tui/release_check.go:18-26; test internal/tui/release_check_test.go:26-28). The header must not grow when a notice appears (verified: internal/tui/header_contract_test.go:201-233).

#### Destination bar
- Format: "◆ 1 Library  2 DLL Catalog  3 Monitor  4 Settings". Digits are shown only in Browse. The active entry is cyan, bold and marked with ◆ (verified: internal/tui/destination_bar.go:27-51; internal/tui/layout_render.go:76).

#### Library list (sidebar)
- Title row: "Scope", or "Scope [Z-A|DLLs|Profile]" when sorted, or "Selected: N total, M visible" in select mode (verified: internal/tui/sidebar.go:319-326).
- Active filters line: "●DLLs ◆Profile" (verified: internal/tui/sidebar.go:328-338).
- The search input appears while searching or when a query exists (verified: internal/tui/sidebar.go:340-343).
- Find row ("  Find games", then "> Find games  type a name" when focused). It shows when games exist, no select mode is on, no search is open and the query is empty. Up from the first row reaches it and Down leaves it (verified: internal/tui/sidebar.go:118-132,192-196,386-398). Enter or typing a printable unbound key starts Search (verified: internal/tui/layout_actions.go:136-141,287-289; test internal/tui/profile_vocabulary_test.go:255-278). The Find row is not in tui-navigation.md.
- Rows: "> " marks the cursor. In select mode "✓ " marks a selected game. Indicators " ●" (has DLLs) and " ◆" (has a profile) follow the name. Names are truncated with "...". A " i/N" scroll line and the legend "● DLLs  ◆ profile" close the list (verified: internal/tui/sidebar.go:400-446).
- The "All games" row is hidden when a query, a filter or select mode is active (verified: internal/tui/sidebar.go:289-292).
- The search input limit is 30 characters. Matching is a case-insensitive substring of the name (verified: internal/tui/sidebar.go:71,232-237).

#### Profile page (Library Detail)
- Three header lines (verified: internal/tui/profile_header.go:20-59):
  - "<Game> · App ID <id> · DLLs: <family version, ...> · <status>". The DLL part is dropped when it does not fit. For All games the line is "All games · <status>".
  - "Own settings (N): label value · ..." or "Own settings: none, every value follows All games". All games shows "Games follow these values unless they set their own."
  - The section strip "[Proton]  DLSS  GPU  CPU  Overlay  DLLs". DLLs appears for games only.
- Status words: "Saved at once", "Saving…", "Not saved: <err>" (verified: internal/tui/detail_sections.go:77-86).
- Row format: `"%-*s  %-12s  %s"` (label, value, source). The focused row starts with "> ". An own value carries a magenta ◆ (verified: internal/tui/detail.go:816-851).
- The focused row is followed by wrapped notes:
  - the field description;
  - on All games, "Actions (0): Apply to games with their own value (N games)" (verified: internal/tui/detail.go:853-860,915-924).
- Scrolling keeps the focused row and its notes in view (verified: internal/tui/detail.go:863-871).
- Value and source words come from internal/profile/vocabulary.go (On, Off, none, "from All games", "this game · All games: X", "set here", "system default"). Retired words must never appear (verified: internal/profile/vocabulary.go:14-48; test internal/tui/profile_vocabulary_test.go:34-55).

#### Game DLLs section
- Layout (verified: internal/tui/content_views.go:93-168):
  - A "DLL versions" block: "Family: version" lines, or "No managed DLL installed" plus "Supported families: ...", then "Backup available".
  - The last result, if any.
  - Game facts, only when they fit: "Install directory", "Proton prefix", "Steam launch option: spela %command%".
  - The controls are reserved first.
- Controls (verified: internal/tui/content_views.go:170-204):
  - A title "DLL actions: <game>".
  - The rows "Install DLL", "Update DLLs (N file[s])" and "Restore originals (N file[s])".
  - Each unavailable row gets an "    Unavailable: <reason>" line.
  - An explanation line.
  - A hint: "↑/↓ Choose action · Enter <name>" when focused, or "Tab: Focus DLL controls" when not.

#### Message bar
- One line. Styles: info (dim), success (green) and error (red), with a flash background for phases 1 and 2 at 100 ms. It clears after 5 s, matched by timestamp (verified: internal/tui/messagebar.go:18-20,47-119).

#### Footer
- Two rows of "Key: Label", built from the bindings available in the current context. Destination keys, Quit and menu-only bindings are left out. Rows wrap at the width. More than 2 rows are dropped silently (verified: internal/tui/layout_actions.go:342-361; internal/tui/layout_render.go:104-125).
- In Search the footer shows "Input  Apply  Clear search  Cancel" and "Tab: next control  Enter: <label>" (verified: internal/tui/layout_render.go:91-93).
- In an in-pane overlay it shows that flow's hint (verified: internal/tui/layout_render.go:94-103).

### 6. Keymap

#### Model
- Modes: Browse, Search, Edit and Overlay, plus ModeAny for Ctrl+C. Focus: List, Detail or Any (verified: internal/tui/keymap.go:16-31).
- A binding holds a mode, focus, scope, action, description, keys, an availability function and a Menu flag (verified: internal/tui/keymap.go:183-192).
- `Lookup` precedence: exact mode before ModeAny; inside each, exact focus before FocusAny. The first available match wins. Otherwise the first unavailable match is returned. Otherwise an explicit no-op (verified: internal/tui/keymap.go:216-254). "escape" is normalised to "esc" (verified: :217-219).
- `describeBinding` relabels by context. Examples: "Batch actions", "Find games", "Previous action", "Scroll up", "Follow All games", "Set here", and DLL labels (verified: internal/tui/keymap.go:519-591).
- A Printable key matches any single printable rune (verified: internal/tui/keymap.go:284-296).

#### Bindings

| Mode | Focus | Key | Action and label | Availability |
| --- | --- | --- | --- | --- |
| Any | Any | Ctrl+C | Interrupt (quit now) | always (verified: internal/tui/keymap.go:428) |
| Browse | Any | 0 | Actions | always (:429) |
| Browse | Any | 1 2 3 4 | Library, DLL Catalog, Monitor, Settings | always (:430-433) |
| Browse | Any | Tab | Switch pane | `CanSwitchPane` (:434; computed at internal/tui/layout_actions.go:71) |
| Browse | Detail | ← → | Previous/Next section | profile page, and a section exists that way (:435-436,311-331) |
| Browse | List | ↑ ↓ | Previous/Next item | list has items (:437-438) |
| Browse | List | Enter | Open selection, Find games or Batch actions | there is a selection (:439,532-537) |
| Browse | List | ← → | Previous/Next group | Settings and Catalog only (:440-441,340-345) |
| Browse | List | Space | Select game | Library game row (:442-444) |
| Browse | Detail | ↑ ↓ | Previous/Next field, Previous/Next action, or Scroll | fields, DLL actions or overflow (:445-446,515-517) |
| Browse | Detail | Enter | Edit value, or the selected DLL action name | editable field, or the selected DLL action is available (:450-456) |
| Browse | Detail | Space | Set for this game, Follow All games, Set here or Use system default (Library); Next value (Settings) | `CanCycle` (:457,554-566) |
| Search | Any | Tab | Next control | (:495) |
| Search | Any | Enter | Apply search, Clear search or Cancel search, by focused control | (:496; internal/tui/search_controls.go:26-31) |
| Search | Any | printable | Search text | (:497) |
| Edit | Any | Tab | Next control | (:498) |
| Edit | Any | Esc | Cancel edit | (:499) |
| Edit | Any | Enter | Apply or cancel edit, by focus | (:500; internal/tui/editor_host.go:181-188) |
| Edit | Any | ← → | Previous/Next choice, Choose button, or Move cursor | (:501-502,521-526) |
| Edit | Any | ↑ ↓ | Previous/Next choice | choice input focused (:503-504,606-608) |
| Edit | Any | Space | Toggle, or Type space | input focused (:505,527-530) |
| Edit | Any | printable | Edit text | (:506) |
| Overlay | Any | Tab, Enter, ↑ ↓, ← → | Next control, Activate, Previous/Next, Previous/Next button | (:507-512) |

Menu-only bindings (no keys) are listed in section 7.

#### Input precedence and routing
- The order is Ctrl+C, decision, Actions, Help, batch menu, in-pane overlay, Search, Edit, Browse (verified: internal/tui/layout_actions.go:98-145). The doc states the same order (doc says: docs/design/tui-navigation.md:37-40).
- In Edit, text keys go to the editor before command lookup. Printable keys, Backspace, Delete, Home, End, Left, Right and Space count as text when a text input has focus. A picker takes only printable non-space keys, for type-ahead (verified: internal/tui/layout_actions.go:125-135,148-164).
- A Browse key that is unknown or unavailable is a no-op (verified: internal/tui/layout_actions.go:142-144; test internal/tui/routing_contract_test.go:39-52).
- Digits are text in Search and Edit (verified: tests internal/tui/routing_contract_test.go:24-37,75-92; internal/tui/keymap_test.go:9-31).
- Every dispatch checks availability and relevance again (verified: internal/tui/layout_actions.go:220-225).

#### Focus rules
- Tab toggles List and Detail in Browse. It is blocked while a pane modal is open (verified: internal/tui/layout_handlers.go:130-140).
- There is no Shift+Tab binding (verified: internal/tui/keymap.go:427-513). PLAN.md asks for "Tab and reverse-Tab" (doc says: PLAN.md:53). The test that claims reverse Tab sends plain tab (verified: internal/tui/shell_contract_test.go:30-34).
- Enter on a list row moves focus to Detail. In Library this happens through `gameConfirmedMsg` and `defaultProfileConfirmedMsg` (verified: internal/tui/layout_handlers.go:153-170; internal/tui/layout_actions.go:292-328).
- A Catalog mutation started from List reveals Detail. Closing the result restores List (verified: internal/tui/layout_actions.go:174-186; test internal/tui/content_actions_test.go:230-261).

#### Escape
- Code: Esc is bound only in Edit mode. It cancels an editor or picker (verified: internal/tui/keymap.go:499). It is a no-op in Browse (test internal/tui/layout_test.go:475-485), in Help (test :40-51), in DLL confirmations (tests internal/tui/dll_mutation_test.go:28-32, internal/tui/content_test.go:103-108) and in Search (test internal/tui/sidebar_test.go:288-295; no Search esc binding at internal/tui/keymap.go:495-497).
- The doc agrees: "Escape closes an open editor or picker" (doc says: docs/design/tui-navigation.md:26; docs/tui-testing.md:127).
- PLAN.md disagrees: "Escape owns mode-specific back/cancel behavior" (doc says: PLAN.md:143).

#### Quit
- Quit is an Actions row only. It is unavailable while busy, with the reason "wait for pending operations" (verified: internal/tui/keymap.go:494).
- `requestQuit` (verified: internal/tui/scope_navigation.go:68-77):
  - Refused while a rescan, batch, DLL operation, Catalog update or install is running (verified: internal/tui/layout_actions.go:13-15).
  - Waits with "Saving, then quitting" while profile or Settings writes or apply-own-values are pending (verified: :19-22).
  - Cancelled if a write fails, so the error stays readable (verified: internal/tui/layout_handlers.go:269,423,442).
- Ctrl+C quits at once from anywhere, including below the minimum size (verified: internal/tui/layout_actions.go:100-102; internal/tui/layout.go:173-175).

#### Help
- Help opens only from Actions. It captures the invoking context (verified: internal/tui/layout_actions.go:237-241).
- Sections (verified: internal/tui/help.go:54-105):
  - "After closing Help: <dest> · <focus>", with the keys available now.
  - "Actions in this context (open with 0 after closing Help)", with reasons.
  - Four "Reference: <mode> controls" tables.
- Header lines: "Keyboard shortcuts" and "Reference only. Close Help before using these controls." (verified: internal/tui/help.go:128).
- ↑ ↓ scroll the body. Tab toggles Close. Enter closes only while Close has focus (verified: internal/tui/layout_handlers.go:75-94).
- Defect: the reference tables append the availability reason whenever it is non-empty. Several functions return a reason even when the action is available. Examples: Tab at internal/tui/keymap.go:434 and list moves at :437-439. Help therefore prints "Switch pane; no other eligible pane" while Tab works (verified: internal/tui/help.go:94-99) (inferred effect).

### 7. Actions menu

- Rendering: the title is "Actions · <Destination> · <focus>", with the focus in lower case (verified: internal/tui/actions_menu.go:72). The cursor starts on the first available row (verified: internal/tui/layout_actions.go:227-236). ↑ ↓ wrap (verified: :198-205). An unavailable row shows " (<reason>)" and is dimmed. Enter runs only an available row. Tab goes to Close. The hint reads "Tab: next control  ↑ ↓: choose  Enter: run" (verified: internal/tui/actions_menu.go:64-103).
- Rows come from `Menu:true` bindings in table order, filtered by `actionRelevant`, with the mode forced to Browse (verified: internal/tui/keymap.go:447-494; internal/tui/actions_menu.go:18-62). Reset rows come first (test internal/tui/profile_vocabulary_test.go:175-185).

| Context | Rows in order (verified by derivation from internal/tui/keymap.go:447-494 and internal/tui/actions_menu.go:18-56) |
| --- | --- |
| Library, List focus | Search games; Toggle DLL filter; Toggle profile filter; Clear filters (reset sort to A-Z); Sort: A-Z; Sort: Z-A; Sort: DLLs first; Sort: profiles first; Select all filtered games; Clear selected filtered games; Update selected games (N visible); Rescan games; Layout: Standard; Layout: Compact; Layout: Focused; Help; Quit |
| Library, game Profile, Detail focus | Follow All games; Follow All games for all settings; Edit value; Rescan games; Layout ×3; Help; Quit |
| Library, All games, Detail focus | Use system default; Use system default for all settings; Apply to games with their own value; Edit value; Rescan games; Layout ×3; Help; Quit |
| Library, game DLLs, Detail focus | Rescan games; Install DLL: <game>; Update stale DLLs: <game> (N); Restore originals: <game> (N file[s]); Layout ×3; Help; Quit. "Edit value" is left out when DLL actions exist (verified: internal/tui/actions_menu.go:46-49). |
| Library, Detail, no scope (empty library) | Rescan games; Layout ×3; Help; Quit (inferred from relevance at internal/tui/actions_menu.go:27-37) |
| DLL Catalog, either pane | Deployment group: "Update all stale deployments (N DLL[s])", then Layout ×3, Help, Quit. Library group: Layout ×3, Help, Quit. Catalog has no Rescan row (verified: internal/tui/actions_menu.go:25-26,33-35). |
| Monitor | Layout ×3; Help; Quit |
| Settings, List | Layout ×3; Help; Quit |
| Settings, Detail | Edit value (path or list options only, since Editable means CanOpen and not CanCycle; verified: internal/tui/layout_actions.go:55-59); Layout ×3; Help; Quit |

Reason strings, verbatim:
- From the keymap (verified: internal/tui/keymap.go:302-608): "select a game", "select a profile", "first section", "last section", "Library only", "no groups in this list", "open Profile", "nothing to clear", "open All games", "saving", "no game has its own value", "open DLLs", "open a DLL view", "open Deployment", "no backup", "no items", "no selection", "select a game row", "no editable field", "no field", "no selected filtered games", "wait for pending work", "no updates for selected filtered games", "wait for pending operations", "no other eligible pane", "no fields, actions or overflowing content", "focus the input", "focus a choice input", "unavailable in this context".
- From game content (verified: internal/tui/content_actions.go:81-106): "finish the current DLL operation", "no stale DLLs for this game", "no backup for this game", "backup has no original files".
- From the Catalog (verified: internal/tui/dlls_resource.go:315-329): "no stale cached deployments" and "not a catalog action".

### 8. Local controls in overlays and editors

- **Decision dialog** (whole-page Follow or system default, and Apply to games with their own value) (verified: internal/tui/scope_navigation.go:15-22,81-184):
  - Choices are [Cancel, <action title>]. Focus starts on Cancel (0). Focus -1 is the scrollable body.
  - Tab cycles body, Cancel, action. ← → move between buttons. ↑ ↓ scroll the body. Enter on the body does nothing.
  - The hint is "Tab: next control  ← →: choose  Enter: <choice>".
  - The body comes from profile vocabulary helpers, for example "This clears the ... afterwards" and "cannot be undone" (verified: tests/e2e/tui_journeys_test.go:500-505; internal/tui/apply_own_values_test.go:99).
- **Batch menu** (verified: internal/tui/layout.go:476-499; internal/tui/layout_handlers.go:15-72):
  - Title "Batch action (N visible games)". One control, "Update selected games (N DLLs)", plus Close.
  - "No available updates for these visible selected games" when there are no targets.
  - Enter opens a confirmation, or runs at once when Confirm destructive actions is off.
  - The result scrolls with ↑ ↓. Tab then Enter closes.
- **Search** (verified: internal/tui/search_controls.go:14-78):
  - Four controls: Input, Apply, Clear search, Cancel. Tab cycles them.
  - Enter applies by focus. Apply keeps the query. Clear empties it. Cancel restores the List captured when Search opened (test internal/tui/scope_navigation_test.go:195-210).
  - Results update the Detail while typing (test internal/tui/shell_contract_test.go:50-84).
- **Field editor** (numbers, text, paths) (verified: internal/tui/detail.go:958-993; internal/tui/editor_host.go:62-332):
  - Lines: "Edit <Label>", "Value before edit: <v>", the input with the "▏" cursor, a unit hint ("<hint> Typing replaces the value."), "Invalid value: <err>", the controls "Input  Apply  Cancel" (focused one prefixed "> "), and "Tab control  Enter apply|cancel edit  Backspace delete  Esc cancel".
  - Integer editors replace the prefilled value on the first keystroke (verified: internal/tui/editor_host.go:70,87-98). Moving the cursor first switches to in-place editing (test internal/tui/profile_vocabulary_test.go:121-130).
  - Tab cycles 3 controls. ← → on the button row move between Apply and Cancel (verified: internal/tui/editor_host.go:154-168).
  - An invalid value keeps the editor open with focus on Input (verified: internal/tui/detail.go:303-322).
- **Picker** (booleans and fixed choices) (verified: internal/tui/detail.go:173-193,598-658,997-1061; internal/tui/editor_host.go:228-260):
  - The first row is "Follow All games (<value>)", or "Use system default (<value>)" on All games. Then On/Off or the allowed values. An explicit empty value reads "none", except for VRR.
  - Typing jumps to a match. The same letter again moves to the next match. A letter that matches nothing restarts the search.
  - Title "Edit <Label>  jump: <typed>". Hint "Type to jump  ↑ ↓ choose  Tab control  Enter apply  Esc cancel".
- **DLL confirmation** (verified: internal/tui/dll_mutation.go:55-195):
  - Title, "N DLL target(s)", one item per target ("<game> · <family> · <cur> → <target>" then the path), "Backup: <policy>", and "[ Cancel ]  [ Confirm ]".
  - Cancel is selected first. Tab toggles between the body (scroll) and the buttons. ← → toggle the selection. Enter in the body does nothing.
  - Hints: "←/→ Choose · Enter Activate · Tab Details", then "↑/↓ Scroll · Tab Buttons".
- **DLL progress**: "Work is running. Controls return when it finishes." No input is taken and no Cancel is offered (verified: internal/tui/content_views.go:28-30,100-102; internal/tui/content_actions.go:216-220).
- **DLL result**: a scrollable body and "[ Close ]". Hints "Enter Close · Tab Details" and "↑/↓ Scroll · Tab Close" (verified: internal/tui/dll_mutation.go:217-256).
- **Install chooser**: a family list with Cancel, then a version list with Back and Cancel. The first version is labelled "(latest)". Loading shows "Loading...", and an empty list shows "No versions available" (verified: internal/tui/content_views.go:24-89; internal/tui/content_actions.go:325-389).

### 9. Profile editing and persistence (current model)

- Every change is saved at once.
  - The pane calls `saveResolvedProfile` or `saveDefaultProfile` whenever `DetailModel.UpdateAction` reports a change (verified: internal/tui/pane_actions.go:31-39).
  - The message is "Saved: <summary>" or "Not saved: <summary>: <err>". When the saved profile is not on screen, the game name is added (verified: internal/tui/layout_handlers.go:428-452).
  - Summaries: "HDR On", "HDR follows All games", "HDR uses the system default (All games)", "All settings follow All games", "All settings use the system default (All games)" (verified: internal/tui/detail.go:281-301,714-734).
- The write queue (verified: internal/tui/content.go:100-174):
  - One write is active at a time.
  - A queued write for the same app collapses into the newest and keeps the oldest `before`.
  - Each write runs `profile.MergeChanges(current, before, desired)` inside `profile.Mutate` or `profile.MutateDefault`, under the file lock (verified: internal/tui/profile_save_serialization_test.go:15-96).
  - After a failure, the next queued write for that app starts from the failed request's `before`.
- `DetailModel` keeps both `raw` and `persisted`. "Dirty" means a write is in flight or failed (verified: internal/tui/detail.go:48-82,133-164). An earlier completion never marks a later change saved (verified: :437-444). A failed write puts the stored value back on screen when nothing newer is queued (verified: internal/tui/content_routing.go:43-46; internal/tui/resource_view.go:291-293).
- Space (verified: internal/tui/detail.go:547-594):
  - Own value to following: the old value is remembered in `lastOwn` and the field resets.
  - Following to own: a boolean takes the opposite of the effective value. Another field gets its remembered value back, or opens its editor when there is none.
  - `lastOwn` lives in one DetailModel, and `SetGame` builds a new one (verified: internal/tui/content.go:241). Undo memory is therefore lost when the game changes (inferred).
- Moving between games keeps the field under the cursor and the current aspect (verified: internal/tui/content.go:222-224,242; test internal/tui/profile_flow_test.go:15-28). Reopening the same game keeps state (test internal/tui/scope_navigation_test.go:78-100).
- Section jump (verified: internal/tui/layout_handlers.go:105-126; internal/tui/detail_sections.go:19-57):
  - Left and Right go to the first field of the neighbouring section.
  - On a game, Right from Overlay opens DLLs, and Left from DLLs returns to the first Overlay field.
  - Nothing wraps.
- Resets:
  - Follow All games / Use system default (one field). The row is unavailable when the field is not its own (verified: internal/tui/keymap.go:357-367).
  - "... for all settings" asks first, through the decision dialog (verified: internal/tui/scope_navigation.go:81-90).
- Apply to games with their own value (All games only) (verified: internal/tui/scope_navigation.go:102-122; internal/tui/layout_handlers.go:414-426; tests internal/tui/apply_own_values_test.go):
  - Names the games and asks first.
  - Clears only that field in each game, through `Services.FollowAllGames`.
  - Reports partial failures: "Not saved for <game> (<err>). HDR: 1 game follows All games".
  - Blocked while a write runs ("saving").
- Write state words: "Saving…" (warning colour) and "Not saved: ..." (error colour) in the header (verified: internal/tui/profile_header.go:26-33).

### 10. Settings

- Draft and config split (verified: internal/tui/options_modal.go:16-37,558-628):
  - Space cycles and saves at once.
  - A change made during a write sets `resave`. The next write follows.
  - Success moves `config`. Failure resets the draft to `config` and shows "Not saved: <err>".
  - Messages "Saved: <Label> <value>" and "Not saved: <err>" (verified: internal/tui/layout_handlers.go:257-270; internal/tui/options_modal.go:584-594).
- Enter (verified: internal/tui/options_modal.go:227-265):
  - Opens an editor only for path and string-list options. These use a bubbles textinput with a 256-character limit.
  - The editor validates with `option.Set` on a cloned draft.
  - On a cyclic option, Enter does nothing.
- A saved config feeds two settings into the views (verified: internal/tui/display_settings.go:7-29):
  - `confirm_destructive` off skips only the update confirmation. Install and restore always ask.
  - `compact_mode` sets the density at startup and whenever the setting itself changes. A layout picked from Actions otherwise stays.
- `log_level` is applied after each save (verified: internal/tui/services.go:44-50).
- A saved Steam path applies to later manual scans (verified: internal/tui/layout_rescan_test.go:50-66).

### 11. Search, sort, filters, multi-select, batch

- Sort modes: A-Z, Z-A, DLLs first, profiles first, each chosen explicitly from Actions (verified: internal/tui/sidebar.go:17-26,139-141). The DLL filter needs at least one detected DLL. The profile filter needs `ProfileExists` (verified: :239-244).
- "Clear filters" also clears the search and resets the sort to A-Z (verified: internal/tui/sidebar.go:212-217).
- Space toggles a game. Select all and Clear selected act only on filtered games (verified: internal/tui/sidebar.go:147-172). Select mode freezes Detail (verified: internal/tui/scope_navigation.go:40-42) and hides the All games and Find rows (verified: internal/tui/sidebar.go:195,290).
- Enter with visible selected games opens the batch menu. Hidden selections never join a batch (verified: internal/tui/layout_actions.go:278-285; internal/tui/sidebar.go:500-511).
- A rescan drops selections whose games are gone (verified: internal/tui/sidebar.go:465-482).

### 12. DLL mutation flows

- Library game DLLs (verified: internal/tui/content_actions.go:161-288):
  - Update targets come from the manifest's latest version (verified: internal/tui/dll_mutation.go:36-51; internal/tui/content.go:382-402). The update downloads when needed (`cachedOnly=false`) (verified: internal/tui/content.go:301).
  - Restore targets come from the backup metadata, including files not detected now. The targets are checked again before dispatch, and a changed backup shows "Backup changed: review restore" (verified: internal/tui/content_actions.go:180-194,290-314).
  - Install offers only families the game already has, or all families when it has none (verified: internal/tui/content.go:335-367).
- Catalog update-all: targets are DLLs older than the newest cached version, and the update uses the cache only (`cachedOnly=true`) (verified: internal/tui/dlls_resource.go:286-295,451-463,476-506).
- Batch (selected games): uses the manifest's latest version and `cachedOnly=false` (verified: internal/tui/layout.go:443-449).
- Confirm destructive actions applies to updates only. Install and restore always confirm (verified: internal/tui/content_actions.go:268-273; internal/tui/dlls_resource.go:396-403; internal/tui/layout_handlers.go:65-69).
- Cancelling writes "<op> cancelled, no operation ran" (verified: internal/tui/dll_mutation.go:94-96).
- Request IDs reject stale installer results. Update checks are keyed by appID (verified: internal/tui/content_actions.go:391-421; internal/tui/content_routing.go:52-60).
- Completion messages:
  - "DLLs already up to date", "DLL update: X updated, Y current, Z failed[; path: err]"
  - "Restore: already current", "Original DLLs restored!", "Install: already current", "DLL installed successfully!"
  - "Updated DLLs for X/Y games", "Selected games are already current"
  - "Update-all: ...", "Update-all: already current"
  - (verified: internal/tui/layout_handlers.go:277-337; internal/tui/layout.go:455-470; internal/tui/dlls_resource.go:500-503)
- After any mutation, every view switches to the returned game records (verified: internal/tui/layout_handlers.go:341-354).

### 13. Background lifecycle

- **Startup** (verified: internal/tui/layout.go:117-122,133-149; cmd/spela/commands/tui.go:24-43):
  - `cmd tui` takes a runtime lock, loads the DB, and exits with a stderr message if the config is invalid.
  - It runs `steam.RefreshIfNeeded` synchronously.
  - The TUI then starts its own async rescan under the same condition: an empty DB or rescan on startup. This is a double scan (verified: internal/steam/refresh.go:31-39).
  - If no Steam installation is found, the synchronous rescan fails and the TUI does not start (inferred from internal/steam/refresh.go:22-24 and cmd/spela/commands/tui.go:39-42).
- **Rescan** (verified: internal/tui/layout.go:418-429; internal/tui/layout_handlers.go:366-410; tests internal/tui/rescan_lifecycle_test.go):
  - The request ID increments, and the config is cloned as a snapshot.
  - While busy, Rescan, Quit and batch update are disabled.
  - Stale results are ignored.
  - Results that arrive outside Browse wait in `pendingRescan` until Browse returns.
  - Messages: "Scanning games...", "Rescanning games...", "Rescan complete: N games found", "Rescan failed: <err>".
- **Auto-update**: runs once at start, after the startup scan if one runs. It reports `AutoUpdateResult.Notice()` and stays silent when nothing changed (verified: internal/tui/auto_update.go:13-42; test internal/tui/auto_update_test.go).

### 14. Services interface

`Services` holds function fields so tests can swap in fakes (verified: internal/tui/services.go:15-68). Results return as typed `tea.Msg` values carrying identities: request IDs, appIDs or timestamps.

| Dependency | How it is called | Where |
| --- | --- | --- |
| LoadConfig | sync at construction | internal/tui/layout.go:79 |
| SaveConfig | async Cmd | internal/tui/options_modal.go:575-580 |
| ScanGames | async Cmd | internal/tui/layout.go:418-429 |
| LoadProfile / LoadDefaultProfile | sync on game switch, when All games opens (reads every game profile), and on refresh | internal/tui/content.go:231-232; internal/tui/resource_view.go:57-58,321-335 |
| ProfileExists | sync, for every visible sidebar row on every render, and in filter and sort | internal/tui/sidebar.go:242,252,532 |
| BackupExists | sync | internal/tui/content.go:219,244 |
| LoadDLLBackup | sync, inside availability and label checks that run on every key and render | internal/tui/content_actions.go:68-77,108-130; internal/tui/layout_actions.go:74-83 |
| KnownDLLTypes / ListCachedDLLs | sync | internal/tui/dlls_resource.go:163-175,207-219 |
| BatchUpdateDLLs / InstallDLL / RestoreDLLs / AutoUpdateDLLs / FollowAllGames | async Cmd | internal/tui/content.go:297-316,409-419; internal/tui/scope_navigation.go:114-120 |
| VKD3DNotice | wired, never called in the TUI | internal/tui/services.go:31-34,66 |
| Bypass: dll.GetManifest | async, but outside Services | internal/tui/content.go:339,373,394 |
| Bypass: dll.LoadManifest | sync, at construction and on rescan | internal/tui/layout.go:93; internal/tui/layout_handlers.go:396 |
| Bypass: profile.Mutate / MutateDefault | async, outside Services | internal/tui/content.go:117-130 |
| Bypass: release.Check, gpu/cpu metrics | async; the metrics function is swappable only through the e2e build tag | internal/tui/release_check.go:20; internal/tui/header.go:87-110; internal/tui/header_e2e.go:14-38 |

### 15. Styles, colours and glyphs

- There is one theme, "neon-accent-dark" (verified: internal/tui/styles.go:72-150):
  - Bg #0a0a14, Fg #e8e8f0, FgMuted #6a6a80, AccentOverride #ff5fd2, AccentFocus #5ff0ff, Border #202030.
  - Success #7ce38b, Error #ff6e6e, Warning #ffd866.
  - Thermal stops #5ff0ff, #5fbaff, #7ce38b, #ffd866, #ff9e5f, #ff6e6e at 0, .3, .5, .7, .85 and 1.0 (verified: internal/tui/thermal.go:18-25).
- Bg is never painted except as the selection background and the flash foreground (verified: internal/tui/styles.go:191-194; internal/tui/messagebar.go:104-115). Readability on light terminals is therefore a risk (inferred). Whether NO_COLOR is honoured is unverified.
- Style roles:
  - The focused row and focused border are cyan and bold.
  - Following rows use FgMuted. Own rows use Fg plus a magenta ◆.
  - Titles are magenta and bold.
  - Selected is cyan background with dark text.
  - (verified: internal/tui/styles.go:183-252)
- Glyphs:
  - ▸ marks the focused pane title and the focused control or menu row. > marks a list or field cursor.
  - ◆ has several meanings: active destination (internal/tui/destination_bar.go:46), own value (internal/tui/detail.go:764), has profile (internal/tui/sidebar.go:533), the profile filter tag (:334), and a stale cell in a matrix that is never drawn (internal/tui/dlls_resource.go:751).
  - ● has DLLs. ✓ selected. ✗ and ⚠ alerts.
  - Sparkline ▁ to █. Gauge blocks ▏ to █ and ░. Text cursor ▏. Truncation … and "...".
  - Rounded borders ╭╮╰╯. Monitor heading prefix ›. Separator ·.
- The CLI imports this palette: `tui.CLIPrimary` and related helpers (verified: internal/tui/styles.go:291-317; cmd/spela/commands/launch.go:110; cmd/spela/commands/cpu.go:77).

### 16. Wrap or clamp, per control

- **Wrap:**
  - the Actions cursor (internal/tui/layout_actions.go:198-205);
  - the Settings option cursor (internal/tui/options_modal.go:343) and Settings groups (internal/tui/layout_actions.go:316);
  - the Catalog group toggle (:299);
  - the editor Tab cycle (internal/tui/editor_host.go:154);
  - the search Tab cycle (internal/tui/search_controls.go:39);
  - the picker arrows (internal/tui/editor_host.go:295).
- **Clamp:**
  - profile fields (internal/tui/detail.go:255-258) and section jumps (internal/tui/detail_sections.go:41);
  - the sidebar (internal/tui/sidebar.go:126,132);
  - DLL lists (internal/tui/dlls_resource.go:70,76);
  - Monitor (internal/tui/layout_actions.go:330-335);
  - game DLL actions (internal/tui/content_actions.go:246-251);
  - install lists (:335-347).
- All cites are verified.

### 17. Visible-string contract from the e2e journeys

These exact strings are asserted on rendered screens (verified: tests/e2e/tui_journeys_test.go, tests/e2e/tui_editor_kinds_test.go, tests/e2e/tui_dll_direct_test.go, tests/e2e/tui_resize_test.go):

- **Shell:** "Library", "▸ List", "▸ Detail", "◆ 2 DLL Catalog", the regex `0[: ]+[Aa]ctions`, the regex `(?:\[1\]|1) Library`, "Actions ·", "▸ Close", "Keyboard shortcuts", "Resize terminal", "Current size: WxH".
- **Profile:** "[Proton]" through "[DLLs]", "Saved at once", "Own settings", "> All games", "Saved: HDR follows All games", "Saved: HDR On (All games)", "Saved: All settings use the system default (All games)", "set here", "this game", "from All games", "system default", "Enter: Edit value", "Space".
- **Editors:** "Edit Super resolution", "Follow All games (balanced)", "Type to jump", "Esc cancel", "Value before edit: 0", "> Input", "Typing replaces the value", "Backspace delete", "Invalid value: invalid integer: x", "x▏", "Saved: Clock offset 1234 MHz".
- **Library:** "Selected: 1 total, 1 visible", "Enter: Batch actions", "Enter: Open selection", "Batch action (1 visible games)", "No available updates", "Enter: close", "Scope [Z-A]", "●DLLs", "◆Profile", "No games found", "Rescan complete: 0 games found", "Settings for paths", "No games match".
- **DLLs:** "DLL actions: Cyberpunk 2077", "Update DLLs (1 file)", "▸ Install DLL", "Restore originals (0 files)", "no backup for this game", "Tab: Focus DLL controls", "Confirm DLL update", "1 DLL target(s)", "[ Cancel ]", "[ Confirm ]", "DLL update cancelled", "DLL update: 1 updated, 0 current, 0 failed", "Tab Close", "Enter Close", "Newly added DLLs stay installed", "DLL restore completed", "DLL install completed", "Select a supported DLL family", "Select DLSS version", "Tab Buttons", "Backup:", "cancelled, no operation ran", "Install directory", "Steam launch option: spela %command%".
- **Monitor and Catalog:** "▸ GPU", "▸ CPU", "Deployment", "Update all stale deployments", "no stale cached deployments".
- **Header:** the CPU line regex `CPU:?\s+\d+%.*\s+\d+MHz\s*$`, and no line wider than the terminal.

Strings that must not appear: "Ctrl+S", "Unsaved changes", "Esc" (in Help and confirmations), "Ctrl+C" (in Help), "? /", "Enter/Y", "Enter:" (when nothing can run).

### 18. Testing procedure and harness

- **Manual review** (doc says: docs/tui-testing.md:1-240; AGENTS.md:62-120):
  - Use termctrl 1.2.1 or later with CSI Z support. A compatibility test exists at tests/e2e/tui_resize_test.go:18-29.
  - Run a compiled binary in a named session. Use `wait` instead of sleeps, and read `show`.
  - Send only keys that are on screen.
  - Use isolated XDG roots, and never mutate real game files.
  - Resize the same session through 120x40, 80x24 and 72x20.
  - Stop sessions and check `termctrl list`.
- **E2E harness** (verified: tests/e2e/e2e_init_test.go:16-46; tests/e2e/terminal_client.go:27-124; tests/e2e/mock_helper.go:37-285):
  - Builds `go build -tags e2e`.
  - Fixtures: 3 games (Cyberpunk 2077 with DLSS and DLSS-G 3.7.0, Witcher 3 with DLSS 3.5.0, Elden Ring with no DLLs), config.yaml, a default.yaml without an overrides map, 1091500.yaml, and manifest.json (DLSS 3.8.0, DLSS-G 3.7.0).
  - A per-test termctrl runtime dir.
  - `sendDisplayed` refuses any key not visible on screen (verified: tests/e2e/tui_journeys_test.go:113-136).
  - Quit is checked by waiting for `XDG_RUNTIME_DIR/spela/spela.pid` to disappear (verified: :214-224).
  - Hooks: `SPELA_E2E_METRICS=successive-widths` (verified: internal/tui/header_e2e.go:15) and the artifact env vars `SPELA_E2E_RESIZE_ARTIFACTS` and `SPELA_E2E_EDITOR_ARTIFACTS`.
  - The harness imports `internal/profile` and `internal/dll` to parse files (verified: tests/e2e/tui_journeys_test.go:15; tests/e2e/tui_dll_direct_test.go:16).
- `tests/e2e/test_run_data` is an empty directory.

### 19. Binding non-goals (PLAN.md and the basic-key plan)

- No three-zone shell and no focusable rail (doc says: PLAN.md:141-142).
- `q` is not back. Quit is a distinct action (doc says: PLAN.md:143).
- No command palette, not even as a deferred binding or help entry (doc says: PLAN.md:145).
- Context does not navigate subsystems (doc says: PLAN.md:144).
- ANSI snapshots, GUI parity and rmux are not contracts (doc says: PLAN.md:146-147).
- No launch redesign, new fields or new DLL capabilities (doc says: PLAN.md:148-149).
- No pin action (doc says: PLAN.md:91-93,104-105).
- No workflow may need Escape, Shift+Tab, letters, punctuation or function keys (doc says: docs/design/tui-basic-key-navigation-plan.md:18-21; docs/design/tui-navigation.md:25-29).
- The TUI does not launch games or apply privileged tuning (doc says: docs/design/tui-navigation.md:5-7; test internal/tui/help_test.go:128-134).

### 20. Dead or test-only code (do not port as parity)

Production never renders or calls these; only tests do:
- `renderBreadcrumbs`, `renderModalBox`, `renderBatchContent`, `buildTopBorder`, `truncateHeight` (internal/tui/layout.go:310-362,372-396,472-474).
- `executeBatchDLLUpdate` (:451-453).
- The Catalog `renderLibrary` table, the `renderDeployment` matrix, `renderCell` and `renderFooter` (internal/tui/dlls_resource.go:623-779).
- `RenderContextBar` and `globalKeys` (internal/tui/help.go:148-236).
- `renderOptionsBody` (internal/tui/options_modal.go:645-702).
- `ViewProfileAspect` with its "Destination: this game · values inherited from Defaults" banner (internal/tui/content.go:322-328; internal/tui/content_views.go:207-220).
- `UpdateEditor`, `handleGlobalKeys`, `startPathEditing`, `updatePathEditing`, `UpdateList`.
- `FocusSearch` and `IndicatorStyle` have no callers at all.

(verified by grep, results saved in the scratchpad, file dead.txt)

### 21. mejl UX patterns compared with spela's contract

| mejl pattern (mejl source) | Spela | Evidence |
| --- | --- | --- |
| Fuzzy palette on `:` / Ctrl-k (mejl/docs/terminal-ux.md:494,521) | **Conflict.** Spela bans a palette. The nearest equivalent is the 0 Actions menu: not fuzzy, scoped by context, with reasons. | PLAN.md:145; internal/tui/keymap_test.go:86-90; internal/tui/help_test.go:121 |
| `q` back then quit, `Q` quit now (mejl/docs/terminal-ux.md:527; mejl/README.md:121) | **Conflict.** `q` is not back and does nothing. Quit is an Actions row that waits for writes. Ctrl+C is the immediate quit. | PLAN.md:143; internal/tui/layout_test.go:43,530-546; internal/tui/scope_navigation.go:68-77 |
| Esc and `u` back one level (mejl/docs/terminal-ux.md:197,526) | **Conflict, with one point of agreement.** Esc cancels only an editor or picker. It is a no-op in Browse, Search and every overlay. `u` is a forbidden letter. Spela's own PLAN.md:143 leans toward mejl here. | internal/tui/keymap.go:499; internal/tui/layout_test.go:40-51,475-485; docs/design/tui-navigation.md:26 |
| Non-overlapping prefixes with which-key (mejl/docs/terminal-ux.md:157,199) | **Conflict.** Spela has no prefixes and no letters. | docs/design/tui-navigation.md:25-27; docs/design/tui-basic-key-navigation-plan.md:18-21 |
| Named presets, overrides, custom presets, key editor (mejl/README.md:84,98,129; mejl/docs/README.md:152) | **Silent.** The keymap is a compile-time table. Presets that add letter commands would break the basic-key rule the user restated. | internal/tui/keymap.go:427; docs/design/tui-profile-settings-ux-review-plan.md:110-114 |
| Mouse on by default, in-house hit testing (mejl/docs/architecture.md:23; mejl/README.md:91) | **Silent.** No mouse code at all, and the termctrl review is keyboard-only. | grep found no mouse handling in internal/tui (verified); docs/tui-testing.md:125-131 |
| One-line hint bar that follows focus (mejl/docs/terminal-ux.md:173) | **Partial agreement.** Spela's footer follows focus and comes from the same keymap. It is 2 rows, not clickable, and cannot be hidden ("Show hints" was removed). | internal/tui/layout_render.go:90-126; internal/tui/options_modal_test.go:48; TODO.md:62 |
| Filterable `?` help overlay | **Conflict** on the key and on filtering. Help opens from Actions, `?` is ignored or typed as text, and Help can only scroll. Both agree that help is contextual and built from the keymap. | internal/tui/keymap.go:493; internal/tui/help_test.go:78; internal/tui/routing_contract_test.go:54-73; internal/tui/help.go:107-120 |
| Undo instead of confirmation for reversible actions (mejl/docs/architecture.md:37) | **Partial agreement.** Space is a one-key undo per field. There is no undo stack. Whole-page resets, apply-to-games and DLL install and restore still confirm. | internal/tui/detail.go:542-580; internal/tui/scope_navigation.go:81-122; internal/tui/display_settings.go:5-9 |
| TOML edited in place with toml_edit from a settings screen (mejl/README.md:102; mejl/docs/rust-crates.md:251) | **Format conflict.** Spela uses YAML (config.yaml, profiles/*.yaml). Both write back from a settings screen and keep values that did not change. Whether comments survive is unverified for spela. | AGENTS.md (YAML rule); internal/tui/options_modal_test.go:174-206 |
| Breakpoints at 160 and 100 columns (mejl/docs/terminal-ux.md:253-255,503) | **Partial agreement.** Both switch to one pane under 100 columns. Spela also uses a 30-row threshold. Silent at 160. | internal/tui/layout_render.go:12-24 |
| `doctor` command (mejl/README.md:70) | **Silent.** No such command. | grep in cmd/spela (verified) |
| Demo mode backed by a fake (mejl/README.md:20-23) | **Silent.** Fakes exist only in tests, and the Services bypasses make a full fake impossible today. The e2e fixtures and the metrics hook are a partial demo. | internal/tui/testhelpers_test.go:24-56; section 14 |

### 22. Rewrite implications (inferred)

- Port the keymap as a data table with a context struct and availability closures. Then Actions, the footer and Help cannot drift from dispatch. Fix the Help reason defect.
- Keep the two overlay kinds separate in the ratatui layout. Reserve the controls before the content.
- Define one backend trait covering every disk, network and hardware call, including the current bypasses. Move per-frame sync I/O (ProfileExists, LoadDLLBackup) into cached state that refreshes on events.
- Keep request identities for rescan, DLL install and update checks, and coalescing write queues for profiles and Settings.
- Decide on the double startup scan and on the stale definitions that differ between Catalog and Library before porting.
- The e2e journeys are the best black-box parity oracle. They need a Rust binary, an equivalent metrics hook, and file parsing that does not import Go packages.

### 23. Doc, test and code mismatches

See the structured `conflicts` list.

### Conflicts and mismatches

- **Draft model in PLAN.md and other docs, autosave in code.** PLAN.md phase 3 and the basic-key plan and acceptance docs describe:
- drafts and a dirty state;
- Save, Discard and Ctrl+S;
- guards on replacing a draft.

The current contract and the code autosave every change and have no draft, Save or Discard. Evidence: PLAN.md:72-96; docs/design/tui-basic-key-navigation-plan.md:154-198; docs/design/tui-basic-key-acceptance.md:18-19 versus docs/design/tui-navigation.md:112-118; internal/tui/pane_actions.go:31-39
- **Where a game opens.** PLAN.md and the basic-key plan open a selected game on Overview. The code has no Overview: a game opens on Profile, or on DLLs if DLLs was the current aspect. Evidence: PLAN.md:25,57; docs/design/tui-basic-key-navigation-plan.md:88 versus internal/tui/resource_view.go:351-358; internal/tui/detail_test.go:532-534
- **Meaning of Space on profile fields.** The space/DLL acceptance doc says Space cycles (default), true, false, (default). The code switches between following and own value: a boolean takes the opposite value, other fields put back the previous value. The word (default) is now retired. Evidence: docs/design/tui-space-and-dll-controls-acceptance.md:11 versus internal/tui/detail.go:547-594; internal/profile/vocabulary.go:39
- **Show hints setting.** The historical docs and a mock require checks with hints turned off. The setting is gone from the TUI inventory, so those checks cannot run. Evidence: docs/design/tui-basic-key-navigation-plan.md:218-219,247,282; docs/design/tui-basic-key-acceptance.md:22,72; docs/design/tui-space-and-dll-controls-acceptance.md:19; tui-profile-settings-ux-review-mocks.md:462 versus internal/tui/options_modal_test.go:48; TODO.md:62
- **Escape semantics inside spela.** PLAN.md says Escape owns mode-specific back and cancel. In the code, Esc only cancels an editor or picker, and tests assert it is a no-op in Help, Browse, confirmations and Search. Evidence: PLAN.md:50,143 versus internal/tui/keymap.go:499; internal/tui/layout_test.go:40-51,475-485; internal/tui/dll_mutation_test.go:28-32; internal/tui/sidebar_test.go:288-295
- **Tests that send Esc to close Actions.** Two tests press Esc while the Actions menu is open, expecting it to close. Esc is a no-op in Overlay mode. The following "0" is ignored too, and the tests pass only because menuBindings is recomputed. Evidence: internal/tui/apply_own_values_test.go:67-70,196-198; internal/tui/layout_actions.go:108-110,190-218; internal/tui/keymap.go:507-512
- **Reverse Tab.** PLAN.md asks for Tab and reverse-Tab. No Shift+Tab binding exists, and the test labelled reverse Tab sends a plain tab. Evidence: PLAN.md:53 versus internal/tui/keymap.go:427-513; internal/tui/shell_contract_test.go:30-34
- **Help reference shows reasons for available keys.** The Help reference tables add the availability reason whenever it is non-empty. Several functions return a reason even when available. For example, Help prints "Switch pane; no other eligible pane" while Tab works. Evidence: internal/tui/help.go:94-99; internal/tui/keymap.go:434,437-439; docs/design/tui-navigation.md:67-68
- **Find row is not documented.** A "Find games" row above the list starts Search on Enter or on typing. The navigation contract never mentions it. On that row, digits 0 to 4 still act as Browse commands, so a search cannot begin with a digit there (inferred). Evidence: internal/tui/sidebar.go:192-196,386-398; internal/tui/layout_actions.go:136-145; internal/tui/profile_vocabulary_test.go:234-278; docs/design/tui-navigation.md (absent)
- **UX review status is stale.** The review and its plan say they await the user's choice and that nothing is implemented. Alternative B is in the code, yet the redesign TODO is still open. Evidence: docs/design/tui-profile-settings-ux-review.md:3-5; docs/design/tui-profile-settings-ux-review-plan.md:3-7; TODO.md:40 versus internal/tui/profile_header.go:20-59; internal/tui/detail.go:542-580
- **Mocks differ from the code.** Examples:
- The B-allgames mock uses wording that a test forbids ("keep their own").
- B-settings shows a one-page Settings that was never built.
- B shows "↑ ↓: Option" in Settings Detail, but Detail Up/Down are unavailable there. Evidence: tui-profile-settings-ux-review-mocks.md:420,445-489 versus internal/tui/profile_vocabulary_test.go:202-206; internal/tui/options_modal.go:358-386; internal/tui/keymap.go:445-446,515-517
- **Tests pin views production never renders.** These views are dead in production but tested:
- deployment matrix with ◆ stale cells;
- Catalog library table;
- RenderContextBar;
- renderOptionsBody;
- breadcrumbs;
- the inherited-only banner (with the retired word "Defaults");
- executeBatchDLLUpdate. Evidence: internal/tui/dlls_resource_test.go:60-106,171-191; internal/tui/help_test.go:10-110; internal/tui/options_modal_test.go:208-221,282-297; internal/tui/shell_contract_test.go:93; internal/tui/content_supported_test.go:347,376; internal/tui/layout_test.go:703-783
- **Stale test names, comments and vacuous tests.** Many tests mention retired keys: rail, r reserved for Task 5, P and p, ctrl+f, ctrl+r, "q on rail", j and k. One test passes no matter what: it presses "?" twice and expects Help closed, but "?" is unbound. The sidebar_test header says the sidebar is games-only, yet a test asserts the pinned All games row. Evidence: internal/tui/layout_test.go:215-222,259-307; internal/tui/testhelpers_test.go:429-438; internal/tui/sidebar_test.go:9-12,99-130,164-173
- **ANSI colour asserted in tests.** PLAN.md says ANSI snapshots are not permanent contracts. Detail tests still assert exact truecolor escape triples. Evidence: PLAN.md:146-147 versus internal/tui/detail_test.go:628-653
- **Legacy theme handling.** A styles.go comment says legacy theme values are ignored at load and dropped on the next save. Tests assert the value is kept through load and save. Evidence: internal/tui/styles.go:100-103 versus internal/tui/styles_test.go:153-176; internal/tui/options_modal_test.go:174-201
- **Double startup scan.** cmd tui runs steam.RefreshIfNeeded synchronously before the TUI starts. The TUI then starts its own async rescan under the same condition (empty DB or rescan_on_startup). Evidence: cmd/spela/commands/tui.go:39-42; internal/steam/refresh.go:31-39; internal/tui/layout.go:117-122,141-147
- **Empty library when Steam is missing.** The docs require an empty library to stay interactive with recovery hints. If no Steam installation is found, the synchronous rescan returns an error and the TUI never starts (inferred). Evidence: docs/design/tui-navigation.md:74-75; docs/tui-testing.md:116-118 versus internal/steam/refresh.go:22-24; cmd/spela/commands/tui.go:39-42
- **Two definitions of stale.** Library DLL updates and batch updates compare against the manifest's latest version and may download. Catalog update-all compares against the newest cached version and uses the cache only. The two views can disagree about the same DLL. Evidence: internal/tui/dll_mutation.go:36-51; internal/tui/content.go:301 versus internal/tui/dlls_resource.go:286-295,477
- **Editor prefill does not match the row.** The clock offset row shows "none", but its editor says "Value before edit: 0". The open TODO describes this. The e2e test pins the current behavior. Evidence: tests/e2e/tui_editor_kinds_test.go:102,113-114; internal/tui/detail.go:195-199; TODO.md:7
- **Settings free-number editor.** The doc says Enter opens "a text or free-number editor". Every int Settings option has choices and cycles with Space. No free-number editor is reachable in Settings. Evidence: docs/design/tui-navigation.md:194-195 versus internal/tui/active_state_contract_test.go:84-93; internal/tui/options_modal.go:481-484
- **Services seam is incomplete.** Services exists for injection, but manifest reads, profile writes, the release check and metrics bypass it. VKD3DNotice is wired but unused. Synchronous disk calls run inside render and context building. Evidence: internal/tui/services.go:13-38,98-117; internal/tui/content.go:117-130,339,373,394; internal/tui/layout.go:93; internal/tui/sidebar.go:532; internal/tui/content_actions.go:68-77
- **Footer overflow.** The doc says essential controls are always visible. The footer keeps only 2 rows of hints and drops the rest without notice. Order protects "0: Actions" (inferred). Evidence: docs/design/tui-navigation.md:55-57 versus internal/tui/layout_render.go:104-125
- **Destination-retention doc versus nav package.** The doc says each destination keeps its cursor, group and filters. nav.SelectDestination resets sub-sections. Retention works only because the TUI bypasses that function, and tests still call it to build state. Evidence: docs/design/tui-navigation.md:12-13; internal/nav/nav.go:200-218; internal/tui/layout.go:213-221; internal/tui/active_state_contract_test.go:16-28
- **Per-field undo memory is lost on game change.** The doc says Space undoes either move with the same key. The remembered own value lives in a DetailModel that SetGame rebuilds. After a game switch, Space on a non-boolean following field opens the editor instead of restoring (inferred). Evidence: docs/design/tui-navigation.md:120-124; internal/tui/detail.go:74-76,583-594; internal/tui/content.go:241
- **Basic-key rule against mejl's adopted UX.** The user's request is to adopt mejl's terminal UX, which uses letters, a palette, q/Q, Esc/u, prefixes, presets, mouse and `?`. On 2026-10-14 the user directed that basic keys only stays the rule, and PLAN.md bans a palette and q-as-back. Evidence: docs/design/tui-profile-settings-ux-review-plan.md:110-114; PLAN.md:143-145; mejl/docs/terminal-ux.md:494,521,526-527
- **Config format.** mejl edits TOML in place with toml_edit. Spela stores YAML, and existing user files and e2e fixtures are YAML. Evidence: mejl/README.md:102; mejl/docs/rust-crates.md:251 versus AGENTS.md (YAML rule); tests/e2e/mock_helper.go:161-202

### Open questions

- Keys: should the Rust TUI keep spela's rule of basic keys only (digits 0 to 4, arrows, Tab, Enter, Space; Esc only in editors; Ctrl+C), or adopt mejl's letter shortcuts, palette, q/Q, `?`, prefixes and presets? This reverses the direction you gave on 2026-10-14 (docs/design/tui-profile-settings-ux-review-plan.md:110-114).
- Escape: should Esc close overlays and Search, and back out one level as in mejl? Current spela tests forbid it in Help and in confirmations.
- Mouse: should mouse capture be on by default? Spela has no mouse support, and its termctrl review is keyboard-only.
- Config: should the Rust rewrite read and write the existing YAML files (config.yaml, profiles/*.yaml, games.yaml) unchanged, or move to TOML with migration?
- Parity target for known defects: today's behavior or the fixed behavior? Cases:
- the editor prefills 0 for a 'none' row (TODO uzofpmrkiu);
- the Help reference shows reasons for keys that are available;
- a digit on the Find row switches destination instead of starting a search;
- the startup scan runs twice.
- If Steam cannot be found at startup, should the TUI still start with the empty-library recovery screen, or keep today's startup error (inferred)?
- Stale DLLs: should Catalog and Library share one definition (manifest latest or newest cached)? Today they differ.
- Dead views: should the deployment matrix with stale markers, the Catalog library table, and the 'values inherited from Defaults' banner be built in Rust? Only tests reach them today.
- Breakpoints: keep spela's single threshold (100 columns or 30 rows, floor 80x24), or adopt mejl's 160 and 100 column breakpoints? Spela has two panes, not three.
- Footer: keep spela's 2-row, non-clickable footer, or move to mejl's one-line, clickable, hideable hint bar? If it can be hidden, which controls must stay visible?
- Undo: should mejl-style undo (a stack, undo replacing confirmation) replace the confirmations for whole-page Follow, Apply to games with their own value, and DLL restore and install? Should per-field undo survive a game switch?
- E2E: keep the Go termctrl e2e suite as a black-box oracle for the Rust binary? It needs:
- a SPELA_E2E_METRICS equivalent;
- the runtime lock path;
- no imports of internal/profile or internal/dll.
- Demo mode: should the Rust TUI get a mejl-style demo backed by a fake? That needs a complete backend trait, covering manifest, profile writes, metrics and release checks.
- Should Shift+Tab, documented in PLAN.md as reverse-Tab, be added? Should the wrap rules for lists be made consistent (Actions and Settings wrap; profile, Library and DLL lists clamp)?
- Theme: keep the single dark neon palette, which does not paint the background? Add a light theme or NO_COLOR handling? (lipgloss behavior is unverified.)
- Should the VKD3D compatibility notice, wired in Services but never shown, appear in the Rust TUI?

## Completeness critic: additions to the spela parity inventory

Labels: (verified: path:line), (inferred), (unverified). Paths are relative to /home/jgabor/git/spela. Nothing was built or run.

### A. Files no reader covered

| File | What it is | Relevant to the Rust rewrite? |
|---|---|---|
| docs/plans/valiant-herding-dolphin.md | Plan for wiring the inert Settings options (TODO hpuwnhwspr), in 7 phases with 4 decisions | Historical. Its findings are stale: it says `check_updates` "has no code at all" (line 26) and that logging has no level control (line 16). Both exist now (internal/release/release.go, internal/config/loglevel.go). It proposed a cobra `PersistentPreRunE` (line 40). The code applies the log level in main() instead (cmd/spela/main.go:117), and no PersistentPreRun exists anywhere in cmd/ (verified: grep). Its decisions 2 to 4 match the shipped code: auto-update skips running and deny-listed games, confirm_destructive applies to update only, and compact_mode maps to the Compact density (verified: internal/config/options.go:90,99-100; internal/tui/display_settings.go). |
| docs/migrations/beans-to-agentera.md | Process record of the backlog migration | Process only, no runtime contract (skimmed). |
| docs/migrations/beans-source/*.md (25 files) | Imported backlog items | Read. Items that bear on parity: spela-117e (CLI output formatting, todo), spela-ftlu (logo in CLI help, todo, same as TODO vwsuxxnada), spela-rcsg (names and IDs in CLI, todo but already done: FindGame does both), spela-2v82, emt9, csr0, cife (XeSS and FSR hosting and automation, draft or todo; the FSR 4 names amd_fidelityfx_upscaler_dx12.dll, _framegeneration_dx12.dll and _loader_dx12.dll are at beans spela-cife:14), spela-s84z and its phases ifu0, fw5t, ixak, hyis, gal2, nvwk (overlay, all todo), spela-m8qo (UPX on release, todo), spela-dcgs (AUR spela-git, in progress, still unpublished per its checklist), spela-x1hf (DLL infra, done; tags `dll-<type>-v<ver>` in the main repo). |
| tests/test_merge_coverage.py, scripts/merge-coverage.py, scripts/check-coverage.py | Python tests and tools that merge Go and frontend coverage into lcov and check totals | Go-only CI tooling, same category as tools/coverage-inventory. The scripts were skimmed, not read. Not needed for the Rust binary. |
| docs/design/gui-integration-design-historical.md, tools/gen-nav-js/** | GUI | Excluded. |

### B. CLI behaviour the reports miss

#### B.1 No CLI surface for All games
- No command calls `profile.MutateDefault`. The only `LoadDefault` callers in cmd/ are readers (verified: cmd/spela/commands/inheritance_render.go:37, launch.go:98).
- So the All games profile (`profiles/default.yaml`) can be edited only in the TUI (or GUI) or by hand. The CLI also has no "Apply to games with their own value". This is a CLI/TUI asymmetry to keep or fix on purpose (inferred).

#### B.2 Setter validation is done by profile.Set, with pass-through errors
- Every `* set` flag goes through `profileChanges.set`, which calls `current.Set(field, value)` inside `profile.Mutate` (verified: inheritance_render.go:151-175).
- Allowed values are enforced there (verified: internal/profile/fields.go:200-205). The error is wrapped as `<key>: unsupported value "<v>" (valid: <list>)` (verified: fields.go:192, 204).
- The list starts with an empty entry, so it reads `(valid: , off, ultra_performance, ...)` (inferred from the allowed lists that begin with "" at fields.go:46-50).
- Consequences (inferred unless marked):
  - `dlss set --sr-mode bogus` fails with `dlss.sr_mode: unsupported value "bogus" (valid: , off, ultra_performance, performance, balanced, quality, dlaa)`.
  - `--sr-preset G`, `H` or `I` fail, even though the flag help says "A-M" (verified help: dlss.go:52; verified allowed list: fields.go:47).
  - `--rr-preset auto` is normalised to `auto` by NormalizeSRPreset (dlss.go:143), then rejected, because rr_preset has no `auto` (fields.go:50).
  - `overlay set --position middle` is rejected (fields.go:94 allowed list; overlay.go:97-99).
  - `gpu set --power-mizer turbo` is rejected. The values adaptive, max, prefer_max_performance and prefer_maximum_performance are accepted (fields.go:84).
  - `cpu set --governor`:
    - First check: the value must be in sysfs `scaling_available_governors`. The error is `--governor: CPU governor "x" is not available (available: a, b)`, or `(available: none)` (verified: cpu_validation.go:11-19; internal/cpu/cpu.go:119-141).
    - Second check: the descriptor list "", performance, powersave, schedutil, ondemand (verified: fields.go:89).
    - So `conservative` or `userspace` pass the first check and fail the second.
  - Error wrapping: these profile errors come back from Mutate unwrapped by the command and are printed by cobra as `Error: ...` plus usage (inferred).
- Live `cpu governor <g>` validates against sysfs before any write, through `cpu.SetGovernor` (verified: internal/cpu/cpu.go:143-146). The CLI report did not mention this pre-check. Without root it writes cpu0 to cpu`runtime.NumCPU()-1` one pkexec call at a time (verified: cpu.go:35-37, 150-157).
- Risk (inferred): NumCPU counts the CPUs in the process affinity mask. CPUs taken offline (for example SMT siblings) or a non-contiguous numbering make the loop miss CPUs or fail.

#### B.3 Which commands refresh the game database
- Only top-level `show`, `list` and `tui` call `steam.RefreshIfNeeded` (CLI report).
- The per-subsystem `show` commands use `loadProfileSnapshot`, which only loads games.yaml (verified: inheritance_render.go:24-48). So do all `set`, `reset` and `profile-reset` commands (inheritance_render.go:177-185; dlss.go:108-117; gpu_set.go:71-80; cpu_set.go:60-69), `launch` (launch.go:41), the `dll` subcommands (dll.go:51, 151, 169) and `denylist` (denylist.go:99, 130, 159). A game added after the last scan is "game not found" in all of them (inferred).

#### B.4 Direct launch prints the summary before Prepare
- For `spela launch <game> <cmd...>`, `PrintLaunchSummary` runs before `Prepare`, and both run before the exec (verified: launch.go:71-83).
- `LoadLaunchProfiles` wraps a broken default.yaml as `load default profile: %w` (verified: launch.go:98-101). In `launch` that becomes `failed to load profile: load default profile: ...` (launch.go:58-61).
- With no game profile and no default.yaml, the effective profile is whatever LoadDefault returns (unverified whether nil or empty). The "No profile or defaults will affect launch preparation" line appears only when it is nil (launch.go:127-129).
- `countProfileOverrides` counts every `true` in the overrides map, including unknown keys that survived load (verified: launch.go:283-294; inferred for unknown keys).

#### B.5 JSON field names and nulls
- `Game` and `DetectedDLL` have yaml tags only, no json tags (verified: internal/game/game.go:20-33). `--json` output therefore uses Go field names: `AppID`, `Name`, `InstallDir`, `LibraryPath`, `PrefixPath`, `DLLs`, `ScannedAt`, and `Path`, `Name`, `Type`, `Version`.
- `PrefixPath` is always present (yaml omitempty does not apply to JSON).
- An empty `DLLs` slice prints as `null` (inferred: Go encoding of a nil slice).
- `scan --json` marshals the `map[uint64]*Game` with decimal string keys (verified: scan.go:40).

#### B.6 `dll update` details
- The type argument is lowercased and matched against detected DLL types through `dll.UpdateGame` (verified: dll.go:149, 163). An unknown type, or a type the game lacks, gives `game does not have a <type> DLL` (dll.go:183-185).
- For xess or fsr the manifest list is empty (data/manifest.json). The item then fails with a resolve error such as `no xess version "" available in manifest`. The exact text depends on the latest-version path (inferred from internal/dll/operations.go:322-329).
- The README promises xess and fsr updates (README.md:126-127), which conflicts with this.
- `dll restore` passes a nil progress callback and prints `Restored original DLLs for <Name>` even when the result is a no-op (verified: dll.go:212-231).

#### B.7 `denylist show`
- The `User-denied games:` heading is printed only when `overrides.denied` is non-empty, preceded by a blank line (verified: denylist.go:88-93).
- `[FORCE-ALLOWED]` is shown only on built-in or file list entries, never on user-denied ones (denylist.go:80-86).
- `LoadOverrides` errors are ignored here (denylist.go:77).

#### B.8 TUI startup details
- `LayoutModel.Init` checks `m.config != nil` for `CheckUpdates` (internal/tui/layout.go:108), then reads `m.config.AutoUpdateDLLs` and `m.config.RescanOnStartup` with no nil check (layout.go:111, 114). If construction can leave config nil, this panics (inferred; whether LoadConfig can yield nil there is unverified).
- Init always schedules `m.pane.content.LoadDLLUpdates()` (layout.go:107). That path calls `dll.GetManifest`, so a TUI start can hit the network when the manifest cache is stale (inferred from the TUI report's bypass table, content.go:339).

### C. Doc and catalog mismatches not in the reports
- AGENTS.md's project structure omits internal/display, internal/nav, internal/proton and internal/release, all of which exist (verified: /bin/ls internal). It also describes internal/cpu as handling "affinity", which nothing implements.
- `currentManifestPolicy` says in a comment that "a refresh interval below one hour falls back to the default". The code only requires `> 0` (verified: internal/dll/manifest.go:177-185). The Settings choices are 1, 6, 12, 24, 48 and 168 (options.go:97), but `intOption.set` accepts any integer (options.go:230-243).
- `--sr-preset` help "A-M" versus the allowed list A-F, J-M (see B.2).
- The README lists xess and fsr as update types while the manifest has none (see B.6).

### D. Coverage tooling (brief)
scripts/merge-coverage.py merges Go cover profiles and frontend lcov by highest hit count, maps byte columns to UTF-8 code points, and rejects paths outside the repository. scripts/check-coverage.py recomputes the totals. Both are pinned by tests/test_merge_coverage.py:36-153 (verified). Like tools/coverage-inventory, they are Go and Svelte specific. A Rust equivalent would be cargo-llvm-cov plus a threshold gate (inferred).

### E. Spot-check log (30 claims)
These claims held against the code:
1. main() order and wrapper before cobra (cmd/spela/main.go:115-133).
2. `--version` works because `Version: version` is set (main.go:27).
3. `Available: %v` (cpu.go:141).
4. `list` prints "No games found" before the JSON branch (list.go:52-59).
5. Delete treats a missing file as success (storage.go:237-245).
6. Lock modes 0700/0600, PID with no newline, EPERM read as dead (lock.go:38-45, 85-93).
7. Runtime fallback `/tmp/runtime-<USER or unknown>/spela` (xdg.go:31-40).
8. Polkit file content (io.github.spela.policy:1-19).
9. IPC layout (ipc.go:12-46).
10. Release regex, 5 s timeout, 1 MiB cap (release.go:28-34).
11. TUI constants (layout.go:27-35).
12. listWidth formula (layout.go:244).
13. Single pane and compact header thresholds (layout_render.go:12-24).
14. Quit reason "wait for pending operations" (keymap.go:494).
15. Help reference tables append any non-empty reason (help.go:94-99).
16. Message clear after 5 s (messagebar.go:18).
17. Search CharLimit 30 (sidebar.go:71).
18. Thermal stops (thermal.go:18-25).
19. GetLatestDLL returns index 0 (manifest.go:227-233).
20. FSR file names and catalog order (detect.go:17-44).
21. Auto preset L/M/K and lowercase `render_preset_<x>` (apply.go:224-249).
22. `--multi-frame` default -1 (dlss.go:60).
23. Backup file name sha256-basename (swap.go:304-307).
24. Tags up to v0.6.2 plus three dll-* tags, and last manifest commit 2026-02-03 (git).
25. e2e bar rows and quit-by-pid-file (tests/e2e/tui_journeys_test.go:89-91, 214-224).
26. Destination bar, "Saved at once" and sidebar empty-state strings (destination_bar.go:27-51; detail_sections.go:77-86; sidebar.go:345-358).

The other four checks found the errors listed in `errors`.

### Not covered

- docs/migrations/beans-to-agentera.md: only headings skimmed. It is a backlog migration process record with no runtime contract.
- scripts/merge-coverage.py and scripts/check-coverage.py: line counts only. Their behaviour is taken from tests/test_merge_coverage.py, and they are Go/Svelte coverage tooling not needed for Rust.
- scripts/aur-submit.sh: not read by me (CLI reader also skipped it; behaviour is pinned by tests/contracts/aur_submit_test.go).
- docs/design/gui-integration-design-historical.md and tools/gen-nav-js/**: GUI, excluded by scope.
- Whether profile.LoadDefault returns nil or an empty profile when default.yaml is missing: not checked. It decides whether launch prints 'No profile or defaults will affect launch preparation'.
- Whether TUI construction can leave LayoutModel.config nil, which would make the unguarded m.config.AutoUpdateDLLs read in Init panic: not checked.
- The exact error text for `dll update <game> xess|fsr` against the empty manifest lists: inferred from operations.go, not traced line by line.
- mejl repository: outside this critic's scope.
