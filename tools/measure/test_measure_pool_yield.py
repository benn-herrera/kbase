"""The pool's category rules and the yield's containment over a synthetic four-claim KB's index rows and records."""

import tempfile
import unittest
from pathlib import Path

from kb_tools.kb_pipeline import CandidateEntry, ClassifyOutcome, UnmarkedRecord

from measure_pool_yield import (
    claim_hit,
    defaulted_pairs,
    near_misses,
    pair_hit,
    read_missed_edges,
    relation_pairs,
    unanchored_claims,
    unsupported_claims,
)


def claim(cid, *, solidity=None, depends_on_count=0):
    return {"node_type": "claim", "id": cid, "canonical_path": f"{cid}.md", "solidity": solidity, "depends_on_count": depends_on_count}


def edge(source, target, relation, origin=None):
    row = {"source": source, "target": target, "relation": relation}
    return row | ({"origin": origin} if origin else {})


CLAIMS = [
    claim("a", depends_on_count=1),
    claim("b"),
    claim("c", solidity=0.8),
    claim("d"),
    {"node_type": "work", "id": "w", "canonical_path": "w.md"},
]
DEPENDS = [
    edge("a", "b", "depends"),
    edge("b", "d", "references"),
    edge("c", "d", "demoted", origin="cited"),
    edge("sup-1", "a", "supports"),
]


def entry(outcome, letter):
    return CandidateEntry(offered=("A", "B"), letter=letter, outcome=outcome)


class Categories(unittest.TestCase):
    def test_relation_pairs_take_one_relation_with_its_origin(self):
        self.assertEqual(relation_pairs(DEPENDS, "demoted"), {("c", "d"): "cited"})
        self.assertEqual(relation_pairs(DEPENDS, "references"), {("b", "d"): ""})

    def test_defaulted_is_every_planned_pair_without_a_letter_bearing_outcome(self):
        record = UnmarkedRecord(
            planned=(("a", "b"), ("a", "c"), ("a", "d"), ("b", "c"), ("b", "d")),
            pairs={
                ("a", "b"): entry(ClassifyOutcome.ANSWERED, "B"),
                ("a", "c"): entry(ClassifyOutcome.REASKED, "A"),
                ("a", "d"): entry(ClassifyOutcome.DEFAULTED, None),
                ("b", "c"): entry(ClassifyOutcome.DRAFTED, None),
                ("c", "d"): entry(ClassifyOutcome.DEFAULTED, None),
            },
        )
        self.assertEqual(sorted(defaulted_pairs(record)), [("a", "d"), ("b", "c"), ("b", "d")])

    def test_unsupported_is_pending_and_targeted_by_no_supporting_relation(self):
        # a: a supports row targets it; b: a depends row does; c: scored; d: only references and demoted target it.
        self.assertEqual(sorted(unsupported_claims(CLAIMS, DEPENDS)), ["d"])

    def test_unanchored_is_every_claim_depending_on_nothing(self):
        self.assertEqual(sorted(unanchored_claims(CLAIMS)), ["b", "c", "d"])

    def test_near_miss_cuts_at_k_and_drops_pairs_answered_a(self):
        pools = {"a": ["b", "c", "d"], "b": ["a"]}
        letters = {("a", "b"): "A", ("a", "c"): "B"}
        self.assertEqual(near_misses(pools, k=2, letters=letters), {("a", "c"): "rank 2; letter B", ("b", "a"): "rank 1; not asked"})


class Containment(unittest.TestCase):
    def test_pair_hit_says_which_way(self):
        members = {("x", "y"), ("z", "w")}
        self.assertEqual(pair_hit(members, ["x"], ["y", "q"]), "forward")
        self.assertEqual(pair_hit(members, ["w"], ["z"]), "reverse")
        self.assertEqual(pair_hit(members | {("y", "x")}, ["x"], ["y"]), "both")
        self.assertEqual(pair_hit(members, ["x"], []), "")

    def test_claim_hit_reads_the_matched_end_of_an_unmatched_edge(self):
        self.assertEqual(claim_hit({"y"}, [], ["y"]), "target")
        self.assertEqual(claim_hit({"x", "y"}, ["x"], ["y"]), "both")
        self.assertEqual(claim_hit({"q"}, ["x"], ["y"]), "")

    def test_missed_edges_are_the_four_miss_classes(self):
        header = "ref_source\tref_target\trecall\tmark_split\tours_sources\tours_targets"
        rows = [
            "r1\tr2\tnothing\tunmarked — needs reading\tx\ty",
            "r1\tr3\tendpoint unmatched (source)\t\t\ty z",
            "r2\tr3\tdepends reversed\t\tx\ty",
            "r3\tr4\tdepends path reversed\t\tx\ty",
            "r4\tr5\tdepends\t\tx\ty",
        ]
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[2] / ".claude-temp") as scratch:
            path = Path(scratch) / "edges.tsv"
            path.write_text("\n".join([header, *rows]) + "\n", encoding="utf-8")
            missed, skipped = read_missed_edges(path)
        self.assertEqual(skipped, [])
        self.assertEqual(
            [(m["edge"], m["recall"], m["unmarked"], m["targets"]) for m in missed],
            [
                ("r1->r2", "nothing", True, ["y"]),
                ("r1->r3", "endpoint unmatched", False, ["y", "z"]),
                ("r2->r3", "depends reversed", False, ["y"]),
            ],
        )


if __name__ == "__main__":
    unittest.main()
