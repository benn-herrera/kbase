"""The comparison's number class over a synthetic volume: the walked number keys the k-th block of the shared counter;
and its statement class over synthetic word sets: token-set overlap, one-to-one."""

import unittest
from pathlib import Path

from latex_numbering import walk_source
from number_match import (
    AMBIGUOUS,
    NO_BLOCK,
    NO_NODE,
    NO_SUCH_WORD,
    NO_WALKED_NUMBER,
    NUMBER_DIFFERS,
    PageBlock,
    ResultKey,
    match_numbers,
    result_key,
    unwalked_blocks,
)
from statement_match import match_statements, overlap

ROOT = Path("main.tex")
SHARED = r"\newtheorem{theorem}{Theorem}[section]\newtheorem{lemma}[theorem]{Lemma}\newtheorem{corollary}[theorem]{Corollary}"


def environments(preamble: str, body: str):
    main = f"\\documentclass{{article}}\n{preamble}\n\\begin{{document}}\n{body}\n\\end{{document}}\n"
    return walk_source(ROOT, {ROOT: main}.get).environments


def env(name: str) -> str:
    return f"\\begin{{{name}}}x\\end{{{name}}}\n"


# Section 1: Theorem 1.1, Lemma 1.2; section 2: Corollary 2.1. pandoc prints them Theorem 1, Lemma 2, Corollary 3.
WALKED = {"vol": environments(SHARED, r"\section{A}" + env("theorem") + env("lemma") + r"\section{B}" + env("corollary"))}
PAGE = [PageBlock("vol", "Theorem 1", "n-thm"), PageBlock("vol", "Lemma 2", "n-lem"), PageBlock("vol", "Corollary 3", None)]


class ResultKeyTest(unittest.TestCase):
    def test_forms(self):
        cases = {
            "Theorem 5.11 — Stability": ResultKey("theorem", "5.11", ""),
            "Proposition 5.10c — Bifurcation": ResultKey("proposition", "5.10", "c"),
            "Conjecture C.5.1 — Sign": ResultKey("conjecture", "C.5.1", ""),
            "Corollary 4.2: Reduction": ResultKey("corollary", "4.2", ""),
            "Lemma on bounds": None,
        }
        for title, expected in cases.items():
            with self.subTest(title=title):
                self.assertEqual(result_key(title), expected)


class MatchNumbersTest(unittest.TestCase):
    def outcomes(self, titles: dict[str, str], *, page=PAGE, walked=WALKED) -> dict[str, tuple[str, str]]:
        return {rid: (o.node_id, o.reason) for rid, o in match_numbers(titles, blocks=page, walked=walked).items()}

    def test_shared_counter_ordinal_keys_the_block(self):
        got = self.outcomes({"r1": "Lemma 1.2 — a", "r2": "Theorem 1.1 — b", "r3": "an unnumbered title"})
        self.assertEqual(got, {"r1": ("n-lem", ""), "r2": ("n-thm", "")})

    def test_reasons(self):
        cases = {
            "Theorem 2.1": NUMBER_DIFFERS,
            "Corollary 2.1a": NO_NODE,
            "Remark 1.1": NO_SUCH_WORD,
        }
        for title, reason in cases.items():
            with self.subTest(title=title):
                self.assertEqual(self.outcomes({"r": title}), {"r": ("", reason)})

    def test_no_block_for_the_walked_environment(self):
        self.assertEqual(self.outcomes({"r": "Theorem 1.1"}, page=PAGE[1:]), {"r": ("", NO_BLOCK)})

    def test_no_walked_number(self):
        walked = {"vol": environments(SHARED + r"\renewcommand{\thetheorem}{T\arabic{theorem}}", env("theorem"))}
        self.assertEqual(self.outcomes({"r": "Theorem 1"}, walked=walked), {"r": ("", NO_WALKED_NUMBER)})

    def test_one_to_one(self):
        got = self.outcomes({"r1": "Theorem 1.1 — a", "r2": "Theorem 1.1 — b"})
        self.assertEqual({rid: node for rid, (node, _) in got.items()}, {"r1": "", "r2": ""})
        self.assertTrue(all(reason.startswith(AMBIGUOUS) for _, reason in got.values()))

    def test_same_number_in_two_volumes_is_ambiguous(self):
        walked = WALKED | {"other": WALKED["vol"]}
        page = PAGE + [PageBlock("other", "Theorem 1", "n-other")]
        node, reason = self.outcomes({"r": "Theorem 1.1"}, page=page, walked=walked)["r"]
        self.assertEqual(node, "")
        self.assertTrue(reason.startswith(AMBIGUOUS))


class UnwalkedBlocksTest(unittest.TestCase):
    def test_a_counter_the_walk_never_reached(self):
        page = PAGE + [PageBlock("vol", "Lemma 4", "n-x"), PageBlock("unwalked", "Lemma 9", "n-y")]
        self.assertEqual(unwalked_blocks(page, WALKED), [PageBlock("vol", "Lemma 4", "n-x")])


def words(text: str) -> frozenset[str]:
    return frozenset(text.split())


class OverlapTest(unittest.TestCase):
    def test_jaccard(self):
        cases = [("a b c", "a b c", 1.0), ("a b c", "b c d", 0.5), ("a b", "c d", 0.0), ("", "", 0.0), ("a a b", "a", 0.5)]
        for left, right, expected in cases:
            with self.subTest(left=left, right=right):
                self.assertEqual(overlap(words(left), words(right)), expected)


class MatchStatementsTest(unittest.TestCase):
    def test_one_to_one_highest_overlap_first(self):
        # r1 overlaps n1 at 1.0 and takes it; r2's best is also n1 (4/5), so it takes n2 (4/6).
        got = match_statements(
            {"r1": [words("a b c d")], "r2": [words("a b c d e")]},
            {"n1": words("a b c d"), "n2": words("a b c e f")},
            threshold=0.6,
        )
        self.assertEqual({rid: oid for rid, (oid, _) in got.matches.items()}, {"r1": "n1", "r2": "n2"})
        self.assertEqual(got.best, {"r1": 1.0, "r2": 0.8})

    def test_any_statement_of_a_claim_may_match(self):
        got = match_statements({"r": [words("p q"), words("a b c")]}, {"n": words("a b c")}, threshold=0.9)
        self.assertEqual(got.matches, {"r": ("n", 1.0)})

    def test_below_threshold_is_unmatched_with_its_best(self):
        got = match_statements({"r": [words("a b c d")], "bare": []}, {"n": words("a b")}, threshold=0.6)
        self.assertEqual((got.matches, got.best), ({}, {"r": 0.5}))

    def test_tie_is_reported(self):
        got = match_statements({"r": [words("a b")]}, {"n1": words("a b"), "n2": words("a b")}, threshold=0.5)
        self.assertEqual(got.matches, {"r": ("n1", 1.0)})
        self.assertEqual([(tie.reference, tie.node, tie.rivals) for tie in got.ties], [("r", "n1", (("r", "n2"),))])


if __name__ == "__main__":
    unittest.main()
