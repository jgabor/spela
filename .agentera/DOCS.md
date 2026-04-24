# Documentation Contract

<!-- Maintained by dokumentera. Last audit: 2026-04-24 -->

## Conventions

```
doc_root:     docs/
style:        technical, concise, section-based markdown with examples. No badges.
auto_gen:     none
versioning:
  version_files: [CHANGELOG.md]
  policy:        semver, bump on release via git-cliff + magefile
```

## Artifact Mapping

| Artifact      | Path                   | Producers              |
|---------------|------------------------|------------------------|
| VISION.md     | VISION.md              | visionera              |
| TODO.md       | TODO.md                | inspektera, manual     |
| CHANGELOG.md  | CHANGELOG.md           | magefile release, realisera |
| PROGRESS.md   | .agentera/PROGRESS.md  | realisera              |
| PLAN.md       | .agentera/PLAN.md      | planera, orkestrera    |
| HEALTH.md     | .agentera/HEALTH.md    | inspektera             |
| DOCS.md       | .agentera/DOCS.md      | dokumentera            |
| DESIGN.md     | .agentera/DESIGN.md    | visualisera            |

## Index

| Document               | Path                           | Last Updated | Status    |
|------------------------|--------------------------------|-------------|-----------|
| README                 | README.md                      | 2026-04-01  | ■ current |
| CLAUDE.md              | CLAUDE.md                      | 2026-04-01  | ■ current |
| Vision                 | VISION.md                      | 2026-04-01  | ■ current |
| Todo                   | TODO.md                        | 2026-04-24  | ■ current |
| Changelog              | CHANGELOG.md                   | 2026-04-24  | ■ current |
| Agentera Plan          | .agentera/PLAN.md              | 2026-04-24  | ■ current |
| Agentera Progress      | .agentera/PROGRESS.md          | 2026-04-24  | ■ current |
| Agentera Health        | .agentera/HEALTH.md            | 2026-04-24  | ■ current |
| Documentation Contract | .agentera/DOCS.md              | 2026-04-24  | ■ current |
| Design System          | .agentera/DESIGN.md            | 2026-04-24  | ■ current |
| NVAPI DRS Reference    | docs/NVAPI.md                  | 2026-03-02  | ■ current |
| Overlay Design (v1)    | docs/design/overlay.md         | 2026-03-17  | ■ current |
| Overlay Design (v0)    | docs/design/overlay-v0.md      | 2026-03-02  | ■ current |
| Overlay Design Review  | docs/design/overlay-review.md  | 2026-03-17  | ■ current |
