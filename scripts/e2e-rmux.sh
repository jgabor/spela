#!/usr/bin/env bash
# End-to-end UX and QA validation for the Spela TUI via rmux.
# Detached session + capture-pane, OK/FAIL/SKIP protocol.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SESSION="${SPELA_E2E_SESSION:-spela-e2e}"
COLS="${SPELA_E2E_COLS:-120}"
ROWS="${SPELA_E2E_ROWS:-40}"
WAIT="${SPELA_E2E_WAIT:-3}"
BUILD="${SPELA_E2E_BUILD:-}"
BINARY="${SPELA_E2E_BINARY:-$ROOT/spela}"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok()   { echo "OK: $*"; }
skip() { echo "SKIP: $*"; }

cleanup() {
	for s in "$SESSION" "${SESSION}-mutate" "${SESSION}-resetall"; do
		rmux kill-session -t "$s" 2>/dev/null || true
	done
	[[ -n "${TMPDIR:-}" && -d "$TMPDIR" ]] && rm -rf "$TMPDIR"
	rm -f "${XDG_RUNTIME_DIR:-/tmp/runtime-$USER}/spela/spela.pid" 2>/dev/null || true
}
trap cleanup EXIT

cd "$ROOT"

if ! command -v rmux >/dev/null; then
	fail "rmux not found in PATH"
fi

# -------------------------------------------------------------------------
# Build
# -------------------------------------------------------------------------
if [[ "${1:-}" == "--build" || -n "$BUILD" ]] || [[ ! -x "$BINARY" ]]; then
	echo "building spela..."
	go build -o "$BINARY" ./cmd/spela
	echo "OK: build complete"
fi
[[ ! -x "$BINARY" ]] && fail "binary not found at $BINARY"

# -------------------------------------------------------------------------
# Test environment (mirrors mock_helper.go's XDG layout)
# -------------------------------------------------------------------------
mkdir -p "${ROOT}/tmp"
TMPDIR="$(mktemp -d "${ROOT}/tmp/spela-e2e-XXXXXX")"
XDG_CONFIG_HOME="$TMPDIR/config"
XDG_DATA_HOME="$TMPDIR/data"
XDG_CACHE_HOME="$TMPDIR/cache"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME"

# Mock game directories and DLL stubs
CP_DIR="$TMPDIR/games/Cyberpunk 2077/bin/x64"
TW3_DIR="$TMPDIR/games/The Witcher 3/bin"
ER_DIR="$TMPDIR/games/ELDEN RING"
mkdir -p "$CP_DIR" "$TW3_DIR" "$ER_DIR"
echo "DLSS 3.7.0"  > "$CP_DIR/nvngx_dlss.dll"
echo "DLSSG 3.7.0" > "$CP_DIR/nvngx_dlssg.dll"
echo "DLSS 3.5.0"  > "$TW3_DIR/nvngx_dlss.dll"

# games.yaml
mkdir -p "$XDG_DATA_HOME/spela"
cat > "$XDG_DATA_HOME/spela/games.yaml" <<-GAMESYAML
games:
  1091500:
    app_id: 1091500
    name: Cyberpunk 2077
    install_dir: $CP_DIR
    scanned_at: "2026-05-26T12:00:00Z"
    dlls:
      - path: $CP_DIR/nvngx_dlss.dll
        name: nvngx_dlss.dll
        type: dlss
        version: "3.7.0"
      - path: $CP_DIR/nvngx_dlssg.dll
        name: nvngx_dlssg.dll
        type: dlssg
        version: "3.7.0"
  292030:
    app_id: 292030
    name: "The Witcher 3: Wild Hunt"
    install_dir: $TW3_DIR
    scanned_at: "2026-05-26T12:00:00Z"
    dlls:
      - path: $TW3_DIR/nvngx_dlss.dll
        name: nvngx_dlss.dll
        type: dlss
        version: "3.5.0"
  1245620:
    app_id: 1245620
    name: "Elden Ring"
    install_dir: $ER_DIR
    scanned_at: "2026-05-26T12:00:00Z"
    dlls: []
updated_at: "2026-05-26T12:00:00Z"
GAMESYAML

# config.yaml
mkdir -p "$XDG_CONFIG_HOME/spela"
cat > "$XDG_CONFIG_HOME/spela/config.yaml" <<-CONFIGYAML
log_level: info
check_updates: false
show_hints: true
preferred_dll_source: techpowerup
compact_mode: false
confirm_destructive: true
rescan_on_startup: false
auto_refresh_manifest: false
CONFIGYAML

# Profiles
mkdir -p "$XDG_CONFIG_HOME/spela/profiles"

cat > "$XDG_CONFIG_HOME/spela/profiles/default.yaml" <<-DEFAULT
dlss:
  sr_mode: balanced
  sr_preset: balanced
  sr_override: false
  fg_enabled: false
  fg_override: false
proton:
  enable_hdr: false
  enable_wayland: false
  enable_ngx_updater: false
DEFAULT

cat > "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml" <<-GAMEPROF
dlss:
  sr_mode: balanced
  sr_preset: quality
  sr_override: false
  fg_enabled: true
  fg_override: true
proton:
  enable_hdr: true
  enable_wayland: false
  enable_ngx_updater: false
overrides:
  dlss.sr_mode: true
  dlss.sr_preset: true
  dlss.sr_override: true
  dlss.fg_enabled: true
  dlss.fg_override: true
  proton.enable_hdr: true
  proton.enable_wayland: true
  proton.enable_ngx_updater: true
GAMEPROF

# manifest.json
mkdir -p "$XDG_CACHE_HOME/spela"
cat > "$XDG_CACHE_HOME/spela/manifest.json" <<-MANIFEST
{"version":"1.0","updated_at":"2026-05-22T21:00:00Z","repository":"helvesec/rmux","dlls":{"dlss":[{"version":"3.8.0","filename":"nvngx_dlss_3.8.0.dll","url":"http://localhost:12345/dlls/nvngx_dlss_3.8.0.dll","sha256":"abcdef1234567890","size":1024,"release_date":"2026-05-22T21:00:00Z"}],"dlssg":[{"version":"3.7.0","filename":"nvngx_dlssg_3.7.0.dll","url":"http://localhost:12345/dlls/nvngx_dlssg_3.7.0.dll","sha256":"abcdef1234567890","size":1024,"release_date":"2026-05-22T21:00:00Z"}]}}
MANIFEST

# Env for spela invocation (include RUNTIME_DIR for PID lock isolation)
RUNTIME_DIR="$TMPDIR/runtime"
mkdir -p "$RUNTIME_DIR"
E2E_ENV="XDG_CONFIG_HOME=$XDG_CONFIG_HOME XDG_DATA_HOME=$XDG_DATA_HOME XDG_CACHE_HOME=$XDG_CACHE_HOME XDG_RUNTIME_DIR=$RUNTIME_DIR"

run_tui() {
	local session="$1" extra_env="${2:-}"
	local env="$E2E_ENV"
	[[ -n "$extra_env" ]] && env="$extra_env $env"
	rmux kill-session -t "$session" 2>/dev/null || true
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$env $BINARY tui" Enter
	sleep "$WAIT"
}

capture() {
	rmux capture-pane -t "$1" -p
}

# Save profile before mutation tests
cp "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml" \
   "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml.bak"

# =====================================================================
# UX checks
# =====================================================================
echo ""
echo "UX checks"
echo "------------------------------------------------------------"

# --- UX-1: Smoke — TUI starts and shows game list ---
ux_smoke() {
	run_tui "$SESSION"
	local screen
	screen="$(capture "$SESSION")"

	if ! grep -q "Cyberpunk 2077" <<<"$screen"; then
		echo "$screen" >&2
		fail "game list did not appear"
	fi
	if ! grep -qE "The Witcher" <<<"$screen"; then
		echo "$screen" >&2
		fail "The Witcher 3 not in game list"
	fi
	if ! grep -q "Elden Ring" <<<"$screen"; then
		echo "$screen" >&2
		fail "Elden Ring not in game list"
	fi
	if ! grep -q "Library" <<<"$screen"; then
		echo "$screen" >&2
		fail "three-zone Library destination not visible"
	fi
	ok "smoke — game list visible"
}
ux_smoke

# --- UX-2: Game detail — enter into Cyberpunk ---
ux_detail() {
	# Context zone: search and confirm Cyberpunk (default row is All games)
	rmux send-keys -t "$SESSION" Tab "/" "Cyber" Enter Enter
	sleep 1
	# Switch to Overview aspect (game opens with Profile aspect inherited from root defaults)
	rmux send-keys -t "$SESSION" "1"
	sleep 0.5

	local screen
	screen="$(capture "$SESSION")"

	if ! grep -q "App ID:" <<<"$screen"; then
		echo "$screen" >&2
		fail "game detail did not load"
	fi
	if ! grep -q "1091500" <<<"$screen"; then
		echo "$screen" >&2
		fail "App ID 1091500 not visible in detail"
	fi
	ok "game detail — App ID and info present"
}
ux_detail

# --- UX-3: Search filter ---
ux_search() {
	rmux send-keys -t "$SESSION" Escape
	sleep 0.3
	rmux send-keys -t "$SESSION" Tab "/" "Cyber" Enter Enter
	sleep 1

	local screen
	screen="$(capture "$SESSION")"

	if ! grep -q "Cyberpunk 2077" <<<"$screen"; then
		echo "$screen" >&2
		fail "Cyberpunk 2077 not visible after search"
	fi
	ok "search filter — matching result shown"
}
ux_search

# --- UX-4: Empty state ---
ux_empty() {
	rmux send-keys -t "$SESSION" Escape
	sleep 0.3
	rmux send-keys -t "$SESSION" Tab "/" "nonexistent" Enter
	sleep 1

	local screen
	screen="$(capture "$SESSION")"

	if ! grep -q "No games found" <<<"$screen"; then
		echo "$screen" >&2
		fail "empty state 'No games found' not shown"
	fi
	ok "empty state — 'No games found'"
}
ux_empty

# --- UX-5: Help overlay ---
ux_help() {
	rmux send-keys -t "$SESSION" Escape
	sleep 0.3
	rmux send-keys -t "$SESSION" "?"
	sleep 1

	local screen
	screen="$(capture "$SESSION")"

	if grep -q "Keyboard shortcuts" <<<"$screen"; then
		ok "help overlay — opened via ?"
	elif grep -q "Help" <<<"$screen"; then
		ok "help overlay — opened via ? (alt label)"
	else
		echo "$screen" >&2
		fail "help overlay did not open"
	fi
}
ux_help

# --- UX-6: Settings destination ---
ux_options() {
	rmux send-keys -t "$SESSION" Escape
	sleep 0.3
	rmux send-keys -t "$SESSION" "4"
	sleep 1

	local screen
	screen="$(capture "$SESSION")"

	if ! grep -qE "Options|Settings|Show hints|Theme" <<<"$screen"; then
		echo "$screen" >&2
		fail "settings destination did not open"
	fi
	ok "settings destination — opened via 4"
}
ux_options

# --- UX-7: Default profile scope ---
ux_defaults() {
	rmux send-keys -t "$SESSION" Escape
	sleep 0.3
	rmux send-keys -t "$SESSION" "1"
	sleep 0.5
	rmux send-keys -t "$SESSION" Tab
	sleep 0.3
	# Clear any leftover search/filter from prior UX tests
	rmux send-keys -t "$SESSION" "C"
	sleep 0.3
	# Navigate to top (All games) and select
	rmux send-keys -t "$SESSION" "k" "k" "k" "k" Enter
	sleep 1

	local screen
	screen="$(capture "$SESSION")"

	if ! grep -q "All games" <<<"$screen"; then
		echo "$screen" >&2
		fail "default profile scope did not load"
	fi
	if ! grep -qE "SR mode|Root profile|HDR" <<<"$screen"; then
		echo "$screen" >&2
		fail "expected profile fields in defaults"
	fi
	ok "defaults section — profile fields visible"
}
ux_defaults

# --- UX-8: DLL aspect navigation ---
ux_dll_aspect() {
	rmux send-keys -t "$SESSION" Escape
	sleep 0.3
	rmux send-keys -t "$SESSION" Tab "/" "Cyber" Enter Enter
	sleep 1
	rmux send-keys -t "$SESSION" "3"
	sleep 1

	local screen
	screen="$(capture "$SESSION")"

	if ! grep -q "DLSS" <<<"$screen"; then
		echo "$screen" >&2
		fail "DLL aspect table not visible"
	fi
	ok "DLL aspect — version table visible"
}
ux_dll_aspect

# Kill the shared UX session
rmux kill-session -t "$SESSION" 2>/dev/null || true

# =====================================================================
# QA checks (file-system contracts + data integrity)
# =====================================================================
echo ""
echo "QA checks"
echo "------------------------------------------------------------"

# --- QA-1: XDG contract ---
qa_xdg() {
	local missing=0
	for p in \
		"$XDG_CONFIG_HOME/spela/config.yaml" \
		"$XDG_CONFIG_HOME/spela/profiles/default.yaml" \
		"$XDG_CONFIG_HOME/spela/profiles/1091500.yaml" \
		"$XDG_DATA_HOME/spela/games.yaml" \
		"$XDG_CACHE_HOME/spela/manifest.json"; do
		[[ -f "$p" ]] || { echo "  missing: $p" >&2; ((missing++)); }
	done
	[[ $missing -eq 0 ]] || fail "$missing XDG file(s) missing"
	ok "XDG contract — all expected files present"
}
qa_xdg

# --- QA-2: Config has required keys ---
qa_config() {
	local missing=0
	for key in log_level check_updates show_hints preferred_dll_source; do
		grep -q "^$key:" "$XDG_CONFIG_HOME/spela/config.yaml" || { echo "  missing key: $key" >&2; ((missing++)); }
	done
	[[ $missing -eq 0 ]] || fail "$missing config key(s) missing"
	ok "config contract — required keys present"
}
qa_config

# --- QA-3: Games database integrity ---
qa_games_db() {
	local db="$XDG_DATA_HOME/spela/games.yaml"

	grep -q "updated_at:" "$db" || fail "games.yaml missing updated_at"

	local names
	names="$(grep "^    name:" "$db" | sed 's/.*name: //')"
	grep -q "Cyberpunk 2077" <<<"$names"  || fail "games.yaml missing Cyberpunk 2077"
	grep -q "Elden Ring"      <<<"$names"  || fail "games.yaml missing Elden Ring"
	grep -q "The Witcher 3"   <<<"$names"  || fail "games.yaml missing The Witcher 3 (Wild Hunt)"

	ok "games database — 3 entries with correct names"
}
qa_games_db

# --- QA-4: Profile YAML structure ---
qa_profile() {
	# Use the backup saved before UX operations (TUI navigation may
	# auto-save profile fields, but the original structure is what
	# we want to validate here).
	local prof="$XDG_CONFIG_HOME/spela/profiles/1091500.yaml.bak"

	[[ -f "$prof" ]] || fail "profile backup not found at $prof"
	grep -q "^overrides:" "$prof"     || fail "game profile missing overrides section"
	grep -q "proton.enable_hdr" "$prof" || fail "profile missing proton.enable_hdr override"

	ok "profile structure — overrides and sections present"
}
qa_profile

# -------------------------------------------------------------------------
# Mutation QA (fresh sessions to avoid state leakage)
# Debug helper: count lines in the overrides section
count_overrides() {
	local f="$1"
	# Count indented override lines under the "overrides:" header.
	# Uses 2+ spaces to handle both hand-written (2-space) and yaml.v3
	# marshaled (4-space) indentation.
	sed -n '/^overrides:/,$ p' "$f" | grep -cE '^[[:space:]]{2,}[a-z]' || true
}

qa_mutation() {
	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"

	local prof="$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"
	local before_count
	before_count="$(count_overrides "$prof")"

	local m_session="${SESSION}-mutate"
	rmux kill-session -t "$m_session" 2>/dev/null || true
	rmux new-session -d -s "$m_session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$m_session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$m_session")"
	grep -q "Cyberpunk 2077" <<<"$screen" || fail "TUI did not start for mutation test"

	# Enter Cyberpunk detail via context search
	rmux send-keys -t "$m_session" Tab "/" "Cyber" Enter Enter
	sleep "$WAIT"

	screen="$(capture "$m_session")"
	grep -qE "App ID:|HDR|SR mode|reset" <<<"$screen" || fail "game detail did not load for mutation"

	# Press r to reset the currently focused field
	rmux send-keys -t "$m_session" "r"
	sleep 1

	# Poll for a decrease in override count (any field could be focused)
	local waited=0
	local after_count="$before_count"
	while [[ "$after_count" -ge "$before_count" ]]; do
		sleep 0.5
		after_count="$(count_overrides "$prof")"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done

	if [[ "$after_count" -ge "$before_count" ]]; then
		echo "  before=$before_count after=$after_count" >&2
		grep -A 20 "^overrides:" "$prof" >&2
		rmux kill-session -t "$m_session" 2>/dev/null || true
		fail "profile reset (r) did not clear any override (was $before_count, now $after_count)"
	fi

	ok "profile reset — single field (r) cleared override (was $before_count, now $after_count)"
	rmux kill-session -t "$m_session" 2>/dev/null || true
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
}
qa_mutation

# --- QA-5b: Reset all (Shift+R) ---
qa_reset_all() {
	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"

	local prof="$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"

	local r_session="${SESSION}-resetall"
	rmux kill-session -t "$r_session" 2>/dev/null || true
	rmux kill-session -t "${SESSION}-mutate" 2>/dev/null || true
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$r_session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$r_session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$r_session")"
	if ! grep -q "Cyberpunk 2077" <<<"$screen"; then
		echo "  screen output:" >&2
		echo "$screen" >&2
		fail "TUI did not start for reset-all test"
	fi

	rmux send-keys -t "$r_session" Tab "/" "Cyber" Enter Enter
	sleep "$WAIT"

	# Shift+R = capital R (resets all overrides)
	rmux send-keys -t "$r_session" "R"
	sleep 1

	# Poll until the overrides header is gone or has zero entries
	local waited=0
	local after_count
	after_count="$(count_overrides "$prof")"
	while [[ "$after_count" -gt 0 ]]; do
		sleep 0.5
		after_count="$(count_overrides "$prof")"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done

	if [[ "$after_count" -gt 0 ]]; then
		echo "  overrides after R: $(grep -A 20 '^overrides:' "$prof")" >&2
		rmux kill-session -t "$r_session" 2>/dev/null || true
		fail "ResetAll (R) did not clear all overrides ($after_count remain)"
	fi

	ok "ResetAll (Shift+R) — all overrides cleared"
}
qa_reset_all

# Clean up mutation sessions
rmux kill-session -t "${SESSION}-mutate" 2>/dev/null || true
rmux kill-session -t "${SESSION}-resetall" 2>/dev/null || true

# =====================================================================
# Extended QA: root defaults editing, pin, sidebar, multi-select, options
# =====================================================================
echo ""
echo "Extended QA checks"
echo "------------------------------------------------------------"

# Back up default.yaml before root defaults mutation tests
cp "$XDG_CONFIG_HOME/spela/profiles/default.yaml" \
   "$XDG_CONFIG_HOME/spela/profiles/default.yaml.bak"

# --- QA: Root defaults — h/l three-state bool cycling ---
qa_root_cycle() {
	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/default.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/default.yaml"

	local prof="$XDG_CONFIG_HOME/spela/profiles/default.yaml"
	local session="${SESSION}-rootcycle"
	rmux kill-session -t "$session" 2>/dev/null || true
	# Wait for PID lock to clear
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$session")"
	grep -qE "Cyberpunk 2077|Library" <<<"$screen" || fail "TUI did not start for root cycle test"

	# Navigate to root defaults: Tab (ZoneContext) then Enter (select "All games")
	rmux send-keys -t "$session" Tab Enter
	sleep 1

	screen="$(capture "$session")"
	grep -qE "SR mode|All games|VKD3D" <<<"$screen" || fail "root defaults view did not load"

	# Navigate to proton.vkd3d_heap (index 3): j 3 times from index 0
	rmux send-keys -t "$session" "j"; sleep 0.3
	rmux send-keys -t "$session" "j"; sleep 0.3
	rmux send-keys -t "$session" "j"; sleep 0.3

	# Press l to cycle: (default) → true
	rmux send-keys -t "$session" "l"
	sleep 0.5

	# Poll default.yaml for vkd3d_heap: true
	waited=0
	local found=""
	while [[ "$found" == "" ]]; do
		sleep 0.5
		found="$(grep 'vkd3d_heap: true' "$prof" 2>/dev/null || true)"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ -n "$found" ]] || fail "vkd3d_heap: true not found in default.yaml"

	# Verify screen shows "true" (not "(default)")
	screen="$(capture "$session")"
	grep -q "true" <<<"$screen" || fail "screen does not show 'true' after cycling to true"

	# Press l to cycle: true → false
	rmux send-keys -t "$session" "l"
	sleep 0.5

	# Poll: vkd3d_heap: true should disappear (omitempty), but proton.vkd3d_heap in overrides
	waited=0
	while grep -q 'vkd3d_heap: true' "$prof" 2>/dev/null; do
		sleep 0.5
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	# Assert proton.vkd3d_heap is in overrides section
	grep -q 'proton.vkd3d_heap' "$prof" 2>/dev/null \
		|| fail "proton.vkd3d_heap should be in overrides after cycling to false"

	# Press l to cycle: false → (default)
	rmux send-keys -t "$session" "l"
	sleep 0.5

	# Poll: proton.vkd3d_heap should NOT be in default.yaml
	waited=0
	while grep -q 'proton.vkd3d_heap' "$prof" 2>/dev/null; do
		sleep 0.5
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	grep -q 'proton.vkd3d_heap' "$prof" 2>/dev/null \
		&& fail "proton.vkd3d_heap should not be in default.yaml after cycling back to (default)"

	ok "root defaults — h/l cycles bool through three states"

	rmux kill-session -t "$session" 2>/dev/null || true
	# Wait for PID cleanup
	waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done

	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/default.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/default.yaml"
}
qa_root_cycle

# --- QA: Root defaults — r (reset focused) and R (reset all) ---
qa_root_reset() {
	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/default.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/default.yaml"

	local prof="$XDG_CONFIG_HOME/spela/profiles/default.yaml"
	local session="${SESSION}-rootreset"
	rmux kill-session -t "$session" 2>/dev/null || true
	# Wait for PID lock to clear
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$session")"
	grep -qE "Cyberpunk 2077|Library" <<<"$screen" || fail "TUI did not start for root reset test"

	# Navigate to root defaults: Tab (ZoneContext) then Enter (select "All games")
	rmux send-keys -t "$session" Tab Enter
	sleep 1

	# Navigate to proton.vkd3d_heap (index 3): j 3 times from index 0
	rmux send-keys -t "$session" "j"; sleep 0.3
	rmux send-keys -t "$session" "j"; sleep 0.3
	rmux send-keys -t "$session" "j"; sleep 0.3

	# Press l to set vkd3d_heap to true
	rmux send-keys -t "$session" "l"
	sleep 0.5

	# Poll for vkd3d_heap: true in default.yaml
	waited=0
	local found=""
	while [[ "$found" == "" ]]; do
		sleep 0.5
		found="$(grep 'vkd3d_heap: true' "$prof" 2>/dev/null || true)"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ -n "$found" ]] || fail "vkd3d_heap: true not found after pressing l"

	# Press r to reset focused field
	rmux send-keys -t "$session" "r"
	sleep 0.5

	# Poll for proton.vkd3d_heap NOT in default.yaml
	waited=0
	while grep -q 'proton.vkd3d_heap' "$prof" 2>/dev/null; do
		sleep 0.5
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	grep -q 'proton.vkd3d_heap' "$prof" 2>/dev/null \
		&& fail "reset focused (r) did not clear proton.vkd3d_heap override"
	ok "root defaults — r resets focused field"

	# Now set multiple fields to test R (reset all)
	# Cursor is still on vkd3d_heap after the reset — press l directly
	rmux send-keys -t "$session" "l"
	sleep 0.5

	# Poll for vkd3d_heap: true
	waited=0
	found=""
	while [[ "$found" == "" ]]; do
		sleep 0.5
		found="$(grep 'vkd3d_heap: true' "$prof" 2>/dev/null || true)"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ -n "$found" ]] || fail "vkd3d_heap: true not found before R test"

	# Navigate back to enable_hdr (index 0): k 3 times
	rmux send-keys -t "$session" "k"; sleep 0.3
	rmux send-keys -t "$session" "k"; sleep 0.3
	rmux send-keys -t "$session" "k"; sleep 0.3

	# Set enable_hdr to true
	rmux send-keys -t "$session" "l"
	sleep 0.5

	# Poll for enable_hdr: true
	waited=0
	found=""
	while [[ "$found" == "" ]]; do
		sleep 0.5
		found="$(grep 'enable_hdr: true' "$prof" 2>/dev/null || true)"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ -n "$found" ]] || fail "enable_hdr: true not found before R test"

	# Check that default.yaml has overrides
	local override_count
	override_count="$(count_overrides "$prof")"
	[[ "$override_count" -gt 0 ]] \
		|| fail "expected overrides in default.yaml before R, got $override_count"

	# Press R to reset all
	rmux send-keys -t "$session" "R"
	sleep 1

	# Poll for no overrides in default.yaml
	waited=0
	override_count="$(count_overrides "$prof")"
	while [[ "$override_count" -gt 0 ]]; do
		sleep 0.5
		override_count="$(count_overrides "$prof")"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ "$override_count" -eq 0 ]] \
		|| fail "ResetAll (R) did not clear all overrides ($override_count remain)"

	ok "root defaults — r resets focused field, R resets all"

	rmux kill-session -t "$session" 2>/dev/null || true
	# Wait for PID cleanup
	waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done

	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/default.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/default.yaml"
}
qa_root_reset

# --- QA: Pin (p) binding on game profile ---
qa_pin() {
	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"

	local prof="$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"
	local session="${SESSION}-pin"
	rmux kill-session -t "$session" 2>/dev/null || true
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$session")"
	grep -q "Cyberpunk 2077" <<<"$screen" || fail "TUI did not start for pin test"

	# Navigate to Cyberpunk profile: search + confirm, then switch to Profile aspect
	rmux send-keys -t "$session" Tab "/" "Cyber" Enter Enter
	sleep 1
	rmux send-keys -t "$session" "2"
	sleep 1

	screen="$(capture "$session")"
	if ! grep -qE "App ID|1091500|SR mode|HDR|reset" <<<"$screen"; then
		echo "$screen" >&2
		fail "game profile did not load for pin test"
	fi

	# Navigate to proton.vkd3d_heap (field index 3): press j three times
	rmux send-keys -t "$session" "j"; sleep 0.3
	rmux send-keys -t "$session" "j"; sleep 0.3
	rmux send-keys -t "$session" "j"; sleep 0.3

	# Record override count before pinning
	local before_count after_count
	before_count="$(count_overrides "$prof")"

	# Pin the focused field
	rmux send-keys -t "$session" "p"

	# Poll for override count increase (async save)
	waited=0
	after_count="$before_count"
	while [[ "$after_count" -le "$before_count" ]]; do
		sleep 0.5
		after_count="$(count_overrides "$prof")"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ "$after_count" -gt "$before_count" ]] || fail "pin (p) did not add override (was $before_count, now $after_count)"

	# Verify proton.vkd3d_heap appears in the overrides section
	sed -n '/^overrides:/,$ p' "$prof" | grep -q "proton.vkd3d_heap" \
		|| fail "pin did not add proton.vkd3d_heap to overrides"

	# Idempotency: pressing p again should NOT add a duplicate
	local pinned_count="$after_count"
	rmux send-keys -t "$session" "p"
	sleep 1
	after_count="$(count_overrides "$prof")"
	[[ "$after_count" -eq "$pinned_count" ]] \
		|| fail "pin (p) added duplicate on re-pin (was $pinned_count, now $after_count)"

	ok "pin binding — p pins inherited field, idempotent on re-pin"

	rmux kill-session -t "$session" 2>/dev/null || true
	waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done

	# Restore original profile
	cp "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml.bak" \
	   "$XDG_CONFIG_HOME/spela/profiles/1091500.yaml"
}
qa_pin

# --- QA: Sidebar filters (d, P, C) and sort (s) ---
qa_sidebar_filters() {
	local session="${SESSION}-filters"
	rmux kill-session -t "$session" 2>/dev/null || true
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$session")"
	grep -q "Cyberpunk 2077" <<<"$screen" || fail "TUI did not start for filters test (Cyberpunk)"
	grep -q "Elden Ring"    <<<"$screen" || fail "TUI did not start for filters test (Elden Ring)"
	grep -q "Witcher"      <<<"$screen" || fail "TUI did not start for filters test (Witcher 3)"

	# Move to sidebar (ZoneContext)
	rmux send-keys -t "$session" Tab
	sleep 0.5

	# Filter: hasDLLs (d) — Elden Ring has no DLLs, Cyberpunk does
	rmux send-keys -t "$session" "d"
	sleep 0.5
	screen="$(capture "$session")"
	if grep -q "Elden Ring" <<<"$screen"; then
		echo "$screen" >&2
		fail "hasDLLs filter (d) did not hide Elden Ring"
	fi
	if ! grep -q "Cyberpunk" <<<"$screen"; then
		echo "$screen" >&2
		fail "hasDLLs filter (d) hid Cyberpunk"
	fi

	# Clear filters (C) — everything reappears
	rmux send-keys -t "$session" "C"
	sleep 0.5
	screen="$(capture "$session")"
	if ! grep -q "Elden Ring" <<<"$screen"; then
		echo "$screen" >&2
		fail "clear filters (C) did not restore Elden Ring"
	fi

	# Filter: hasProfile (Shift+P) — only Cyberpunk has a profile
	rmux send-keys -t "$session" "P"
	sleep 0.5
	screen="$(capture "$session")"
	if grep -q "Elden Ring" <<<"$screen"; then
		echo "$screen" >&2
		fail "hasProfile filter (P) did not hide Elden Ring"
	fi
	if grep -q "Witcher" <<<"$screen"; then
		echo "$screen" >&2
		fail "hasProfile filter (P) did not hide Witcher 3"
	fi
	if ! grep -q "Cyberpunk" <<<"$screen"; then
		echo "$screen" >&2
		fail "hasProfile filter (P) hid Cyberpunk"
	fi

	# Clear filters (C)
	rmux send-keys -t "$session" "C"
	sleep 0.5

	# Sort: cycle mode (s) and verify the first game line changes
	local before_sort after_sort
	before_sort="$(capture "$session")"
	rmux send-keys -t "$session" "s"
	sleep 0.5
	after_sort="$(capture "$session")"

	local before_first after_first
	before_first="$(grep -m1 -E 'Cyberpunk|Elden Ring|Witcher' <<<"$before_sort" || true)"
	after_first="$(grep -m1 -E 'Cyberpunk|Elden Ring|Witcher' <<<"$after_sort" || true)"
	[[ -n "$before_first" && -n "$after_first" ]] \
		|| fail "could not locate game lines for sort check"
	[[ "$before_first" != "$after_first" ]] \
		|| fail "sort (s) did not change first game line"

	ok "sidebar filters — d/P/C filters and s sort cycle work"

	rmux kill-session -t "$session" 2>/dev/null || true
	waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
}
qa_sidebar_filters

# --- QA: Multi-select and batch action menu ---
qa_multiselect() {
	local session="${SESSION}-multi"
	rmux kill-session -t "$session" 2>/dev/null || true
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$session")"
	grep -q "Cyberpunk 2077" <<<"$screen" || fail "TUI did not start (multi-select)"

	# Focus the sidebar (ZoneContext); cursor starts on "All games" (item 0).
	# Note: j/k are intercepted by profile-subsystem cycling when Profile aspect
	# is active (TUI starts on root defaults → Profile). Use search to position
	# the cursor on a game item, then clear the search with C to reveal all
	# games. Escape can't be used here — it's intercepted by the layout's
	# back-navigation handler, not the sidebar.
	rmux send-keys -t "$session" Tab
	sleep 0.5
	rmux send-keys -t "$session" "/" "Cyber" Enter
	sleep 0.5
	# Cursor is now on Cyberpunk (only filtered game). Clear search with C
	# (reaches sidebar since it's not intercepted by context_nav) to reveal
	# all games — cursor stays on Cyberpunk via cursor-restore logic.
	rmux send-keys -t "$session" "C"
	sleep 0.5

	# Toggle selection on Cyberpunk — activates select mode
	rmux send-keys -t "$session" Space
	sleep 0.5

	screen="$(capture "$session")"
	grep -q "Select" <<<"$screen" || fail "select mode indicator not visible after space"
	local count1
	count1="$(grep -oE 'Select \([0-9]+\)' <<<"$screen" | grep -oE '[0-9]+' | head -n1 || true)"
	count1="${count1:-0}"
	[[ "$count1" -ge 1 ]] || fail "expected at least 1 selected after space, got $count1"

	# Select all filtered games
	rmux send-keys -t "$session" "a"
	sleep 0.5

	screen="$(capture "$session")"
	grep -q "Select" <<<"$screen" || fail "select mode indicator not visible after 'a'"
	local count2
	count2="$(grep -oE 'Select \([0-9]+\)' <<<"$screen" | grep -oE '[0-9]+' | head -n1 || true)"
	count2="${count2:-0}"
	[[ "$count2" -gt "$count1" ]] || fail "select-all did not raise count (was $count1, now $count2)"

	# Open the batch action menu
	rmux send-keys -t "$session" Enter
	sleep 1

	screen="$(capture "$session")"
	if ! grep -qE "Batch action|Update all DLLs" <<<"$screen"; then
		echo "$screen" >&2
		fail "batch action menu did not open"
	fi

	# Close the batch action menu
	rmux send-keys -t "$session" Escape
	sleep 0.5

	screen="$(capture "$session")"
	if grep -q "Batch action" <<<"$screen"; then
		echo "$screen" >&2
		fail "batch action menu still visible after esc"
	fi

	ok "multi-select — space/a/enter/esc batch action flow works"

	rmux kill-session -t "$session" 2>/dev/null || true
	waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
}
qa_multiselect

# --- QA: Options modal save flow ---
qa_options_save() {
	local config_file="$XDG_CONFIG_HOME/spela/config.yaml"
	cp "$config_file" "$config_file.bak"

	local session="${SESSION}-options"
	rmux kill-session -t "$session" 2>/dev/null || true
	local waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done
	rmux new-session -d -s "$session" -x "$COLS" -y "$ROWS"
	rmux send-keys -t "$session" "$E2E_ENV $BINARY tui" Enter
	sleep "$WAIT"

	local screen
	screen="$(capture "$session")"
	grep -q "Cyberpunk 2077" <<<"$screen" || fail "TUI did not start (options save)"

	# Open the Settings destination (options modal, embedded mode).
	# Pressing "4" sets zone=Primary; Tab twice to reach ZoneContent where
	# the options modal accepts j/l/s keys.
	rmux send-keys -t "$session" "4"
	sleep 1
	rmux send-keys -t "$session" Tab
	sleep 0.3
	rmux send-keys -t "$session" Tab
	sleep 0.5

	screen="$(capture "$session")"
	if ! grep -qE "Show hints|Theme|Settings" <<<"$screen"; then
		echo "$screen" >&2
		fail "settings destination did not open"
	fi

	# Move cursor to show_hints (2nd option in Display section, index 1)
	rmux send-keys -t "$session" "j"
	sleep 0.3

	screen="$(capture "$session")"
	if ! grep -qE "Show hints|show_hints" <<<"$screen"; then
		echo "$screen" >&2
		fail "show_hints option not visible after moving cursor"
	fi

	# Cycle the bool value: true → false
	rmux send-keys -t "$session" "l"
	sleep 0.3

	# Save the change
	rmux send-keys -t "$session" "s"
	sleep 1

	# Poll config.yaml for the persisted change (async save)
	local found=""
	waited=0
	while [[ "$found" == "" ]]; do
		sleep 0.5
		found="$(grep 'show_hints: false' "$config_file" 2>/dev/null || true)"
		waited=$((waited + 1))
		[[ $waited -gt 6 ]] && break
	done
	[[ -n "$found" ]] || fail "show_hints: false not found in config.yaml after save"

	ok "options modal — j/l/s saves changed value to config.yaml"

	rmux kill-session -t "$session" 2>/dev/null || true
	waited=0
	while [[ -f "$RUNTIME_DIR/spela/spela.pid" ]]; do
		sleep 0.3; waited=$((waited + 1))
		[[ $waited -gt 10 ]] && break
	done

	# Restore original config
	cp "$config_file.bak" "$config_file"
}
qa_options_save

# Clean up extended QA sessions
for s in "${SESSION}-rootcycle" "${SESSION}-rootreset" "${SESSION}-pin" \
         "${SESSION}-filters" "${SESSION}-multi" "${SESSION}-options"; do
	rmux kill-session -t "$s" 2>/dev/null || true
done

# =====================================================================
# Pass
# =====================================================================
echo ""
echo "Running Go e2e tests..."
go test -tags e2e ./tests/e2e/... -count=1

echo "PASS: all UX and QA checks passed (session: $SESSION)"
