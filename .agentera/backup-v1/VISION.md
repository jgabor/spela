# Spela

## North Star

Linux gamers who left Windows behind should not leave control behind too.

On Windows, NVIDIA App turns a game into one remembered intent: profile, DLSS,
driver settings, overlay, and launch behavior move together. On Linux, that
intent fractures across MangoHud, LACT, ProtonPlus, DLSS tools, GameMode, Steam
launch options, and hand-edited environment variables. The tools exist, but the
game does not have one trusted home.

Spela makes the game profile that home. Configure the game once, launch it the
normal Steam way with `spela %command%`, and trust Spela to prepare the session,
explain what changed, watch what happens, and restore the system afterward.

The dream is not another launcher. The dream is a Linux gaming rig that feels
deliberate: every game carries its own known-good operating envelope, every
change is reversible, and performance knowledge compounds instead of leaking
into scattered config files.

## Who It's For

### The Dual-Booter Who Stopped Dual-Booting

They ran Windows for years. They know what 144fps with DLSS Quality looks like.
They know their GPU can hold a stable offset with safe thermals. They switched
to Linux full-time because they believe in it, but they refuse to accept less
control than they had before.

Their frustration: every new game repeats the same ritual. Find the prefix,
check which DLSS DLL shipped, set compatibility flags, tune the GPU, shape the
overlay, and hope Proton did not change the ground under them. None of it is
impossible. All of it is waste.

### The Linux-Native Who Got Serious Hardware

They never dual-booted. They bought a powerful NVIDIA GPU and discovered that
Linux gaming performance is not one control surface. MangoHud shows the problem.
LACT changes hardware state. Proton tools change compatibility. Steam launch
options carry brittle one-off knowledge.

Their frustration: the tools do not share intent. A GPU profile does not know
which game needs it. A DLSS swap does not know which overlay should report it.
An environment tweak works until they forget why it exists.

### The Steam Wrapper User

They do not want another place to launch games. Their library already lives in
Steam, and their muscle memory should stay there. They want to add
`spela %command%`, configure the profile once, and keep pressing Play.

Their frustration: launchers that pretend to own the session but cannot track
the real process. They would rather have honest wrapper behavior than a polished
button that skips cleanup.

## Principles

- **Correctness over convenience.** Never mutate system or game state without a safe restore path.
- **Transparency over magic.** Show every env var, DLL, profile value, warning, and privileged change.
- **Profile as source of truth.** One per-game intent owns launch preparation, DLLs, hardware, overlay, and environment.
- **Wrapper-first honesty.** Steam `%command%` is the preferred path; Spela avoids unsafe launch claims.
- **Unity over fragmentation.** Resist becoming another single-purpose tool in the Linux gaming pile.
- **Depth over breadth.** Master NVIDIA, DLSS, NVML, and Proton edges before chasing vendor parity.

## Direction

**Trusted per-game profiles.** Spela's core promise is one profile that composes
DLSS, GPU, CPU, Proton, overlay, and environment behavior. Defaults remain live.
Game overrides are explicit. Every interface sees the same resolved truth.

**Steam-native lifecycle.** Spela is not trying to replace Steam. The primary
path is the wrapper: `spela %command%`. That lets Spela prepare before the game,
preserve Steam's command environment, run the real process, and clean up when it
exits. If Spela cannot honestly track lifetime, it says so.

**Resource-centric control.** The TUI and GUI are configuration and inspection
surfaces, not launchers. Games, DLLs, Defaults, and Metrics are peer resources.
The UI exists to show state, edit intent, expose inheritance, and report what
the wrapper will do later.

**Next-generation overlay.** The long frontier is a Vulkan overlay that does not
only display numbers. A thin rendering layer and a Go intelligence process can
share live telemetry, detect throttling, explain regressions, and eventually let
users tune GPU behavior from inside the session. MangoHud reports. Spela should
understand.

**DLSS and frame-generation intelligence.** DLSS is no longer one DLL swap. DLSS
4, model presets, Ray Reconstruction, Frame Generation, and Multi Frame
Generation turn upscaling into policy. Spela should know which model belongs to
which game, GPU, resolution, and user preference, then make that choice visible
and reversible.

**NVIDIA depth.** Spela remains NVIDIA-first. Deep NVML integration, direct
metrics, throttle reasons, clock offsets, power limits, fan control, and
driver-specific behavior matter more than shallow cross-vendor checkboxes. AMD
and Intel can come later, after NVIDIA control is exhaustive.

**Community memory.** The horizon is shared performance knowledge: known-good
profiles, DLSS recommendations, compatibility notes, and session comparisons
that help the next player start from evidence instead of folklore.

## Identity

### Personality

▸ precise · honest · unapologetic

### Voice

Spela speaks like a technical friend who respects your time. It names the real
condition, the real action, and the real risk. No hedging, no marketing gloss,
no generic failure text.

It says "direct Steam URI launch cannot track cleanup" instead of pretending a
button is safe. It says which DLSS model is active, which value is inherited,
and which privileged change will happen before it asks for trust.

### Emotional Register

Control room at night. Dark, precise, dense with useful signal. Spela should
feel grounding: the moment where a messy Linux gaming stack becomes one
instrument panel and every indicator means something.

### Naming

▸ Swedish: *spela* means "to play"; direct, plain, and rooted.
▸ Internal names use full words over abbreviations.
▸ Cleverness loses to clarity whenever a user or agent must act on the name.
