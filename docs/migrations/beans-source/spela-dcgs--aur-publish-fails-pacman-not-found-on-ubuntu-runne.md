---
# spela-dcgs
title: Publish spela-git to AUR
status: in-progress
type: bug
priority: normal
created_at: 2026-03-02T14:37:47Z
updated_at: 2026-09-01T16:11:26Z
---

The previous release-time AUR action never created either package: it first depended on pacman on an Ubuntu runner, then could not use the passphrase-protected SSH key. The README consequently advertised packages that do not exist.

The prepared replacement follows nvidiactl's proven package-specific AUR Git flow while using the separate jgabor account. Spela now has one honest VCS package, checked-in PKGBUILD/.SRCINFO metadata, and a dry-run-first submit script that verifies the authenticated AUR account is jgabor and rejects aur-mutker. Wails is upgraded from 2.12 to 2.15 so clean builds work with Arch's Go 1.27.

## Checklist

- [x] Replace the failing release-time deploy action with direct AUR Git submission
- [x] Remove the nonexistent stable package claim from README.md
- [x] Validate PKGBUILD, .SRCINFO, account guard, clean source build, package build, tests, lint, and vulnerabilities
- [ ] Commit and push the prepared Spela changes; origin/main is currently 49 commits behind
- [ ] Run ./scripts/aur-submit.sh --publish as jgabor
- [ ] Verify the AUR RPC returns spela-git and test paru -S spela-git
