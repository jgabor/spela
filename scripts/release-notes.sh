#!/bin/sh
set -eu

tag=${1-}
if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
  printf 'invalid release tag %s; expected vX.Y.Z\n' "$tag" >&2
  exit 2
fi
version=${tag#v}

# The section must be `## [X.Y.Z] - YYYY-MM-DD` (docs/releasing.md): an
# `Unreleased` or undated heading is refused, so a tag cannot publish notes
# that were never dated.
awk -v version="$version" '
  index($0, "## [" version "]") == 1 {
    rest = substr($0, length("## [" version "]") + 1)
    if (rest !~ /^ - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/) {
      printf "CHANGELOG.md: the %s heading must be \"## [%s] - YYYY-MM-DD\", not \"%s\"\n", version, version, $0 > "/dev/stderr"
      undated = 1
      exit 1
    }
    found = 1
    next
  }
  found && /^## \[/ { exit }
  found { print; if (NF) content = 1 }
  END { if (undated || !found || !content) exit 1 }
' CHANGELOG.md
