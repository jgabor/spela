#!/usr/bin/env python3
"""Enforce repository aggregate LCOV line coverage."""

import argparse
from pathlib import Path


def totals(path):
    found = hit = 0
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        if line.startswith("LF:"):
            found += int(line[3:])
        elif line.startswith("LH:"):
            hit += int(line[3:])
    if not found:
        raise ValueError(f"no LCOV line records in {path}")
    return found, hit


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("coverage_file")
    parser.add_argument("--minimum", type=float, default=90.0)
    args = parser.parse_args()
    found, hit = totals(args.coverage_file)
    percent = 100 * hit / found
    print(f"aggregate line coverage: {hit}/{found} ({percent:.2f}%)")
    if percent < args.minimum:
        raise SystemExit(
            f"line coverage {percent:.2f}% is below required {args.minimum:.2f}%"
        )


if __name__ == "__main__":
    main()
