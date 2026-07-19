#!/bin/sh
set -eu

tag=${1-}
if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
  printf 'invalid release tag %s; expected vX.Y.Z\n' "$tag" >&2
  exit 2
fi
version=${tag#v}

awk -v version="$version" '
  $0 == "## [" version "]" || index($0, "## [" version "] - ") == 1 { found = 1; next }
  found && /^## \[/ { exit }
  found { print; if (NF) content = 1 }
  END { if (!found || !content) exit 1 }
' CHANGELOG.md
