# Releasing spela

GitHub Actions is the only release publisher. A pushed `vX.Y.Z` tag starts
`.github/workflows/release.yml`. Local tooling never creates, replaces or
deletes a GitHub release.

The AUR has the separate `spela-git` VCS package. It follows the upstream
default branch, so it does not need an update for every release.

## Release contract

For a tag `vX.Y.Z`:

- The tag is newer than `v0.6.2`, the last Go release. Go's release check
  reads the same tags, so a Rust release must sort above every Go one.
- The workspace version in `Cargo.toml` is `X.Y.Z`. The workflow fails when
  the tag and the Cargo version differ.
- `CHANGELOG.md` has a reviewed `## [X.Y.Z] - YYYY-MM-DD` section, and the tag
  points at the commit that has it.
- The full Rust gate passes: `release.yml` calls `rust.yml` for format,
  clippy, the tests, the release perf report, MSRV and cargo-deny.
- The release has the executable `spela-linux-amd64`, built with
  `cargo build --release --locked -p spela --target x86_64-unknown-linux-gnu`,
  and `checksums.txt` with its SHA-256.
- The release body is exactly the matching changelog section body, without its
  heading and without any other section (`scripts/release-notes.sh`).
- The build sets `SPELA_VERSION` to `X.Y.Z`, the mark of a release build that
  the release check reads with `option_env!`.

Source and package builds name the binary `spela`. Only the GitHub release copy
is called `spela-linux-amd64`.

## Operator sequence

Pick `X.Y.Z` by semantic versioning, set it as `version` in
`[workspace.package]` in `Cargo.toml`, and run the gate:

```bash
VERSION=X.Y.Z
cargo update --workspace
just check
just deny
```

Edit the `## [X.Y.Z]` section of `CHANGELOG.md` by hand, with today's date in
its heading, until it is accurate and complete. `git cliff --tag "v$VERSION"
--unreleased` prints the commits since the last tag as a starting point. Do not
run `git cliff -o CHANGELOG.md`, which rewrites the whole file and drops the
hand-written sections. Check the notes the release will get:

```bash
sh scripts/release-notes.sh "v$VERSION"
```

Commit the version and the changelog, then tag that commit:

```bash
git add Cargo.toml Cargo.lock CHANGELOG.md
git commit -m "chore(release): v$VERSION"
git tag -a "v$VERSION" -m "Release v$VERSION"
git show --stat "v$VERSION"
```

Nothing so far publishes. When the commit and tag have been reviewed, push
them:

```bash
git push origin HEAD
git push origin "v$VERSION"
```

Pushing the tag is the publication boundary. Watch the tag's workflow in GitHub
Actions. Do not recreate a failed release locally: fix the cause and use a new
version, so published history stays immutable.

## Source installs

```bash
just build-release
sudo just install
```

`just install` copies `target/release/spela` to `/usr/bin/spela` and the polkit
policy to `/usr/share/polkit-1/actions/io.github.spela.policy`, the paths the
AUR package uses. Steam runs launch options with `PATH=/usr/bin:/bin`, and the
policy names `/usr/bin/spela`, so the paths are fixed. `just install <dir>`
stages the same files under `<dir>` instead.

## AUR package

`pkg/aur/PKGBUILD` is the source of truth for `spela-git`, and
`pkg/aur/.SRCINFO` is its generated form. The package builds with cargo:
`prepare()` fetches the locked crates and `build()` runs
`cargo build --frozen --release -p spela` with `SPELA_VERSION` set to the
pkgver, such as `0.7.0.r6.g087d1c4`, as Go's AUR build did. spela reports that
version, and the TUI's release check compares its release part, `0.7.0`, with
the tags (decision 68). It installs the binary, the polkit policy and the
licence. Submit the files only when the package metadata or
build recipe changes, not for ordinary commits or tags. The AUR builds the
upstream default branch, so submit the cargo recipe only once the Rust tree is
on it.

Check that `.SRCINFO` matches the PKGBUILD after editing it:

```bash
(cd pkg/aur && makepkg --printsrcinfo | diff -u .SRCINFO -)
```

Look at the exact AUR diff first, without publishing:

```bash
./scripts/aur-submit.sh
```

The script refuses to go on unless SSH authenticates to the AUR as the
`jgabor` account. `AUR_SSH_TARGET` may select a dedicated SSH host alias, but
that alias must authenticate as `jgabor`. After reviewing the dry run, publish:

```bash
./scripts/aur-submit.sh --publish
```

Both runs fetch the PKGBUILD's GitHub source and remote tags into fresh
temporary storage. `makepkg --nobuild --noprepare --nodeps` runs `pkgver()` and
updates the temporary PKGBUILD, and `makepkg --printsrcinfo` then writes the
matching metadata. This needs makepkg and network access, but compiles nothing,
fetches no crates and installs no dependencies. Local commits and tags do not
decide the version. A publish prepares again, so upstream changes since the
preview can change its diff.

Source, build and output folders stay inside the temporary workspace even when
the makepkg configuration names shared folders. The checkout's PKGBUILD,
`.SRCINFO` and sources are untouched, and the temporary files are removed on
exit. A preparation failure stops before any commit or push. Only PKGBUILD and
`.SRCINFO` are staged, and an unchanged result neither commits nor pushes.
`AUR_COMMIT_NAME` and `AUR_COMMIT_EMAIL` override the Git identity for the
publish.

## DLL manifest

`data/manifest.json` on `main` is fetched by every installed spela, Go and
Rust, so its URL and schema do not change (decision 14). Two workflows feed it,
both through the Rust `dlss-updater` in `crates/dlss-updater`:

- `dll-manifest.yml` runs daily, on dispatch and on pushes to `main`. For each
  DLSS family it reads the TechPowerUp page with `dlss-updater -json -type
  <type>`. Exit 0 means a newer version than the manifest's highest, 2 means up
  to date, and anything else fails the run. A new DLL is downloaded, released
  as `dll-<type>-v<version>` in this repository, and added with
  `dlss-updater add`.
- `dll-release.yml` releases a DLL by hand to `jgabor/spela-dlls` and adds it
  the same way.

`dlss-updater add` writes the family's real file name, such as
`nvngx_dlss.dll`, reads the SHA-256 and size from the DLL, replaces an entry of
the same version, and keeps each family newest first, so an older release never
becomes the first entry Go clients take as the latest. It writes the manifest
with spela's own types, so the first change after the switch also sorts the
family keys by name.

## Nonpublishing verification

The release contract tests in `crates/dlss-updater/tests/` read the workflows,
the PKGBUILD and the justfile, run the tag validation and the version check
with hostile input, extract release notes, stage `just install` in a temp
folder, and run `aur-submit.sh` against fake ssh and git with the real
makepkg. They run in `just check`, create no tags, releases or AUR commits,
and reach no network. The AUR test is skipped where makepkg is missing.

```bash
cargo test -p dlss-updater
```

The tag workflow builds from a clean checkout. To do the same by hand:

```bash
dir=$(mktemp -d)
git archive --format=tar HEAD | tar -x -C "$dir"
cd "$dir"
SPELA_VERSION=X.Y.Z cargo build --release --locked -p spela --target x86_64-unknown-linux-gnu
cp target/x86_64-unknown-linux-gnu/release/spela spela-linux-amd64
sha256sum spela-linux-amd64 > checksums.txt
sha256sum -c checksums.txt
```
