"""The comparison's `statement` class: a reference claim's statement matched to the node of ours whose statement shares
the most of its words, by token-set overlap at or above a threshold, one-to-one.

Stdlib only, so the rule is testable without kb_tools; compare_to_pristine.py tokenises both sides and calls it.
"""

from collections.abc import Mapping, Sequence
from dataclasses import dataclass


@dataclass(frozen=True)
class Tie:
    """A pair taken while another pair at the same overlap, sharing one of its ends, was still open."""

    reference: str
    node: str
    overlap: float
    rivals: tuple[tuple[str, str], ...]  # (reference claim, node of ours) pairs passed over


@dataclass(frozen=True)
class StatementMatches:
    matches: dict[str, tuple[str, float]]  # reference claim -> (node of ours, overlap)
    best: dict[str, float]  # reference claim with a statement -> its best overlap with any node offered
    ties: list[Tie]


def overlap(a: frozenset[str], b: frozenset[str]) -> float:
    """Jaccard: the words both share over the words either holds; 0.0 when neither holds any."""
    either = a | b
    return len(a & b) / len(either) if either else 0.0


def match_statements(
    statements: Mapping[str, Sequence[frozenset[str]]], ours: Mapping[str, frozenset[str]], *, threshold: float
) -> StatementMatches:
    """Each reference claim's best node of ours, one-to-one, highest overlap taken first.

    ``statements`` maps a reference claim to the token sets of its statements (one per page marking it; any one may
    match); ``ours`` maps a node of ours to its statement's token set. A pair's overlap is the best over the claim's
    statements. Pairs at or above ``threshold`` are taken in descending overlap, ties by (reference, node), each
    only while both ends are free.
    """
    scores = {
        (rid, oid): max(overlap(words, node_words) for words in texts)
        for rid, texts in statements.items()
        if texts
        for oid, node_words in ours.items()
    }
    best: dict[str, float] = {}
    for (rid, _), score in scores.items():
        best[rid] = max(best.get(rid, 0.0), score)
    candidates = sorted(
        ((score, rid, oid) for (rid, oid), score in scores.items() if score >= threshold),
        key=lambda found: (-found[0], found[1], found[2]),
    )
    matches: dict[str, tuple[str, float]] = {}
    taken: set[str] = set()
    ties: list[Tie] = []
    for score, rid, oid in candidates:
        if rid in matches or oid in taken:
            continue
        rivals = tuple(
            (other_rid, other_oid)
            for other_score, other_rid, other_oid in candidates
            if other_score == score
            and (other_rid, other_oid) != (rid, oid)
            and (other_rid == rid or other_oid == oid)
            and other_rid not in matches
            and other_oid not in taken
        )
        if rivals:
            ties.append(Tie(reference=rid, node=oid, overlap=score, rivals=rivals))
        matches[rid] = (oid, score)
        taken.add(oid)
    return StatementMatches(matches=matches, best=best, ties=ties)
