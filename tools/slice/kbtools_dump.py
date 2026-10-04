"""Dump what kb_tools' own readers see in KB trees, as one JSON (and so YAML) document.

The output schema is tools/slice/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import dataclasses
import enum
import json
import logging
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any

from kb_tools.kb_claimgraph import conform, inventory, tree
from kb_tools.kb_claimgraph.report import ClaimGraphError
from kb_tools.kb_docgraph.text import markdown_tokens
from kb_tools.kb_index_lib import UPLINK_MARKER

logger = logging.getLogger(__name__)

ROLES = ("ref", "kbase")


@dataclasses.dataclass(frozen=True)
class Root:
    paper: str
    role: str
    path: str


def _plain(value: Any) -> Any:
    """A reader value as JSON-ready data: dataclasses by their own field names, sets sorted."""
    if dataclasses.is_dataclass(value) and not isinstance(value, type):
        return {field.name: _plain(getattr(value, field.name)) for field in dataclasses.fields(value)}
    if isinstance(value, enum.Enum):
        return value.value
    if isinstance(value, (frozenset, set)):
        return sorted(_plain(item) for item in value)
    if isinstance(value, (list, tuple)):
        return [_plain(item) for item in value]
    return value


def _tree_entries(read: tree.Tree) -> list[dict]:
    return [
        {
            "path": path,
            "heading": read.documents[path].heading,
            "parent": read.parents.get(path),
            "children": list(read.children[path]),
        }
        for path in sorted(read.documents)
    ]


def _tokens(read: tree.Tree) -> dict[str, list[str]]:
    return {path: markdown_tokens(document.text) for path, document in read.documents.items()}


def _document_forms(document: tree.Document) -> dict[str, int]:
    """Element counts of the load-bearing forms, each over the text the matching inventory scan reads."""
    lines = document.lines
    unquoted = tree.unquote(document.text)
    unmarked = tree.unquote(tree.strip_markers(document.text))
    return {
        "uplink_lines": int(bool(lines) and UPLINK_MARKER in lines[0]),
        "anchors": sum(1 for _ in tree.ANCHOR_RE.finditer(unmarked)),
        "citation_spans": sum(1 for _ in inventory.CITATION_SPAN_RE.finditer(unmarked)),
        "math_fences": sum(1 for line in unquoted.splitlines() if line.strip() == inventory.FENCE_OPEN),
        "labelled_blocks": sum(1 for line in lines if inventory.LABEL_LINE_RE.match(line)),
    }


def _forms(read: tree.Tree) -> dict[str, dict[str, int]]:
    return {path: _document_forms(document) for path, document in read.documents.items()}


def _conformance(read: tree.Tree) -> dict:
    try:
        conform.gate(read)
    except ClaimGraphError as failure:
        return {"passed": False, "failure": {"check": failure.check, "detail": failure.detail}}
    return {"passed": True, "failure": None}


def _inventory(read: tree.Tree) -> dict:
    return _plain(inventory.scan(read))


def _attempt(root: Root, field: str, errors: dict[str, str], reader: Callable[[], Any]) -> Any:
    """``reader()``, or None with the failure recorded under ``field``: one bad tree must not stop the dump."""
    try:
        return reader()
    except Exception as exc:  # noqa: BLE001 - any reader failure is a finding about this tree
        errors[field] = f"{type(exc).__name__}: {exc}"
        logger.warning("%s %s %s: %s failed: %s", root.role, root.paper, root.path, field, errors[field])
        return None


def dump_root(root: Root) -> dict:
    errors: dict[str, str] = {}
    entry: dict[str, Any] = {
        "path": root.path,
        "role": root.role,
        "paper": root.paper,
        "errors": errors,
        "tree": None,
        "tokens": None,
        "forms": None,
        "conformance": None,
        "inventory": None,
    }
    read = _attempt(root, "tree", errors, lambda: tree.read(Path(root.path)))
    if read is None:
        return entry
    entry["tree"] = _attempt(root, "tree", errors, lambda: _tree_entries(read))
    entry["tokens"] = _attempt(root, "tokens", errors, lambda: _tokens(read))
    entry["forms"] = _attempt(root, "forms", errors, lambda: _forms(read))
    entry["conformance"] = _attempt(root, "conformance", errors, lambda: _conformance(read))
    entry["inventory"] = _attempt(root, "inventory", errors, lambda: _inventory(read))
    return entry


def _roots(parser: argparse.ArgumentParser, args: argparse.Namespace) -> list[Root]:
    roots: list[Root] = []
    for role in ROLES:
        for value in getattr(args, role) or ():
            paper, sep, path = value.partition("=")
            if not sep or not paper:
                parser.error(f"--{role} {value!r}: expected <paper id>=<kb-root path>")
            if not Path(path).is_dir():
                parser.error(f"--{role} {value!r}: {path!r} is not a directory")
            roots.append(Root(paper=paper, role=role, path=path))
    if not roots:
        parser.error("at least one --ref or --kbase is required")
    pairs = [(root.paper, root.role) for root in roots]
    duplicated = sorted({pair for pair in pairs if pairs.count(pair) > 1})
    if duplicated:
        parser.error(f"(paper, role) given more than once: {duplicated}")
    return sorted(roots, key=lambda root: (root.paper, root.role, root.path))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    for role in ROLES:
        parser.add_argument(f"--{role}", action="append", metavar="ID=KB_ROOT", help=f"a {role} kb-root; repeatable")
    parser.add_argument("--out", required=True, type=Path, help="output file")
    args = parser.parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")
    roots = _roots(parser, args)
    if not args.out.parent.is_dir():
        parser.error(f"--out {str(args.out)!r}: {str(args.out.parent)!r} is not a directory")

    document = {"roots": [dump_root(root) for root in roots]}
    text = json.dumps(document, sort_keys=True, indent=2, ensure_ascii=False) + "\n"
    args.out.write_text(text, encoding="utf-8", newline="\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
