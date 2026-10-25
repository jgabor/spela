# Rust crates for the spela rewrite (outside the UI)

## Summary

This is the crate reference for a Rust rewrite of spela's CLI and TUI. It covers everything outside the UI: NVML, YAML, VDF, PE, HTTP, processes, locks, mmap, paths, time, logging and testing. It follows the method of `/home/jgabor/git/mejl/docs/rust-crates.md`. Every version, release date, license and MSRV below comes from the live crates.io API, fetched on 2026-10-25. Repo signals come from `gh api`, and advisories come from `rustsec/advisory-db`. Three probe crates were built and run on this machine, along with a Go program that writes real yaml.v3 and encoding/json fixtures from spela's struct definitions. So the starter set is known to resolve, compile, run, and read and write spela's real formats.

Main findings and what they mean for the rewrite:

- **YAML:** use [serde-saphyr](https://crates.io/crates/serde-saphyr) 2.0.0 with `ser_options! { indent_step: 4, compact_list_indent: false, prefer_block_scalars: false }`. Its output is byte-identical to yaml.v3 for `profile.yaml`, empty profiles (`overrides: {}`) and deny-list `overrides.yaml`. `games.yaml` and `config.yaml` differ only in quoting. Go's yaml.v3 reads every Rust-written file back to an equal value (verified: probe, `goref check`). It is the only serde YAML crate tested that accepts `smt: yes` as `true`, as yaml.v3 does (verified: Go and Rust probes). `serde_yaml` is deprecated and archived. `serde_yml` and `libyml` are flagged unsound (RUSTSEC-2025-0068 and -0067).
- **NVML:** [nvml-wrapper](https://crates.io/crates/nvml-wrapper) 0.13.0 covers every NVML call spela makes. It loads `libnvidia-ml.so.1` with libloading at runtime. A missing driver is a plain error (verified: probe).
- **VDF:** [keyvalues-parser](https://crates.io/crates/keyvalues-parser) 0.2.4 parses all real Steam files on this machine. Its grammar is more correct than spela's line parser, so a few edge cases change (see Steam VDF).
- **PE version:** [object](https://crates.io/crates/object) 0.40.0 (+1 crate) with a direct port of Go's `.rsrc` scan. pelite, goblin and the port agreed on 1,224 real DLLs (verified: probe).
- **HTTP:** blocking [ureq](https://crates.io/crates/ureq) 3.5.0 on a worker thread. reqwest plus tokio (mejl's stack) costs 27 more crates, 12.5 s more clean build time (measured on the first probe) and 3.3 MB more binary.
- **System:** std plus rustix, signal-hook 0.3, memmap2 and tempfile. `std::fs::File::lock` (stable since 1.89) replaces `syscall.Flock`. rustix and signal-hook 0.3 are already in crossterm's tree, so they cost nothing.
- **Build cost (measured, final starter set):** 177 crates. A clean release build takes 18.5 s with a warm registry, an incremental release 9.0 s and an incremental debug 0.3 s. The stripped binary is 5.8 MB, or 4.2 MB with `opt-level = "s"` and `panic = "abort"`.
- **MSRV:** 1.89, set by std `File::lock` and serde-saphyr 2.0. 1.88 fails and 1.91 builds (verified: local toolchains). mejl declares 1.88 (verified: `/home/jgabor/git/mejl/Cargo.toml:9`).
- **License traps:** `exe` (GPL-3.0), `new-vdf-parser` (LGPL-2.1), `attohttpc` (MPL-2.0), `steamy-vdf` (WTFPL), `singe-nvml` (non-standard).

## How this was gathered

- **Crate metadata:** `https://crates.io/api/v1/crates/<name>` gave `max_stable_version`, that version's `created_at`, `license`, `rust_version` and `features`. `.../<name>/<version>/dependencies` gave dependency requirements. The User-Agent named the research purpose. All table values are (verified: crates.io API, 2026-10-25) unless a row says otherwise.
- **Repo signals:** `gh api repos/<owner>/<repo>` for 113 GitHub repos (verified: GitHub API, 2026-10-25). "Open" counts both issues and pull requests. **Advisories:** `gh api repos/rustsec/advisory-db/contents/crates/<name>`.
- **Source checks:** crate sources in the local cargo registry, after `cargo fetch` in the scratch probes. Also Go module cache sources for pflag v1.0.10 and cobra v1.10.2, and rust-src for std.
- **Go reference program:** `goref` in scratch. It copies spela's structs from `internal/profile/profile.go`, `internal/game/*.go`, `internal/config/config.go`, `internal/denylist/denylist.go`, `internal/dll/manifest.go` and `internal/overlay/ipc.go`. It writes fixtures with yaml.v3 v3.0.1 and encoding/json. It checks that Go can read Rust-written YAML, and it reads the Rust-written overlay IPC file.
- **Probes**, all in `/tmp/claude-1000/-home-jgabor-git-spela/c455c1a5-ee6c-4ad4-a8a6-b6b0f16f5d64/scratchpad/crate-probe/`:
  - `starter/`: the first starter set, used for the measurements and the reqwest variant.
  - `final/`: the proposed starter set.
  - `compare/`: YAML backends, PE parsers and VDF alternatives side by side.
  - `goref/`: the Go reference program.

  Build settings: edition 2024, rustc 1.99.1, 16 cores, `lto = "thin"`, `codegen-units = 1`, `strip = true`.
- **Safety:** NVML was only queried read-only. Setters were compiled in a function that is never called. No pkexec or sudo was run. Real Steam files and spela's DLL cache were only read, and only counts were printed.
- **Verdict key** (same as mejl): **Pick** = in the starter set. **Later** = add when the feature arrives. **Alt** = viable alternative. **Ref** = study, do not depend on. **Avoid** = do not use, reason given.

## What spela needs (derived from the Go code)

| Need | Go evidence | Rust approach |
|---|---|---|
| NVML init, device 0, metrics (temperature, power, power limit, utilization, memory, graphics and memory clocks, fan, throttle reasons) | `internal/gpu/nvml.go:16-90` | nvml-wrapper |
| NVML setters: locked GPU and memory clocks and their resets, power limit (W×1000), fan speed per fan and default, number of fans | `internal/gpu/nvml.go:93-168`, `internal/gpu/nvidia.go:61-92` | nvml-wrapper |
| NVML info: name, driver, power-limit constraints | `internal/gpu/nvml.go:170-218` | nvml-wrapper |
| NVML initialized eagerly in every process | `cmd/spela/main.go:118-119` | same |
| nvidia-smi fallback (query CSV; `-lgc`, `-lmc`, `-pl`, `-rgc`, `-rmc`, elevated when not root) | `internal/gpu/nvidia.go:114-132`, `:168-207`, `internal/gpu/driver_version.go:54` | std `Command` |
| Hidden `apply-profile` root command, called through `pkexec <self>` | `cmd/spela/commands/apply_profile.go:36-63`, `internal/privilege/privilege.go:120-130`, `internal/profile/apply.go:96-147` | clap hidden subcommand, std `current_exe` and `canonicalize` |
| pkexec lookup, exit code 126 means dismissed, stdin piped to `tee` | `internal/privilege/privilege.go:33-80`, `:132-138` | std `Command`, `which` |
| euid check | `internal/privilege/privilege.go:40-42` | `rustix::process::geteuid` |
| sysfs: governor read and write, available governors, SMT active and control, scaling_cur_freq | `internal/cpu/cpu.go:39-142`, `:197-216` | std fs |
| procfs: cpuinfo, meminfo, loadavg | `internal/cpu/cpu.go:144-174`, `:221-257` | std fs |
| `/proc/<pid>/{cwd,exe,cmdline}` scan for running games | `internal/game/running.go:14-100` | std fs |
| `scx_loader` lookup, `systemctl is-active scx.service` | `internal/cpu/cpu.go:176-185` | std `Command`, `which` |
| `kscreen-doctor -o` and `output.<uuid>.vrrpolicy.<p>` with a 10 s timeout and `LC_ALL=C`, stdout check for "applying config failed!" | `internal/display/vrr.go:24-34`, `:89-100`, `:129-206` | std `Command`, `wait-timeout`, `regex-lite` |
| Launch the child with an extended env and inherited stdio; forward SIGINT and SIGTERM; run cleanup in reverse order | `internal/launcher/launcher.go:219-264`, `internal/env/env.go:22-32` | std `Command`, signal-hook 0.3, rustix `kill_process` |
| Wrapper-mode detection before CLI parsing (env assignments, `--`, `LookPath`) | `internal/launcher/launcher.go:266-353`, `cmd/spela/main.go:121-127` | hand-written, `which` |
| Overlay IPC: 80-byte file, `MAP_SHARED`, seqlock in an atomic u32, little-endian fields, 500 ms collector | `internal/overlay/ipc.go:12-240`, `internal/overlay/setup.go:13-34`, `internal/overlay/collector.go:42-61`, `internal/launcher/launcher.go:203-211` | memmap2, `AtomicU32`, std thread |
| Single-instance PID file (O_EXCL plus kill 0) | `internal/lock/lock.go:35-93`, `cmd/spela/commands/tui.go:24-27` | std, rustix `test_kill_process` (or flock, as mejl does) |
| Cross-process flock on `games.lock` and `.mutation.lock` | `internal/game/database.go:52-82`, `internal/profile/storage.go:221-235` | std `File::lock` |
| Atomic temp-file plus rename writes (no fsync) | `internal/game/database.go:106-137`, `internal/profile/storage.go:87-118`, `internal/dll/swap.go:276-302`, `internal/release/release.go:141-163` | tempfile `persist` |
| YAML: `config.yaml` over defaults, `profiles/*.yaml`, `games.yaml`, `lists/denylist.yaml`, `lists/overrides.yaml`; `config show` prints YAML | `internal/config/config.go:66-96`, `internal/profile/storage.go:19-118`, `internal/profile/migrate_sr_preset.go:35-66`, `internal/game/database.go:84-137`, `internal/denylist/denylist.go:44-107`, `cmd/spela/commands/config.go:42` | serde-saphyr |
| JSON: manifest cache, backups, update-check state, every CLI `--json` output | `internal/dll/manifest.go:35-111`, `internal/dll/swap.go:20-88`, `internal/release/release.go:58-163`, `cmd/spela/commands/scan.go:40`, `list.go:61`, `show.go:51`, `profile.go:148`, `dlss.go:92`, `gpu_set.go:152`, `cpu_set.go:116`, `proton.go:160`, `overlay.go:165` | serde_json plus a Go-escaping formatter |
| Go `time.Time` values in YAML and JSON (RFC 3339 with nanoseconds and the local offset) | `internal/game/game.go:25`, `internal/game/database.go:44`, `internal/dll/manifest.go:37-58`, `internal/dll/swap.go:23`, `internal/release/release.go:58-61` | jiff (`Pieces` plus `Timestamp`) |
| HTTP GET: manifest, DLL download with progress, GitHub tags API (5 s timeout, 1 MiB limit, `Accept` header) | `internal/dll/manifest.go:22-30`, `:113-139`, `internal/dll/download.go:43-113`, `internal/release/release.go:28-39`, `:90-122` | ureq, sha2 |
| Manifest URL validation (https, or http to loopback only) | `internal/config/paths.go:63-77` | `ureq::http::Uri`, `std::net::IpAddr` |
| SHA-256 of cached and downloaded DLLs, digest in backup names, hex validation | `internal/dll/download.go:45`, `:67-99`, `internal/dll/swap.go:304-307`, `internal/dll/operations.go:364-370` | sha2 plus a small hex helper |
| Steam VDF: `libraryfolders.vdf`, `appmanifest_*.acf`, `config/config.vdf` CompatToolMapping | `internal/steam/vdf.go:13-104`, `internal/steam/library.go:34-124`, `internal/steam/appmanifest.go:20-62`, `internal/proton/resolver.go:129-163` | keyvalues-parser |
| Steam root candidates; `toolmanifest.vdf` stat; byte search in the Proton launch script | `internal/steam/library.go:18-32`, `internal/steam/filter.go:73-77`, `internal/proton/resolver.go:88-101` | std |
| PE `VS_FIXEDFILEINFO` from the `.rsrc` section of a recursive DLL walk | `internal/dll/detect.go:54-162` | object plus a ported scan, walkdir |
| Dotted numeric version compare; release regex; driver version parse; CachyOS tag regex | `internal/dll/version.go:11-52`, `internal/release/release.go:41-45`, `internal/proton/driver.go:44-137`, `internal/proton/build_version.go:8-65` | hand-written, regex-lite |
| Tool-name regexes; ANSI and UUID regexes | `internal/game/database.go:23-40`, `internal/display/vrr.go:121-124` | regex-lite |
| XDG config, data, cache and runtime directories, with spela's fallbacks | `internal/xdg/xdg.go:10-62` | hand-written (etcetera is Alt) |
| slog text handler to stderr, level from config | `internal/logging/logging.go:13-38`, `cmd/spela/main.go:117` | tracing plus tracing-subscriber |
| cobra CLI: about 16 command groups, `Changed()` semantics, the default `completion` command | `cmd/spela/main.go:31-48`, `cmd/spela/commands/*.go`, Go cache `cobra@v1.10.2/completions.go:745`, `command.go:1113` | clap derive, clap_complete |
| CLI colors (lipgloss styles) | `internal/tui/styles.go:291-317` | anstyle plus anstream |
| Profile field access by dot-path through reflection (35 keys) | `internal/profile/inheritance.go:84-107`, `internal/profile/storage.go:156` | no crate; a `macro_rules!` table (inferred) |
| Archives (zip, tar) | none at runtime. Only `tests/contracts/release_contract_test.go:4` uses `archive/tar` | not needed |
| D-Bus | none. VRR goes through the `kscreen-doctor` CLI (`internal/display/vrr.go:27`) and polkit through the `pkexec` CLI (`internal/privilege/privilege.go:53`) | not needed |

## NVML and GPU control

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [nvml-wrapper](https://crates.io/crates/nvml-wrapper) | 0.13.0 | 2026-08-31 | MIT OR Apache-2.0 | 1.60.0 | **Pick** | Has every setter spela uses: `set_gpu_locked_clocks(GpuLockedClocksSetting::Numeric{..})`, `reset_gpu_locked_clocks`, `set_mem_locked_clocks`, `reset_mem_locked_clocks`, `set_power_management_limit` (mW), `set_fan_speed(fan, pct)`, `set_default_fan_speed`, `num_fans` (verified: `nvml-wrapper-0.13.0/src/device.rs:5411`, `5442`, `5467`, `5497`, `5634`, `2060`, `2085`, `2106`; probe compiled them all). It also has true VF offsets (`set_gpc_clock_vf_offset` `:1811`, `set_mem_clock_vf_offset` `:1866`, `set_clock_offset` `:5732`) and `set_power_mizer_mode` (`:5659`), which matter for spela's unused `power_mizer` field. Setters take `&mut Device`, and `Device<'_>` borrows `Nvml`, so keep `Nvml` in a `OnceLock` or look up the device per call (inferred). Repo rust-nvml/nvml-wrapper: 216 stars, 10 open, pushed 2026-08-31. |
| [nvml-wrapper-sys](https://crates.io/crates/nvml-wrapper-sys) | 0.10.0 | 2026-08-31 | MIT OR Apache-2.0 | 1.60.0 | Pick (transitive) | Generated bindings, loaded through libloading ^0.8.1 (verified: deps API). |
| [libloading](https://crates.io/crates/libloading) | 0.9.0 | 2025-11-05 | ISC | 1.88.0 | Pick (transitive, 0.8.9 resolved) | nvml-wrapper pins ^0.8.1, so 0.8.9 resolves. Adding 0.9 directly would make a second copy (verified: `cargo tree`). |
| [baracuda-nvml](https://crates.io/crates/baracuda-nvml) | 0.1.0-alpha.25 | 2026-10-25 | MIT OR Apache-2.0 | - | Avoid | Alpha, 6 stars, published today. |
| [singe-nvml](https://crates.io/crates/singe-nvml) | 0.1.0-alpha.9 | 2026-07-01 | non-standard | - | Avoid | Alpha, non-SPDX license. |

Behaviour notes (verified: probe on an RTX 4070 Ti, driver 615.78.08, read-only):

- The default library name is the constant `"libnvidia-ml.so.1"` (verified: `nvml-wrapper-0.13.0/src/lib.rs:162`). The doc comment says `"libnvidia-ml.so"` (`lib.rs:270`). The code is the one that matters: `.so.1` is the runtime soname the driver ships.
- With a missing library, `Nvml::builder().lib_path(..).init()` returns `LibloadingError("...cannot open shared object file...")` and does not panic. The release binary's `ldd` output lists only libgcc_s, libc and the loader. NVML is never linked, so the binary starts on systems without NVIDIA.
- `Nvml::init` took 4.1 to 16.6 ms. A sweep of 12 queries (temperature, power, limit, constraints, utilization, memory, two clocks, fan 0, number of fans, throttle reasons, name) took 7.1 to 7.5 ms in debug and release alike, so the time is in the driver. The TUI's metric polling should run off the UI thread (inferred).
- Cost: 10 exclusive crates, including darling 0.20 (via `wrapcenum-derive`) and thiserror 1. Both show as duplicates beside ratatui's darling 0.24 and our thiserror 2 (verified: `cargo tree -d`, `-i`).

## YAML

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [serde-saphyr](https://crates.io/crates/serde-saphyr) | 2.0.0 | 2026-10-13 | MIT OR Apache-2.0 | 1.89 | **Pick** | Pure Rust, built on granit-parser. Its serializer options let it reproduce yaml.v3's layout: `indent_step: 4`, `compact_list_indent: false`, `prefer_block_scalars: false` (verified: `ser/options.rs:74-167`; probe gave byte-identical profile and overrides files). It accepts YAML 1.1 bools for typed bool targets, like yaml.v3. It rejects duplicate keys and tab indentation, like yaml.v3 (verified: probe and Go samples). 239 stars, 2 open, pushed 2026-10-25. **Churn risk:** 1.0.0 shipped 2026-07-31 and 2.0.0 shipped 2026-10-13. |
| [yaml_serde](https://crates.io/crates/yaml_serde) | 0.10.7 | 2026-08-18 | MIT OR Apache-2.0 | 1.82 | Alt | "serde_yaml maintained by The YAML Organization" (yaml/yaml-serde: 102 stars, 15 open). Uses [libyaml-rs](https://crates.io/crates/libyaml-rs) 0.3.0, which is libyaml translated by c2rust (verified: deps API). It round-trips all fixtures and Go reads its output (verified: probe). It has a fixed 2-space layout and rejects `smt: yes`. +3 crates. |
| [serde_norway](https://crates.io/crates/serde_norway) | 0.9.42 | 2024-12-21 | MIT OR Apache-2.0 | 1.71.1 | Alt | A serde_yaml fork on `unsafe-libyaml-norway`, recommended by RUSTSEC-2025-0068. Same results as yaml_serde. Last release 2024-12. |
| [serde_yaml](https://crates.io/crates/serde_yaml) | 0.9.34+deprecated | 2024-03-25 | MIT OR Apache-2.0 | 1.64 | Avoid | Deprecated, and the repo is archived (verified: GitHub API). |
| [serde_yml](https://crates.io/crates/serde_yml) | 0.0.13 | 2026-05-27 | MIT OR Apache-2.0 | 1.85.0 | Avoid | RUSTSEC-2025-0068 (unsound, unmaintained). Now a shim that forwards to noyalib. Repo archived. |
| [libyml](https://crates.io/crates/libyml) | 0.0.6 | 2026-05-29 | MIT OR Apache-2.0 | 1.56.0 | Avoid | RUSTSEC-2025-0067 (unsound). Deprecated shim. |
| [unsafe-libyaml](https://crates.io/crates/unsafe-libyaml) | 0.2.11 | 2024-03-17 | MIT | 1.56 | Avoid | RUSTSEC-2024-0449 (unmaintained). Archived. Used by serde_yaml_ng. |
| [serde_yaml_ng](https://crates.io/crates/serde_yaml_ng) | 0.10.0 | 2024-05-26 | MIT | 1.64 | Avoid | Sits on unsafe-libyaml. Last release 2024. |
| [serde_yaml_bw](https://crates.io/crates/serde_yaml_bw) | 2.5.8 | 2026-09-07 | MIT OR Apache-2.0 | - | Alt | Same author as serde-saphyr. Depends on both granit-parser and unsafe-libyaml-norway, plus regex (verified: deps API). |
| [noyalib](https://crates.io/crates/noyalib) | 0.0.58 | 2026-10-15 | MIT OR Apache-2.0 | 1.89.0 | Avoid | Pre-0.1, 11 stars, 12 open. From the serde_yml author. Has RUSTSEC-2026-0333 (DoS budgets, fixed in 0.0.53). |
| [saphyr](https://crates.io/crates/saphyr) / [saphyr-parser](https://crates.io/crates/saphyr-parser) | 0.2.2 | 2026-10-15 | MIT OR Apache-2.0 | 1.85.0 | Ref | Untyped YAML 1.2 library (339 stars). No serde. |
| [yaml-rust2](https://crates.io/crates/yaml-rust2) | 0.13.0 | 2026-09-11 | MIT OR Apache-2.0 | 1.85.0 | Ref | Maintained fork of yaml-rust. No serde. |
| [yaml-rust](https://crates.io/crates/yaml-rust) | 0.4.5 | 2021-01-03 | MIT/Apache-2.0 | - | Avoid | RUSTSEC-2024-0320 (unmaintained). |
| [yaml-edit](https://crates.io/crates/yaml-edit) | 0.3.5 | 2026-10-22 | Apache-2.0 | 1.70 | Later | Lossless editor (rowan). Only needed if spela ever has to keep user comments. |

Round-trip results (verified: probe; fixtures generated by yaml.v3 v3.0.1 from spela's structs):

| File | serde-saphyr (tuned) | serde-saphyr (`yaml_12`, `no_lang_directive`) | yaml_serde | serde_norway | Go reads the output back |
|---|---|---|---|---|---|
| `profile.yaml` (nested sections, `smt: false`, overrides map) | byte-identical | byte-identical | equal value, 2-space | equal value, 2-space | all equal |
| `profile_empty.yaml` (`overrides: {}`) | byte-identical | byte-identical | byte-identical | byte-identical | all equal |
| `games.yaml` (u64 keys, times, sequences) | quotes differ only | quotes differ only | layout differs | layout differs | all equal |
| `config.yaml` | quote style differs | quote style differs | layout differs | layout differs | all equal |
| `overrides.yaml`, `overrides_empty.yaml` (`allowed: []`) | byte-identical | byte-identical | layout differs | layout differs | all equal |

What spela relies on, and how the Rust side must match it:

- yaml.v3 writes 4-space indentation and indents sequences under their key. It sorts `u64` map keys numerically, writes empty maps as `{}` and nil slices without `omitempty` as `[]`. It writes `time.Time` unquoted as RFC 3339 with nanoseconds, and the zero time as `0001-01-01T00:00:00Z` (verified: fixtures). serde-saphyr quotes timestamps and `310.5.3`. yaml.v3 still decodes a quoted timestamp into `time.Time` (verified: `goref check`).
- **Legacy profile detection:** `Overrides` is nil when the key is missing, so the profile is migrated (verified: `internal/profile/storage.go:52-62`, `internal/profile/profile.go:40-48`). Saving always writes `overrides: {}` (verified: `internal/profile/reset_persistence_test.go:151-166`). In Rust, use `Option<BTreeMap<String, bool>>` with a `serialize_with` that writes `{}` for `None` (verified: probe).
- **Config defaults:** missing keys keep their defaults (verified: `internal/config/config.go:76-79`). Use `#[serde(default)]` on a struct whose `Default` returns spela's defaults (verified: probe).
- **Struct `omitempty`:** omitting all-zero sections needs `skip_serializing_if = "is_default"` (verified: probe).
- **No comment preservation today:** every save re-marshals the whole struct (verified: `internal/profile/storage.go:96`, `internal/game/database.go:113`, `internal/config/config.go:89`, `internal/denylist/denylist.go:101`). So comment-preserving crates are not needed for parity.
- **Trap:** serde-saphyr's default `prefer_block_scalars: true` folds strings over 80 characters into `>` blocks at spaces, which hits long install paths (verified: probe).

## JSON

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [serde](https://crates.io/crates/serde) / [serde_json](https://crates.io/crates/serde_json) | 1.0.229 / 1.0.154 | 2026-07-18 / 2026-10-11 | MIT OR Apache-2.0 | 1.56 / 1.71 | **Pick** | Manifest, backups, update-check and `--json` output. |

Parity notes (verified: probe compared with Go fixtures):

- The `--json` outputs marshal structs that only carry yaml tags, so the keys are Go field names: `AppID`, `DLLs`, `ScannedAt`, `SRMode`, `VKD3DHeap`, `SMT` (verified: `internal/game/game.go:18-33`, `internal/profile/profile.go:31-99`, fixtures). Every field is always present, and nil slices print as `null`.
- Go escapes `<`, `>`, `&`, U+2028 and U+2029, and sorts `map[uint64]` keys as strings (`"1091500"` before `"252490"`).
- A serde_json `Formatter` that overrides `write_string_fragment`, used with explicit `#[serde(rename = "AppID")]` fields and `BTreeMap<String, _>`, gives byte-identical `scan.json`. Plain `to_string_pretty` does not.

## CLI

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [clap](https://crates.io/crates/clap) | 4.6.7 | 2026-09-14 | MIT OR Apache-2.0 | 1.85 | **Pick** | Derive API. `Option<T>` reproduces cobra's `Flags().Changed()` (verified: probe). Hidden `apply-profile` stays out of help (verified: probe). |
| [clap_complete](https://crates.io/crates/clap_complete) | 4.6.11 | 2026-09-15 | MIT OR Apache-2.0 | 1.85 | **Pick** | cobra adds a `completion` command by default (verified: Go cache `cobra@v1.10.2/completions.go:745-746`, `command.go:1113`), and spela never disables it (verified: grep, no `CompletionOptions`). Generated scripts in the probe: bash 9,286 B, zsh 6,608 B, fish 3,504 B. +1 crate. |
| [anstyle](https://crates.io/crates/anstyle) / [anstream](https://crates.io/crates/anstream) | 1.0.14 / 1.0.0 | 2026-03-13 / 2026-02-11 | MIT OR Apache-2.0 | 1.66.0 | **Pick** | Already in clap's tree (0 extra crates, measured). anstream strips color when stdout is not a terminal, which changes output compared with lipgloss (see conflicts). |
| [clap_mangen](https://crates.io/crates/clap_mangen) | 0.3.3 | 2026-08-12 | MIT OR Apache-2.0 | 1.85 | Later | Man pages for packaging. |
| [owo-colors](https://crates.io/crates/owo-colors) | 4.4.0 | 2026-08-27 | MIT | 1.83 | Alt | Fine, but anstyle comes free with clap. |
| [bpaf](https://crates.io/crates/bpaf) / [argh](https://crates.io/crates/argh) / [lexopt](https://crates.io/crates/lexopt) / [pico-args](https://crates.io/crates/pico-args) | 0.9.28 / 0.1.19 / 0.3.2 / 0.5.0 | 2026-09-17 / 2026-03-16 / 2026-02-28 / 2022-06-04 | MIT OR Apache-2.0 / BSD-3-Clause / MIT / MIT | 1.68 / - / - / - | Avoid | Smaller, but clap matches mejl and cobra's feature set. |

CLI traps (verified):

- pflag takes the next argument as a flag's value even when it starts with `-` (verified: Go cache `pflag@v1.0.10/flag.go:1010-1013`). So `spela gpu set X --clock-offset -100` works in Go. clap rejects it ("unexpected argument") unless the flag sets `allow_negative_numbers = true` (verified: probe).
- Wrapper mode (`spela %command%`) must be detected before clap parses, as `cmd/spela/main.go:121-127` does. See the PATH collision in conflicts.

## Steam VDF and KeyValues

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [keyvalues-parser](https://crates.io/crates/keyvalues-parser) | 0.2.4 | 2026-05-17 | MIT OR Apache-2.0 | 1.81.0 | **Pick** | pest grammar. `Obj` is `BTreeMap<Key, Vec<Value>>`, so duplicate keys keep every value; take `.last()` to match Go's map (verified: `src/lib.rs:73`). Supports `literal_special_chars` for raw backslashes (verified: `src/lib.rs:32-63`). On this machine it parsed the real `libraryfolders.vdf` (2 libraries), all 59 real `appmanifest_*.acf` files and the 105 KiB `config.vdf` in 24 to 31 ms (verified: probe). +5 crates. Hosted on Codeberg, so GitHub signals were not checked. |
| [keyvalues-serde](https://crates.io/crates/keyvalues-serde) | 0.2.4 | 2026-05-18 | MIT OR Apache-2.0 | 1.81.0 | Later | Typed serde on top of the parser. Fine for appmanifest, but spela walks nested maps by key (verified: `internal/proton/resolver.go:142-147`). |
| [steamlocate](https://crates.io/crates/steamlocate) | 2.1.1 | 2026-08-13 | MIT | 1.83.0 | Alt | Locates Steam, libraries and apps. It found 2 libraries and 59 apps here (verified: probe). Its `app_id` is `u32` while spela uses `u64` (verified: `src/app.rs:117`). Spela's own root candidates include the Flatpak path (verified: `internal/steam/library.go:18-32`). +4 crates on top of keyvalues-parser. |
| [vdf-reader](https://crates.io/crates/vdf-reader) | 0.3.4 | 2026-07-11 | MIT | 1.86.0 | Alt | logos plus miette. It accepted the real files, but its 2 µs parse of a 105 KiB file with `IgnoredAny` suggests lazy or partial parsing (unverified). |
| [steam-vdf-parser](https://crates.io/crates/steam-vdf-parser) | 0.1.2 | 2026-07-14 | MIT OR Apache-2.0 | 1.88 | Alt | Zero-copy winnow parser for binary and text VDF. 8 stars. |
| [new-vdf-parser](https://crates.io/crates/new-vdf-parser) | 0.2.0 | 2025-10-13 | LGPL-2.1 | 1.79 | Avoid (license) | LGPL. Binary appinfo and shortcuts formats spela does not read. |
| [valve-keyvalue](https://crates.io/crates/valve-keyvalue) | 1.1.0 | 2026-09-22 | Apache-2.0 | - | Avoid | 71 downloads, self-hosted git. |
| [vdf-serde](https://crates.io/crates/vdf-serde) / [steamy-vdf](https://crates.io/crates/steamy-vdf) | 0.3.0 / 0.2.0 | 2020-08-31 / 2016-05-01 | MIT OR Apache-2.0 / WTFPL | - | Avoid | Stale. |

Behaviour differences from spela's parser (verified: probe and `internal/steam/vdf.go`):

- **Escapes:** keyvalues-parser turns `\\` into `\` by default. The Go tokenizer keeps backslashes (`vdf.go:61-79`).
- **Trailing comments:** a pair followed by `// comment` is kept by keyvalues-parser. Go gets more than two tokens and silently drops the pair (`vdf.go:41-46`).
- **Brace placement:** `"key" {` on one line parses with keyvalues-parser. Go needs `{` alone on its line (`vdf.go:25`).
- **Duplicate keys:** Go keeps the last value; keyvalues-parser keeps all of them.

## PE version resources

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [object](https://crates.io/crates/object) | 0.40.0 | 2026-08-01 | Apache-2.0 OR MIT | 1.85 | **Pick** | With `default-features = false, features = ["read_core", "pe", "std"]` it costs +1 crate (memchr is already present) (verified: tree cost). It has no `VS_VERSIONINFO` parser, so port Go's scan: find `.rsrc`, search for UTF-16 `VS_VERSION_INFO`, then `0xFEEF04BD` (verified: `internal/dll/detect.go:92-152`). That is about 25 lines and gives exact parity. gimli-rs, 859 stars. |
| [goblin](https://crates.io/crates/goblin) | 0.10.7 | 2026-05-28 | MIT | 1.85.0 | Alt | Parses `resource_data.version_info.fixed_info.file_version_ms/ls` (verified: `src/pe/resource.rs:936-948`, `src/pe/mod.rs:92`). Slower because it parses the whole PE (21.6 ms for 1,224 files). +4 crates. |
| [pelite](https://crates.io/crates/pelite) | 0.10.0 | 2022-11-04 | MIT | - | Alt | Walks the resource directory: `resources()?.version_info()?.fixed()?.dwFileVersion` (verified: `src/resources/find.rs:98`, `version_info.rs:123`). Fastest (0.3 ms for 1,224 files). The last release is from 2022, though the repo was pushed 2026-10-14 with 0 open. +5 crates. |
| [editpe](https://crates.io/crates/editpe) | 0.2.4 | 2026-08-12 | BSD-2-Clause | - | Avoid | A resource editor, more than needed. |
| [exe](https://crates.io/crates/exe) | 0.5.7 | 2026-04-19 | GPL-3.0 | - | Avoid (license) | GPL. |
| [pe-parser](https://crates.io/crates/pe-parser) / [pers](https://crates.io/crates/pers) | 0.7.0 / 0.1.0 | 2025-03-12 / 2026-10-22 | MIT / MIT OR Apache-2.0 | - | Avoid | Small or brand new (pers has 8 downloads). |

Agreement (verified: probe `compare pe`): all three methods (pelite, goblin, and the Go-style scan on object) agree on 1,224 real DLLs from spela's DLL cache and the Wine x86_64 and i386 folders, 412 of them with a version. The cached DLSS DLLs read as `310.5.3` (53 MiB file) and `310.5` (dlssg 310.5.0), matching Go's formatting rules (`internal/dll/detect.go:154-162`).

## Archives

Not needed at runtime. The manifest points at raw `.dll` files (verified: `data/manifest.json`, `internal/dll/download.go:58-86`). Only a release contract test reads a tar (verified: `tests/contracts/release_contract_test.go:4`). If that test is ported: [tar](https://crates.io/crates/tar) 0.4.46 (2026-05-18, MIT OR Apache-2.0, MSRV 1.63; RUSTSEC-2026-0067 and -0068 exist, check them against the version used) and [flate2](https://crates.io/crates/flate2) 1.1.10 (2026-08-28). [zip](https://crates.io/crates/zip) 9.1.0 (2026-10-24, MIT, MSRV 1.88) has four advisories, the latest RUSTSEC-2026-0369. Avoid unless needed.

## Hashing and hex

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [sha2](https://crates.io/crates/sha2) | 0.11.1 | 2026-10-12 | MIT OR Apache-2.0 | 1.88 | **Pick** | **Trap:** in 0.11 the output is a hybrid-array `Array` that no longer implements `LowerHex`, so `format!("{:x}", digest)` stops compiling. The probe hit error E0277 and needed a small hex helper. 0.10.9 (2025-04-30) still works with `{:x}`. mejl pins `sha2 = "0.10"` (verified: `/home/jgabor/git/mejl/Cargo.toml:72`). +8 crates. |
| [hex](https://crates.io/crates/hex) / [const-hex](https://crates.io/crates/const-hex) | 0.4.3 / 1.19.3 | 2021-03-03 / 2026-09-16 | MIT OR Apache-2.0 | - / 1.64 | Alt | +1 crate. A 5-line helper covers spela's two uses (encode, and validate 32-byte hex at `internal/dll/operations.go:364-370`). |

## HTTP and TLS

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [ureq](https://crates.io/crates/ureq) | 3.5.0 | 2026-10-21 | MIT OR Apache-2.0 | 1.85 | **Pick** | Blocking, so it fits a worker thread next to mejl's plain-loop UI thread. Its defaults are rustls with **ring** and webpki roots, plus gzip (verified: features API). Timeouts map onto Go's: `timeout_recv_response(30 s)` for the response header and `timeout_global(120 s)` for the whole call (verified: `src/config.rs:761`, `908`). Streams the body to a hasher and a tempfile; 8 MiB took 139 ms over loopback (verified: probe). Re-exports `http` (verified: `src/lib.rs:583`), so `ureq::http::Uri` handles manifest URL validation. +20 crates. |
| [reqwest](https://crates.io/crates/reqwest) + [tokio](https://crates.io/crates/tokio) | 0.13.5 + 1.54.1 | 2026-09-08 / 2026-10-21 | MIT OR Apache-2.0 / MIT | 1.85.0 / 1.85 | Alt | mejl's stack. Measured instead of ureq: 238 crates against 211, a 31.0 s clean release against 20.2 s, and a 10.0 MB binary against 6.7 MB (first probe). Worth it only if spela grows async work or shares code with mejl (inferred). |
| [url](https://crates.io/crates/url) | 2.5.8 | 2026-01-05 | MIT OR Apache-2.0 | 1.63 | Avoid (cost) | Costs 26 exclusive crates (idna plus the ICU stack) just to validate one URL (verified: tree cost). Use `ureq::http::Uri` (verified: probe accepts and rejects the same five cases as `internal/config/paths.go:63-77`). |
| [attohttpc](https://crates.io/crates/attohttpc) / [minreq](https://crates.io/crates/minreq) | 0.31.0 / 3.0.0 | 2026-05-25 / 2026-06-15 | MPL-2.0 / ISC | - / 1.63 | Avoid | ureq is more widely used. attohttpc is MPL. |

TLS and proxy notes (verified: ureq source):

- ureq's default root certificates are `RootCerts::WebPki`, a bundled Mozilla list (`src/tls/mod.rs:370-375`). Go uses the system pool (inferred: Go stdlib). For parity, enable the `platform-verifier` feature and set `TlsConfig::builder().root_certs(RootCerts::PlatformVerifier)`. This compiled in the final probe (verified), but TLS was not exercised at runtime (unverified).
- ureq reads proxy settings from the environment by default (`src/config.rs:1073`). Go's DLL client builds a custom `http.Transport` without `Proxy` (verified: `internal/dll/manifest.go:22-27`), so it ignores `HTTPS_PROXY`. The release check uses the default transport (`internal/release/release.go:39`), which honours it (inferred: Go stdlib).

## Processes, signals and privilege

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [rustix](https://crates.io/crates/rustix) | 1.1.5 | 2026-09-16 | Apache-2.0 WITH LLVM-exception OR Apache-2.0 OR MIT | 1.65 | **Pick** | Features `process`, `fs`: `geteuid().is_root()`, `kill_process`, `test_kill_process` (signal 0), `Pid::from_child` (verified: `src/process/id.rs:53`, `kill.rs:16`, `:65`; probe). Already in crossterm's tree (0 extra crates, measured). |
| [signal-hook](https://crates.io/crates/signal-hook) | 0.3.18 (0.4.5 is latest) | 2025-05-09 | MIT OR Apache-2.0 | - | **Pick 0.3** | crossterm 0.29 depends on `signal-hook ^0.3.17` (verified: deps API). 0.4.5 would add a second copy (verified: `cargo tree -d`). mejl also pins `"0.3"` (`/home/jgabor/git/mejl/Cargo.toml:76`). The probe forwarded SIGTERM to a child (`sleep 30`), which exited by signal 15 (verified). |
| [wait-timeout](https://crates.io/crates/wait-timeout) | 0.2.1 | 2025-02-03 | MIT/Apache-2.0 | - | **Pick** | `ChildExt::wait_timeout` gives `kscreen-doctor` its 10 s limit (`internal/display/vrr.go:25`). +1 crate. Also comes in through assert_cmd. |
| [which](https://crates.io/crates/which) | 8.0.6 | 2026-08-26 | MIT | 1.70 | **Pick** | `exec.LookPath` equivalent for pkexec, scx_loader and wrapper mode. +1 crate. |
| [nix](https://crates.io/crates/nix) | 0.31.3 | 2026-05-11 | MIT | 1.69 | Alt | Fine, but rustix is already in the tree. |
| [libc](https://crates.io/crates/libc) | 0.2.191 | 2026-10-16 | MIT OR Apache-2.0 | 1.65 | Alt | mejl uses raw libc for `flock` and `raise` (verified: `/home/jgabor/git/mejl/crates/mejl/src/lock.rs:153`, `terminal/mod.rs:263`). rustix and std cover spela's needs safely. |
| [ctrlc](https://crates.io/crates/ctrlc) | 3.5.2 | 2026-02-10 | MIT/Apache-2.0 | 1.69.0 | Avoid | SIGINT only. |
| [process-wrap](https://crates.io/crates/process-wrap) | 10.0.1 | 2026-09-23 | Apache-2.0 OR MIT | 1.87.0 | Later | Process groups, if spela ever signals a game's whole process tree. Go signals only the direct child (`internal/launcher/launcher.go:243-244`). |

## Shared memory (overlay IPC)

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [memmap2](https://crates.io/crates/memmap2) | 0.9.12 | 2026-10-21 | MIT OR Apache-2.0 | 1.65 | **Pick** | `MmapMut::map_mut` on an 80-byte file. RUSTSEC-2026-0186 (unchecked ranges in `advise_range` and `flush_range`) is patched in ≥0.9.11 (verified: advisory). |
| [mmap-rs](https://crates.io/crates/mmap-rs) / [shared_memory](https://crates.io/crates/shared_memory) | 0.7.0 / 0.12.4 | 2025-09-25 / 2022-03-01 | Apache-2.0 OR MIT / MIT OR Apache-2.0 | 1.85 / - | Avoid | Heavier or stale (shared_memory pulls nix 0.23 and rand 0.8). |
| [seqlock](https://crates.io/crates/seqlock) | 0.2.0 | 2023-03-30 | Apache-2.0/MIT | - | Avoid | In-process type. The wire layout here is fixed by spela, so a hand-written `AtomicU32` seqlock is needed. |

Probe (verified): the Rust writer did seq store (Relaxed), a Release fence, the payload copy, then seq store (Release), 1,000 times in 0.5 ms. A Go reader copied from `internal/overlay/ipc.go:172-206` then decoded `seq=2000/2000 temp=61 power=250.500 fan=41 alert=1 severity=2 visible=1 position=3`. The Go writer stores the seq with `atomic.StoreUint32` and the payload with plain writes (`internal/overlay/ipc.go:131-167`). The Rust side should keep the fence ordering shown (inferred).

## File locking and single instance

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| std `File::lock`, `try_lock`, `unlock` | Rust 1.89 | - | - | 1.89 | **Pick** | `#[stable(feature = "file_lock", since = "1.89.0")]` (verified: rust-src `library/std/src/fs.rs:858-1077`). Backed by `flock(LOCK_EX)` and friends on Unix (verified: `library/std/src/sys/fs/unix.rs:1336-1454`). The probe showed a second handle getting `WouldBlock` (verified). Replaces `syscall.Flock` at `internal/game/database.go:65` and `internal/profile/storage.go:230`. |
| [fs4](https://crates.io/crates/fs4) | 1.1.0 | 2026-04-28 | MIT OR Apache-2.0 | 1.75.0 | Alt | Only needed for an MSRV below 1.89. +1 crate. |
| [fd-lock](https://crates.io/crates/fd-lock) | 4.0.4 | 2025-03-10 | MIT OR Apache-2.0 | - | Alt | Its RAII guard style is a matter of taste. |
| [fs2](https://crates.io/crates/fs2) / [fslock](https://crates.io/crates/fslock) / [file-guard](https://crates.io/crates/file-guard) | 0.4.3 / 0.2.1 / 0.2.0 | 2018 / 2021 / 2024 | MIT/Apache-2.0 / MIT / MIT | - | Avoid | Stale. |

Spela's single-instance lock is a PID file plus `kill(pid, 0)` (verified: `internal/lock/lock.go:35-93`). mejl uses a non-blocking `flock` instead (verified: `/home/jgabor/git/mejl/crates/mejl/src/lock.rs:153-160`), which avoids PID reuse. Keeping the "already running (PID: N)" message would mean writing the PID into the locked file (inferred).

## procfs and sysfs

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| std fs | - | - | - | - | **Pick** | A full `/proc` scan (647 processes, `readlink` of cwd and exe, cmdline) took 4.5 ms (verified: probe). sysfs governor and SMT files read fine. |
| [procfs](https://crates.io/crates/procfs) | 0.18.0 | 2025-08-30 | MIT OR Apache-2.0 | 1.70 | Alt | Typed procfs. +7 crates. Not needed. |
| [sysinfo](https://crates.io/crates/sysinfo) | 0.39.6 | 2026-07-09 | MIT | 1.95 | Avoid | MSRV 1.95, heavy, and spela reads specific files. |

`runtime.NumCPU()` is used to count sysfs CPU folders (verified: `internal/cpu/cpu.go:35-37`, `:91-99`). `std::thread::available_parallelism()` returned 16, matching the 16 `cpuN` folders here (verified: probe). It also respects cgroup quotas, which `cpu%d` iteration would not need (inferred).

## XDG paths

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| hand-written module | - | - | - | - | **Pick** | Mirrors `internal/xdg/xdg.go:10-62` exactly. Any non-empty `XDG_*` value is used even when relative (`:11`), and the runtime directory falls back to `/tmp/runtime-$USER/spela` (`:35-39`). Adopt mejl's `Paths` struct with `Paths::under(base)` for tests (verified: `/home/jgabor/git/mejl/crates/mejl/src/paths.rs:40-90`). |
| [etcetera](https://crates.io/crates/etcetera) | 0.11.0 | 2025-10-28 | MIT OR Apache-2.0 | 1.87.0 | Alt | mejl's choice. It ignores relative `XDG_*` values (verified: `src/base_strategy/xdg.rs:175-181`) and returns `None` for a missing runtime dir (`:210-212`). Probe with `XDG_CONFIG_HOME=relative/cfg` and an empty `XDG_RUNTIME_DIR`: spela gave `relative/cfg/spela` and `/tmp/runtime-jgabor/spela`; etcetera gave `~/.config/spela` and `None` (verified). |
| [directories](https://crates.io/crates/directories) / [dirs](https://crates.io/crates/dirs) / [xdg](https://crates.io/crates/xdg) / [home](https://crates.io/crates/home) | 6.0.0 / 7.0.0 / 3.0.0 / 0.5.12 | 2025-01-12 / 2026-09-05 / 2025-05-04 / 2025-10-23 | MIT OR Apache-2.0 | - / - / 1.60.0 / 1.88 | Avoid | `directories` is archived. The others add nothing over a 60-line module. |

## Versions

The `semver` crate is not needed and would change behaviour. Spela compares dotted integers of any length, with `v` stripped and bad parts read as 0 (verified: `internal/dll/version.go:11-47`). DLSS versions such as `310.5` are not valid semver. The release check uses `^v?\d+\.\d+\.\d+$` (verified: `internal/release/release.go:41`). [semver](https://crates.io/crates/semver) 1.0.28 (2026-04-04) and [version-compare](https://crates.io/crates/version-compare) 0.2.1 (2025-10-31): Avoid.

## Logging

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [tracing](https://crates.io/crates/tracing) | 0.1.44 | 2025-12-18 | MIT | 1.65.0 | **Pick** | Same as mejl. |
| [tracing-subscriber](https://crates.io/crates/tracing-subscriber) | 0.3.23 | 2026-03-13 | MIT | 1.65.0 | **Pick** | Writes to stderr with a `LevelFilter` built from `log_level`, matching `internal/logging/logging.go:16-38`. **Trap:** the fmt layer emitted ANSI codes to a redirected stderr (verified: probe), so set `.with_ansi(stderr.is_terminal())`. The record format differs from slog's `key=value` text handler (inferred). |
| [tracing-appender](https://crates.io/crates/tracing-appender) | 0.2.5 | 2026-04-17 | MIT | 1.63.0 | Later | mejl writes daily rolling log files (verified: `/home/jgabor/git/mejl/crates/mejl/src/logging.rs:265-271`). Spela logs to stderr even while the TUI owns the terminal (verified: `internal/logging/logging.go:16`). Brings `time` 0.3 (already present through ratatui). |
| [log](https://crates.io/crates/log) / [env_logger](https://crates.io/crates/env_logger) | 0.4.34 / 0.11.11 | 2026-08-22 / 2026-06-25 | MIT OR Apache-2.0 | 1.71 | Avoid | Use tracing, to match mejl. |

## Time

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [jiff](https://crates.io/crates/jiff) | 0.2.38 | 2026-10-06 | Unlicense OR MIT | 1.70 | **Pick** | `fmt::temporal::Pieces::parse` gives the offset and `Timestamp` gives the instant. `display_with_offset` reproduces Go's RFC3339Nano output exactly for `+02:00`, `Z`, the zero time and trimmed fractions (verified: probe). **Trap:** `Zoned` parsing needs a bracketed time zone annotation, and `Zoned` prints one, so it does not suit Go's offset-only format (inferred from jiff docs). Default features are enough; the `serde` feature is not needed with a custom wrapper type. +2 crates. |
| [chrono](https://crates.io/crates/chrono) / [time](https://crates.io/crates/time) / [humantime](https://crates.io/crates/humantime) | 0.4.45 / 0.3.55 / 2.4.0 | 2026-06-04 / 2026-08-01 / 2026-07-02 | MIT OR Apache-2.0 | 1.62.0 / 1.88.0 / 1.60 | Avoid | jiff matches mejl. `time` comes in through ratatui anyway. |

## Atomic file writes

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [tempfile](https://crates.io/crates/tempfile) | 3.27.0 | 2026-03-11 | MIT OR Apache-2.0 | 1.63 | **Pick** | `NamedTempFile::new_in(dir)`, `set_permissions`, optional `sync_all`, then `persist` matches Go's create, chmod, write, close and rename (verified: probe). Go never fsyncs (verified: `internal/dll/swap.go:283-302`). +3 crates. |
| [atomic-write-file](https://crates.io/crates/atomic-write-file) | 0.3.1 | 2026-08-11 | BSD-3-Clause | 1.85 | Alt | Fsyncs the file and its directory. Pulls nix 0.31 and rand 0.10. +6 crates. |
| [atomicwrites](https://crates.io/crates/atomicwrites) | 0.4.4 | 2024-09-19 | MIT | - | Avoid | Last release 2024. |

## D-Bus

[zbus](https://crates.io/crates/zbus) 6.1.0 (2026-10-23, MIT, MSRV 1.89): **Avoid**. Nothing in spela uses D-Bus. VRR shells out to `kscreen-doctor` (verified: `internal/display/vrr.go:27`), and polkit is reached through the `pkexec` binary (verified: `internal/privilege/privilege.go:53`). KScreen's D-Bus API could replace parsing `kscreen-doctor -o` output later (inferred).

## Regex, PATH lookup and directory walking

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [regex-lite](https://crates.io/crates/regex-lite) | 0.1.9 | 2026-02-03 | MIT OR Apache-2.0 | 1.65 | **Pick** | All of spela's patterns are ASCII, `(?i)`, `\s`, `\d` and anchors (verified: `internal/game/database.go:23-29`, `internal/display/vrr.go:121-124`, `internal/proton/build_version.go:8`, `internal/release/release.go:41`). +1 crate. |
| [regex](https://crates.io/crates/regex) | 1.13.1 | 2026-07-15 | MIT OR Apache-2.0 | 1.65 | Alt | Only if Unicode classes are needed. |
| [walkdir](https://crates.io/crates/walkdir) | 2.5.0 | 2024-03-01 | Unlicense/MIT | - | **Pick** | Matches `filepath.WalkDir`: symlinks are not followed by default (inferred). +2 crates. |
| [glob](https://crates.io/crates/glob) | 0.3.4 | 2026-07-21 | MIT OR Apache-2.0 | 1.63.0 | Avoid | `read_dir` plus a prefix and suffix filter replaces `filepath.Glob("appmanifest_*.acf")` (verified: probe). |

## Terminal (re-check of mejl's pins)

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [ratatui](https://crates.io/crates/ratatui) | 0.30.2 | 2026-06-19 | MIT | 1.88.0 | **Pick** | Still the latest stable, the same pin as mejl (verified: crates.io API; `/home/jgabor/git/mejl/Cargo.toml:24`). 23,083 stars, pushed 2026-10-24. |
| [crossterm](https://crates.io/crates/crossterm) | 0.29.0 | 2025-04-05 | MIT | 1.63.0 | **Pick** | Still the latest, the same pin as mejl (`Cargo.toml:25`). 4,251 stars, 255 open, pushed 2026-10-25. Already carries rustix 1 and signal-hook 0.3 (verified: deps API). |
| [tui-input](https://crates.io/crates/tui-input) | 0.15.5 | 2026-09-26 | MIT | - | Later | mejl's research picked it, but mejl as built does not list it (verified: `/home/jgabor/git/mejl/Cargo.toml`). This is a UI-scope decision. |

A ratatui `TestBackend` snapshot with insta 1.49.0 passed in the probe (verified).

## Testing

| Crate | Version | Released | License | MSRV | Verdict | Why |
|---|---|---|---|---|---|---|
| [insta](https://crates.io/crates/insta) | 1.49.0 | 2026-10-03 | Apache-2.0 | 1.66.0 | **Pick (dev)** | TestBackend snapshots (verified: probe), YAML and JSON golden output. |
| [tempfile](https://crates.io/crates/tempfile) | 3.27.0 | 2026-03-11 | MIT OR Apache-2.0 | 1.63 | **Pick (dev)** | Isolated XDG roots. |
| [assert_cmd](https://crates.io/crates/assert_cmd) | 2.2.2 | 2026-05-11 | MIT OR Apache-2.0 | 1.85 | **Pick (dev)** | Runs the binary with a per-child env. The Go tests call `t.Setenv` 162 times in 47 files (verified: grep), and `std::env::set_var` is `unsafe` (verified: rust-src `library/std/src/env.rs:359`). A probe test isolated `XDG_CONFIG_HOME` and `XDG_RUNTIME_DIR` this way (verified). |
| [predicates](https://crates.io/crates/predicates) | 3.1.4 | 2026-02-11 | MIT OR Apache-2.0 | 1.74 | Later (dev) | Assertions for assert_cmd. |
| [vt100](https://crates.io/crates/vt100) | 0.18.1 | 2026-10-19 | MIT | 1.85 | Later (dev) | Parse the screen of an in-process PTY. AGENTS.md mandates Terminal Control (`termctrl`) for TUI review, and `tests/e2e/terminal_client.go:28-51` drives it, which works unchanged for a Rust binary (verified). |
| [rexpect](https://crates.io/crates/rexpect) | 0.7.1 | 2026-05-14 | MIT OR Apache-2.0 | 1.85.0 | Alt (dev) | Current nix ^0.31 (verified: deps API). |
| [expectrl](https://crates.io/crates/expectrl) | 0.9.0 | 2026-05-11 | MIT | - | Avoid | nix ^0.26 would add a second nix. |
| [portable-pty](https://crates.io/crates/portable-pty) | 0.9.0 | 2025-02-11 | MIT | - | Avoid | nix ^0.28, bitflags 1, lazy_static, winreg (verified: deps API). |
| [proptest](https://crates.io/crates/proptest) | 1.12.0 | 2026-10-10 | MIT OR Apache-2.0 | 1.88 | Later (dev) | Fuzzing version compare, VDF and the wrapper argument parser. |
| [serial_test](https://crates.io/crates/serial_test) | 4.0.1 | 2026-07-25 | MIT | 1.93.1 | Avoid | MSRV 1.93.1. Inject paths instead. |
| [mockito](https://crates.io/crates/mockito) / [httpmock](https://crates.io/crates/httpmock) / [wiremock](https://crates.io/crates/wiremock) / [tiny_http](https://crates.io/crates/tiny_http) | 1.7.2 / 0.9.0 / 0.6.5 / 0.12.0 | 2026-02-02 / 2026-10-20 / 2025-08-24 / 2022-10-06 | MIT / MIT / MIT/Apache-2.0 / MIT OR Apache-2.0 | 1.70 / 1.88.0 / - / - | Alt | Go uses `httptest` in 10 test files (verified: grep). A `std::net::TcpListener` thread was enough in the probe. mockito, httpmock and wiremock pull in tokio and hyper. tiny_http has RUSTSEC-2020-0031. |
| [assert_fs](https://crates.io/crates/assert_fs) / [trycmd](https://crates.io/crates/trycmd) / [snapbox](https://crates.io/crates/snapbox) / [rstest](https://crates.io/crates/rstest) | 1.1.4 / 1.2.1 / 1.2.2 / 0.28.0 | 2026-05-26 / 2026-07-21 / 2026-05-26 / 2026-10-18 | MIT OR Apache-2.0 | 1.85 | Later | trycmd fits CLI transcript tests of `--json` output. |
| [divan](https://crates.io/crates/divan) | 0.1.21 | 2025-04-10 | MIT OR Apache-2.0 | 1.80.0 | Later (dev) | Benchmarks, as in mejl. |

## Version and license traps

| Trap | Effect | What to do |
|---|---|---|
| sha2 0.11 output has no `LowerHex` (verified: compile error E0277) | `format!("{:x}", digest)` breaks. mejl pins 0.10 | Use a hex helper, or pin 0.10.9 to match mejl |
| signal-hook 0.4.5 next to crossterm's `^0.3.17` | A second signal-hook in the tree | Pin `"0.3"` like mejl |
| nvml-wrapper 0.13.0 needs libloading ^0.8, thiserror ^1, darling 0.20 | Duplicates thiserror 1/2 and darling 0.20/0.24 | Accept (small) |
| ureq's ring brings getrandom 0.2; tempfile brings 0.4 | Duplicate getrandom | Accept |
| clap derive (syn 3) next to other proc-macros on syn 2 | Duplicate syn | Accept |
| `url` 2.5.8 costs 26 crates (idna, ICU) | Big tree for one validation | Use `ureq::http::Uri` |
| reqwest 0.13 `rustls` uses aws-lc-rs (from mejl's doc) | C build step. Measured +12.5 s clean, +3.3 MB | Prefer ureq (ring) |
| ureq defaults to webpki roots | Corporate or custom CAs fail where Go succeeds | `platform-verifier` plus `RootCerts::PlatformVerifier` |
| serde-saphyr defaults: 2-space indent, folds long strings | Output differs from yaml.v3. Long paths become `>` blocks | `ser_options!{indent_step: 4, compact_list_indent: false, prefer_block_scalars: false}` |
| serde-saphyr 2.0.0 is 12 days old, two majors in 3 months | API churn | Pin exactly, wrap behind one module |
| serde_yml, libyml, unsafe-libyaml, yaml-rust advisories | Unsound or unmaintained | Avoid |
| memmap2 < 0.9.11 | RUSTSEC-2026-0186 | ≥ 0.9.12 |
| `exe` GPL-3.0, `new-vdf-parser` LGPL-2.1, `attohttpc` MPL-2.0, `steamy-vdf` WTFPL, `singe-nvml` non-standard | License | Use the picks |
| clap rejects `--flag -100` | Breaks `spela gpu set X --clock-offset -100` | `allow_negative_numbers = true` on numeric flags |
| Rust `println!` panics on EPIPE (verified: "failed printing to stdout: Broken pipe") | `spela list --json \| head` crashes | Restore `SIGPIPE` to default at start, or handle `BrokenPipe` on writes |
| tracing-subscriber fmt writes ANSI to non-TTY stderr | Escape codes in redirected logs | `.with_ansi(is_terminal)` |
| `std::env::set_var` is unsafe (edition 2024) | Go's `t.Setenv` pattern does not port | `Paths::under(base)`, or the assert_cmd child env |
| MSRV outliers: sysinfo 1.95, serial_test 1.93.1, serde-saphyr 1.89, zbus 1.89 | Raises the MSRV | Only serde-saphyr is in the starter. MSRV 1.89 |

## Recommended starter dependency set

Every uncommented line below was in the `final/` probe. On 2026-10-25 with rustc 1.99.1, the probe resolved, compiled, ran all its subcommands and passed an insta snapshot test and an assert_cmd test (verified). It also built with 1.91 and was refused by 1.88 (verified). The commented lines have versions verified on crates.io. Building them together with the starter set is (unverified), except reqwest and tokio, which were measured as a variant. The crate split is (inferred) from mejl's layout (`/home/jgabor/git/mejl/Cargo.toml:13-21`).

```toml
[workspace]
resolver = "3"
members = ["crates/*"]
default-members = ["crates/spela"]

[workspace.package]
version = "0.1.0"
edition = "2024"
rust-version = "1.89"   # std::fs::File::lock (1.89) and serde-saphyr 2.0 (1.89)
license = "MIT"
repository = "https://github.com/jgabor/spela"

[workspace.dependencies]
# Internal (proposed split, inferred)
# spela-core = { path = "crates/spela-core" }   # profiles, config, game db, manifest, VDF, PE
# spela-system = { path = "crates/spela-system" } # NVML, sysfs, pkexec, kscreen-doctor, IPC, launcher
# spela-ui = { path = "crates/spela-ui" }

# TUI (same pins as mejl)
ratatui = "0.30.2"
crossterm = "0.29.0"

# CLI
clap = { version = "4.6.7", features = ["derive"] }
clap_complete = "4.6.11"   # cobra ships a `completion` command by default
anstyle = "1.0.14"         # already in clap's tree
anstream = "1.0.0"         # strips colors when stdout is not a terminal

# Formats
serde = { version = "1.0.229", features = ["derive"] }
serde_json = "1.0.154"     # plus a Go-escaping Formatter for --json parity
serde-saphyr = "2.0.0"     # ser_options!{indent_step: 4, compact_list_indent: false, prefer_block_scalars: false}
keyvalues-parser = "0.2.4"
object = { version = "0.40.0", default-features = false, features = ["read_core", "pe", "std"] }  # plus a port of detect.go's scan

# GPU
nvml-wrapper = "0.13.0"    # dlopens libnvidia-ml.so.1; no link-time NVIDIA dependency

# Network and integrity
ureq = { version = "3.5.0", features = ["platform-verifier"] }  # rustls + ring; set RootCerts::PlatformVerifier
sha2 = "0.11.1"            # output has no LowerHex; hex-encode by hand

# System
rustix = { version = "1.1.5", features = ["process", "fs"] }  # geteuid, kill, test_kill_process; already in crossterm's tree
signal-hook = "0.3.18"     # same major as crossterm; 0.4 would add a second copy
memmap2 = "0.9.12"         # >= 0.9.11 (RUSTSEC-2026-0186)
tempfile = "3.27.0"        # NamedTempFile::persist for atomic writes
wait-timeout = "0.2.1"     # kscreen-doctor 10 s timeout
walkdir = "2.5.0"
regex-lite = "0.1.9"
which = "8.0.6"

# Time, logging, errors
jiff = "0.2.38"
tracing = "0.1.44"
tracing-subscriber = "0.3.23"
anyhow = "1.0.104"
thiserror = "2.0.21"

# Add when needed
# tracing-appender = "0.2.5"   # if the TUI logs to a file like mejl
# etcetera = "0.11.0"          # only if spec-strict XDG is accepted (see conflicts)
# goblin = { version = "0.10.7", default-features = false, features = ["std", "pe32", "pe64"] }  # parsed VS_FIXEDFILEINFO
# clap_mangen = "0.3.3"
# process-wrap = "10.0.1"      # if games are signalled as a process group
# reqwest = { version = "0.13.5", default-features = false, features = ["rustls", "http2", "json", "stream"] }  # only with tokio
# tokio = { version = "1.54.1", features = ["rt-multi-thread", "macros", "sync", "time", "signal"] }

# Tests and benches
insta = "1.49.0"
assert_cmd = "2.2.2"
# predicates = "3.1.4"
# vt100 = "0.18.1"
# rexpect = "0.7.1"
# proptest = "1.12.0"
# divan = "0.1.21"

[profile.release]
lto = "thin"
codegen-units = 1
strip = true

[profile.dev.package."*"]
opt-level = 1   # as in mejl
```

Deliberately left out:

- `url`: 26 crates.
- `pelite`: last release 2022.
- `etcetera`: changes spela's XDG semantics.
- `reqwest`/`tokio`: measured cost. Revisit only for shared async code.
- `zbus`: no D-Bus need.
- `procfs`/`sysinfo`: std is enough.
- `semver`: wrong semantics for DLSS versions.

## Compile time and binary size

Measured on this machine: rustc 1.99.1 (LLVM 23.1.1), 16 cores, 30 GB RAM, Linux x86_64. Release profile `lto = "thin"`, `codegen-units = 1`, `strip = true`. No cargo config file, no `RUSTC_WRAPPER`. mold is installed but was not configured (verified: environment check). The registry was warm, because `cargo fetch` had already run, so these clean builds exclude downloads. mejl's 48.8 s included downloads, so the numbers are not directly comparable. One run each; treat them as rough.

"Crates" counts the unique packages in `cargo tree -e normal,build`.

| Build | Crates | Wall time (CPU) | Binary |
|---|---|---|---|
| First probe (pelite, url, etcetera, tracing-appender, ureq json), clean release | 211 | 20.2 s (118 s user) | 6.7 MB |
| First probe, incremental release after touching `main.rs` | 211 | 10.0 s | 6.7 MB |
| First probe, clean debug | 211 | 6.6 s | 124 MB (debug info) |
| First probe, incremental debug | 211 | 0.35 s (recompile confirmed by `Compiling` line) | |
| First probe, `opt-level = "s"`, `panic = "abort"`, clean | 211 | 13.6 s | 4.9 MB |
| First probe with reqwest 0.13.5 and tokio 1.54.1 instead of ureq, clean release | 238 | 31.0 s (166 s user) | 10.0 MB |
| **Final starter set** (above), clean release | **177** | **18.5 s** (96 s user) | **5.8 MB** |
| Final starter set, incremental release | 177 | 9.0 s | 5.8 MB |
| Final starter set, clean debug | 177 | 6.2 s | |
| Final starter set, incremental debug | 177 | 0.3 s | |
| Final starter set, `opt-level = "s"`, `panic = "abort"`, clean | 177 | 12.3 s | 4.2 MB |

Exclusive crate cost of each pick, measured by removing it from the first probe's manifest: url 26, ureq 20, serde-saphyr 12, nvml-wrapper 10, sha2 8, keyvalues-parser 5, pelite 5, tracing-subscriber 5, tracing-appender 4, tempfile 3, jiff 2, walkdir 2. clap_complete, etcetera, which, regex-lite and memmap2 cost 1 each. signal-hook, rustix and anstream cost 0. Alternatives added to the same base: object 1, goblin 4, yaml_serde 3, serde_norway 3, steamlocate 4, procfs 7, fs4 1, atomic-write-file 6, wait-timeout 1, hex 1.

Duplicate versions in the final tree (verified: `cargo tree -d -e normal`):

- darling 0.20.11 (nvml-wrapper's wrapcenum-derive) and 0.24.1 (ratatui's instability).
- getrandom 0.2.17 (ring) and 0.4.3 (tempfile).
- syn 2 and 3.
- thiserror 1.0.69 (nvml-wrapper) and 2.0.21.

Startup of the release binary (`--version`) took 1 ms. Startup does not include NVML init, which spela runs eagerly at `cmd/spela/main.go:118`; that adds 4 to 17 ms.

## Appendix: repo signals

GitHub API, 2026-10-25. "Open" includes pull requests.

| Repo | Stars | Open | Last push | Archived |
|---|---|---|---|---|
| rust-nvml/nvml-wrapper (nvml-wrapper, -sys) | 216 | 10 | 2026-08-31 | no |
| nagisa/rust_libloading | 1479 | 19 | 2026-08-02 | no |
| bourumir-wyngs/serde-saphyr | 239 | 2 | 2026-10-25 | no |
| bourumir-wyngs/granit-parser | 12 | 1 | 2026-10-25 | no |
| yaml/yaml-serde | 102 | 15 | 2026-08-18 | no |
| yaml/libyaml-rs | 3 | 0 | 2026-03-11 | no |
| cafkafk/serde-norway | 58 | 11 | 2025-08-04 | no |
| dtolnay/serde-yaml | 1015 | 50 | 2024-03-25 | yes |
| dtolnay/unsafe-libyaml | 38 | 1 | 2024-03-21 | yes |
| sebastienrousseau/serde_yml | 120 | 0 | 2026-05-28 | yes |
| sebastienrousseau/libyml | 5 | 0 | 2026-05-29 | yes |
| sebastienrousseau/noyalib | 11 | 12 | 2026-10-19 | no |
| saphyr-rs/saphyr | 339 | 28 | 2026-10-23 | no |
| Ethiraric/yaml-rust2 | 265 | 3 | 2026-09-11 | no |
| acatton/serde-yaml-ng | 114 | 20 | 2025-09-14 | no |
| bourumir-wyngs/serde-yaml-bw | 37 | 0 | 2026-09-07 | no |
| jelmer/yaml-edit | 11 | 1 | 2026-10-22 | no |
| clap-rs/clap | 16800 | 452 | 2026-10-06 | no |
| rust-cli/anstyle | 175 | 24 | 2026-10-20 | no |
| WilliamVenner/steamlocate-rs | 61 | 2 | 2026-10-07 | no |
| mexus/steam-vdf-parser | 8 | 0 | 2026-07-14 | no |
| Tormak9970/Steam-Art-Manager (new-vdf-parser) | 323 | 11 | 2026-10-24 | no |
| CasualX/pelite | 352 | 0 | 2026-10-14 | no |
| m4b/goblin | 1557 | 89 | 2026-08-30 | no |
| gimli-rs/object | 859 | 33 | 2026-10-23 | no |
| Systemcluster/editpe | 18 | 1 | 2026-08-12 | no |
| frank2/exe-rs | 83 | 7 | 2026-04-22 | no |
| RustCrypto/hashes (sha2) | 2280 | 37 | 2026-10-23 | no |
| algesten/ureq | 2217 | 41 | 2026-10-22 | no |
| seanmonstar/reqwest | 11888 | 486 | 2026-10-19 | no |
| tokio-rs/tokio | 33512 | 445 | 2026-10-25 | no |
| servo/rust-url | 1577 | 206 | 2026-07-31 | no |
| bytecodealliance/rustix | 2134 | 172 | 2026-10-15 | no |
| vorner/signal-hook | 878 | 28 | 2026-10-04 | no |
| nix-rust/nix | 3081 | 370 | 2026-10-18 | no |
| harryfei/which-rs | 271 | 0 | 2026-09-01 | no |
| alexcrichton/wait-timeout | 78 | 7 | 2025-05-17 | no |
| RazrFalcon/memmap2-rs | 659 | 19 | 2026-10-21 | no |
| al8n/fs4 | 120 | 1 | 2026-04-28 | no |
| yoshuawuyts/fd-lock | 89 | 11 | 2026-06-17 | no |
| eminence/procfs | 449 | 41 | 2026-06-26 | no |
| GuillaumeGomez/sysinfo | 2752 | 58 | 2026-10-25 | no |
| lunacookies/etcetera | 153 | 2 | 2026-08-04 | no |
| dirs-dev/directories-rs | 836 | 19 | 2025-01-12 | yes |
| BurntSushi/jiff | 2961 | 55 | 2026-10-14 | no |
| BurntSushi/walkdir | 1554 | 58 | 2026-06-23 | no |
| rust-lang/regex | 4045 | 87 | 2026-10-23 | no |
| tokio-rs/tracing | 6938 | 899 | 2026-10-09 | no |
| Stebalien/tempfile | 1470 | 12 | 2026-10-23 | no |
| andreacorbellini/rust-atomic-write-file | 36 | 0 | 2026-08-11 | no |
| ratatui/ratatui | 23083 | 202 | 2026-10-24 | no |
| crossterm-rs/crossterm | 4251 | 255 | 2026-10-25 | no |
| mitsuhiko/insta | 2979 | 80 | 2026-10-10 | no |
| assert-rs/assert_cmd | 564 | 27 | 2026-10-04 | no |
| doy/vt100-rust | 123 | 12 | 2026-10-19 | no |
| rust-cli/rexpect | 391 | 34 | 2026-09-18 | no |
| zhiburt/expectrl | 216 | 20 | 2026-05-11 | no |
| wezterm/wezterm (portable-pty) | 29344 | 1934 | 2026-10-19 | no |
| palfrey/serial_test | 295 | 0 | 2026-09-30 | no |
| z-galaxy/zbus | 764 | 55 | 2026-10-24 | no |

keyvalues-parser, keyvalues-serde and vdf-reader are on Codeberg, and dirs is on Codeberg; those were not checked.

## Open questions

- **Must Rust-written files stay readable by the Go binary (side-by-side install, downgrade)?** Today Go reads all serde-saphyr output back as equal values (verified). Byte-identity holds only for profile and overrides files. Resolve: decide on a migration policy.
- **Is the `--json` output a contract?** It currently uses Go field names (`AppID`, `DLLs`, `SRMode`), null slices and HTML escaping. Resolve: check scripts and AUR users, then decide whether to keep it or move to snake_case.
- **Should "clock offset" keep locking clocks to [0, 2100+offset], or become a real VF offset?** nvml-wrapper has `set_gpc_clock_vf_offset` and `set_clock_offset`. Resolve: live test as root on the target GPU, then decide.
- **Should known subcommands win over PATH binaries in wrapper detection?** Resolve: decide. A live test with `/usr/bin/overlay` present would confirm the collision, which is only verified statically here.
- **XDG:** keep spela's semantics (relative `XDG_*` accepted, `/tmp/runtime-$USER` fallback), or adopt etcetera like mejl?
- **TLS and proxy parity:** should DLL downloads honour `HTTPS_PROXY`, which Go does not? Should the platform verifier be used? Resolve: decide, then test against a proxy and a custom CA.
- **Async stack:** std threads plus ureq (measured cheaper) or mejl's tokio plus reqwest, for code sharing?
- **Logging in TUI mode:** stderr (parity) or a log file (mejl)?
- **Should `power_mizer` and `affinity` (marked "Not used yet") be implemented in the rewrite?** nvml-wrapper has `set_power_mizer_mode`. Affinity would need `sched_setaffinity` (rustix `process` feature, inferred).
- **NVML timing:** the 12-query sweep takes about 7 ms here. Is that acceptable on the UI thread? Measure per call on the target hardware.
- **MSRV:** 1.89 exactly was not tested (no 1.89 toolchain installed). Resolve: `cargo +1.89 build` once it is installed.
- **Completion parity:** cobra's completion uses its hidden `__complete` command for dynamic completion (inferred). clap_complete's static scripts complete differently. Is static completion enough?
- **Wrapper-mode exit codes:** Go exits 1 when the game fails and does not propagate the game's exit code (`cmd/spela/main.go:123-126`). Should Rust propagate it?

## Conflicts and open questions from this research

### Conflicts and mismatches

- **Overlay IPC design doc vs code.** The design doc describes a single file at $XDG_RUNTIME_DIR/spela-overlay-<pid>.dat with a 64-byte header and event, command and data ring offsets. The code writes $XDG_RUNTIME_DIR/spela/overlay-<appid>.dat with a 16-byte header, one 64-byte state block and no rings. A Rust writer must follow the code, because the external Zig layer reads it. Evidence: docs/design/overlay.md:148, :156-168 vs internal/overlay/setup.go:18, internal/overlay/ipc.go:12-46, internal/xdg/xdg.go:31-40
- **GPU 'clock offset' is a clock lock, not an offset.** The flag help says 'GPU graphics clock offset (MHz)' and 'GPU core clock offset in MHz'. The code locks graphics clocks to [0, 2100+offset] with SetGpuLockedClocks or nvidia-smi -lgc. The profile field text does disclose '2100 MHz plus this offset'. NVML (and nvml-wrapper) have real VF offset setters that spela does not use. The overlay doc also speaks of 'clock offsets'. Evidence: cmd/spela/commands/apply_profile.go:44, cmd/spela/commands/gpu_set.go:41 vs internal/gpu/nvidia.go:15-21; internal/profile/fields.go:79; docs/design/overlay.md:131; nvml-wrapper-0.13.0/src/device.rs:1811, :5732
- **GPU 'memory offset' is an absolute max MHz, and help strings disagree.** LockMemoryClocks(offset) locks memory clocks to [0, offset]. The help text says 'GPU memory clock offset in MHz' in one place and 'GPU memory clock (MHz)' in another. The field description says 'at most this value'. Evidence: internal/gpu/nvidia.go:23-28; cmd/spela/commands/gpu_set.go:42; cmd/spela/commands/apply_profile.go:45; internal/profile/fields.go:80
- **Negative clock values wrap silently.** maxMHz := 2100 + offset is converted with uint32(), and memory offset is passed as uint32(offset), with no range check (no ValueRange on these fields). A negative memory offset or a clock offset below -2100 becomes about 4.29e9 MHz. A Rust u32::try_from would turn this into an error, which changes behaviour. Evidence: internal/gpu/nvidia.go:16-18, :24-25; internal/profile/fields.go:79-80 (ValueRange nil)
- **Restore resets locked clocks even when the profile never set them.** The field text says '0 leaves clocks unchanged'. But every hardware cleanup runs apply-profile --reset, and that path always calls resetClocks (GPU and memory locked clocks) before restoring power, fan, governor and SMT. A profile that only changes the governor still resets clocks that something else may have locked. Evidence: internal/profile/apply.go:125-127; cmd/spela/commands/apply_profile.go:122-129; internal/profile/fields.go:79
- **Wrapper-mode detection can capture real subcommands.** IsWrapperMode treats any bare first argument found on PATH (no extension, .exe or .sh) as a game command, and main() checks it before cobra parses. On this machine /usr/bin/overlay (mesa-demos) exists, so `spela overlay set ...` would launch that binary instead of running the overlay subcommand. Verified from the code and PATH only; spela was not run. The test makes bare names on PATH wrappers by design, and no test covers collisions with subcommand names. Evidence: internal/launcher/launcher.go:266-295 (LookPath :287); cmd/spela/main.go:121-127; internal/launcher/wrapper_boundaries_test.go:30; `command -v overlay` -> /sbin/overlay owned by mesa-demos 9.0.0-7
- **Driver version parser comment vs code.** The comment says ParseDriverVersion 'Accepts 1-3 numeric components'. The code accepts up to 4 and ignores the fourth. Evidence: internal/proton/driver.go:39-40 vs :51, :75-76
- **Spela's VDF parser vs real VDF grammar.** The Go line tokenizer keeps backslashes, drops a key/value pair that has a trailing // comment (more than two tokens), needs '{' alone on its line, and keeps the last duplicate key. keyvalues-parser unescapes '\\', keeps the commented pair, accepts '"key" {', and keeps every duplicate. A Rust port with keyvalues-parser parses more files than Go does. Evidence: internal/steam/vdf.go:25, :41-46, :61-79; probe compare vdf (inline comment kept, brace on key line parsed); keyvalues-parser-0.2.4/src/lib.rs:32-63, :73
- **Atomic-write policy is inconsistent.** The game DB, profiles, DLL cache, backups, manifest and update-check state use temp file plus rename, without fsync. config.yaml and lists/overrides.yaml use plain os.WriteFile, so a crash can truncate them. Evidence: internal/game/database.go:106-137, internal/profile/storage.go:87-118, internal/dll/swap.go:276-302, internal/release/release.go:141-163 vs internal/config/config.go:95, internal/denylist/denylist.go:106
- **nvml-wrapper doc vs code on the library name.** The docs say initialization looks for 'libnvidia-ml.so'. The code constant is 'libnvidia-ml.so.1'. The code is what runs, and it is the right soname. Evidence: nvml-wrapper-0.13.0/src/lib.rs:270 vs :162
- **Spela's XDG semantics vs the spec and mejl's choice.** Spela uses any non-empty XDG_* value, even a relative one, and falls back to /tmp/runtime-$USER/spela when XDG_RUNTIME_DIR is unset. etcetera (used by mejl) ignores relative values and returns None for the runtime dir. With XDG_CONFIG_HOME=relative/cfg the probe gave 'relative/cfg/spela' for spela's rules and '~/.config/spela' for etcetera. Evidence: internal/xdg/xdg.go:11-14, :31-40; etcetera-0.11.0/src/base_strategy/xdg.rs:175-181, :210-212; /home/jgabor/git/mejl/crates/mejl/src/paths.rs:21, :57-77; probe xdg run
- **Logging destination: spela vs mejl.** Spela's slog handler writes to stderr in every mode, including while the TUI owns the terminal. mejl writes rolling log files through tracing-appender. Adopting mejl's approach changes where users find logs. Evidence: internal/logging/logging.go:16; /home/jgabor/git/mejl/crates/mejl/src/logging.rs:265-271
- **mejl's sha2 pin vs the current release.** mejl pins sha2 0.10. 0.11.1 is current, and its digest output no longer implements LowerHex, so code shared with mejl that uses format!("{:x}") would break on an upgrade. Evidence: /home/jgabor/git/mejl/Cargo.toml:72; crates.io sha2 0.11.1 (2026-10-12); probe compile error E0277 at the format!("{:x}") calls
- **MSRV: mejl 1.88 vs spela starter 1.89.** std::fs::File::lock (stable 1.89.0) and serde-saphyr 2.0.0 (rust-version 1.89) need 1.89. mejl declares 1.88. 1.88 refuses to build the probe; 1.91 builds it. Evidence: /home/jgabor/git/mejl/Cargo.toml:9; rust-src library/std/src/fs.rs:858; crates.io serde-saphyr 2.0.0 rust_version 1.89; probe cargo +1.88 / +1.91
- **TLS trust and proxy behaviour: Go vs ureq defaults.** Go verifies against the system pool (inferred). ureq defaults to bundled webpki roots. Go's DLL and manifest client uses a custom Transport with no Proxy field, so it ignores HTTPS_PROXY. The release check uses the default transport, which honours it (inferred). ureq reads proxy settings from the environment for every request. Evidence: internal/dll/manifest.go:22-27; internal/release/release.go:39; ureq-3.5.0/src/tls/mod.rs:370-375; ureq-3.5.0/src/config.rs:1073
- **CLI --json output uses Go field names.** The structs printed by --json carry only yaml tags, so encoding/json writes the Go names (AppID, DLLs, SRMode, VKD3DHeap). Empty fields are always present, nil slices print as null, and <, >, & are HTML-escaped. YAML on disk uses snake_case, so the two formats disagree. Matching byte for byte needs explicit serde renames and a custom formatter. Evidence: internal/game/game.go:18-33; internal/profile/profile.go:31-99; cmd/spela/commands/scan.go:40, list.go:61, show.go:51, profile.go:148, dlss.go:92; probe scan.json byte-identical only with the Go formatter
- **CLI colors are always on in Go.** CLI helpers render lipgloss styles with fmt.Printf, so ANSI codes are emitted even when piped (inferred: lipgloss v2 Style.Render does not downsample). anstream would strip them for non-TTY output, which changes the output. Evidence: internal/tui/styles.go:291-317; cmd/spela/commands/launch.go:110-118
- **Broken pipe behaviour.** Go dies quietly from SIGPIPE when stdout is a closed pipe (inferred: Go runtime). Rust's println! panics with 'failed printing to stdout: Broken pipe' (seen in the probe), so `spela list --json | head` would print a panic message. Evidence: probe run4.txt (two panics with `| head`)
- **serde-saphyr output is not byte-identical for games.yaml and config.yaml.** It quotes RFC 3339 timestamps and version strings such as 310.5.3, and uses double quotes where yaml.v3 uses single quotes. Go yaml.v3 reads them back to equal values, so this is a textual difference only. yaml_serde and serde_norway use a fixed 2-space layout. Evidence: probe diffs.txt; goref check OK for every backend
- **GUI-only config surface still in the YAML.** theme is GUI-only and OptionByJSONKey and the Wails JSON key column exist only for Wails. A CLI and TUI rewrite must still round-trip the theme key in config.yaml, or it is lost on save. Evidence: docs/configuration.md:11, :25; internal/config/options.go:98, :116-124; internal/config/config.go:43

### Open questions

- Must files written by the Rust rewrite stay readable by the Go binary (side-by-side install or downgrade)? Go reads all serde-saphyr output back as equal values today, but byte-identity holds only for profile and overrides files.
- Is the `--json` output a contract to keep (Go field names such as AppID, DLLs and SRMode, null slices, HTML escaping), or may the rewrite move to snake_case?
- Should 'clock offset' keep locking clocks to [0, 2100+offset], and 'memory offset' keep being a max MHz, or switch to real NVML VF offsets (set_gpc_clock_vf_offset, set_clock_offset)? This needs a live root test on the target GPU.
- Should known subcommands take priority over PATH binaries in wrapper detection? `/usr/bin/overlay` (mesa-demos) captures `spela overlay ...` here; verified statically only.
- XDG semantics: keep spela's rules (relative XDG_* accepted, /tmp/runtime-$USER fallback) for parity, or adopt etcetera's spec-strict rules as mejl does?
- Should DLL and manifest downloads honour HTTPS_PROXY (Go does not) and use the system trust store through ureq's platform-verifier? This needs a test against a proxy and a custom CA.
- Async stack: std threads with blocking ureq (measured 27 fewer crates, 12.5 s faster clean build, 3.3 MB smaller) or mejl's tokio and reqwest for shared code?
- Where should logs go while the TUI runs: stderr (parity) or a rolling file (mejl)?
- Should the 'Not used yet' fields power_mizer (nvml-wrapper set_power_mizer_mode exists) and cpu affinity be implemented in the rewrite or kept inert?
- Is a roughly 7 ms NVML metric sweep (12 queries, RTX 4070 Ti, driver 615.78.08) acceptable for TUI refresh, or must polling move off the UI thread? Measure per call on the target hardware.
- MSRV: is 1.89 acceptable? 1.89 itself was not tested because no 1.89 toolchain is installed (1.88 fails, 1.91 builds).
- Is static clap_complete output acceptable in place of cobra's default `completion` command, which uses dynamic completion through a hidden `__complete` command (inferred)?
- Should wrapper mode propagate the game's exit code? Go prints the error and exits 1, and skips the deferred NVML shutdown.
- Should the single-instance lock move from PID file plus kill(pid, 0) to flock, as mejl does, while keeping the 'already running (PID: N)' message?
