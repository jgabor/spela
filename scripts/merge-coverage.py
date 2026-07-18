#!/usr/bin/env python3
"""Convert Go coverprofiles to source-line LCOV and merge frontend LCOV."""

import argparse
import re
from collections import defaultdict
from pathlib import Path, PurePosixPath


DELIMITERS_ONLY = re.compile(r"^[{}()\[\],;]*$")


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--go", action="append", default=[], dest="go_profiles")
    parser.add_argument("--frontend")
    parser.add_argument("--output", required=True)
    parser.add_argument("--root", default=".")
    return parser.parse_args()


def module_path(root):
    for line in (root / "go.mod").read_text(encoding="utf-8").splitlines():
        if line.startswith("module "):
            return line.split(None, 1)[1].strip()
    raise ValueError(f"module declaration not found in {root / 'go.mod'}")


def normalize_source(source, root, module=None, base=None):
    """Return a repository-relative POSIX source path."""
    source = source.replace("\\", "/")
    candidate = Path(source)
    if candidate.is_absolute():
        try:
            return candidate.resolve().relative_to(root).as_posix()
        except ValueError as error:
            raise ValueError(f"coverage source is outside repository: {source}") from error

    if module and (source == module or source.startswith(module + "/")):
        source = source[len(module) :].lstrip("/")
    elif base:
        base_path = PurePosixPath(base)
        source_path = PurePosixPath(source)
        if not source_path.parts[: len(base_path.parts)] == base_path.parts:
            source = (base_path / source_path).as_posix()

    normalized = Path(source)
    if ".." in normalized.parts:
        raise ValueError(f"coverage source escapes repository: {source}")
    return normalized.as_posix().lstrip("./")


def uncomment(source):
    """Remove Go comments while preserving strings and source line boundaries."""
    result = []
    index = 0
    block_comment = False
    quote = None
    escaped = False
    while index < len(source):
        char = source[index]
        following = source[index + 1] if index + 1 < len(source) else ""
        if block_comment:
            if char == "*" and following == "/":
                block_comment = False
                result.extend("  ")
                index += 2
            else:
                result.append("\n" if char == "\n" else " ")
                index += 1
            continue
        if quote:
            result.append(char)
            if quote != "`" and escaped:
                escaped = False
            elif quote != "`" and char == "\\":
                escaped = True
            elif char == quote:
                quote = None
            index += 1
            continue
        if char in ('"', "'", "`"):
            quote = char
            result.append(char)
            index += 1
        elif char == "/" and following == "/":
            while index < len(source) and source[index] != "\n":
                result.append(" ")
                index += 1
        elif char == "/" and following == "*":
            block_comment = True
            result.extend("  ")
            index += 2
        else:
            result.append(char)
            index += 1
    return "".join(result)


def executable_lines(source_text, start_line, start_column, end_line, end_column):
    """Return executable physical lines in a Go coverprofile source span.

    Coverprofile columns are byte offsets, one-based, and the end position is
    exclusive. Coverage blocks may contain several statements on several lines;
    LCOV must therefore record every non-comment, non-delimiter source line in
    the span rather than only its first line.
    """
    source_lines = source_text.encode("utf-8").splitlines(keepends=True)
    selected = []
    for line_number in range(start_line, end_line + 1):
        if line_number < 1 or line_number > len(source_lines):
            raise ValueError(f"coverage span references missing line {line_number}")
        line = source_lines[line_number - 1]
        start = start_column - 1 if line_number == start_line else 0
        end = end_column - 1 if line_number == end_line else len(line)
        selected.append(line[start:end])

    try:
        selected_text = b"".join(selected).decode("utf-8")
    except UnicodeDecodeError as error:
        raise ValueError("coverage span splits a UTF-8 code point") from error
    cleaned = uncomment(selected_text).splitlines()
    result = set()
    for offset, line in enumerate(cleaned):
        compact = "".join(line.split())
        if compact and not DELIMITERS_ONLY.fullmatch(compact):
            result.add(start_line + offset)
    return result


def parse_go_profiles(paths, root):
    lines = defaultdict(dict)
    module = module_path(root)
    sources = {}
    record_pattern = re.compile(
        r"^(.*):(\d+)\.(\d+),(\d+)\.(\d+)\s+(\d+)\s+(\d+)$"
    )
    for profile_path in paths:
        with Path(profile_path).open(encoding="utf-8") as profile:
            header = next(profile, "").strip()
            if not header.startswith("mode:"):
                raise ValueError(f"invalid Go coverprofile header in {profile_path}")
            for raw_record in profile:
                if not raw_record.strip():
                    continue
                match = record_pattern.match(raw_record.strip())
                if not match:
                    raise ValueError(f"invalid Go coverage record: {raw_record.rstrip()}")
                (raw_source, start_line, start_column, end_line, end_column,
                 _statements, count) = match.groups()
                source = normalize_source(raw_source, root, module=module)
                source_path = root / source
                if source not in sources:
                    sources[source] = source_path.read_text(encoding="utf-8")
                covered = int(count)
                for line in executable_lines(
                    sources[source],
                    int(start_line), int(start_column), int(end_line), int(end_column),
                ):
                    lines[source][line] = max(lines[source].get(line, 0), covered)
    return lines


def parse_frontend_lcov(path, root):
    lines = defaultdict(dict)
    if not path:
        return lines
    frontend_root = Path(path).resolve().parent.parent
    current_source = None
    for raw in Path(path).read_text(encoding="utf-8").splitlines():
        if raw.startswith("SF:"):
            source = raw[3:]
            if Path(source).is_absolute():
                current_source = normalize_source(source, root)
            else:
                base = frontend_root.relative_to(root).as_posix()
                current_source = normalize_source(source, root, base=base)
        elif raw.startswith("DA:"):
            if current_source is None:
                raise ValueError("frontend LCOV DA record precedes SF record")
            line, count, *_ = raw[3:].split(",")
            line, count = int(line), int(count)
            lines[current_source][line] = max(
                lines[current_source].get(line, 0), count
            )
        elif raw == "end_of_record":
            current_source = None
    return lines


def merge_lines(*inputs):
    merged = defaultdict(dict)
    for records in inputs:
        for source, source_lines in records.items():
            for line, count in source_lines.items():
                merged[source][line] = max(merged[source].get(line, 0), count)
    return merged


def write_lcov(path, lines):
    lines_found = lines_hit = 0
    with Path(path).open("w", encoding="utf-8") as output:
        for source in sorted(lines):
            output.write("TN:Spela\n")
            output.write(f"SF:{source}\n")
            hit = 0
            for line, count in sorted(lines[source].items()):
                normalized_count = int(count > 0)
                output.write(f"DA:{line},{normalized_count}\n")
                hit += normalized_count
            found = len(lines[source])
            output.write(f"LF:{found}\n")
            output.write(f"LH:{hit}\n")
            output.write("end_of_record\n")
            lines_found += found
            lines_hit += hit
    return lines_found, lines_hit


def main():
    args = parse_args()
    root = Path(args.root).resolve()
    destination = Path(args.output)
    destination.parent.mkdir(parents=True, exist_ok=True)
    go_records = parse_go_profiles(args.go_profiles, root)
    records = merge_lines(
        go_records,
        parse_frontend_lcov(args.frontend, root),
    )
    lines_found, lines_hit = write_lcov(destination, records)
    percent = 100 * lines_hit / lines_found if lines_found else 0
    print(f"aggregate line coverage: {lines_hit}/{lines_found} ({percent:.2f}%)")
    print(destination)


if __name__ == "__main__":
    main()
