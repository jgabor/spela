# KDE VRR discovery compatibility

Spela uses the ordinary KDE Wayland session's `kscreen-doctor -o` output.
Research and verification on 2026-09-14 used read-only commands only:
`kscreen-doctor --help`, `kscreen-doctor -j`, `kscreen-doctor -o`, and
`pacman -Q libkscreen plasma-workspace`. Both installed packages were 6.7.5-1.
The output exposed a UUID, enabled/connected state, priority 1, and `Vrr: Never`.
JSON exposed priority and numeric policy but omitted UUID and VRR capability,
so it is not sufficient for safe restoration by itself.

The upstream [Plasma 6.5 doctor.cpp](https://github.com/KDE/libkscreen/blob/Plasma/6.5/src/doctor/doctor.cpp)
confirms:

- `outputs()` prints `Output: <id> <name> <uuid>`, state, priority and VRR.
- `setPrimary()` sets priority 1; the first enumerated output is not necessarily primary.
- `Vrr: Never`, `Automatic`, or `Always` is printed only when the output has
  the VRR capability; otherwise it prints `incapable`.
- `findOutput()` accepts UUIDs, names, or numeric IDs.
- `output.<uuid>.vrrpolicy.<never|automatic|always>` changes the policy.
- Configuration failures can print `applying config failed!` while exiting zero;
  Spela treats that response as an error for both apply and restore.

The parser removes ANSI color sequences and accepts those required fields only
when unambiguous. Missing UUIDs (including older formats), duplicate identities,
multiple primaries, missing state or unknown policy cause warning-only no-ops.
Unrelated mode, geometry and color fields are ignored. This is feature detection,
not a blanket compatibility claim for every Plasma version.

Restoration re-discovers the original UUID and checks that it is still connected,
enabled and VRR-capable. It does not re-select the primary or fall back to a
connector/ID that could now identify a replacement. The setter also uses UUID,
so a disconnect between discovery and mutation cannot redirect the command to
a replacement connector. KDE remains responsible for applying each command;
Spela cannot make the separate discovery and mutation calls atomic.

Automated display fixtures cover primary ordering, all policies, colored output,
missing/ambiguous discovery, unsupported sessions, command failures, changes in
primary, disconnection and replacement. Launcher tests use a PATH-local fake
doctor to exercise preparation, process exit/start failure, handled SIGTERM and
dry-run behavior. No test invokes a real display mutation.
