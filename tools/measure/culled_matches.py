"""The reference claims whose exact match, in a comparison made before the cull, is a node the build's cull removed.

Every input, output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import logging
import sys
from collections import Counter
from collections.abc import Mapping, Sequence
from pathlib import Path

from kb_tools import kb_yaml

import compare_to_pristine as compare

logger = logging.getLogger(__name__)

#: The cull record's file name, beside the build's kb-root.
CULL_RECORD = "kb-build-cull.yaml"
EXACT_CLASSES = ("number", "statement")
MATCH_COLUMNS = ("ref_id", "ref_title", "class", "ours_id")
CULLED_HEADER = ("ref_id", "ref_title", "class", "ours_id", "into", "survivor")


def file_argument(raw: str) -> Path:
    path = Path(raw)
    if not path.is_file():
        raise argparse.ArgumentTypeError(f"{raw!r} is not a file")
    return path


def read_claim_matches(path: Path) -> list[dict[str, str]]:
    """``claim-matches.tsv``'s rows by column name; refused when it lacks a column this measurement reads."""
    lines = path.read_text(encoding="utf-8").splitlines()
    header = lines[0].split("\t") if lines else []
    absent = [column for column in MATCH_COLUMNS if column not in header]
    if absent:
        raise ValueError(f"not a claim-matches.tsv of compare_to_pristine.py: no column {', '.join(absent)}")
    return [dict(zip(header, line.split("\t"))) for line in lines[1:] if line.strip()]


def _cull_record_list(path: Path, key: str) -> list:
    record = kb_yaml.parse(path.read_text(encoding="utf-8"))
    if not isinstance(record, Mapping) or not isinstance(record.get(key), list):
        raise ValueError(f"not a cull record: no {key} list")
    return record[key]


def read_culls(path: Path) -> dict[str, str]:
    """The cull record's ``culls``: each culled claim -> the claim it was culled into."""
    return {str(cull["id"]): str(cull["into"]) for cull in _cull_record_list(path, "culls")}


def read_cull_links(path: Path) -> set[frozenset[str]]:
    """The cull record's ``links``, every inferred link that survived the cull, each as its unordered pair of ids."""
    return {frozenset((str(link["a"]), str(link["b"]))) for link in _cull_record_list(path, "links")}


def survivor(node: str, culls: Mapping[str, str]) -> str:
    """Where ``node``'s cull chain ends: the first claim along ``into`` that was not itself culled."""
    seen = {node}
    while node in culls:
        node = culls[node]
        if node in seen:
            logger.warning("the cull record's into chain returns to %s", node)
            break
        seen.add(node)
    return node


def culled_matches(rows: Sequence[Mapping[str, str]], culls: Mapping[str, str]) -> list[tuple[str, ...]]:
    """One row (``CULLED_HEADER``) per exact-class match whose node was culled, in the table's order."""
    return [
        (row["ref_id"], row["ref_title"], row["class"], row["ours_id"], culls[row["ours_id"]], survivor(row["ours_id"], culls))
        for row in rows
        if row["class"] in EXACT_CLASSES and row["ours_id"] in culls
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--matches", required=True, type=file_argument, help="a pre-cull comparison's claim-matches.tsv")
    parser.add_argument("--cull", required=True, type=file_argument, help="the build's kb-build-cull.yaml")
    parser.add_argument(
        "--out", required=True, type=compare.scratch_directory_argument, help="output directory under .claude-temp/"
    )
    args = parser.parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")

    try:
        matches = read_claim_matches(args.matches)
    except (OSError, ValueError) as exc:
        print(f"--matches {str(args.matches)!r}: unreadable: {exc}", file=sys.stderr)
        return 1
    try:
        culls = read_culls(args.cull)
    except (OSError, ValueError, KeyError, TypeError) as exc:
        print(f"--cull {str(args.cull)!r}: unreadable: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    rows = culled_matches(matches, culls)
    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    compare.write_tsv(out, "culled-matches.tsv", CULLED_HEADER, rows)
    summary = [
        "# exact matches whose node the cull removed",
        "",
        f"script: {Path(__file__).resolve()}",
        f"matches: {args.matches}",
        f"cull: {args.cull}",
        "",
        f"culls in the record: {len(culls)}",
        f"exact-class matches whose node was culled: {len(rows)}",
        "by class:",
        *(compare.counted(Counter(row[2] for row in rows)) or ["  none"]),
        f"reference claims affected: {len({row[0] for row in rows})}",
        "",
    ]
    (out / "summary.md").write_text("\n".join(summary), encoding="utf-8", newline="\n")
    print(f"wrote culled-matches.tsv, summary.md under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
