# Spela

## North Star

Linux gamers who left Windows behind shouldn't leave their performance behind too.
Spela is the hardware layer that Windows never needed to build — because NVIDIA
Control Panel, DLSS Swapper, and SpecialK already existed there. On Linux, that
tooling is scattered across shell scripts, manual DLL copies, and tribal knowledge.
Spela replaces all of it with a single tool that gives power users surgical control
over every knob between their hardware and their games — and remembers what works.

## Who It's For

**The dual-booter who stopped dual-booting.** They ran Windows for years. They know
what 144fps with DLSS Quality looks like. They know their 4090 can hold +150 core
with stable thermals. They switched to Linux full-time because they believe in it,
but they refuse to accept that "gaming on Linux" means less control than they had
before. They don't want magic — they want the same level of precision they had on
Windows, without the OS.

Their frustration: every new game means the same ritual. Find the prefix, check
which DLSS version shipped, swap the DLL, set the env vars, configure the shader
cache, hope Proton didn't change something since last time. It's not hard — it's
tedious. And tedium compounds across a library of 50+ games.

## Principles

- **Correctness over convenience.** Never guess. If Spela sets a value, it's the
  right value. Backups before mutations, verification after. No silent failures,
  no "close enough."
- **Transparency over magic.** Show everything Spela does — every env var set,
  every DLL swapped, every clock offset applied. The user should be able to
  reproduce any action by hand.
- **Composability over features.** Small, orthogonal pieces that combine. Profiles,
  launching, DLL management, hardware control, and the overlay are separate concerns
  that compose. Resist monolithic "do everything" commands.
- **Parity over novelty.** The bar is Windows tooling (NVIDIA Control Panel, DLSS
  Swapper, SpecialK, MSI Afterburner). Match that capability on Linux before
  inventing new things.

## Direction

**Next-generation overlay.** A Vulkan layer that replaces MangoHud — not by doing
the same thing prettier, but by being fundamentally smarter. "Visually smart,
logically dumb": a thin rendering layer (C++/ImGui with SDF fonts and custom GPU
charts) paired with a Go intelligence process connected via shared memory. Runtime
GPU tuning from inside the game — clock offsets, power limits, fan curves — through
NVML setters, something no other Linux tool offers. Smart alerts that detect thermal
throttling and suggest fixes. Session comparison that tells you when performance
regresses. The overlay doesn't just show numbers; it understands what they mean.

**Full hardware control.** Spela grows beyond NVIDIA into AMD and Intel GPUs.
Deeper CPU tuning — schedutil, CFS bandwidth, NUMA awareness. Fan curves, power
profiles, thermal management. Spela becomes the definitive hardware abstraction
layer for Linux gaming, where every tunable the kernel exposes is accessible
through a clean, composable interface.

**Game intelligence.** Spela learns. Community-shared profiles so a new Cyberpunk
player doesn't start from scratch. Per-game recommendations based on hardware —
"your 4070 with this game at 1440p runs best with DLSS Balanced." Automatic
detection of what settings a game actually supports. Knowledge that compounds
across the community, not just across one user's library.
