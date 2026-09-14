# Releasing Spela

GitHub Actions is the only versioned release publisher. A pushed `v*` tag starts
`.github/workflows/release.yml`; local tooling must not create, replace, or
delete GitHub releases.

The AUR contains the independent `spela-git` VCS package. It follows the latest
upstream branch and therefore does not need an update for every Spela release.

## Release contract

For a tag `vX.Y.Z`:

- `CHANGELOG.md` has a reviewed `## [X.Y.Z] - YYYY-MM-DD` section.
- The tag points at the commit containing that changelog section.
- The GitHub release contains the executable `spela-linux-amd64` and
  `checksums.txt` with its SHA-256 checksum.
- The release body is exactly the matching changelog section body (not the
  heading and not an adjacent release).
- CI, the tag workflow, source builds, and the AUR PKGBUILD use the `Build` Mage
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

## AUR package

`pkg/aur/PKGBUILD` is the source of truth for the `spela-git` package;
`pkg/aur/.SRCINFO` is its checked-in generated representation. Submit them only
when the package metadata or build recipe changes, not for ordinary upstream
commits or tags.

First inspect the exact AUR diff without publishing:

```bash
./scripts/aur-submit.sh
```

The script refuses to continue unless SSH authenticates to the AUR as the
separate `jgabor` account. `AUR_SSH_TARGET` may select a dedicated SSH host
alias, but that alias must authenticate as `jgabor`. After reviewing the dry
run, publish explicitly:

```bash
./scripts/aur-submit.sh --publish
```

Both invocations fetch the PKGBUILD's GitHub source and remote tags into fresh
temporary storage. `makepkg --nobuild --nodeps` runs the existing `pkgver()` and
updates the temporary PKGBUILD; `makepkg --printsrcinfo` then generates matching
metadata. This requires makepkg and network access, but does not compile Spela
or install dependencies. Local-only commits and tags do not determine the version.
Publish prepares again, so upstream changes since the preview can change its diff.

Source/build/output directories are pinned inside the temporary workspace even
when makepkg configuration specifies shared directories. The original checkout's
PKGBUILD, `.SRCINFO`, and sources are untouched, and temporary files are removed
on exit. Preparation failures stop before committing or pushing. Only PKGBUILD
and `.SRCINFO` are staged; an unchanged result does not commit or push.
`AUR_COMMIT_NAME` and `AUR_COMMIT_EMAIL` override the Git identity for publication.

## Nonpublishing verification

The release contract tests inspect the workflow and PKGBUILD, extract release
notes, and build/checksum the artifact from a clean tracked-source snapshot.
They do not create tags, GitHub releases, or AUR commits.

```bash
go test ./tests/contracts/...
(cd pkg/aur && makepkg --printsrcinfo | diff -u .SRCINFO -)
./scripts/aur-submit.sh
```
