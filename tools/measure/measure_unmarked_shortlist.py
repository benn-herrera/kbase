"""Shortlist recall: how many of the reference's unmarked missed edges the production shortlist reaches at K.

Every input, output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import logging
import sys
from collections import Counter
from pathlib import Path

from kb_tools import kb_pipeline
from kb_tools.kb_claimgraph import (
    attribute,
    classify,
    equation_sites,
    graph,
    hand_named,
    inventory,
    shortlist,
    unmarked,
)

import compare_to_pristine as compare

logger = logging.getLogger(__name__)

KS = (1, 3, 5)
EDGE_COLUMNS = ("ref_source", "ref_target", "mark_split", "ours_sources", "ours_targets")


class Build:
    """One kb-root's nodes, statements and existing candidate pairs, read through production code."""

    def __init__(self, kb_root: Path):
        documents = compare.read_tree(kb_root)
        sites = inventory.scan(documents)
        authored = graph.read(documents, sites)
        statement = classify.statements(documents, authored, sites)
        self.node_pass = kb_root.parent / kb_pipeline.NODE_PASS_RELPATH
        try:
            record = kb_pipeline.read_node_pass(kb_root.parent)
            self.node_pass_note = "read" if record is not None else "none found; every prose reference unjudged"
        except kb_pipeline.NodePassRecordError as error:
            record = None
            self.node_pass_note = f"unreadable, so every prose reference unjudged: {error}"
            logger.warning("%s", self.node_pass_note)
        narrowed = attribute.narrow(documents, authored, sites, record)
        blocks = {node.id for node, _, _ in hand_named._block_claims(authored, sites)}

        self.nodes = dict(sorted(authored.nodes.items()))
        self.kind = {
            nid: "equation" if node.equation is not None else "block" if nid in blocks else "prose"
            for nid, node in self.nodes.items()
        }
        self.document = {nid: node.document for nid, node in self.nodes.items()}
        self.statement = {nid: statement(node) for nid, node in self.nodes.items()}
        self.sources = [nid for nid, kind in self.kind.items() if kind != "equation"]
        self.candidate_pairs = [candidate.pair for candidate in narrowed.candidates]
        self.own_equations = equation_sites.own_equations(documents, authored, sites)
        excluded = {frozenset(pair) for pair in self.candidate_pairs}
        self.pool_size = {
            s: sum(1 for t in self.nodes if t != s and frozenset((s, t)) not in excluded) for s in self.sources
        }

    def asks_at(self, k: int) -> int:
        """The asks a per-source top-``k`` costs."""
        return sum(min(k, size) for size in self.pool_size.values())


def read_misses(path: Path) -> tuple[list[dict[str, object]], list[str]]:
    """The rows of compare_to_pristine's edges.tsv whose mark split is unmarked, and a note per unusable row."""
    lines = path.read_text(encoding="utf-8").splitlines()
    header = lines[0].split("\t") if lines else []
    absent = [column for column in EDGE_COLUMNS if column not in header]
    if absent:
        raise ValueError(f"not an edges.tsv of compare_to_pristine.py: no column {', '.join(absent)}")
    measured, skipped = [], []
    for number, line in enumerate(lines[1:], start=2):
        if not line.strip():
            continue
        cells = line.split("\t")
        if len(cells) != len(header):
            skipped.append(f"line {number}: {len(cells)} fields, the header has {len(header)}")
            continue
        row = dict(zip(header, cells))
        if row["mark_split"] == compare.UNMARKED:
            measured.append(
                {
                    "edge": f"{row['ref_source']}->{row['ref_target']}",
                    "sources": row["ours_sources"].split(),
                    "targets": row["ours_targets"].split(),
                }
            )
    return sorted(measured, key=lambda miss: miss["edge"]), skipped


def best(ranked: dict[str, dict[str, int]], sources: list[str], targets: list[str]) -> tuple[int | None, str]:
    """The best rank any (s, t) pair reaches on s's shortlist, and that pair."""
    found = [(ranked[s][t], f"{s}->{t}") for s in sources if s in ranked for t in targets if t in ranked[s]]
    return min(found) if found else (None, "")


def edges_argument(raw: str) -> Path:
    path = Path(raw)
    if not path.is_file():
        raise argparse.ArgumentTypeError(f"{raw!r} is not a file")
    return path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ours", required=True, type=compare.kb_root_argument, help="the built kb-root")
    parser.add_argument(
        "--edges", required=True, type=edges_argument, help="compare_to_pristine.py's edges.tsv for the same --ours"
    )
    parser.add_argument(
        "--out", required=True, type=compare.scratch_directory_argument, help="output directory under .claude-temp/"
    )
    args = parser.parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")

    try:
        measured, skipped = read_misses(args.edges)
    except (OSError, UnicodeDecodeError, ValueError) as error:
        print(f"--edges {str(args.edges)!r}: {error}", file=sys.stderr)
        return 1
    try:
        ours = Build(args.ours)
    except Exception as exc:  # noqa: BLE001 - a kb-root no reader can open is an unusable argument
        print(f"--ours {str(args.ours)!r}: unreadable: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    ordered = shortlist.rank(ours.statement, sources=ours.sources, candidate_pairs=ours.candidate_pairs)
    ranked = {source: {t: rank for rank, t in enumerate(targets, start=1)} for source, targets in ordered.items()}
    planned = set(
        unmarked.plan(ours.nodes, ours.statement, candidate_pairs=ours.candidate_pairs, own=ours.own_equations).pairs
    )

    foreign = sorted({nid for miss in measured for nid in (*miss["sources"], *miss["targets"]) if nid not in ours.nodes})
    miss_rows = []
    for miss in measured:
        sources = [s for s in miss["sources"] if s in ours.nodes and ours.kind[s] != "equation"]
        targets = [t for t in miss["targets"] if t in ours.nodes]
        reverse_sources = [t for t in targets if ours.kind[t] != "equation"]
        nearest = min(
            (unmarked.locality(ours.document[s], ours.document[t]) for s in sources for t in targets if s != t),
            key=unmarked.LOCALITIES.index,
            default="",
        )
        forward, pair = best(ranked, sources, targets)
        backward, _ = best(ranked, reverse_sources, sources)
        miss["locality"], miss["ranks"] = nearest, (forward, backward)
        miss["stage"] = (
            any((s, t) in planned for s in sources for t in targets),
            any((t, s) in planned for s in sources for t in targets),
        )
        miss_rows.append((miss["edge"], nearest, "" if forward is None else str(forward), "" if backward is None else str(backward), pair))

    curve_rows = []
    for k in KS:
        forward = sum(1 for m in measured if m["ranks"][0] is not None and m["ranks"][0] <= k)
        either = sum(1 for m in measured if any(rank is not None and rank <= k for rank in m["ranks"]))
        curve_rows.append((str(k), str(ours.asks_at(k)), str(forward), str(either)))

    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    compare.write_tsv(out, "misses.tsv", ("edge", "locality", "fwd", "rev", "best_pair"), miss_rows)
    compare.write_tsv(out, "curve.tsv", ("K", "asks", "forward", "either"), curve_rows)

    kinds = Counter(ours.kind.values())
    stage_forward = sum(1 for m in measured if m["stage"][0])
    stage_either = sum(1 for m in measured if any(m["stage"]))
    lines = [
        "# Unmarked shortlist recall",
        "",
        f"script: {Path(__file__).resolve()}",
        f"ours: {args.ours}",
        f"edges: {args.edges}",
        f"node-pass record {ours.node_pass}: {ours.node_pass_note}",
        "",
        f"nodes: {len(ours.nodes)} (block {kinds.get('block', 0)}, prose {kinds.get('prose', 0)}, "
        f"equation {kinds.get('equation', 0)}); sources {len(ours.sources)}; existing candidates {len(ours.candidate_pairs)}",
        f"misses measured: {len(measured)} (edges.tsv rows whose mark_split is '{compare.UNMARKED}')",
        f"edges.tsv rows skipped as malformed: {len(skipped)}",
        *[f"  {note}" for note in skipped],
        f"node ids in edges.tsv that this build does not have: {len(foreign)}" + (f" ({' '.join(foreign)})" if foreign else ""),
        "",
        "## Locality of the misses (nearest matched pair)",
        "",
        *[f"  {name}: {sum(1 for m in measured if m['locality'] == name)}" for name in unmarked.LOCALITIES],
        f"  no pair: {sum(1 for m in measured if not m['locality'])}",
        "",
        "## Per-source top-K under shortlist.rank: misses reached, forward / either direction",
        "",
        "| K | asks | forward | either |",
        "|---|---|---|---|",
        *[f"| {k} | {asks} | {forward} | {either} |" for k, asks, forward, either in curve_rows],
        "",
        f"## The stage's own shortlist (unmarked.plan, K = {shortlist.K}, own equations left out)",
        "",
        f"  misses reached, forward / either: {stage_forward} / {stage_either}",
        "",
    ]
    (out / "summary.md").write_text("\n".join(lines), encoding="utf-8", newline="\n")
    print(f"wrote misses.tsv, curve.tsv, summary.md under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
