# Go command transcripts

The output of the read-only commands of the Go binary, run against the
sandbox cases in this folder's parent. 161 commands; 3 of them
are marked unstable, because two runs gave different output or the output
comes in Go map order (see below).

## Regenerate

Build the Go binary outside the repository, then run goref from the
repository root:

```bash
GOPROXY=off GOFLAGS=-mod=mod go build -o /tmp/spela-go-ref/spela-go ./cmd/spela
GOPROXY=off GOFLAGS=-mod=mod go run ./tools/goref transcripts -bin /tmp/spela-go-ref/spela-go
```

The build is the CLI and TUI only: the GUI and its Wails build tag were
removed in Phase 1, so it needs no frontend. Its version is `dev`.

## How each command runs

For each command and each of two runs, goref copies the sandbox case into a
fresh temp folder `$ROOT`, replaces `/fixture-root` with `$ROOT` in its
text files, and runs the binary from `$ROOT` with exactly this environment
and no stdin:

```bash
env -i HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
  http_proxy=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9 \
  HOME=$ROOT/home USER=fixture LOGNAME=fixture PATH=$ROOT/empty-bin \
  XDG_CONFIG_HOME=$ROOT/config XDG_DATA_HOME=$ROOT/data \
  XDG_CACHE_HOME=$ROOT/cache XDG_RUNTIME_DIR=$ROOT/runtime \
  LANG=C.UTF-8 TERM=dumb TZ=UTC \
  /tmp/spela-go-ref/spela-go ARGS...
```

Then `$ROOT` is replaced by `/fixture-root` in stdout and stderr.

- PATH is an empty folder. Go checks for wrapper mode before it parses
  commands, and treats any first word that is a program on PATH as a game to
  launch (on this machine `spela overlay` would run `/sbin/overlay`). With an
  empty PATH no word resolves, and pkexec, nvidia-smi and kscreen-doctor
  cannot be found either.
- Help is only read through `help <command>`, never `<command> --help`.
  The help paths come from walking Go's own "Available Commands" lists.
- goref refuses any command outside a fixed read-only list
  (`checkReadOnly` in `tools/goref/transcripts.go`), and `launch` without
  `--dry-run` before any `--`.
- Commands that change hardware, game files or spela's files, `scan` and
  `tui` are not run. `gpu info`, `cpu info` and the other live readers
  are not recorded, because their output is this machine's.
- The proxy variables point at `http://127.0.0.1:9`, a closed loopback
  port, with no `NO_PROXY`, so a request to a remote host fails at once.
- The sandboxes set `rescan_on_startup`, `check_updates` and
  `auto_refresh_manifest` to false and `dll_manifest_url` to
  `http://127.0.0.1:9/`, where nothing listens. The manifest cache is seeded
  from `data/manifest.json` with that URL as its source, so no fetch is tried.
  The empty sandbox has no config, so `dll check-updates` is not run there:
  goref refuses `dll check-updates` in any case whose `dll_manifest_url` is
  not loopback.
- `dll list` without a game and `dll check-updates` print games in Go map
  order, so they are always marked unstable. Their order is not part of the
  contract; compare them as sets of lines.
- The descriptor heap notice that `proton show` prints depends on the machine
  that runs goref: on the NVIDIA driver NVML reports and whether it meets
  580.94.16. On the machine that wrote these transcripts the driver met the
  minimum, so the notices name only the Proton build. A driver version in a
  notice (`(detected: <ver>)` or `, driver <ver>)`) is replaced by `<driver>`
  and the run is marked redacted.
- Go writes 24-bit ANSI color codes on every run, even to a pipe and with
  `NO_COLOR` set. The transcripts keep them; strip `\x1b\[[0-9;]*m` to compare
  text.

## Files

`index.json` lists every command with its arguments, exit code, stdout file,
stderr file (only when stderr was not empty) and whether two runs matched.
Each command has `<sandbox>/<name>.out` and, when stderr was not empty,
`<sandbox>/<name>.err`.

## Commands

| Sandbox | Command line | Exit | Files | Notes |
|---|---|---|---|---|
| `sandbox/main` | `spela-go --version` | 0 | `version.out` | |
| `sandbox/main` | `spela-go -v` | 0 | `version-short.out` | |
| `sandbox/main` | `spela-go` | 0 | `root.out` | |
| `sandbox/main` | `spela-go --help` | 0 | `root-help-flag.out` | |
| `sandbox/main` | `spela-go help` | 0 | `help.out` | |
| `sandbox/main` | `spela-go help completion` | 0 | `help-completion.out` | |
| `sandbox/main` | `spela-go help config` | 0 | `help-config.out` | |
| `sandbox/main` | `spela-go help cpu` | 0 | `help-cpu.out` | |
| `sandbox/main` | `spela-go help denylist` | 0 | `help-denylist.out` | |
| `sandbox/main` | `spela-go help dll` | 0 | `help-dll.out` | |
| `sandbox/main` | `spela-go help dlss` | 0 | `help-dlss.out` | |
| `sandbox/main` | `spela-go help gpu` | 0 | `help-gpu.out` | |
| `sandbox/main` | `spela-go help help` | 0 | `help-help.out` | |
| `sandbox/main` | `spela-go help launch` | 0 | `help-launch.out` | |
| `sandbox/main` | `spela-go help list` | 0 | `help-list.out` | |
| `sandbox/main` | `spela-go help overlay` | 0 | `help-overlay.out` | |
| `sandbox/main` | `spela-go help profile` | 0 | `help-profile.out` | |
| `sandbox/main` | `spela-go help proton` | 0 | `help-proton.out` | |
| `sandbox/main` | `spela-go help scan` | 0 | `help-scan.out` | |
| `sandbox/main` | `spela-go help show` | 0 | `help-show.out` | |
| `sandbox/main` | `spela-go help tui` | 0 | `help-tui.out` | |
| `sandbox/main` | `spela-go help apply-profile` | 0 | `help-apply-profile.out` | |
| `sandbox/main` | `spela-go help completion bash` | 0 | `help-completion-bash.out` | |
| `sandbox/main` | `spela-go help completion fish` | 0 | `help-completion-fish.out` | |
| `sandbox/main` | `spela-go help completion powershell` | 0 | `help-completion-powershell.out` | |
| `sandbox/main` | `spela-go help completion zsh` | 0 | `help-completion-zsh.out` | |
| `sandbox/main` | `spela-go help config set` | 0 | `help-config-set.out` | |
| `sandbox/main` | `spela-go help config show` | 0 | `help-config-show.out` | |
| `sandbox/main` | `spela-go help cpu governor` | 0 | `help-cpu-governor.out` | |
| `sandbox/main` | `spela-go help cpu info` | 0 | `help-cpu-info.out` | |
| `sandbox/main` | `spela-go help cpu profile-reset` | 0 | `help-cpu-profile-reset.out` | |
| `sandbox/main` | `spela-go help cpu set` | 0 | `help-cpu-set.out` | |
| `sandbox/main` | `spela-go help cpu show` | 0 | `help-cpu-show.out` | |
| `sandbox/main` | `spela-go help cpu smt` | 0 | `help-cpu-smt.out` | |
| `sandbox/main` | `spela-go help denylist allow` | 0 | `help-denylist-allow.out` | |
| `sandbox/main` | `spela-go help denylist check` | 0 | `help-denylist-check.out` | |
| `sandbox/main` | `spela-go help denylist deny` | 0 | `help-denylist-deny.out` | |
| `sandbox/main` | `spela-go help denylist show` | 0 | `help-denylist-show.out` | |
| `sandbox/main` | `spela-go help dll check-updates` | 0 | `help-dll-check-updates.out` | |
| `sandbox/main` | `spela-go help dll list` | 0 | `help-dll-list.out` | |
| `sandbox/main` | `spela-go help dll restore` | 0 | `help-dll-restore.out` | |
| `sandbox/main` | `spela-go help dll update` | 0 | `help-dll-update.out` | |
| `sandbox/main` | `spela-go help dlss reset` | 0 | `help-dlss-reset.out` | |
| `sandbox/main` | `spela-go help dlss set` | 0 | `help-dlss-set.out` | |
| `sandbox/main` | `spela-go help dlss show` | 0 | `help-dlss-show.out` | |
| `sandbox/main` | `spela-go help gpu info` | 0 | `help-gpu-info.out` | |
| `sandbox/main` | `spela-go help gpu profile-reset` | 0 | `help-gpu-profile-reset.out` | |
| `sandbox/main` | `spela-go help gpu reset` | 0 | `help-gpu-reset.out` | |
| `sandbox/main` | `spela-go help gpu set` | 0 | `help-gpu-set.out` | |
| `sandbox/main` | `spela-go help gpu show` | 0 | `help-gpu-show.out` | |
| `sandbox/main` | `spela-go help overlay reset` | 0 | `help-overlay-reset.out` | |
| `sandbox/main` | `spela-go help overlay set` | 0 | `help-overlay-set.out` | |
| `sandbox/main` | `spela-go help overlay show` | 0 | `help-overlay-show.out` | |
| `sandbox/main` | `spela-go help profile create` | 0 | `help-profile-create.out` | |
| `sandbox/main` | `spela-go help profile delete` | 0 | `help-profile-delete.out` | |
| `sandbox/main` | `spela-go help profile list` | 0 | `help-profile-list.out` | |
| `sandbox/main` | `spela-go help profile reset` | 0 | `help-profile-reset.out` | |
| `sandbox/main` | `spela-go help profile show` | 0 | `help-profile-show.out` | |
| `sandbox/main` | `spela-go help proton reset` | 0 | `help-proton-reset.out` | |
| `sandbox/main` | `spela-go help proton set` | 0 | `help-proton-set.out` | |
| `sandbox/main` | `spela-go help proton show` | 0 | `help-proton-show.out` | |
| `sandbox/main` | `spela-go bogus` | 1 | `unknown-command.out`, `unknown-command.err` | |
| `sandbox/main` | `spela-go dll` | 0 | `parent-dll.out` | |
| `sandbox/main` | `spela-go profile` | 0 | `parent-profile.out` | |
| `sandbox/main` | `spela-go denylist` | 0 | `parent-denylist.out` | |
| `sandbox/main` | `spela-go list` | 0 | `list.out` | |
| `sandbox/main` | `spela-go list --with-dlls` | 0 | `list-with-dlls.out` | |
| `sandbox/main` | `spela-go list --json` | 0 | `list-json.out` | |
| `sandbox/main` | `spela-go list --with-dlls --json` | 0 | `list-with-dlls-json.out` | |
| `sandbox/main` | `spela-go show 9100010` | 0 | `show-id.out` | |
| `sandbox/main` | `spela-go show 'Glimmer Run'` | 0 | `show-name.out` | |
| `sandbox/main` | `spela-go show 'Paper Comet: Director'\''s Cut'` | 0 | `show-name-with-colon.out` | |
| `sandbox/main` | `spela-go show 9100030` | 0 | `show-no-dlls.out` | |
| `sandbox/main` | `spela-go show 9100070` | 0 | `show-second-library.out` | |
| `sandbox/main` | `spela-go show --json 9100010` | 0 | `show-json.out` | |
| `sandbox/main` | `spela-go show 9100040` | 1 | `show-tool.out`, `show-tool.err` | |
| `sandbox/main` | `spela-go show 9100050` | 1 | `show-pending-update.out`, `show-pending-update.err` | |
| `sandbox/main` | `spela-go show missing` | 1 | `show-missing.out`, `show-missing.err` | |
| `sandbox/main` | `spela-go show 'glimmer run'` | 1 | `show-case-mismatch.out`, `show-case-mismatch.err` | |
| `sandbox/main` | `spela-go show` | 1 | `show-no-args.out`, `show-no-args.err` | |
| `sandbox/main` | `spela-go profile list` | 0 | `profile-list.out` | |
| `sandbox/main` | `spela-go profile show 9100010` | 0 | `profile-show.out` | |
| `sandbox/main` | `spela-go profile show 9100020` | 0 | `profile-show-created.out` | |
| `sandbox/main` | `spela-go profile show 9100030` | 0 | `profile-show-none.out` | |
| `sandbox/main` | `spela-go profile show 'Frostline Arena'` | 0 | `profile-show-by-name.out` | |
| `sandbox/main` | `spela-go profile show --json 9100010` | 0 | `profile-show-json.out` | |
| `sandbox/main` | `spela-go profile show --json 9100020` | 0 | `profile-show-json-created.out` | |
| `sandbox/main` | `spela-go profile show --json 9100030` | 1 | `profile-show-json-none.out`, `profile-show-json-none.err` | |
| `sandbox/main` | `spela-go profile show missing` | 1 | `profile-show-missing.out`, `profile-show-missing.err` | |
| `sandbox/main` | `spela-go config show` | 0 | `config-show.out` | |
| `sandbox/main` | `spela-go dlss show 9100010` | 0 | `dlss-show.out` | |
| `sandbox/main` | `spela-go dlss show --json 9100010` | 0 | `dlss-show-json.out` | |
| `sandbox/main` | `spela-go dlss show 9100030` | 0 | `dlss-show-none.out` | |
| `sandbox/main` | `spela-go dlss show --json 9100030` | 0 | `dlss-show-json-none.out` | |
| `sandbox/main` | `spela-go gpu show 9100010` | 0 | `gpu-show.out` | |
| `sandbox/main` | `spela-go gpu show --json 9100010` | 0 | `gpu-show-json.out` | |
| `sandbox/main` | `spela-go gpu show 9100030` | 0 | `gpu-show-none.out` | |
| `sandbox/main` | `spela-go gpu show --json 9100030` | 0 | `gpu-show-json-none.out` | |
| `sandbox/main` | `spela-go cpu show 9100010` | 0 | `cpu-show.out` | |
| `sandbox/main` | `spela-go cpu show --json 9100010` | 0 | `cpu-show-json.out` | |
| `sandbox/main` | `spela-go cpu show 9100030` | 0 | `cpu-show-none.out` | |
| `sandbox/main` | `spela-go cpu show --json 9100030` | 0 | `cpu-show-json-none.out` | |
| `sandbox/main` | `spela-go proton show 9100010` | 0 | `proton-show.out` | |
| `sandbox/main` | `spela-go proton show --json 9100010` | 0 | `proton-show-json.out` | |
| `sandbox/main` | `spela-go proton show 9100030` | 0 | `proton-show-none.out` | |
| `sandbox/main` | `spela-go proton show --json 9100030` | 0 | `proton-show-json-none.out` | |
| `sandbox/main` | `spela-go overlay show 9100010` | 0 | `overlay-show.out` | |
| `sandbox/main` | `spela-go overlay show --json 9100010` | 0 | `overlay-show-json.out` | |
| `sandbox/main` | `spela-go overlay show 9100030` | 0 | `overlay-show-none.out` | |
| `sandbox/main` | `spela-go overlay show --json 9100030` | 0 | `overlay-show-json-none.out` | |
| `sandbox/main` | `spela-go proton show 9100020` | 0 | `proton-show-ge.out` | |
| `sandbox/main` | `spela-go proton show 9100070` | 0 | `proton-show-unresolved-tool.out` | |
| `sandbox/main` | `spela-go proton show 9100060` | 0 | `proton-show-unsupported-tool.out` | |
| `sandbox/main` | `spela-go dll list` | 0 | `dll-list.out` | unstable |
| `sandbox/main` | `spela-go dll list 9100010` | 0 | `dll-list-game.out` | |
| `sandbox/main` | `spela-go dll list 9100020` | 0 | `dll-list-game-other-families.out` | |
| `sandbox/main` | `spela-go dll list 9100030` | 0 | `dll-list-game-none.out` | |
| `sandbox/main` | `spela-go dll list 9100060` | 0 | `dll-list-unknown-version.out` | |
| `sandbox/main` | `spela-go dll check-updates` | 0 | `dll-check-updates.out` | unstable |
| `sandbox/main` | `spela-go denylist show` | 0 | `denylist-show.out` | |
| `sandbox/main` | `spela-go denylist check 9100070` | 0 | `denylist-check-user-denied.out` | |
| `sandbox/main` | `spela-go denylist check 9100010` | 0 | `denylist-check-allowed.out` | |
| `sandbox/main` | `spela-go denylist check 1172470` | 0 | `denylist-check-force-allowed.out` | |
| `sandbox/main` | `spela-go denylist check 252490` | 0 | `denylist-check-built-in.out` | |
| `sandbox/main` | `spela-go denylist check 9100999` | 0 | `denylist-check-unknown-id.out` | |
| `sandbox/main` | `spela-go denylist check nope` | 1 | `denylist-check-missing-name.out`, `denylist-check-missing-name.err` | |
| `sandbox/main` | `spela-go launch --dry-run 9100010` | 0 | `launch-dry-run.out` | |
| `sandbox/main` | `spela-go launch --dry-run 9100020` | 0 | `launch-dry-run-created-profile.out` | |
| `sandbox/main` | `spela-go launch --dry-run 9100030` | 0 | `launch-dry-run-no-profile.out` | |
| `sandbox/main` | `spela-go launch --dry-run 9100070` | 0 | `launch-dry-run-denied.out` | |
| `sandbox/main` | `spela-go launch --dry-run 9100060` | 0 | `launch-dry-run-unknown-version.out` | |
| `sandbox/main` | `spela-go launch --dry-run 'Glimmer Run'` | 0 | `launch-dry-run-by-name.out` | |
| `sandbox/main` | `spela-go launch --dry-run 9100010 -- glimmer.exe -dx12` | 0 | `launch-dry-run-with-command.out` | |
| `sandbox/main` | `spela-go launch --dry-run 9100010 glimmer.exe -dx12` | 1 | `launch-dry-run-dash-argument.out`, `launch-dry-run-dash-argument.err` | |
| `sandbox/main` | `spela-go launch --dry-run --game-id 9100020 ignored` | 0 | `launch-dry-run-game-id.out` | |
| `sandbox/main` | `spela-go launch --dry-run missing` | 1 | `launch-dry-run-missing.out`, `launch-dry-run-missing.err` | |
| `sandbox/errors` | `spela-go profile list` | 0 | `profile-list.out` | |
| `sandbox/errors` | `spela-go profile show 9100030` | 1 | `profile-show-broken-yaml.out`, `profile-show-broken-yaml.err` | |
| `sandbox/errors` | `spela-go profile show 9100060` | 1 | `profile-show-invalid-vrr.out`, `profile-show-invalid-vrr.err` | |
| `sandbox/errors` | `spela-go profile show --json 9100030` | 1 | `profile-show-json-broken-yaml.out`, `profile-show-json-broken-yaml.err` | |
| `sandbox/errors` | `spela-go show 9100030` | 0 | `show-broken-profile.out` | |
| `sandbox/errors` | `spela-go dlss show 9100060` | 1 | `dlss-show-invalid-vrr.out`, `dlss-show-invalid-vrr.err` | |
| `sandbox/errors` | `spela-go proton show 9100030` | 1 | `proton-show-broken-yaml.out`, `proton-show-broken-yaml.err` | |
| `sandbox/errors` | `spela-go launch --dry-run 9100030` | 1 | `launch-dry-run-broken-yaml.out`, `launch-dry-run-broken-yaml.err` | |
| `sandbox/errors` | `spela-go launch --dry-run 9100060` | 1 | `launch-dry-run-invalid-vrr.out`, `launch-dry-run-invalid-vrr.err` | |
| `sandbox/errors` | `spela-go launch --dry-run 9100010` | 0 | `launch-dry-run-fine.out` | |
| `sandbox/empty` | `spela-go list` | 1 | `list.out`, `list.err` | |
| `sandbox/empty` | `spela-go list --json` | 1 | `list-json.out`, `list-json.err` | |
| `sandbox/empty` | `spela-go show 9100010` | 1 | `show.out`, `show.err` | |
| `sandbox/empty` | `spela-go profile list` | 0 | `profile-list.out` | |
| `sandbox/empty` | `spela-go config show` | 0 | `config-show.out` | |
| `sandbox/empty` | `spela-go denylist show` | 0 | `denylist-show.out` | |
| `sandbox/empty` | `spela-go denylist check 1172470` | 0 | `denylist-check-built-in.out` | |
| `sandbox/empty` | `spela-go dll list` | 0 | `dll-list.out` | unstable |
| `sandbox/empty` | `spela-go launch --dry-run 9100010` | 1 | `launch-dry-run.out`, `launch-dry-run.err` | |
| `e2e/library` | `spela-go list --with-dlls` | 0 | `list-with-dlls.out` | |
| `e2e/library` | `spela-go profile list` | 0 | `profile-list.out` | |
| `e2e/library` | `spela-go profile show 9100110` | 0 | `profile-show.out` | |
| `e2e/library` | `spela-go dlss show 9100110` | 0 | `dlss-show.out` | |
| `e2e/library` | `spela-go launch --dry-run 9100110` | 0 | `launch-dry-run.out` | |
| `e2e/library` | `spela-go launch --dry-run 9100130` | 0 | `launch-dry-run-no-profile.out` | |
