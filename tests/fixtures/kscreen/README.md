# kscreen-doctor fixtures

`doctor-o.txt` has the shape of `kscreen-doctor -o` stdout from the
development machine (KDE Plasma 6.8.1, libkscreen 6.8.1, one DisplayPort
monitor), captured read only on 2026-10-26. It keeps the ANSI color codes
kscreen-doctor writes even when stdout is a pipe, and the exact line forms.

The monitor's own details are left out. The output UUID is the fictional
`4f1e2d3c-5b6a-4789-8abc-def012345601`, the mode list, geometry, scale and SDR
brightness are generic values, and the other HDR, brightness and color lines
are cut. What is left is every line Go's parser reads (`Output:`, `enabled`,
`connected`, `priority` and `Vrr:`) plus one ignored line of each kind: a
top-level property, a nested HDR property, and `Color resolution`, which
kscreen-doctor prints with no color reset. The output has no serial number.

How it was captured, and what Go's parser needs from it, is in
[`docs/phase0-checks.md`](../../../docs/phase0-checks.md).
