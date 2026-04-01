# Issues

## GUI

### ~~1. DLL database not persisted after operations~~ (Resolved)

Fixed in prior refactoring. All three methods now call `db.Save()` and `dll.ScanDirectory()`.

### ~~2. Game launch bypasses launcher package~~ (Resolved)

Fixed in prior refactoring. `LaunchGame()` now uses `launcher.New(g)` + `l.Launch()`.

### ~~3. Missing profile fields~~ (Resolved)

Fixed in prior refactoring. `ProfileInfo` struct now includes all TUI v0.2.0 fields.

### 4. DLSS-D column missing from DLL display

**Severity:** Medium
**Files:** `internal/gui/frontend/src/lib/GameDetail.svelte:404-416`

DLL version grid hardcodes DLSS, DLSS-G, XESS, FSR columns but omits DLSS-D, which the TUI now displays.

### 5. No DLL operation progress indicator

**Severity:** Medium
**Files:** `internal/gui/frontend/src/lib/GameDetail.svelte`

No visual feedback (spinner, progress bar, or status text) during DLL update/install/restore operations.

### 6. DLL operation error messages incomplete

**Severity:** Medium
**Files:** `internal/gui/frontend/src/lib/GameDetail.svelte`

Database save errors are not surfaced to the user.

## CLI

### ~~7. Incomplete DLSS set command flags~~ (Resolved)

Fixed in prior refactoring. All flags now present in `dlss set`.

### 8. No CLI commands for GPU/CPU/Overlay/Ludusavi settings

**Severity:** Low
**Files:** `cmd/spela/commands/`

Profile fields exist in the profile package but have no CLI subcommands.
These are also marked "coming soon" in the TUI, so parity is acceptable for now.

## Overlay

### ~~9. NVML setter privilege model undecided~~ (Resolved)

Migrated to batched `pkexec spela apply-profile` with go-nvml setters + polkit policy.
GPU setters use NVML directly when root (µs-level), CPU setters write sysfs directly.
Single pkexec round-trip replaces 4× `pkexec nvidia-smi` (~2.6s → ~65ms cached).
Polkit policy (`io.github.spela.apply-profile`) provides session-cached auth via `auth_admin_keep`.
