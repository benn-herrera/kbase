"""Volume roots found under a sources directory and mapped to our volumes by kbase's slug of their title."""

import contextlib
import io
import tempfile
import unittest
from pathlib import Path

from compare_to_pristine import argument_parser, kbase_plain, kbase_slug, latex_title_text, map_sources, read_sources, source_lines, source_roots

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"


def write(root: Path, name: str, text: str) -> Path:
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def document(title: str = "", body: str = "") -> str:
    return f"\\documentclass{{article}}\n{title}\n\\begin{{document}}\n{body}\n\\end{{document}}\n"


def slug_of_title(title: str) -> str:
    return kbase_slug(kbase_plain(latex_title_text(title)))


class SourceRootsTest(unittest.TestCase):
    def test_a_file_another_names_in_input_or_include_is_not_a_root(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            sources = Path(scratch)
            main = write(sources, "main.tex", document(body="\\input{partA}\n\\include{parts/partB.tex}"))
            write(sources, "partA.tex", document())
            write(sources, "parts/partB.tex", document())
            other = write(sources, "other/standalone.tex", document())
            write(sources, ".hidden/ignored.tex", document())
            write(sources, "commented.tex", "% \\documentclass{article}\n")
            self.assertEqual(source_roots(read_sources(sources), directory=sources), [main, other])

    def test_a_name_resolves_from_the_naming_files_directory(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            sources = Path(scratch)
            main = write(sources, "volume/main.tex", document(body="\\input{child}"))
            write(sources, "volume/child.tex", document())
            self.assertEqual(source_roots(read_sources(sources), directory=sources), [main])


class TitleSlugTest(unittest.TestCase):
    def test_multi_line_title_with_breaks_thanks_and_groups(self):
        title = "Is Human \\textbf{Attention} Still Necessary?\\\\\n\\large A {Synthesis}\\thanks{Part 1 of a series.}"
        self.assertEqual(latex_title_text(title), "Is Human Attention Still Necessary? A Synthesis")
        self.assertEqual(slug_of_title(title), "is-human-attention-still-necessary-a-synthesis")

    def test_quotation_marks_are_deleted_not_split(self):
        self.assertEqual(slug_of_title("The Dictator's~Dilemma"), "the-dictators-dilemma")

    def test_a_cut_mid_word_goes_back_to_the_last_hyphen(self):
        # 48 characters end inside "subtitle".
        title = "Does the Quiet Harbour Keep Its Boats?\\\\ \\emph{A Longer Subtitle Follows}"
        self.assertEqual(slug_of_title(title), "does-the-quiet-harbour-keep-its-boats-a-longer")

    def test_empty_is_untitled(self):
        self.assertEqual(kbase_slug(" — "), "untitled")

    def test_no_title_slugs_the_file_stem(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            sources = Path(scratch)
            write(sources, "Main_Volume.tex", document())
            self.assertEqual(map_sources(sources, ["mainvolume"]).mapped(), {"mainvolume": sources / "Main_Volume.tex"})


class SourceMappingTest(unittest.TestCase):
    def test_an_unmatched_root_and_an_unmatched_volume_are_both_listed(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            sources = Path(scratch)
            alpha = write(sources, "a.tex", document("\\title{Alpha}"))
            beta = write(sources, "b.tex", document("\\title{Beta}"))
            mapping = map_sources(sources, ["gamma", "alpha"])
            self.assertEqual(mapping.mapped(), {"alpha": alpha})
            self.assertEqual(
                source_lines(mapping),
                [f"sources: {sources}", f"  {alpha} -> alpha", f"  {beta}: no volume takes slug beta", "  volume gamma: no root"],
            )

    def test_two_roots_taking_one_volume_map_neither(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            sources = Path(scratch)
            write(sources, "a.tex", document("\\title{Alpha}"))
            write(sources, "b.tex", document("\\title{\\emph{Alpha}}"))
            self.assertEqual(map_sources(sources, ["alpha"]).mapped(), {})


class ArgumentsTest(unittest.TestCase):
    def test_sources_and_volume_root_together_are_refused(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            directory = Path(scratch)
            root = write(directory, "root.tex", document())
            arguments = ["--ours", scratch, "--reference", scratch, "--out", str(directory / "out")]
            exclusive = ["--sources", scratch, "--volume-root", str(root)]
            with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as refused:
                argument_parser().parse_args([*arguments, *exclusive])
            self.assertEqual(refused.exception.code, 2)


if __name__ == "__main__":
    unittest.main()
