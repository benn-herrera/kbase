"""The reach of an unmarked miss read from the build's own records, over a synthetic record and edges.tsv row."""

import tempfile
import unittest
from pathlib import Path
from unittest import mock

from kb_tools.kb_pipeline import CandidateEntry, ClassifyOutcome, UnmarkedRecord

import measure_unmarked_shortlist
from measure_unmarked_shortlist import (
    ANSWERED_NO,
    LINK_ABSENT,
    LINK_KEPT,
    NEVER_PLANNED,
    NOT_ANSWERED,
    miss_reach,
    pair_reaches,
    read_misses,
)

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"
HEADER = "ref_source\tref_target\tmark_split\tours_sources\tours_targets"


def answered(letter: str) -> CandidateEntry:
    return CandidateEntry(offered=("A", "B"), letter=letter, outcome=ClassifyOutcome.ANSWERED)


class RecordReachTest(unittest.TestCase):
    def miss(self) -> dict[str, object]:
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            path = Path(scratch) / "edges.tsv"
            path.write_text(f"{HEADER}\nr1\tr2\tunmarked — needs reading\ts\tt\n", encoding="utf-8")
            [miss], skipped = read_misses(path)
        self.assertEqual(skipped, [])
        return miss

    def test_unmarked_reach_reads_the_planned_record(self):
        miss = self.miss()
        linked = {frozenset(("s", "t"))}
        cases = [
            (UnmarkedRecord(planned=(("s", "t"),), pairs={("s", "t"): answered("A")}), linked, LINK_KEPT),
            (UnmarkedRecord(planned=(("t", "s"),), pairs={("t", "s"): answered("A")}), set(), LINK_ABSENT),
            (UnmarkedRecord(planned=(("s", "t"),), pairs={("s", "t"): answered("B")}), linked, ANSWERED_NO),
            (UnmarkedRecord(planned=(("s", "t"),), pairs={}), set(), NOT_ANSWERED),
            (UnmarkedRecord(planned=(("s", "u"),), pairs={("s", "t"): answered("A")}), linked, NEVER_PLANNED),
        ]
        with mock.patch.object(measure_unmarked_shortlist.shortlist, "rank", side_effect=AssertionError("ranked")):
            for record, links, expected in cases:
                with self.subTest(expected=expected, planned=record.planned):
                    pairs = pair_reaches(miss["sources"], miss["targets"], record=record, links=links)
                    self.assertEqual(miss_reach(pairs), expected)

    def test_the_pair_carries_the_letter_where_asked(self):
        record = UnmarkedRecord(planned=(("s", "t"),), pairs={("s", "t"): answered("B")})
        [pair] = pair_reaches(["s"], ["t"], record=record, links=set())
        self.assertEqual((pair.planned_way(), pair.letter_forward, pair.letter_reverse, pair.linked), ("forward", "B", None, False))


if __name__ == "__main__":
    unittest.main()
