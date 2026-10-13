"""The cull-aware listing over a synthetic matches table and cull record."""

import tempfile
import unittest
from pathlib import Path

from culled_matches import CULLED_HEADER, culled_matches, read_cull_links, read_culls

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"

CULL_RECORD = """about: "The co-linearity cull's build record."
entries:
  - a: clm-aaaaaa
    b: clm-bbbbbb
    pass: 1
    verdict: a-adds-nothing
    outcome: answered
culls:
  - id: clm-aaaaaa
    into: clm-bbbbbb
  - id: clm-cccccc
    into: clm-aaaaaa
links:
  - a: clm-bbbbbb
    b: clm-dddddd
    harvest: unmarked
"""


def match(ref_id: str, match_class: str, ours_id: str) -> dict[str, str]:
    return {"ref_id": ref_id, "ref_title": ref_id.upper(), "class": match_class, "ours_id": ours_id}


class CulledMatchesTest(unittest.TestCase):
    def test_culled_exact_match_is_listed(self):
        rows = [match("r", "statement", "n"), match("q", "cosine", "n"), match("p", "number", "kept")]
        [row] = culled_matches(rows, {"n": "m"})
        self.assertEqual(
            dict(zip(CULLED_HEADER, row)),
            {"ref_id": "r", "ref_title": "R", "class": "statement", "ours_id": "n", "into": "m", "survivor": "m"},
        )

    def test_survivor_is_the_end_of_the_into_chain(self):
        [row] = culled_matches([match("r", "number", "n")], {"n": "m", "m": "k"})
        self.assertEqual(dict(zip(CULLED_HEADER, row))["into"], "m")
        self.assertEqual(dict(zip(CULLED_HEADER, row))["survivor"], "k")


class CullRecordTest(unittest.TestCase):
    def read(self, text: str, reader):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            path = Path(scratch) / "kb-build-cull.yaml"
            path.write_text(text, encoding="utf-8")
            return reader(path)

    def test_culls_and_links_read_from_the_record(self):
        self.assertEqual(self.read(CULL_RECORD, read_culls), {"clm-aaaaaa": "clm-bbbbbb", "clm-cccccc": "clm-aaaaaa"})
        self.assertEqual(self.read(CULL_RECORD, read_cull_links), {frozenset(("clm-bbbbbb", "clm-dddddd"))})

    def test_a_record_without_the_list_is_refused(self):
        without_culls = CULL_RECORD.split("culls:")[0] + "links: []\n"
        with self.assertRaisesRegex(ValueError, "no culls list"):
            self.read(without_culls, read_culls)
        with self.assertRaisesRegex(ValueError, "no links list"):
            self.read(CULL_RECORD.split("links:")[0], read_cull_links)


if __name__ == "__main__":
    unittest.main()
