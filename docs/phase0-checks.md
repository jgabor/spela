# Phase 0 read-only checks

Claude ran these checks on the development machine on 2026-10-26, under
decision 63. Every command only read files or queried state. None of them
wrote, launched a game, needed root or changed hardware or display settings.
Checks that change hardware or launch games stay with the user (see
[Left for the user](#left-for-the-user)).

The machine: CachyOS, kernel 7.3.0-3-cachyos, KDE Plasma 6.8.1 on Wayland,
the Arch `steam` package 1.0.0.87-3, one DisplayPort monitor. Steam's root is
`~/.local/share/Steam`, with a second library elsewhere, written `<library2>`
below.

Personal data is left out: no account data, and no app ids or game names from
the user's own library. Where a check needed real paths, only counts and
version strings are quoted.

## Proton tool folders

Commands:

```bash
ls -la ~/.steam
readlink -f ~/.steam/root ~/.steam/steam
ls -la /usr/share/steam/compatibilitytools.d
ls -la ~/.steam/root/compatibilitytools.d
cat ~/.local/share/Steam/compatibilitytools.d/GE-Proton11-7/compatibilitytool.vdf
ls -d ~/.local/share/Steam/steamapps/common/*Proton* <library2>/steamapps/common/*Proton*
ls -d ~/.local/share/Steam/steamapps/common/SteamLinuxRuntime* <library2>/steamapps/common/SteamLinuxRuntime*
```

What they showed:

- `/usr/share/steam/compatibilitytools.d` does not exist here. Go's resolver
  never looks there anyway: it checks only `<steam root>/compatibilitytools.d/<name>`
  and `<steam root>/steamapps/common/<name>` (verified:
  `internal/proton/resolver.go:168-179`).
- `~/.steam/root` and `~/.steam/steam` are both symlinks to
  `~/.local/share/Steam`. Its `compatibilitytools.d` holds three folders:
  `GE-Proton11-7`, `GE-Proton11-11` and `UMU-Proton-10.0-4`.
- A tool's name is not its folder name. `GE-Proton11-7/compatibilitytool.vdf`
  declares the internal name `GE-Proton11-7-x86_64`, and that internal name is
  what Steam writes into `CompatToolMapping`. So Go's folder lookup by name
  misses every GE build here.
- Built-in Proton is mapped by short names (`proton_experimental`,
  `proton_9`, `proton_8`, `proton_hotfix`), while the folders are
  `Proton - Experimental`, `Proton 9.0 (Beta)`, `Proton 8.0`, `Proton Hotfix`
  and so on. Most of them live in the second library, not under the Steam
  root. Go looks for `<steam root>/steamapps/common/proton_experimental`, which
  never exists.
- `SteamLinuxRuntime_sniper` is the only mapped name with a folder of the same
  name, and that folder is in the second library, so Go misses it too.
- The mapping also names three Proton-CachyOS builds
  (`proton-cachyos-10.0-20260312-slr-x86_64_v3` and two more) for which no
  folder exists in any place checked. Go's build tag pattern
  `cachyos[-_]?(\d+)\.(\d+)-(\d{8})` does match that name shape.

Result: on this machine Go's resolver resolves no mapped tool at all, so the
descriptor heap check always ends in "could not resolve active Proton for this
game" (verified: `internal/proton/notice.go:153-166`). A resolver that reads
the internal names from each `compatibilitytool.vdf`, looks in every library's
`steamapps/common`, and maps the built-in short names to their app manifests
would find most of them (inferred, not tried).

The fixture sandbox copies these shapes: a GE folder whose internal name
differs, a built-in `proton_experimental` mapping with a `Proton - Experimental`
folder (`tests/fixtures/go/sandbox/main/home/.steam/steam/`).

## CompatToolMapping key casing

Command (prints the key path down to depth 4 with names only, then the one
`CompatToolMapping` block):

```bash
F=~/.local/share/Steam/config/config.vdf
awk '{ line=$0; gsub(/^[ \t]+/,"",line) } line=="{" {d++; next} line=="}" {d--; next}
     { n=split(line,a,"\""); if (n<=3 && d<=4) print d": "a[2] }' "$F"
awk 'BEGIN{inb=0} { line=$0; t=line; gsub(/^[ \t]+/,"",t) }
     t ~ /^"CompatToolMapping"$/ {inb=1; d=0}
     inb { print line; if (t=="{") d++; if (t=="}") { d--; if (d==0) exit } }' "$F"
grep -n -i -o '"compattoolmapping"' "$F"
```

What it showed:

- The path is `InstallConfigStore` > `Software` > `Valve` > `Steam` >
  `CompatToolMapping`, in exactly the casing Go's resolver uses (verified:
  `internal/proton/resolver.go:142-147`). A case-insensitive grep finds the
  key once, so there is no second spelling.
- Each entry has this shape (app id replaced):

  ```text
  "<app id>"
  {
      "name"      "GE-Proton10-34"
      "config"    ""
      "priority"  "250"
  }
  ```

- The global default is the entry `"0"` with priority `"75"`; game entries have
  `"250"`, and one Steam Linux Runtime entry has `"85"`. The global default here
  is `GE-Proton11-7-x86_64`.
- There are 29 entries. One key is `4263293584`, a non-Steam shortcut id above
  2^31. Go reads ids as uint64, so it is fine there; a Rust parser must not use
  i32 (inferred).

## StateFlags

Commands:

```bash
for L in ~/.local/share/Steam <library2>; do
  grep -h -E '^\s*"StateFlags"' $L/steamapps/appmanifest_*.acf | awk '{print $2}' | sort | uniq -c
done
# then, for every manifest whose StateFlags is not 4: its name, BytesToDownload,
# BytesDownloaded and UpdateResult
```

What they showed:

| Library | StateFlags 4 | StateFlags 6 | Other |
|---|---|---|---|
| Steam root | 4 | 1 | 0 |
| Second library | 47 | 7 | 0 |

Every value that is not 4 is 6: installed (4) plus update required (2). All
eight have `BytesToDownload` above zero, `BytesDownloaded` 0 and
`UpdateResult` 0. Five are Steam tools (Proton Experimental and four Steam
Linux Runtimes) and three are games.

Go keeps an app only when `StateFlags == 4` (verified:
`internal/steam/appmanifest.go:60-62`). So the three games with a pending update
drop out of Go's scan until Steam updates them, and their profiles and DLLs
disappear from `list` and the TUI meanwhile. Testing bit 4 instead
(`StateFlags & 4 != 0`) would keep them (inferred). The fixture sandbox has one
such game (`Pending Patch`, 9100050, StateFlags 6), and the transcripts show Go
dropping it (`show-pending-update`).

## kscreen-doctor -o

Commands:

```bash
kscreen-doctor -o                                         # in this agent's shell
WAYLAND_DISPLAY=wayland-1 QT_QPA_PLATFORM=wayland kscreen-doctor -o
WAYLAND_DISPLAY=wayland-1 QT_QPA_PLATFORM=wayland kscreen-doctor -j   # only to look for serial numbers
pacman -Q libkscreen kscreen plasma-workspace steam
```

What they showed:

- In a shell without `WAYLAND_DISPLAY` (this agent ran from a tty session),
  `kscreen-doctor -o` aborts with SIGABRT and dumps core (exit 134). It needs
  the desktop session's Wayland socket. A spela process started outside the
  graphical session would hit the same abort (inferred).
- With the session's socket it lists one output. With the ANSI codes removed,
  and the monitor's own details (UUID, modes, geometry, scale, brightness)
  replaced by generic values as in the fixture:

  ```text
  Output: 1 DP-3 4f1e2d3c-5b6a-4789-8abc-def012345601
  	enabled
  	connected
  	priority 1
  	DisplayPort
  	replication source:0
  	Modes:  1:1920x1080@60.00*!  2:1280x720@60.00
  	Custom modes: None
  	Geometry: 0,0 1920x1080
  	Scale: 1
  	Rotation: 1
  	Overscan: 0
  	Vrr: Never
  	RgbRange: unknown
  	HDR: enabled
  	...
  ```

- kscreen-doctor writes ANSI color codes even when stdout is a pipe. Go strips
  them with a regex before it parses (verified: `internal/display/vrr.go:121-148`).
- Go's parser needs, per output, a `Output: <index> <name> <uuid>` line with
  exactly four fields and a non-zero UUID, then one each of
  `enabled`/`disabled`, `connected`/`disconnected`, `priority <n>` and
  `Vrr: <policy>` (verified: `internal/display/vrr.go:137-206`). This output has
  all of them; `Vrr: Never` is lowercased to `never`. The Modes, Geometry, HDR
  and color lines are ignored.
- Neither `-o` nor `-j` shows a serial number. The JSON has `id`, `name`,
  `vrrPolicy` (0 here), `priority`, `enabled`, `connected` and mode data, and no
  UUID.
- The output, cut to the lines a parser reads plus one ignored line of each
  kind and with generic values, is saved as
  `tests/fixtures/kscreen/doctor-o.txt`.

## PE versions of installed DLLs

Commands. The DLL paths came from spela's own game cache, read only, so no
recursive search ran over the libraries:

```bash
grep -E '^\s+- path: .*\.dll$' ~/.local/share/spela/games.yaml | sed 's/^ *- path: //' | grep -i nvngx_dlss
GOPROXY=off GOFLAGS=-mod=mod go run ./tools/goref pe-version FILE...
```

`goref pe-version` calls Go's own `dll.GetDLLVersion` and also prints the four
raw `VS_FIXEDFILEINFO` file version parts. It opens each file read only.

What it showed, for the 26 DLSS, DLSS-G and DLSS-D DLLs in 12 games:

| Raw file version | Go's string | Files |
|---|---|---|
| 310.5.3.0 | `310.5.3` | 22 |
| 310.7.0.0 | `310.7` | 2 |
| 2.3.11.0 | `2.3.11` | 1 |
| 2.2.11.0 | `2.2.11` | 1 |

- Every string equals the version Go's scan stored in `games.yaml`.
- `data/manifest.json` writes versions with three parts (`310.5.3`, `310.5.0`).
  A DLL whose build part is 0 formats with two parts (`310.7`), because Go's
  formatter drops a zero build when the revision is also zero (verified:
  `internal/dll/detect.go:154-162`). Go's `CompareVersions` treats missing parts
  as 0, so the comparisons still work, but `Manifest.GetDLLVersion` matches
  versions by string equality (verified: `internal/dll/manifest.go:235-248`), so
  a detected `310.7` would never equal a manifest `310.7.0`. Rust should
  compare versions as numbers everywhere (inferred).
- The two `310.7` DLLs are newer than the manifest's latest `310.5.3`; Go does
  not offer a downgrade for them.
- No installed DLL has a non-zero revision, so the four-part branch is covered
  only by the fixture PE stubs (`libxess.dll` 1.3.1.33 in the sandbox).

## Go behaviour found while writing the fixtures

These came out of running the real Go code for `tests/fixtures/go` (see its
`README.md` and `transcripts/COMMANDS.md`). The evidence is in the named
fixtures.

- The CLI always writes 24-bit ANSI color codes, even to a pipe, with
  `TERM=dumb` and with `NO_COLOR=1`. lipgloss renders without a color profile
  writer (verified: `internal/tui/styles.go:292-317`; every transcript).
- `--version` and `-v` print `spela version dev` for a build without
  ldflags (`transcripts/sandbox/main/version.out`).
- Legacy profile migration pins a key the legacy file leaves out to its zero
  value when All games sets it to non-zero. In
  `profiles/legacy-go-test-clock`, `gpu.shader_cache` and
  `gpu.threaded_optimization` become own Off values; in
  `profiles/legacy-with-default`, `proton.enable_wayland` becomes own Off.
- yaml.v3 accepts `y`, `yes`, `on`, `off`, `Yes`, `TRUE` and `NO` as booleans,
  in values and in the overrides map (`profiles/yaml11-bools`).
- An explicit zero is stored only as a marker: Go writes
  `gpu.threaded_optimization: true` under `overrides` with no
  `threaded_optimization` key (`sandbox/main/config/spela/profiles/9100010.yaml`).
- `profile.List` parses `9100129.yml` and `09100130.yaml`, but `Load` opens
  `<appid>.yaml`, so neither ever loads (`profiles/file-names`).
- A `default.yaml` with `vrr: Always` (wrong case) fails to load, and with it
  every legacy game profile and every launch (`profiles/invalid-vrr`).
- Go saves `log_level: ""` and loads it back without error; only
  `config.Validate` rejects it. The same holds for `log_level: verbose` and for
  unknown keys, which load silently (`config/explicit-zeros`,
  `config/retired-keys`).
- `dll list` and `dll check-updates` print games in map order, which changed
  between two runs. goref now marks both unstable whatever two runs show
  (`transcripts/index.json`, `stable: false`).
- `launch --dry-run <game> game.exe -dx12` fails with `unknown shorthand flag:
  'd' in -dx12`; the game's arguments need `--` first. The summary then shows
  the game query as part of the command: `Command: 9100010 glimmer.exe -dx12`.
- `show` for a game whose profile does not parse still exits 0 and prints
  `Profile: yes` (`transcripts/sandbox/errors/show-broken-profile.out`).
- `denylist check` for a force-allowed built-in game that is not installed
  prints the id as the name: `1172470 is allowed for DLL swapping`.
- `backup.json` and `update-check.json` end without a newline. `games.yaml`
  `updated_at` and the manifest cache `fetched_at` always come from the clock.
- `list` and `show` on an empty game cache with no Steam install fail with
  `could not find Steam installation`, also with `--json`.

## Left for the user

These change hardware or launch games, so Claude did not run them:

- The restore spike: one `pkexec` prompt per launch under Steam, gamescope and
  Big Picture (decision 30).
- The NVML root test for the real clock and memory offsets (decisions 9 and 32).
- Whether re-onlined CPUs get the restored governor after SMT goes off and on.
- Applying a KDE VRR policy.
- Whether dxvk-nvapi accepts `off`, `render_preset_default` and multi-frame 4,
  whether the DLSS indicator persists, and whether
  `PROTON_ENABLE_NGX_UPDATER=1` overrides a swapped DLL.
- The GitHub tags API, TLS and proxy behaviour, and the daily dlss-updater job.
