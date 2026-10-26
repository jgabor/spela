# spela

Linux gaming configuration tool for NVIDIA GPUs, being rewritten in Rust: per-game
profiles, DLSS and Proton settings, DLL management with backups, and temporary
GPU and CPU tuning around Steam launches. The rewrite covers the CLI and the TUI
with full feature parity. There is no GUI.

Research and architecture for the rewrite live in `docs/` (start with
`docs/README.md`, which records decisions 1 to 66, and `docs/architecture.md`).
The architecture and terminal UX follow mejl (`~/git/mejl`) wherever that fits.

## Layout

| Path | Role |
|---|---|
| `crates/` | The Rust workspace. The crate split is set in `docs/architecture.md` |
| `cmd/`, `internal/`, and the Go code in `tools/` and `tests/` | The Go implementation. It stays untouched as the parity reference until the user's live launch checks pass, then the whole Go tree goes in one commit. The fixtures in `tests/fixtures/` stay |
| `data/polkit/` | Polkit policy for privileged operations |
| `docs/` | Research, architecture, plans, field and config references |

The Go GUI (`internal/gui`, Wails, the frontend, `internal/nav/gui.go`,
`tools/gen-nav-js`) is not ported. It is the one part of the Go tree deleted
early, in Phase 1.

## Commands

- Build: `cargo build`
- Gate before handing off: `just check` (`cargo fmt --all --check`, `cargo clippy --workspace --all-targets --locked -- -D warnings`, `cargo test --workspace --locked`, and `just msrv`, which runs `cargo +1.89 check --workspace --all-targets --locked`)
- Pre-commit hook that runs the gate: `just hooks` once per checkout
- Go reference tests: `go test ./...`

## Hard rules

- No code, test or tool call runs `pkexec` or `sudo`, or changes GPU clocks, power limits, fans, the CPU governor, SMT or display settings on this machine. Tests use fakes for NVML, sysfs, polkit and the display.
- No test writes to real game files or Steam libraries. Use fixture libraries in temp dirs.
- No test or tool call contacts a remote host (DLL manifest and downloads, release checks). Tests use fixtures or a loopback fake.
- Tests never touch the real XDG config, data, cache or state dirs. Use temp dirs.
- Files written by the Go version (`config.yaml`, `profiles/*.yaml`, `lists/*.yaml`, `games.yaml`) are migrated to the rewrite's own shape the first time they are read. A migration keeps every value the user set, or, where decisions 10, 31, 46 and 65 drop it, records it in `migration.toml` and the migrated file's header. It leaves the original file in place as a backup, and is tested against fixtures written by the Go version.
- DLL backups written by the Go version (`backup.json` and both backup file name schemes) are read in place and never moved or reshaped. New data goes in new top-level keys that Go ignores, never in `files[]`.
- Privileged changes go through one batched `pkexec` round trip, and each applied change registers its restore before the game starts.
- The UI thread never awaits and never blocks on NVML, `nvidia-smi`, `pkexec`, the network or a Steam scan.
- Docs and comments: plain words, sentence case headings, no em dashes.
- Inline `#[cfg(test)] mod tests` blocks go last in their file.
- Commits use conventional commit format (`feat:`, `fix:`, `refactor:` and so on).

## TUI review

Review every change that can affect the terminal UI in a real terminal with
[Terminal Control](https://github.com/anomalyco/terminal-control) 1.2.1 or newer.
Read the rendered screen with `termctrl show`; logs, raw stdout, screenshots and
unit tests do not count as visual review. Wait for visible content instead of
sleeping, and send only keys shown in the current frame.

```bash
cargo build
termctrl start spela-tui-review --cols 120 --rows 40 -- ./target/debug/spela tui
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl show spela-tui-review
termctrl resize spela-tui-review --cols 80 --rows 24
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl show spela-tui-review
termctrl stop spela-tui-review
```

Check 120x40 and 80x24. Use a unique session name when reviews may run at the
same time, isolated XDG dirs for anything that rescans or saves, and always stop
the session. Never approve privileged or destructive actions against real game
files during a review. `docs/tui-testing.md` has the full checklist.
