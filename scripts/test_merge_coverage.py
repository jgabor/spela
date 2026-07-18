import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).parent


def load_script(name):
    spec = importlib.util.spec_from_file_location(name, SCRIPT_DIR / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


merge = load_script("merge-coverage")
check = load_script("check-coverage")


class CoverageMergeTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        (self.root / "go.mod").write_text("module example.test/project\n", encoding="utf-8")
        (self.root / "pkg").mkdir()

    def tearDown(self):
        self.temporary.cleanup()

    def profile(self, name, records):
        path = self.root / name
        path.write_text("mode: atomic\n" + "\n".join(records) + "\n", encoding="utf-8")
        return path

    def test_multiline_blocks_keep_true_executable_source_lines(self):
        (self.root / "pkg" / "multi.go").write_text(
            "package pkg\n\nfunc f() {\n\tvalue := call(\n\t\tfirst, // argument\n\t\tsecond,\n\t)\n\t/* comment only */\n\tuse(value)\n}\n",
            encoding="utf-8",
        )
        profile = self.profile(
            "multi.out",
            ["example.test/project/pkg/multi.go:3.10,10.2 3 0"],
        )
        self.assertEqual(
            merge.parse_go_profiles([profile], self.root)["pkg/multi.go"],
            {4: 0, 5: 0, 6: 0, 9: 0},
        )

    def test_overlapping_blocks_and_duplicate_profiles_merge_by_max_hit(self):
        (self.root / "pkg" / "overlap.go").write_text(
            "package pkg\nfunc f() {\n\tone()\n\ttwo()\n}\n", encoding="utf-8"
        )
        first = self.profile(
            "first.out",
            ["example.test/project/pkg/overlap.go:2.10,5.2 2 0"],
        )
        second = self.profile(
            "second.out",
            [
                "example.test/project/pkg/overlap.go:2.10,4.7 1 0",
                "example.test/project/pkg/overlap.go:3.2,4.7 1 7",
            ],
        )
        self.assertEqual(
            merge.parse_go_profiles([first, second], self.root)["pkg/overlap.go"],
            {3: 7, 4: 7},
        )

    def test_build_tag_profiles_merge_same_line_by_max_hit(self):
        (self.root / "pkg" / "tagged.go").write_text(
            "package pkg\nfunc tagged() { run() }\n", encoding="utf-8"
        )
        default = self.profile(
            "default.out", ["example.test/project/pkg/tagged.go:2.15,2.24 1 0"]
        )
        tagged = self.profile(
            "tagged.out", ["example.test/project/pkg/tagged.go:2.15,2.24 1 3"]
        )
        self.assertEqual(
            merge.parse_go_profiles([default, tagged], self.root)["pkg/tagged.go"],
            {2: 3},
        )

    def test_utf8_byte_columns_do_not_shift_coverage_to_adjacent_line(self):
        source = (
            "package pkg\n"
            "func f() {\n"
            '\t_ = "éééé"; x()\n'
            "\ty()\n"
            "}\n"
        )
        (self.root / "pkg" / "utf8.go").write_text(source, encoding="utf-8")
        third_line = source.splitlines(keepends=True)[2]
        start_column = len(third_line.split("x()", 1)[0].encode("utf-8")) + 1
        profile = self.profile(
            "utf8.out",
            [f"example.test/project/pkg/utf8.go:3.{start_column},4.5 2 1"],
        )
        self.assertEqual(
            merge.parse_go_profiles([profile], self.root)["pkg/utf8.go"],
            {3: 1, 4: 1},
        )

    def test_utf8_span_must_end_on_code_point_boundary(self):
        (self.root / "pkg" / "split.go").write_text(
            'package pkg\nvar value = "é"\n', encoding="utf-8"
        )
        profile = self.profile(
            "split.out", ["example.test/project/pkg/split.go:2.14,2.15 1 1"]
        )
        with self.assertRaisesRegex(ValueError, "splits a UTF-8 code point"):
            merge.parse_go_profiles([profile], self.root)

    def test_path_normalization(self):
        absolute = self.root / "pkg" / "source.go"
        self.assertEqual(merge.normalize_source(str(absolute), self.root), "pkg/source.go")
        self.assertEqual(
            merge.normalize_source(
                r"example.test\project\pkg\source.go",
                self.root,
                module="example.test/project",
            ),
            "pkg/source.go",
        )
        with self.assertRaisesRegex(ValueError, "outside repository"):
            merge.normalize_source("/outside/source.go", self.root)

    def test_frontend_records_normalize_and_merge_with_max_hit(self):
        frontend = self.root / "internal" / "gui" / "frontend"
        coverage = frontend / "coverage"
        coverage.mkdir(parents=True)
        source = frontend / "src" / "App.svelte"
        source.parent.mkdir()
        source.write_text("<main/>\n", encoding="utf-8")
        lcov = coverage / "lcov.info"
        lcov.write_text(
            f"SF:{source}\nDA:1,0\nend_of_record\n"
            "SF:src/App.svelte\nDA:1,4\nend_of_record\n",
            encoding="utf-8",
        )
        self.assertEqual(
            merge.parse_frontend_lcov(lcov, self.root),
            {"internal/gui/frontend/src/App.svelte": {1: 4}},
        )

    def test_written_totals_are_recomputed_and_checkable(self):
        output = self.root / "lcov.info"
        totals = merge.write_lcov(output, {"pkg/a.go": {2: 0, 3: 2}})
        self.assertEqual(totals, (2, 1))
        self.assertEqual(check.totals(output), (2, 1))
        self.assertIn("DA:3,1\n", output.read_text(encoding="utf-8"))
        self.assertNotIn("DA:3,2\n", output.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
