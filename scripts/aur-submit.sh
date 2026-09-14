#!/usr/bin/env bash

set -euo pipefail

readonly package_name=spela-git
readonly expected_account=jgabor
readonly ssh_target="${AUR_SSH_TARGET:-aur@aur.archlinux.org}"
readonly remote="ssh://${ssh_target}/${package_name}.git"
readonly git_ssh_command="ssh -o BatchMode=yes -o ConnectTimeout=10 -o ConnectionAttempts=1"

publish=false
case "${1:-}" in
"") ;;
--publish) publish=true ;;
-h | --help)
  printf 'Usage: %s [--publish]\n' "${0##*/}"
  exit 0
  ;;
*)
  printf 'Usage: %s [--publish]\n' "${0##*/}" >&2
  exit 2
  ;;
esac

for command_name in git makepkg ssh; do
  if ! command -v "$command_name" >/dev/null; then
    printf 'Required command not found: %s\n' "$command_name" >&2
    exit 1
  fi
done

repository_root=$(git rev-parse --show-toplevel)
pkgbuild="$repository_root/pkg/aur/PKGBUILD"
if [[ ! -f $pkgbuild ]]; then
  printf 'PKGBUILD not found: %s\n' "$pkgbuild" >&2
  exit 1
fi

auth_output=$(ssh -o BatchMode=yes -o ConnectTimeout=10 -o ConnectionAttempts=1 \
  -T "$ssh_target" </dev/null 2>&1 || true)
if [[ $auth_output != *"Welcome to AUR, ${expected_account}!"* ]]; then
  printf 'Refusing AUR submission: %s is not authenticated as %s.\n' \
    "$ssh_target" "$expected_account" >&2
  exit 1
fi

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/spela-aur.XXXXXX")
trap 'rm -rf "$temporary_directory"' EXIT
work_directory="$temporary_directory/aur"
preparation_directory="$temporary_directory/prepare"

GIT_SSH_COMMAND="$git_ssh_command" git clone "$remote" "$work_directory"
install -Dm644 "$pkgbuild" "$preparation_directory/PKGBUILD"
(
  cd "$preparation_directory"
  # Command-line assignments override both makepkg.conf and environment settings.
  # Keep both the source mirror and extracted checkout fresh and disposable.
  makepkg_directories=(
    "SRCDEST=$preparation_directory" "BUILDDIR=$preparation_directory"
    "PKGDEST=$preparation_directory" "SRCPKGDEST=$preparation_directory"
    "LOGDEST=$preparation_directory"
  )
  makepkg --nobuild --nodeps "${makepkg_directories[@]}"
  makepkg --printsrcinfo "${makepkg_directories[@]}" >.SRCINFO
)
install -m644 "$preparation_directory/PKGBUILD" "$preparation_directory/.SRCINFO" "$work_directory/"

git -C "$work_directory" add PKGBUILD .SRCINFO
git -C "$work_directory" diff --cached --check
if git -C "$work_directory" diff --cached --quiet; then
  printf '%s is already current in the AUR.\n' "$package_name"
  exit 0
fi

if [[ $publish == false ]]; then
  git -C "$work_directory" diff --cached
  printf 'Dry run only. Re-run with --publish to submit as %s.\n' "$expected_account"
  exit 0
fi

commit_name=${AUR_COMMIT_NAME:-$(git config user.name)}
commit_email=${AUR_COMMIT_EMAIL:-$(git config user.email)}
if [[ -z $commit_name || -z $commit_email ]]; then
  printf 'Set AUR_COMMIT_NAME and AUR_COMMIT_EMAIL before publishing.\n' >&2
  exit 1
fi

git -C "$work_directory" \
  -c user.name="$commit_name" \
  -c user.email="$commit_email" \
  commit -m "Update $package_name"
GIT_SSH_COMMAND="$git_ssh_command" git -C "$work_directory" push origin HEAD:master
printf 'Published %s to the AUR as %s.\n' "$package_name" "$expected_account"
