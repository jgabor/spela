# Plan: GPU Power Limit Profile Support

<!-- Level: light | Created: 2026-04-01 | Status: active -->

## What

Add per-game GPU power limit to the profile system so users can set a custom power limit (watts) that applies at launch and restores on exit — completing the GPU tuning story alongside existing clock offsets.

## Why

VISION.md "NVIDIA depth": expose every tunable the driver offers. The GPU setter and privileged command already exist but the profile can't trigger them. Users who overclock need per-game power limits alongside clock offsets — one without the other is incomplete.

## Constraints

- Existing clock offset and CPU profile behavior must not change
- Power limit restore must capture and restore the previous value, not reset to a fixed default
- Profile YAML format must remain backward-compatible (new field with zero value = no change)

## Acceptance Criteria

▸ GIVEN a profile with a power limit set WHEN a game launches THEN the GPU power limit changes to the configured value
▸ GIVEN a game exits WHEN the power limit was changed at launch THEN the original power limit is restored
▸ GIVEN the CLI WHEN `spela gpu set --power-limit 300` is run THEN the profile saves the power limit
▸ GIVEN the TUI profile widget WHEN navigating GPU settings THEN a power limit field is visible and editable
▸ GIVEN a profile with power limit zero WHEN a game launches THEN the GPU power limit is not modified

## Surprises
