# Spela

## North Star

Linux gamers who left Windows behind shouldn't leave their performance behind too.

On Windows, NVIDIA App gives you per-game profiles, DLSS model selection, and an
overlay — one tool, one config. On Linux, that experience is scattered across four
or five tools that don't talk to each other: MangoHud for the overlay, LACT or GWE
for GPU tuning, DLSS Updater for DLL swaps, GameMode for system tweaks, and manual
env vars for everything else. Every new game means touching three configs.

Spela replaces the juggle. One tool that manages DLSS DLLs, applies GPU and CPU
profiles, launches games with the right environment, and renders an intelligent
overlay — all from a single per-game config that remembers what works. The NVIDIA
App experience, native on Linux, for people who refuse to compromise.

## Who It's For

### The Dual-Booter Who Stopped Dual-Booting

They ran Windows for years. They know what 144fps with DLSS Quality looks like.
They know their 4090 can hold +150 core with stable thermals. They switched to
Linux full-time because they believe in it, but they refuse to accept that "gaming
on Linux" means less control than they had before. They don't want magic — they want
the same level of precision they had on Windows, without the OS.

Their frustration: every new game means the same ritual. Find the prefix, check
which DLSS version shipped, swap the DLL, set the env vars, configure the shader
cache, hope Proton didn't change something since last time. It's not hard — it's
tedious. And tedium compounds across a library of 50+ games.

### The Linux-Native Who Got Serious Hardware

They never dual-booted. They grew up on Linux, gamed on whatever worked, and
didn't think much about GPU tuning. Then they bought a 4070 Ti and realized they
had no idea how to make it perform. They don't miss NVIDIA Control Panel because
they never used it — they just know that MangoHud shows their GPU is throttling and
they have no single place to fix it. They installed LACT for clocks, GOverlay for
the HUD config, and still copy DLLs by hand from a Reddit thread.

Their frustration: not that the tools don't exist, but that the tools don't know
about each other. Changing a GPU profile in LACT doesn't update the overlay.
Swapping a DLSS DLL doesn't update the game's launch config. Every knob lives in
a different app with a different config format. They want one place.

## Principles

- **Correctness over convenience.** Never guess. If Spela sets a value, it's the
  right value. Backups before mutations, verification after. No silent failures,
  no "close enough."
- **Transparency over magic.** Show everything Spela does — every env var set,
  every DLL swapped, every clock offset applied. The user should be able to
  reproduce any action by hand.
- **Unity over fragmentation.** One profile, one config, one tool. Resist becoming
  another single-purpose utility in a five-tool stack.
- **Composability over monoliths.** Small, orthogonal pieces that combine. Profiles,
  launching, DLL management, hardware control, and the overlay are separate concerns
  that compose — but they compose inside Spela, not across five apps.
- **Depth over breadth.** Go deep on NVIDIA before going wide on AMD. Master NVML
  setters, runtime tuning, and driver-level intelligence before chasing vendor parity.

## Direction

**Unified control.** Spela's core promise: one per-game profile that owns DLSS
configuration, GPU clocks, CPU governor, environment variables, overlay position,
and launch parameters. Change one setting, and every layer — CLI, TUI, GUI, overlay,
launcher — sees it. No export, no sync, no "also update your MangoHud config."
ProtonForge and LACT each cover a slice; Spela covers the stack.

**Next-generation overlay.** A Vulkan layer that replaces MangoHud — not by doing
the same thing prettier, but by being fundamentally smarter. A thin C rendering
layer paired with a Go intelligence process connected via shared memory. Runtime
GPU tuning from inside the game — clock offsets, power limits, fan curves — through
NVML setters, something no other Linux tool offers. Smart alerts that detect thermal
throttling and suggest fixes. Session comparison that tells you when performance
regresses. The overlay doesn't just show numbers; it understands what they mean.
This is SpecialK's territory on Windows — unclaimed on Linux.

**NVIDIA depth.** Spela is an NVIDIA-first tool. Deep NVML integration: direct
API calls for metrics (~50x faster than nvidia-smi), driver-reported throttle
reasons, runtime clock and power limit adjustment via privileged setters. The goal
is to expose every tunable NVIDIA's driver offers through a clean interface — the
control that NVIDIA App provides on Windows, but with the transparency and
composability that Linux users expect. AMD and Intel come later, after NVIDIA
depth is exhaustive.

**Game intelligence.** Spela learns. Community-shared profiles so a new Cyberpunk
player doesn't start from scratch. Per-game recommendations based on hardware —
"your 4070 with this game at 1440p runs best with DLSS Balanced." Automatic
detection of what settings a game actually supports. Knowledge that compounds
across the community, not just across one user's library. This is the long horizon
— it requires a critical mass of profiles and users to be meaningful.

## Identity

### Personality

▸ precise · transparent · unapologetic

### Voice

Technical and direct. Spela talks like a knowledgeable friend who respects your
time — no marketing fluff, no hedging, no "we recommend." It says "DLSS 3.8.10
outperforms 3.7.20 on Ada at 1440p" not "you might want to try updating your DLSS."
When something fails, it says what failed and why, not "an error occurred."

### Emotional Register

Empowering. Using Spela should feel like gaining control you didn't know you were
missing. The satisfaction of seeing every knob in one place, clearly labeled, doing
exactly what you told it to. Not exciting — grounding. The feeling of "I understand
my system now."

### Naming

▸ Swedish: *spela* means "to play" — direct, unpretentious, rooted in the language
  of the developer
▸ Internal naming follows the same ethos: plain words, no abbreviations, no cleverness
  for its own sake
