# Releasing Spela

GitHub Actions is the only release publisher. A pushed `v*` tag starts
`.github/workflows/release.yml`; local tooling must not create, replace, or
delete GitHub releases or mutate AUR repositories.

## Release contract

For a tag `vX.Y.Z`:

- `CHANGELOG.md` has a reviewed `## [X.Y.Z] - YYYY-MM-DD` section.
- The tag points at the commit containing that changelog section.
- The GitHub release contains the executable `spela-linux-amd64` and
  `checksums.txt` with its SHA-256 checksum.
- The release body is exactly the matching changelog section body (not the
  heading and not an adjacent release).
- The stable AUR publication uses `pkg/aur/PKGBUILD` with `pkgver=X.Y.Z` and
  the tag archive checksum. The development publication uses
  `pkg/aur/PKGBUILD-git` unchanged.
- CI, the tag workflow, source builds, and both PKGBUILDs use the `Build` Mage
  target as the canonical generation, frozen frontend install, frontend build,
  and embedded Go binary path. Build tags and linker flags live only there.

The resulting binary remains named `spela` for source and package builds. Only
the GitHub release copy is renamed to `spela-linux-amd64`.

## Operator sequence

Choose `X.Y.Z` according to semantic versioning, then generate and review the
changelog before making any release commit or tag:

```bash
VERSION=X.Y.Z
git cliff --tag "v$VERSION" -o CHANGELOG.md
git diff --check
git diff -- CHANGELOG.md
go tool mage coverageCheck
go tool mage test
go tool mage lint
go tool mage build
git status --short
```

Edit `CHANGELOG.md` until its release body is accurate and complete. Commit the
reviewed changelog explicitly, then create an annotated tag at that commit:

```bash
git add CHANGELOG.md
git commit -m "chore(release): v$VERSION"
git tag -a "v$VERSION" -m "Release v$VERSION"
git show --stat "v$VERSION"
```

The commands above do not publish. When the commit and tag have been reviewed,
push them explicitly:

```bash
git push origin HEAD
git push origin "v$VERSION"
```

Pushing the tag is the publication boundary. Monitor the tag workflow in
GitHub Actions. Do not recreate a failed release locally: correct the cause and
use a new version/tag so published history remains immutable.

## Nonpublishing verification

The release contract tests inspect the workflow and PKGBUILDs, extract release
notes, and build/checksum the artifact from a clean tracked-source snapshot.
They do not create tags, GitHub releases, or AUR commits.

```bash
go test ./tests/contracts/...
(cd pkg/aur && makepkg --printsrcinfo -p PKGBUILD)
(cd pkg/aur && makepkg --printsrcinfo -p PKGBUILD-git)
```
