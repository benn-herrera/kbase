"""The sweep's reorderings over a synthetic three-node case: a source, a prose restatement of it that outscores a
block candidate on shared words."""

import unittest

from sweep_shortlist import (
    asks_at,
    best_rank,
    blocks_first,
    drop_restatements,
    split_by_kind,
    stage_pools,
)

STATEMENTS = {
    "s": "attention friction drives market collapse",
    "p": "attention friction drives market collapse quickly",
    "k": "collapse threshold theorem bounds",
}
BLOCKS = {"k"}


def pools(own=frozenset()):
    return stage_pools(STATEMENTS, sources=["s"], candidate_pairs=[], own=own)


class Reorderings(unittest.TestCase):
    def test_today_ranks_the_restatement_first(self):
        self.assertEqual(pools(), {"s": ["p", "k"]})

    def test_own_equations_leave_the_pool(self):
        self.assertEqual(pools(own={("s", "k")}), {"s": ["p"]})

    def test_blocks_first_moves_the_block_ahead(self):
        self.assertEqual(blocks_first(pools(), blocks=BLOCKS), {"s": ["k", "p"]})

    def test_split_ranks_each_kind_in_its_own_list_and_asks_both(self):
        split = split_by_kind(pools(), blocks=BLOCKS)
        self.assertEqual(split, {"s": (["k"], ["p"])})
        self.assertEqual(best_rank(split, ["s"], ["p"]), 1)
        self.assertEqual(best_rank(split, ["s"], ["k"]), 1)
        self.assertEqual(asks_at(split, 1), 2)

    def test_restatement_dropped_block_kept(self):
        self.assertEqual(drop_restatements(pools(), STATEMENTS, threshold=0.6), {"s": ["k"]})

    def test_threshold_above_the_overlap_keeps_the_restatement(self):
        self.assertEqual(drop_restatements(pools(), STATEMENTS, threshold=0.9), {"s": ["p", "k"]})

    def test_rank_is_the_best_over_pairs_and_none_when_unreached(self):
        single = {"s": (["p", "k"],)}
        self.assertEqual(best_rank(single, ["s"], ["k", "p"]), 1)
        self.assertIsNone(best_rank(single, ["s"], ["z"]))
        self.assertEqual(asks_at(single, 5), 2)


if __name__ == "__main__":
    unittest.main()
