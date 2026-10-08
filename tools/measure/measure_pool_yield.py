"""Uncertainty-pool yield: each pool category's size and how many of the reference's missed edges a reader finds in it.

Every input, output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import logging
import sys
from collections.abc import Collection, Iterable, Mapping, Sequence
from pathlib import Path

from kb_tools import kb_load, kb_pipeline

import compare_to_pristine as compare
from measure_unmarked_shortlist import Build, edges_argument
from sweep_shortlist import stage_pools

logger = logging.getLogger(__name__)

Pair = tuple[str, str]

NEAR_MISS_KS = (5, 10, 20, 40)
#: Recall classes of edges.tsv that count as an edge the build missed.
MISSED = (*compare.MISSED, "depends reversed")
#: The relations whose row targeting a claim gives it incoming support.
SUPPORTING = ("depends", "rests-on", "supports", "strengthens")
UNDECIDED = (kb_pipeline.ClassifyOutcome.DEFAULTED, kb_pipeline.ClassifyOutcome.DRAFTED)
PAIR_CATEGORIES = ("demoted", "references", "defaulted")
CLAIM_CATEGORIES = ("unsupported", "unanchored")
DECLARED = (*PAIR_CATEGORIES, *CLAIM_CATEGORIES)
EDGE_COLUMNS = ("ref_source", "ref_target", "recall", "mark_split", "ours_sources", "ours_targets")


# --- the categories ---------------------------------------------------------------------


def relation_pairs(depends_rows: Iterable[Mapping], relation: str) -> dict[Pair, str]:
    """Every depends-on row of ``relation`` as its (source, target) pair, the row's origin as evidence."""
    return {(row["source"], row["target"]): row.get("origin") or "" for row in depends_rows if row["relation"] == relation}


def defaulted_pairs(record: kb_pipeline.UnmarkedRecord) -> dict[Pair, str]:
    """Every planned pair the record holds no letter-bearing outcome for: defaulted, drafted, or never asked."""
    members = {}
    for pair in record.planned or ():
        entry = record.pairs.get(pair)
        if entry is None:
            members[pair] = "not asked"
        elif entry.outcome in UNDECIDED:
            members[pair] = f"{entry.outcome.value}; offered {''.join(entry.offered)}"
    return members


def unsupported_claims(claim_rows: Iterable[Mapping], depends_rows: Iterable[Mapping]) -> dict[str, str]:
    """Every claim with a pending solidity that no ``SUPPORTING`` row targets, its canonical path as evidence."""
    leaned_on = {row["target"] for row in depends_rows if row["relation"] in SUPPORTING}
    return {
        row["id"]: row["canonical_path"]
        for row in claim_rows
        if row["node_type"] == "claim" and row["solidity"] is None and row["id"] not in leaned_on
    }


def unanchored_claims(claim_rows: Iterable[Mapping]) -> dict[str, str]:
    """Every claim whose ``depends_on_count`` is 0, its canonical path as evidence."""
    return {row["id"]: row["canonical_path"] for row in claim_rows if row["node_type"] == "claim" and row["depends_on_count"] == 0}


def near_misses(pools: Mapping[str, Sequence[str]], *, k: int, letters: Mapping[Pair, str | None]) -> dict[Pair, str]:
    """Every (source, target) with target in the source's first ``k`` and not answered ``A``, its rank and letter."""
    members = {}
    for source, targets in pools.items():
        for rank, target in enumerate(targets[:k], start=1):
            pair = (source, target)
            if pair not in letters:
                members[pair] = f"rank {rank}; not asked"
            elif letters[pair] != kb_pipeline.UNMARKED_POINTS_LETTER:
                members[pair] = f"rank {rank}; letter {letters[pair] or 'none'}"
    return members


# --- yield --------------------------------------------------------------------------------


def read_missed_edges(path: Path) -> tuple[list[dict[str, object]], list[str]]:
    """The edges.tsv rows whose recall class is in ``MISSED``, and a note per unusable row."""
    lines = path.read_text(encoding="utf-8").splitlines()
    header = lines[0].split("\t") if lines else []
    absent = [column for column in EDGE_COLUMNS if column not in header]
    if absent:
        raise ValueError(f"not an edges.tsv of compare_to_pristine.py: no column {', '.join(absent)}")
    missed, skipped = [], []
    for number, line in enumerate(lines[1:], start=2):
        if not line.strip():
            continue
        cells = line.split("\t")
        if len(cells) != len(header):
            skipped.append(f"line {number}: {len(cells)} fields, the header has {len(header)}")
            continue
        row = dict(zip(header, cells))
        recall = compare.recall_class(row["recall"])
        if recall in MISSED:
            missed.append(
                {
                    "edge": f"{row['ref_source']}->{row['ref_target']}",
                    "recall": recall,
                    "unmarked": row["mark_split"] == compare.UNMARKED,
                    "sources": row["ours_sources"].split(),
                    "targets": row["ours_targets"].split(),
                }
            )
    return sorted(missed, key=lambda miss: miss["edge"]), skipped


def pair_hit(members: Collection[Pair], sources: Sequence[str], targets: Sequence[str]) -> str:
    """Which way some (source, target) over the edge's matched nodes is a member: forward, reverse, both, or ''."""
    forward = any((s, t) in members for s in sources for t in targets)
    reverse = any((t, s) in members for s in sources for t in targets)
    return "both" if forward and reverse else "forward" if forward else "reverse" if reverse else ""


def claim_hit(members: Collection[str], sources: Sequence[str], targets: Sequence[str]) -> str:
    """Which matched end of the edge holds a member: source, target, both, or ''."""
    source = any(s in members for s in sources)
    target = any(t in members for t in targets)
    return "both" if source and target else "source" if source else "target" if target else ""


# --- the measurement ----------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ours", required=True, type=compare.kb_root_argument, help="the built kb-root, its build records beside it")
    parser.add_argument(
        "--edges", required=True, type=edges_argument, help="compare_to_pristine.py's edges.tsv for the same --ours"
    )
    parser.add_argument(
        "--out", required=True, type=compare.scratch_directory_argument, help="output directory under .claude-temp/"
    )
    args = parser.parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")

    refusal = compare.format_refusal(args.ours)
    if refusal is not None:
        print(f"--ours {str(args.ours)!r}: {refusal}", file=sys.stderr)
        return 1
    try:
        missed, skipped = read_missed_edges(args.edges)
    except (OSError, UnicodeDecodeError, ValueError) as error:
        print(f"--edges {str(args.edges)!r}: {error}", file=sys.stderr)
        return 1
    try:
        claim_rows, claim_problems = kb_load.read_index(args.ours, "claims")
        depends_rows, depends_problems = kb_load.read_index(args.ours, "depends-on")
        if claim_problems or depends_problems:
            raise ValueError(f"{len(claim_problems) + len(depends_problems)} index lines that are not records")
        record = kb_pipeline.read_unmarked(args.ours.parent)
        if record is None or record.planned is None:
            raise ValueError(f"no planned unmarked record {kb_pipeline.UNMARKED_RELPATH} beside it")
        ours = Build(args.ours)
    except Exception as exc:  # noqa: BLE001 - a kb-root or record no reader can open is an unusable argument
        print(f"--ours {str(args.ours)!r}: unreadable: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    pair_members: dict[str, dict[Pair, str]] = {
        "demoted": relation_pairs(depends_rows, "demoted"),
        "references": relation_pairs(depends_rows, "references"),
        "defaulted": defaulted_pairs(record),
    }
    claim_members: dict[str, dict[str, str]] = {
        "unsupported": unsupported_claims(claim_rows, depends_rows),
        "unanchored": unanchored_claims(claim_rows),
    }
    pools = stage_pools(ours.statement, sources=ours.sources, candidate_pairs=ours.candidate_pairs, own=ours.own_equations)
    letters = {pair: entry.letter for pair, entry in record.pairs.items()}
    for k in NEAR_MISS_KS:
        pair_members[f"near-miss@{k}"] = near_misses(pools, k=k, letters=letters)
    categories = [*pair_members, *claim_members]

    hits: dict[str, dict[str, str]] = {
        miss["edge"]: {
            **{name: pair_hit(members, miss["sources"], miss["targets"]) for name, members in pair_members.items()},
            **{name: claim_hit(members, miss["sources"], miss["targets"]) for name, members in claim_members.items()},
        }
        for miss in missed
    }

    claim_count = sum(1 for row in claim_rows if row["node_type"] == "claim")
    size = {name: len(members) for name, members in (*pair_members.items(), *claim_members.items())}

    def found(name: str, *, forward_only: bool = False) -> int:
        wanted = ("forward", "both") if forward_only else ("forward", "reverse", "both", "source", "target")
        return sum(1 for cells in hits.values() if cells[name] in wanted)

    union = sum(1 for cells in hits.values() if any(cells[name] for name in DECLARED))
    union_size = len(set().union(*(pair_members[name] for name in PAIR_CATEGORIES))) + len(
        set().union(*(claim_members[name] for name in CLAIM_CATEGORIES))
    )

    def per_node(count: int) -> str:
        return f"{count / claim_count:.3f}" if claim_count else ""

    pool_rows = [
        (
            name,
            str(size[name]),
            per_node(size[name]),
            str(found(name)),
            str(found(name, forward_only=True)) if name in pair_members else "",
            str(len(missed)),
        )
        for name in categories
    ]
    pool_rows.append(("declared union", str(union_size), per_node(union_size), str(union), "", str(len(missed))))

    out: Path = args.out
    (out / "members").mkdir(parents=True, exist_ok=True)
    compare.write_tsv(out, "pool.tsv", ("category", "members", "per_node", "yield", "yield_forward", "misses"), pool_rows)
    for name, members in pair_members.items():
        rows = [(s, t, evidence) for (s, t), evidence in sorted(members.items())]
        compare.write_tsv(out / "members", f"{name}.tsv", ("source", "target", "evidence"), rows)
    for name, members in claim_members.items():
        compare.write_tsv(out / "members", f"{name}.tsv", ("claim", "evidence"), sorted(members.items()))
    compare.write_tsv(
        out,
        "misses.tsv",
        ("edge", "recall", *categories),
        [(miss["edge"], miss["recall"], *(hits[miss["edge"]][name] for name in categories)) for miss in missed],
    )

    def share(count: int) -> str:
        return f"{count / len(missed):.0%}" if missed else "-"

    unmarked = [miss for miss in missed if miss["unmarked"]]
    curve = []
    for k in NEAR_MISS_KS:
        name = f"near-miss@{k}"
        reached = sum(1 for miss in unmarked if hits[miss["edge"]][name] in ("forward", "both"))
        curve.append(
            f"| {k} | {size[name]} | {per_node(size[name])} | {found(name)} ({share(found(name))}) | "
            f"{found(name, forward_only=True)} | {reached} |"
        )
    lines = [
        "# Uncertainty-pool yield",
        "",
        f"script: {Path(__file__).resolve()}",
        f"ours: {args.ours}",
        f"edges: {args.edges}",
        f"node-pass record {ours.node_pass}: {ours.node_pass_note}",
        "",
        f"claims: {claim_count}; missed edges ({', '.join(MISSED)}): {len(missed)}; of them unmarked: {len(unmarked)}",
        *[f"  {recall}: {sum(1 for miss in missed if miss['recall'] == recall)}" for recall in MISSED],
        f"edges.tsv rows skipped as malformed: {len(skipped)}",
        *[f"  {note}" for note in skipped],
        "",
        "## The declared categories",
        "",
        "Yield counts a missed edge once per category, in either direction for a pair category and at either matched",
        "end for a claim category; forward counts a pair category's hits on (match of A, match of B) alone.",
        "",
        "| category | members | per claim | yield | forward |",
        "|---|---|---|---|---|",
        *[
            f"| {name} | {members} | {node} | {found_count} ({share(int(found_count))}) | {forward} |"
            for name, members, node, found_count, forward, _ in pool_rows
            if not name.startswith("near-miss")
        ],
        "",
        "## The near-miss curve (not declared; for the record)",
        "",
        "Members: (source, target) with target at rank <= K in the source's pool (shortlist.rank, own equations",
        f"left out), not answered {kb_pipeline.UNMARKED_POINTS_LETTER}. The last column counts the unmarked misses",
        f"(of {len(unmarked)}) reached forward.",
        "",
        "| K | members | per claim | yield | forward | unmarked forward |",
        "|---|---|---|---|---|---|",
        *curve,
        "",
    ]
    (out / "summary.md").write_text("\n".join(lines), encoding="utf-8", newline="\n")
    print(f"wrote pool.tsv, misses.tsv, members/, summary.md under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
