"""Shortlist ranking sweep: where candidate rankings put the true target of each unmarked miss, and what K costs.

Every input, output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
Each ranking is a pure function of the stage's inputs — the statements, the sources, the existing candidate pairs
and each source's own equations — plus, where it needs them, the nodes' kinds or titles.
"""

import argparse
import logging
import sys
from collections.abc import Collection, Iterable, Mapping, Sequence
from pathlib import Path

from kb_tools.kb_claimgraph import shortlist, unmarked

import compare_to_pristine as compare
from measure_unmarked_shortlist import Build, edges_argument, read_misses
from statement_match import overlap

logger = logging.getLogger(__name__)

KS = (3, 5, 10)
EXACT_PAIR_CLASS = "statement + statement"

Pair = tuple[str, str]
#: Per source, the lists it asks about, each best first; a target sits in at most one list.
Shortlists = dict[str, tuple[list[str], ...]]


def stage_pools(
    statements: Mapping[str, str],
    *,
    sources: Iterable[str],
    candidate_pairs: Iterable[Pair],
    own: Collection[Pair],
) -> dict[str, list[str]]:
    """Each source's pool best first under ``shortlist.rank``, less its own equations: ``unmarked.plan``'s pool."""
    ranked = shortlist.rank(statements, sources=sources, candidate_pairs=candidate_pairs)
    return {source: [t for t in targets if (source, t) not in own] for source, targets in ranked.items()}


def blocks_first(pools: Mapping[str, Sequence[str]], *, blocks: Collection[str]) -> dict[str, list[str]]:
    """Each pool with its block candidates moved ahead of the rest, either part keeping its order."""
    return {s: [t for t in ts if t in blocks] + [t for t in ts if t not in blocks] for s, ts in pools.items()}


def split_by_kind(pools: Mapping[str, Sequence[str]], *, blocks: Collection[str]) -> Shortlists:
    """Each pool as two lists, its block candidates and the rest, each keeping the pool's order."""
    return {s: ([t for t in ts if t in blocks], [t for t in ts if t not in blocks]) for s, ts in pools.items()}


def restatements(statements: Mapping[str, str], *, source: str, targets: Iterable[str], threshold: float) -> list[str]:
    """The targets whose statement's term set overlaps the source's at ``threshold`` or above."""
    words = frozenset(shortlist.tokens(statements[source]))
    return [t for t in targets if overlap(words, frozenset(shortlist.tokens(statements[t]))) >= threshold]


def drop_restatements(
    pools: Mapping[str, Sequence[str]], statements: Mapping[str, str], *, threshold: float
) -> dict[str, list[str]]:
    """Each pool less its source's restatements (:func:`restatements`)."""
    words = {node: frozenset(shortlist.tokens(text)) for node, text in statements.items()}
    return {s: [t for t in ts if overlap(words[s], words[t]) < threshold] for s, ts in pools.items()}


def title_weighted(statements: Mapping[str, str], *, titles: Mapping[str, str]) -> dict[str, str]:
    """Each statement with its node's title appended once more, so the title's terms count twice in its bag."""
    return {node: f"{text}\n{titles[node]}" for node, text in statements.items()}


def single(pools: Mapping[str, list[str]]) -> Shortlists:
    return {source: (targets,) for source, targets in pools.items()}


def rankings(build: Build) -> dict[str, Shortlists]:
    """Every ranking of the sweep over one build, by name, in report order."""
    blocks = {node for node, kind in build.kind.items() if kind == "block"}
    inputs = {"sources": build.sources, "candidate_pairs": build.candidate_pairs, "own": build.own_equations}
    today = stage_pools(build.statement, **inputs)
    titled = stage_pools(title_weighted(build.statement, titles={n: node.title for n, node in build.nodes.items()}), **inputs)
    deduplicated = drop_restatements(today, build.statement, threshold=compare.STATEMENT_THRESHOLD)
    return {
        "today": single(today),
        "blocks first": single(blocks_first(today, blocks=blocks)),
        "split budget": split_by_kind(today, blocks=blocks),
        "duplicates removed": single(deduplicated),
        "title terms weighted": single(titled),
        "blocks first + duplicates removed": single(blocks_first(deduplicated, blocks=blocks)),
        "split budget + duplicates removed": split_by_kind(deduplicated, blocks=blocks),
    }


def position(lists: Sequence[Sequence[str]], target: str) -> int | None:
    """The target's 1-based rank in whichever of the lists holds it, else None."""
    for ranked in lists:
        if target in ranked:
            return ranked.index(target) + 1
    return None


def best_rank(shortlists: Shortlists, sources: Iterable[str], targets: Iterable[str]) -> int | None:
    """The best rank any (s, t) pair reaches on s's lists."""
    found = [r for s in sources if s in shortlists for t in targets if (r := position(shortlists[s], t)) is not None]
    return min(found, default=None)


def asks_at(shortlists: Shortlists, k: int) -> int:
    """The asks a top-``k`` of every list of every source costs."""
    return sum(min(k, len(ranked)) for lists in shortlists.values() for ranked in lists)


def read_exact_pairs(path: Path) -> list[str]:
    """The edges of edges.tsv the build missed outright with both ends matched by statement, as ``source->target``."""
    lines = path.read_text(encoding="utf-8").splitlines()
    header = lines[0].split("\t") if lines else []
    if "pair_class" not in header or "recall" not in header:
        raise ValueError("not an edges.tsv of compare_to_pristine.py: no pair_class or recall column")
    rows = [dict(zip(header, line.split("\t"))) for line in lines[1:] if line.strip()]
    return sorted(
        f"{row['ref_source']}->{row['ref_target']}"
        for row in rows
        if row.get("pair_class") == EXACT_PAIR_CLASS and row.get("recall") == "nothing"
    )


def read_probe_pairs(path: Path) -> list[tuple[str, str, str]]:
    """The probe's pairs.tsv rows as (pair name, source, target)."""
    lines = path.read_text(encoding="utf-8").splitlines()
    header = lines[0].split("\t") if lines else []
    if not {"pair", "source", "target"} <= set(header):
        raise ValueError("not a probe pairs.tsv: no pair, source or target column")
    rows = [dict(zip(header, line.split("\t"))) for line in lines[1:] if line.strip()]
    return [(row["pair"], row["source"], row["target"]) for row in rows]


def cell(rank: int | None) -> str:
    return "" if rank is None else str(rank)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ours", required=True, type=compare.kb_root_argument, help="the built kb-root")
    parser.add_argument(
        "--edges", required=True, type=edges_argument, help="compare_to_pristine.py's edges.tsv for the same --ours"
    )
    parser.add_argument("--probe-pairs", type=edges_argument, help="a probe's pairs.tsv: each pair's restatement test")
    parser.add_argument(
        "--out", required=True, type=compare.scratch_directory_argument, help="output directory under .claude-temp/"
    )
    args = parser.parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")

    try:
        measured, skipped = read_misses(args.edges)
        exact = set(read_exact_pairs(args.edges))
        probe = read_probe_pairs(args.probe_pairs) if args.probe_pairs else []
    except (OSError, UnicodeDecodeError, ValueError) as error:
        print(f"--edges or --probe-pairs: {error}", file=sys.stderr)
        return 1
    try:
        ours = Build(args.ours)
    except Exception as exc:  # noqa: BLE001 - a kb-root no reader can open is an unusable argument
        print(f"--ours {str(args.ours)!r}: unreadable: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    swept = rankings(ours)
    blocks = {node for node, kind in ours.kind.items() if kind == "block"}
    planned = unmarked.plan(ours.nodes, ours.statement, candidate_pairs=ours.candidate_pairs, own=ours.own_equations)
    today_top = [(s, t) for s, (ts,) in swept["today"].items() for t in ts[: shortlist.K]]

    names = list(swept)
    rank_rows, ranks = [], {}
    for miss in measured:
        sources = [s for s in miss["sources"] if s in ours.nodes and ours.kind[s] != "equation"]
        targets = [t for t in miss["targets"] if t in ours.nodes]
        ranks[miss["edge"]] = {name: best_rank(swept[name], sources, targets) for name in names}
        among_blocks = best_rank(swept["split budget"], sources, [t for t in targets if t in blocks])
        rank_rows.append(
            (
                miss["edge"],
                "yes" if miss["edge"] in exact else "no",
                " ".join(sorted({ours.kind[t] for t in targets})),
                *(cell(ranks[miss["edge"]][name]) for name in names),
                cell(among_blocks),
            )
        )

    def reached(name: str, k: int, edges: Iterable[str]) -> int:
        return sum(1 for edge in edges if (rank := ranks[edge][name]) is not None and rank <= k)

    removed = {s: len(swept["today"][s][0]) - len(swept["duplicates removed"][s][0]) for s in ours.sources}
    exact_measured = [edge for edge in ranks if edge in exact]

    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    compare.write_tsv(out, "ranks.tsv", ("edge", "exact", "target_kinds", *names, "among blocks"), rank_rows)

    table = [f"| {name} | " + " | ".join(f"{reached(name, k, ranks)} / {asks_at(swept[name], k)}" for k in KS) + " |" for name in names]
    exact_table = [f"| {edge} | " + " | ".join(cell(ranks[edge][name]) for name in names) + " |" for edge in exact_measured]
    probe_lines = [
        f"  {pair} ({source} -> {target}): overlap "
        f"{overlap(frozenset(shortlist.tokens(ours.statement[source])), frozenset(shortlist.tokens(ours.statement[target]))):.2f}, "
        + ("removed" if restatements(ours.statement, source=source, targets=[target], threshold=compare.STATEMENT_THRESHOLD) else "kept")
        for pair, source, target in probe
        if source in ours.statement and target in ours.statement
    ]
    lines = [
        "# Shortlist ranking sweep",
        "",
        f"script: {Path(__file__).resolve()}",
        f"ours: {args.ours}",
        f"edges: {args.edges}",
        f"probe pairs: {args.probe_pairs or 'none given'}",
        f"node-pass record {ours.node_pass}: {ours.node_pass_note}",
        f"misses measured: {len(measured)}; edges.tsv rows skipped as malformed: {len(skipped)}",
        f"today's top {shortlist.K} equals unmarked.plan's pairs: {'yes' if tuple(today_top) == planned.pairs else 'NO'}",
        "",
        "## Forward reach of the misses / asks, by ranking and K",
        "",
        f"Cells are misses reached of {len(measured)} / asks that K costs. Split budget asks top K of each list.",
        "",
        "| ranking | " + " | ".join(f"K = {k}" for k in KS) + " |",
        "|---|" + "---|" * len(KS),
        *table,
        "",
        f"## The exact pairs (pair_class '{EXACT_PAIR_CLASS}', recall 'nothing'): forward rank",
        "",
        "| edge | " + " | ".join(names) + " |",
        "|---|" + "---|" * len(names),
        *exact_table,
        "",
        f"## Restatements removed (term-set overlap with the source's statement >= {compare.STATEMENT_THRESHOLD})",
        "",
        f"  candidates removed per source: mean {sum(removed.values()) / len(removed):.2f}, "
        f"max {max(removed.values())}, sources with any {sum(1 for n in removed.values() if n)} of {len(removed)}",
        f"  misses whose every true pair was removed: "
        f"{sum(1 for r in ranks.values() if r['today'] is not None and r['duplicates removed'] is None)}",
        *(["  probe pairs:", *probe_lines] if probe_lines else []),
        "",
    ]
    (out / "sweep.md").write_text("\n".join(lines), encoding="utf-8", newline="\n")
    print(f"wrote ranks.tsv, sweep.md under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
