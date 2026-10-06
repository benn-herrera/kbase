"""The counter walker over small inline volumes: each grammar item, and one construct outside it."""

import unittest
from pathlib import Path

from latex_numbering import walk_source

ROOT = Path("main.tex")


def walk(main: str, **files: str):
    sources = {ROOT: main} | {Path(name): text for name, text in files.items()}
    return walk_source(ROOT, sources.get)


def numbers(main: str, **files: str) -> list[str]:
    return [row.number for row in walk(main, **files).environments]


def document(preamble: str, body: str, doc_class: str = "article") -> str:
    return f"\\documentclass{{{doc_class}}}\n{preamble}\n\\begin{{document}}\n{body}\n\\end{{document}}\n"


THM = "\\begin{{{0}}}x\\end{{{0}}}\n"


class GrammarTest(unittest.TestCase):
    def test_own_counter(self):
        self.assertEqual(numbers(document(r"\newtheorem{theorem}{Theorem}", THM.format("theorem") * 2)), ["1", "2"])

    def test_shared_counter_and_ordinal_in_group(self):
        preamble = r"\newtheorem{theorem}{Theorem}\newtheorem{lemma}[theorem]{Lemma}\newtheorem{remark}{Remark}"
        body = THM.format("lemma") + THM.format("remark") + THM.format("theorem")
        rows = walk(document(preamble, body)).environments
        self.assertEqual([(r.group, r.ordinal_in_group, r.number) for r in rows],
                         [("theorem", 1, "1"), ("remark", 1, "1"), ("theorem", 2, "2")])
        self.assertEqual([r.ordinal for r in rows], [1, 2, 3])

    def test_within_section_resets(self):
        body = r"\section{A}" + THM.format("theorem") * 2 + r"\section{B}" + THM.format("theorem")
        self.assertEqual(numbers(document(r"\newtheorem{theorem}{Theorem}[section]", body)), ["1.1", "1.2", "2.1"])

    def test_within_subsection(self):
        body = r"\section{A}\subsection{a}\subsection{b}" + THM.format("claim") + r"\section{B}\subsection{c}" + THM.format("claim")
        self.assertEqual(numbers(document(r"\newtheorem{claim}{Claim}[subsection]", body)), ["1.2.1", "2.1.1"])

    def test_within_chapter_in_a_chaptered_class(self):
        preamble = r"\newtheorem{theorem}{Theorem}[chapter]\newtheorem{lemma}{Lemma}[section]"
        body = r"\chapter{A}\chapter{B}\section{s}" + THM.format("theorem") + THM.format("lemma")
        self.assertEqual(numbers(document(preamble, body, "book")), ["2.1", "2.1.1"])

    def test_starred_theorem_is_unnumbered_and_not_counted(self):
        preamble = r"\newtheorem{theorem}{Theorem}\newtheorem*{theorem*}{Theorem}"
        rows = walk(document(preamble, THM.format("theorem*") + THM.format("theorem"))).environments
        self.assertEqual([(r.group, r.ordinal_in_group, r.number) for r in rows], [("", None, ""), ("theorem", 1, "1")])

    def test_numberwithin(self):
        body = r"\section{A}\section{B}" + THM.format("theorem")
        self.assertEqual(numbers(document(r"\newtheorem{theorem}{Theorem}\numberwithin{theorem}{section}", body)), ["2.1"])

    def test_starred_sections_do_not_count(self):
        body = r"\section{A}\section*{B}\subsection*{b}" + THM.format("theorem")
        self.assertEqual(numbers(document(r"\newtheorem{theorem}{Theorem}[section]", body)), ["1.1"])

    def test_appendix_letters_sections(self):
        body = r"\section{A}\section{B}\appendix\section{C}" + THM.format("theorem") + r"\section{D}" + THM.format("theorem")
        self.assertEqual(numbers(document(r"\newtheorem{theorem}{Theorem}[section]", body)), ["A.1", "B.1"])

    def test_setcounter(self):
        body = r"\setcounter{theorem}{4}" + THM.format("theorem") + r"\section{A}\setcounter{section}{6}\section{B}" + THM.format("lemma")
        preamble = r"\newtheorem{theorem}{Theorem}\newtheorem{lemma}{Lemma}[section]"
        self.assertEqual(numbers(document(preamble, body)), ["5", "7.1"])

    def test_secnumdepth_stops_stepping(self):
        body = r"\section{A}\subsection{a}\subsection{b}" + THM.format("claim")
        preamble = r"\setcounter{secnumdepth}{1}\newtheorem{claim}{Claim}[subsection]"
        self.assertEqual(numbers(document(preamble, body)), ["1.0.1"])

    def test_preamble_file_and_includes_in_document_order(self):
        main = "\\documentclass{amsart}\n\\input{defs}\n\\begin{document}\n\\include{part}\n\\input{missing}\n" + THM.format("theorem") + "\\end{document}"
        result = walk(main, **{"defs.tex": r"\newtheorem{theorem}{Theorem}" + THM.format("theorem"), "part.tex": THM.format("theorem")})
        self.assertEqual([(r.number, r.file) for r in result.environments], [("1", "part.tex"), ("2", "main.tex")])
        self.assertEqual(result.notes, ["main.tex:5: include 'missing' not found: skipped"])

    def test_comments_are_stripped_but_not_escaped_percents(self):
        body = "% " + THM.format("theorem") + r"50\% " + THM.format("theorem")
        rows = walk(document(r"\newtheorem{theorem}{Theorem}", body)).environments
        self.assertEqual([(r.number, r.line) for r in rows], [("1", 5)])

    def test_text_tex_never_reads_is_skipped(self):
        preamble = r"\newtheorem{theorem}{Theorem}[section]\newcommand{\comment}[1]{}"
        body = (
            r"\comment{\section{Dropped}}\section{A}"
            + "\\iffalse\n" + THM.format("theorem") + "\\fi\n"
            + "\\begin{comment}\n" + THM.format("theorem") + "\\end{comment}\n"
            + THM.format("theorem")
        )
        rows = walk(document(preamble, body)).environments
        self.assertEqual([(r.number, r.line) for r in rows], [("1.1", 10)])

    def test_label_at_the_environments_own_level(self):
        body = r"\begin{theorem}\begin{enumerate}\item\label{item}\end{enumerate}\label{thm}\label{second}\end{theorem}"
        rows = walk(document(r"\newtheorem{theorem}{Theorem}", body)).environments
        self.assertEqual([r.label for r in rows], ["thm"])


class OutsideGrammarTest(unittest.TestCase):
    def test_redefined_counter_format_leaves_its_numbers_empty(self):
        preamble = r"\newtheorem{theorem}{Theorem}\newtheorem{lemma}{Lemma}\renewcommand{\thetheorem}{\Roman{theorem}}"
        result = walk(document(preamble, THM.format("theorem") + THM.format("lemma")))
        self.assertEqual([r.number for r in result.environments], ["", "1"])
        self.assertEqual(len(result.notes), 1)
        self.assertIn(r"\thetheorem", result.notes[0])

    def test_an_alias_leaves_the_aliased_counter_empty_too(self):
        preamble = r"\newtheorem{theorem}{Theorem}\newaliascnt{lemma}{theorem}\newtheorem{lemma}[lemma]{Lemma}"
        self.assertEqual(numbers(document(preamble, THM.format("lemma") + THM.format("theorem"))), ["", ""])

    def test_an_unpaired_iffalse_leaves_every_later_number_empty(self):
        body = THM.format("theorem") + "\\iffalse\\else\\fi" + THM.format("theorem")
        self.assertEqual(numbers(document(r"\newtheorem{theorem}{Theorem}", body)), ["1", ""])


if __name__ == "__main__":
    unittest.main()
