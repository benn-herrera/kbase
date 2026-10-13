"""Read-only comparison of a built KB against a reference KB of the same corpus.

Every output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import itertools
import logging
import re
import sys
from collections import Counter, deque
from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass, replace
from pathlib import Path
from typing import Any

from kb_tools import kb_index_lib, kb_load, kb_schema
from kb_tools.kb_claimgraph import graph, inventory, prose, shortlist, tree
from kb_tools.kb_claimgraph.tree import strip_markers, unquote
from latex_numbering import COMMENT_RE, SPACE_RE, Numbering, braced, read_source, volume_argument, walk
from number_match import (
    NAME_FORMS,
    NAME_NUMBER_RE,
    NumberOutcome,
    PageBlock,
    match_numbers,
    unwalked_blocks,
    walked_environment,
)
from statement_match import StatementMatches, match_statements, overlap

logger = logging.getLogger(__name__)

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"
THRESHOLD = 0.30
STATEMENT_THRESHOLD = 0.6
TOP_K = 5

HEADING_RE = re.compile(r"^#{1,6} ")
# An HTML comment, or a tag: a name right after `<` or `</`, attributes each a name with an optional value. A `<`
# followed by a space, a backslash or a digit is maths (`a < b`, `<\text{…}>`), never a tag.
TAG_RE = re.compile(
    r"<!--.*?-->"
    r"|</?[A-Za-z][A-Za-z0-9-]*(?:\s+[A-Za-z_:][-\w:.]*(?:\s*=\s*(?:\"[^\"]*\"|'[^']*'|[^\s\"'=<>`]+))?)*\s*/?>",
    re.DOTALL,
)
WORD_RE = re.compile(r"[a-z]{2,}")
ENTRY_VOLUME_RE = re.compile(r"^- \[.*\]\(([^/()]+)/index\.md\)$", re.MULTILINE)

MISSES = ("references only", "demoted", "nothing")
UNMATCHED = "endpoint unmatched"
MISSED = (*MISSES, UNMATCHED)
UNMARKED = "unmarked — needs reading"
PRESENT, NONE_FOUND, UNDETERMINED = "present, unused", "none found", "undetermined"
SWEEP = (0.20, 0.25, 0.30, 0.35, 0.40, 0.50)
#: The classes in which some built edge or path recovers the reference edge.
RECOVERING = ("depends", "depends reversed", "depends path", "depends path reversed")
RECALL_CLASSES = (*RECOVERING, "demoted", "references only", "shared node", "nothing", UNMATCHED)
EDGE_HEADER = (
    "ref_source",
    "ref_source_title",
    "ref_target",
    "ref_target_title",
    "recall",
    "witness",
    "path_length",
    "mark_split",
    "marks",
    "ours_sources",
    "ours_targets",
    "scope",
    "document_level",
    "evidence",
    "evidence_basis",
    "pair_class",
    "source_class",
    "source_scores",
    "target_class",
    "target_scores",
)
#: The cascade's classes in the order tried, each with how the summary's heading describes it.
MATCH_CLASS_HEADINGS = {
    "number": "number",
    "statement": "statement overlap",
    "cosine": "TF-IDF cosine over title+statement",
}
MATCH_CLASSES = tuple(MATCH_CLASS_HEADINGS)
PAIR_CLASSES = tuple(f"{a} + {b}" for i, a in enumerate(MATCH_CLASSES) for b in MATCH_CLASSES[i:])
COLUMN = {name: index for index, name in enumerate(EDGE_HEADER)}


# --- arguments and output, shared with measure_unmarked_shortlist.py --------------------


def kb_root_argument(raw: str) -> Path:
    path = Path(raw)
    if not path.is_dir():
        raise argparse.ArgumentTypeError(f"{raw!r} is not a directory")
    return path


def scratch_directory_argument(raw: str) -> Path:
    """An output directory strictly inside the project's .claude-temp/: measurements are never committed."""
    out = Path(raw).resolve()
    if not SCRATCH.is_dir():
        raise argparse.ArgumentTypeError(f"{SCRATCH} does not exist; measurement output lands only under it")
    if SCRATCH not in out.parents:
        raise argparse.ArgumentTypeError(
            f"{raw!r} is not a directory under {SCRATCH}: measurement output lands only in a directory under the "
            "project's .claude-temp/, never where a recipe could commit it"
        )
    if out.exists() and not out.is_dir():
        raise argparse.ArgumentTypeError(f"{raw!r} exists and is not a directory")
    return out


def read_tree(kb_root: Path) -> tree.Tree:
    """``tree.read``, refusing a directory that holds no KB document: there is nothing to measure."""
    documents = tree.read(kb_root)
    if not documents.documents:
        raise ValueError("no KB document under it")
    return documents


def format_refusal(kb_root: Path) -> str | None:
    """Why the installed kb_tools cannot read ``kb_root`` as it stands, else None.

    Its readers take a KB at an older format for one declaring no claims, so an older stamp is refused here rather
    than measured as empty.
    """
    try:
        kb_format = kb_load.open_kb(kb_root)
    except kb_load.FormatRefusal as refusal:
        return str(refusal)
    if kb_format.current:
        return None
    return (
        f"the KB's metadata format is {kb_format.version} and the installed kb_tools reads {kb_schema.FORMAT_VERSION}: "
        "run `kbase refresh` on a copy of its repository and compare the copy; never refresh the fixture itself"
    )


def write_tsv(out: Path, name: str, header: Iterable[str], rows: Iterable[Iterable[str]]) -> None:
    def clean(cell: str) -> str:
        return cell.replace("\t", " ").replace("\r", " ").replace("\n", " ")

    body = ["\t".join(header)] + ["\t".join(clean(cell) for cell in row) for row in rows]
    (out / name).write_text("\n".join(body) + "\n", encoding="utf-8", newline="\n")


def counted(counter: Counter) -> list[str]:
    """``counter``'s entries as summary lines, most first, ties by name."""
    return [f"  {name}: {count}" for name, count in sorted(counter.items(), key=lambda item: (-item[1], item[0]))]


def attempt(failures: list[str], label: str, reader: Callable[[], Any], default: Any) -> Any:
    """``reader()``, or ``default`` with the failure recorded: one bad document must not stop the comparison."""
    try:
        return reader()
    except Exception as exc:  # noqa: BLE001 - any reader failure is a finding about this document
        failures.append(f"{label}: {type(exc).__name__}: {exc}")
        logger.warning("%s", failures[-1])
        return default


# --- text ----------------------------------------------------------------------------


@dataclass(frozen=True)
class Span:
    """A half-open line range of one document: part of the text a claim is read from."""

    document: str
    start: int
    end: int


@dataclass(frozen=True)
class Claim:
    id: str
    title: str
    kind: str
    documents: tuple[str, ...]
    text: str
    spans: tuple[Span, ...] = ()
    printed: tuple[str, ...] = ()
    #: The author's ``\label`` the page carries for it: a block's ``<span id>``, an equation's fence label.
    label: str = ""


def tokens(text: str) -> list[str]:
    plain = TAG_RE.sub(" ", strip_markers(text)).lower()
    return [word for word in WORD_RE.findall(plain) if word not in shortlist.STOPWORDS]


def plain_text(text: str) -> str:
    return re.sub(r"\s+", " ", TAG_RE.sub(" ", unquote(strip_markers(text))).replace("*", "")).strip()


# --- the reference KB ----------------------------------------------------------------


def hosting(documents: tree.Tree, failures: list[str]) -> dict[str, list[str]]:
    hosts: dict[str, list[str]] = {}
    for path, document in sorted(documents.documents.items()):
        fields = attempt(failures, f"reference {path}", lambda: kb_index_lib.parse_frontmatter(document.text), None)
        for claim_id in (fields or {}).get("claims") or ():
            hosts.setdefault(claim_id, []).append(path)
    return hosts


def marker_line(lines: Sequence[str], claim_id: str) -> int | None:
    """The first of ``lines`` holding a Tier-2 marker that names ``claim_id``, else None."""
    return next(
        (number for number, line in enumerate(lines) if any(claim_id in ids for _, ids in kb_index_lib.tier2_markers(line))),
        None,
    )


def reference_section(text: str, claim_id: str) -> str:
    """The claim's statement: from its marker to the next heading or marker, else the leaf's opening section."""
    lines = kb_index_lib.strip_frontmatter(text).splitlines()
    start = marker_line(lines, claim_id)
    if start is None:
        start = next((number for number, line in enumerate(lines) if line.startswith("# ")), -1)
    start += 1
    end = start
    while end < len(lines) and not HEADING_RE.match(lines[end]) and not kb_index_lib.tier2_markers(lines[end]):
        end += 1
    return "\n".join(lines[start:end])


def marked_statement(document: tree.Document, claim_id: str, *, sites: inventory.Inventory, readable: prose.Readable) -> Span | None:
    """The statement ``claim_id``'s Tier-2 marker marks in ``document``, else None.

    The labelled block, math fence or prose paragraph holding the marker's line (the outermost, where they nest);
    else the first of them to open after it, before the next heading or marker — a marker standing on its own line
    above what it marks. None where the document carries no such marker or nothing follows it.
    """
    text = document.text
    line = next((text.count("\n", 0, offset) for offset, ids in kb_index_lib.tier2_markers(text) if claim_id in ids), None)
    if line is None:
        return None
    lines = text.splitlines()
    stop = next(
        (n for n in range(line + 1, len(lines)) if HEADING_RE.match(lines[n]) or kb_index_lib.tier2_markers(lines[n])),
        len(lines),
    )
    extents = sorted(
        [(block.start, block.end) for block in sites.blocks if block.document == document.path]
        + [(fence.start, fence.end) for fence in sites.fences if fence.document == document.path]
        + [(min(paragraph.lines), max(paragraph.lines) + 1) for paragraph in readable.render.paragraphs],
        key=lambda extent: (extent[0], -extent[1]),
    )
    found = next((extent for extent in extents if extent[0] <= line < extent[1]), None)
    found = found or next((extent for extent in extents if line < extent[0] < stop), None)
    return None if found is None else Span(document.path, *found)


@dataclass(frozen=True)
class Reference:
    claims: dict[str, Claim]
    edges: list[tuple[str, str]]
    relations: Counter
    #: Each claim's marked statements, plain text, one per citing page where its marker locates one.
    statements: dict[str, tuple[str, ...]]
    #: The claims whose marker stands on some citing page.
    marked: frozenset[str]


def read_reference(kb_root: Path, failures: list[str]) -> Reference:
    state = kb_index_lib.discover_kb(kb_root, diagnostic_stream=None)
    if not state.claim_entries:
        raise ValueError("no claim entry in its registers: nothing to compare against")
    documents = read_tree(kb_root)
    sites = inventory.scan(documents)
    marked_rows = [row for row in kb_index_lib.build_cites_records(state) if row["tier2_marked"]]
    readable = {
        path: prose.readable(documents.documents[path], sites) for path in sorted({row["leaf_path"] for row in marked_rows})
    }
    statements: dict[str, list[str]] = {}
    for row in marked_rows:
        path, claim_id = row["leaf_path"], row["claim_id"]
        span = attempt(
            failures,
            f"reference {claim_id} statement in {path}",
            lambda: marked_statement(documents.documents[path], claim_id, sites=sites, readable=readable[path]),
            None,
        )
        if span is not None:
            body = "\n".join(documents.documents[path].text.splitlines()[span.start : span.end])
            statements.setdefault(claim_id, []).append(plain_text(body))
    hosts = hosting(documents, failures)
    claims = {}
    for entry in state.claim_entries:
        docs = tuple(hosts.get(entry.id, ()))
        body = attempt(
            failures,
            f"reference {entry.id}",
            lambda: "\n".join(reference_section(documents.documents[doc].text, entry.id) for doc in docs),
            "",
        )
        claims[entry.id] = Claim(id=entry.id, title=entry.title, kind="reference", documents=docs, text=body)
    relations = Counter((edge.relation, edge.target_kind) for entry in state.claim_entries for edge in entry.depends_on)
    edges = sorted(
        {
            (edge.source, edge.target)
            for entry in state.claim_entries
            for edge in entry.depends_on
            if edge.relation == "depends" and edge.target_kind == "claim"
        }
    )
    return Reference(
        claims=claims,
        edges=edges,
        relations=relations,
        statements={claim_id: tuple(texts) for claim_id, texts in statements.items()},
        marked=frozenset(row["claim_id"] for row in marked_rows),
    )


# --- our KB ----------------------------------------------------------------------------


@dataclass(frozen=True)
class Ours:
    claims: dict[str, Claim]
    depends: set[tuple[str, str]]
    references: set[tuple[str, str]]
    #: The ``depends`` pairs the build's cycle breaking cut -> their origin (``cited``, ``inferred``, or empty).
    demoted: dict[tuple[str, str], str]
    anchors: list[inventory.Anchor]
    texts: dict[str, list[str]]
    page_blocks: list[PageBlock]


def edge_sets(
    entries: Sequence[kb_index_lib.ClaimEntry],
) -> tuple[set[tuple[str, str]], set[tuple[str, str]], dict[tuple[str, str], str]]:
    """The live claim-to-claim ``depends`` pairs, the ``references`` pairs, and the ``demoted`` pairs with their
    origin (empty where the row names none), each pair as (source, target)."""
    depends = {
        (edge.source, edge.target)
        for entry in entries
        for edge in entry.depends_on
        if edge.relation == "depends" and edge.target_kind == "claim"
    }
    references = {(edge.source, edge.target) for entry in entries for edge in entry.references}
    demoted = {(edge.source, edge.target): edge.origin or "" for entry in entries for edge in entry.demoted}
    return depends, references, demoted


def _our_claim(
    node: graph.ClaimNode,
    *,
    documents: tree.Tree,
    sites: inventory.Inventory,
    readable: dict[str, prose.Readable],
) -> Claim:
    lines = documents.documents[node.document].text.splitlines()
    blocks = [block for block in sites.claim_blocks() if block.document == node.document]
    block = next((found for found in blocks if found.display == node.locator), None)
    printed: tuple[str, ...] = ()
    label = ""
    if node.equation is not None:
        label = node.equation
        kind = "equation"
        fence = next((f for f in sites.fences if f.document == node.document and node.equation in f.labels), None)
        if fence is None:
            raise ValueError(f"no math fence carries the label {node.equation!r}")
        spans = (Span(node.document, fence.start, fence.end),)
    elif block is not None:
        kind = "block"
        label = block.identifier or ""
        spans = (Span(node.document, block.start, block.end),) + tuple(
            Span(proof.document, proof.start, proof.end)
            for proof in sites.proofs
            if any(subject.document == block.document and subject.start == block.start for subject in proof.subjects)
        )
        name = inventory._printed_name(block.display)
        printed = (plain_text(name).rstrip("."),) if name else ()
    else:
        kind = "prose"
        line = marker_line(lines, node.id)
        if node.document not in readable:
            readable[node.document] = prose.readable(documents.documents[node.document], sites)
        paragraph = None if line is None else readable[node.document].render.paragraph_at(line)
        if paragraph is not None:
            spans = (Span(node.document, min(paragraph.lines), max(paragraph.lines) + 1),)
        else:
            spans = () if line is None else (Span(node.document, line, line + 1),)
    text = "\n".join(
        "\n".join(documents.documents[span.document].text.splitlines()[span.start : span.end]) for span in spans
    )
    return Claim(
        id=node.id, title=node.title, kind=kind, documents=(node.document,), text=text, spans=spans, printed=printed, label=label
    )


def read_ours(kb_root: Path, failures: list[str]) -> Ours:
    documents = read_tree(kb_root)
    sites = inventory.scan(documents)
    authored = graph.read(documents, sites)
    readable: dict[str, prose.Readable] = {}
    claims: dict[str, Claim] = {}
    for node_id, node in sorted(authored.nodes.items()):
        claim = attempt(
            failures,
            f"ours {node_id} in {node.document}",
            lambda: _our_claim(node, documents=documents, sites=sites, readable=readable),
            None,
        )
        if claim is not None:
            claims[node_id] = claim

    depends, references, demoted = edge_sets(kb_index_lib.discover_kb(kb_root, diagnostic_stream=None).claim_entries)
    texts = {path: document.text.splitlines() for path, document in documents.documents.items()}
    located = {(node.document, node.locator): node_id for node_id, node in authored.nodes.items() if node.equation is None}
    page_blocks = [
        PageBlock(
            volume=block.document.split("/")[0],
            printed=plain_text(name).rstrip("."),
            node_id=located.get((block.document, block.display)) if block.claim_bearing else None,
        )
        for block in sites.blocks
        if block.display is not None and (name := inventory._printed_name(block.display))
    ]
    return Ours(
        claims=claims,
        depends=depends,
        references=references,
        demoted=demoted,
        anchors=list(sites.anchors),
        texts=texts,
        page_blocks=page_blocks,
    )


def entry_volumes(kb_root: Path) -> list[str]:
    """Our volume directories in the order the entry point lists them: the build's volume order."""
    text = (kb_root / kb_index_lib.ENTRY_POINT_FILENAME).read_text(encoding="utf-8")
    return ENTRY_VOLUME_RE.findall(text)


# --- volume roots found under a sources directory ----------------------------------------

DOCUMENTCLASS_RE = re.compile(r"\\documentclass(?![A-Za-z])")
SOURCE_INCLUDE_RE = re.compile(r"\\(?:input|include)\s*\{([^}]*)\}|\\input\s+([^\s{}\\]+)")
TITLE_RE = re.compile(r"\\title(?![A-Za-z])\s*(?:\[[^\]]*\]\s*)?")
TITLE_COMMAND_RE = re.compile(r"\\(?:(?P<name>[A-Za-z]+)\*?|(?P<symbol>.?))(?:\s*\[[^\]]*\])?", re.DOTALL)
#: Commands pandoc drops from a title together with their argument.
TITLE_DROPPED = frozenset({"thanks", "footnote", "vspace", "hspace"})
# kbase's `deleted` and `edges` in internal/docgraph/text.go: what `plain` removes from a title's words.
KBASE_DELETED = str.maketrans("", "", "*_~“”‘’\"'")
KBASE_EDGES = "`()[]{}<>.,;:!?|\\/#$&+=-–—…"
KBASE_SLUG_MAX = 48


def latex_title_text(title: str) -> str:
    """A ``\\title`` argument as the plain words pandoc's metadata title carries: line breaks and the commands in
    ``TITLE_DROPPED`` (with their argument) gone, any other command dropped and its braced argument unwrapped."""
    parts: list[str] = []
    position = 0
    while position < len(title):
        char = title[position]
        if char != "\\":
            parts.append(" " if char == "~" else "" if char in "{}" else char)
            position += 1
            continue
        command = TITLE_COMMAND_RE.match(title, position)
        position = command.end()
        name, symbol = command["name"], command["symbol"]
        if name in TITLE_DROPPED:
            argument = braced(title, SPACE_RE.match(title, position).end())
            if argument is not None:
                position = argument[1]
        elif name == "newline" or symbol == "\\" or (symbol and symbol.isspace()):
            parts.append(" ")
        elif symbol and symbol in "&%_#$":
            parts.append(symbol)
    return " ".join("".join(parts).split())


def kbase_plain(title: str) -> str:
    """kbase's ``plain`` over text with no Markdown left in it: its words, emphasis and quotation marks deleted."""
    words = (word.translate(KBASE_DELETED).strip(KBASE_EDGES) for word in title.split())
    return " ".join(word for word in words if word)


def kbase_slug(title: str) -> str:
    """kbase's ``slug`` (internal/docgraph/text.go), the volume directory it names a title; mirrors that rule and
    must change with it."""
    flat = re.sub(r"[^a-z0-9]+", "-", title.strip().lower()).strip("-")
    if len(flat) > KBASE_SLUG_MAX:
        flat = flat[:KBASE_SLUG_MAX]
        if "-" in flat:
            flat = flat[: flat.rindex("-")]
        flat = flat.strip("-")
    return flat or "untitled"


def volume_slug(root: Path, text: str) -> str:
    """The volume directory kbase gives ``root``, from its comment-stripped ``text``: its title's slug, or, as
    kbase does for a document with no title, its file stem's."""
    title = ""
    if (found := TITLE_RE.search(text)) is not None and (argument := braced(text, found.end())) is not None:
        title = latex_title_text(argument[0])
    return kbase_slug(kbase_plain(title or root.stem))


def read_sources(directory: Path) -> dict[Path, str]:
    """Every ``.tex`` file under ``directory`` outside a dot-directory, by path, with its comments removed."""
    texts = {}
    for path in sorted(directory.rglob("*.tex")):
        if any(part.startswith(".") for part in path.relative_to(directory).parent.parts):
            continue
        if (text := read_source(path)) is not None:
            texts[path] = COMMENT_RE.sub(r"\1", text)
    return texts


def source_roots(texts: Mapping[Path, str], *, directory: Path) -> list[Path]:
    """The files of ``texts`` holding ``\\documentclass`` that no file of ``texts`` names in ``\\input`` or
    ``\\include``, by path. A name is read with and without ``.tex``, from the naming file's directory and from
    ``directory``."""
    named: set[Path] = set()
    for path, text in texts.items():
        for match in SOURCE_INCLUDE_RE.finditer(text):
            name = (match[1] if match[1] is not None else match[2]).strip()
            for base in (path.parent, directory):
                named.update({(base / name).resolve(), (base / f"{name}.tex").resolve()})
    return sorted(path for path, text in texts.items() if DOCUMENTCLASS_RE.search(text) and path.resolve() not in named)


@dataclass(frozen=True)
class SourceMapping:
    """The volume roots under a sources directory, each with the volume directory kbase gives it, against our entry
    point's volumes."""

    directory: Path
    slugs: dict[Path, str]  # root -> its volume directory, by root path
    volumes: list[str]  # the entry point's order

    def roots_of(self, volume: str) -> list[Path]:
        return [root for root, slug in self.slugs.items() if slug == volume]

    def mapped(self) -> dict[str, Path]:
        """Each volume exactly one root takes, with that root, in the entry point's order."""
        return {volume: roots[0] for volume in self.volumes if len(roots := self.roots_of(volume)) == 1}


def map_sources(directory: Path, volumes: Sequence[str]) -> SourceMapping:
    texts = read_sources(directory)
    roots = source_roots(texts, directory=directory)
    slugs = {root: volume_slug(root, texts[root]) for root in roots}
    return SourceMapping(directory=directory, slugs=slugs, volumes=list(volumes))


def source_lines(mapping: SourceMapping) -> list[str]:
    """The mapping, then every root no volume took and every volume no single root reached."""
    mapped = mapping.mapped()
    taken = set(mapped.values())
    lines = [f"sources: {mapping.directory}", *(f"  {root} -> {volume}" for volume, root in mapped.items())]
    for root, slug in mapping.slugs.items():
        if root in taken:
            continue
        sharing = len(mapping.roots_of(slug)) if slug in mapping.volumes else 0
        lines.append(f"  {root}: volume {slug} is taken by {sharing} roots" if sharing else f"  {root}: no volume takes slug {slug}")
    for volume in mapping.volumes:
        if volume not in mapped:
            sharing = len(mapping.roots_of(volume))
            lines.append(f"  volume {volume}: {sharing} roots take its slug" if sharing else f"  volume {volume}: no root")
    return lines


# --- edges ----------------------------------------------------------------------------


def shortest_path(edges: set[tuple[str, str]], sources: set[str], targets: set[str]) -> tuple[str, ...] | None:
    """The fewest-edge chain over ``edges`` from some source to some target, sources and neighbours visited in id
    order so ties go the same way every run; None where no target is reached."""
    following: dict[str, set[str]] = {}
    for source, target in edges:
        following.setdefault(source, set()).add(target)
    previous: dict[str, str | None] = dict.fromkeys(sources)
    pending = deque(sorted(sources))
    while pending:
        here = pending.popleft()
        for node in sorted(following.get(here, ())):
            if node in previous:
                continue
            previous[node] = here
            if node in targets:
                path = [node]
                while (step := previous[path[-1]]) is not None:
                    path.append(step)
                return tuple(reversed(path))
            pending.append(node)
    return None


def name_pattern(name: str) -> re.Pattern[str] | None:
    parsed = NAME_NUMBER_RE.match(name)
    if parsed is None:
        return None
    word, number = parsed.group(1).lower(), re.escape(parsed.group(2).rstrip("."))
    return re.compile(rf"\b{NAME_FORMS[word]}\.?\s*~?{number}(?![\d]|\.\d)", re.IGNORECASE)


def _span_text(span: Span, texts: dict[str, list[str]]) -> str:
    return plain_text("\n".join(texts[span.document][span.start : span.end]))


def mark_test(
    *,
    a_nodes: list[Claim],
    b_nodes: list[Claim],
    walked_names: Mapping[str, str],
    anchors: list[inventory.Anchor],
    texts: dict[str, list[str]],
) -> list[str]:
    """Every mark of B inside the text our pipeline reads for A's matched claims.

    B is named only through its matches of ours, never the reference title: the reference may number against an
    earlier version of the sources. ``walked_names`` maps a block node to its environment's word and the author's
    number the walker gives it ("Theorem 5.11"); for a number-class match that is the number it matched on.
    """
    b_hosts = {doc for node in b_nodes for doc in node.documents}
    b_labels = {(node.documents[0].split("/")[0], node.label) for node in b_nodes if node.label}
    names = {name: "ours" for node in b_nodes for name in node.printed}
    for node in b_nodes:
        if node.id in walked_names:
            names.setdefault(walked_names[node.id], "walked")
    patterns = [(name, origin, found) for name, origin in sorted(names.items()) if (found := name_pattern(name)) is not None]
    marks = []
    for a in a_nodes:
        for span in a.spans:
            for anchor in anchors:
                if not (anchor.document == span.document and span.start <= anchor.line < span.end):
                    continue
                if anchor.target in b_hosts:
                    marks.append(f"anchor {a.id}:{anchor.reference_type} -> {anchor.href}")
                if (anchor.document.split("/")[0], anchor.label) in b_labels:
                    marks.append(f"label {a.id}:{anchor.reference_type} -> {anchor.label}")
            body = _span_text(span, texts)
            for name, origin, pattern in patterns:
                hit = pattern.search(body)
                if hit is not None:
                    marks.append(f"name {a.id}: '{hit.group()}' ({origin}: {name})")
    return sorted(set(marks))


def title_test(*, a_nodes: list[Claim], b_nodes: list[Claim], b_reference: Claim, texts: dict[str, list[str]]) -> list[str]:
    """Every title of B the mark test cannot see — no name and number in it — found whole in the text read for A.

    A title is looked for only where it has at least two words: one word alone matches by accident.
    """
    titles = {plain_text(node.title) for node in b_nodes} | {plain_text(b_reference.title)}
    phrases = sorted(title for title in titles if name_pattern(title) is None and len(WORD_RE.findall(title.lower())) >= 2)
    patterns = [(phrase, re.compile(rf"(?<!\w){re.escape(phrase)}(?!\w)", re.IGNORECASE)) for phrase in phrases]
    hits = []
    for a in a_nodes:
        for span in a.spans:
            body = _span_text(span, texts)
            hits += [f"title {a.id}: '{phrase}'" for phrase, pattern in patterns if pattern.search(body)]
    return sorted(set(hits))


@dataclass(frozen=True)
class Verdict:
    """How our graph holds a reference edge: its ``recall`` class, the built edges or path that decided it
    (``edges.tsv`` ``witness``), and the shortest path's edge count for a path class."""

    recall: str
    witness: str = ""
    path_length: int | None = None


def _arrows(pairs: Iterable[tuple[str, str]]) -> str:
    return " ".join(sorted(f"{x}->{y}" for x, y in pairs))


def verdict(
    a: set[str],
    b: set[str],
    *,
    depends: set[tuple[str, str]],
    references: set[tuple[str, str]],
    demoted: Mapping[tuple[str, str], str],
) -> Verdict:
    """How our graph holds A -> B over A's and B's matches, first that applies (README, ``edges.tsv`` ``recall``).

    The edge and path classes are read from x in A to y in B save where both lie in A ∩ B, so an edge between a
    shared node and the rest counts; ``shared node`` is the reading when A ∩ B is non-empty and none of them stands.
    """
    if not a or not b:
        return Verdict(f"{UNMATCHED} (" + " and ".join(end for end, s in (("source", a), ("target", b)) if not s) + ")")
    only_a, only_b = a - b, b - a
    forward = sorted({(x, y) for x in only_a for y in b} | {(x, y) for x in a for y in only_b})
    backward = [(y, x) for x, y in forward]
    if found := [pair for pair in forward if pair in depends]:
        return Verdict("depends", _arrows(found))
    if found := [pair for pair in backward if pair in depends]:
        return Verdict("depends reversed", _arrows(found))
    for name, start, end in (("depends path", a, b), ("depends path reversed", b, a)):
        only_start, only_end = start - end, end - start
        paths = [shortest_path(depends, only_start, end), shortest_path(depends, start, only_end)]
        path = min((found for found in paths if found is not None), key=lambda found: (len(found), found), default=None)
        if path is not None:
            return Verdict(name, "->".join(path), len(path) - 1)
    for pairs, direction in ((sorted(forward), ""), (sorted(backward), ", reversed")):
        cut = next((pair for pair in pairs if pair in demoted), None)
        if cut is not None:
            return Verdict(f"demoted (origin {demoted[cut] or 'not named'}{direction})", _arrows([cut]))
    if found := [pair for pair in forward + backward if pair in references]:
        return Verdict("references only", _arrows(found))
    if a & b:
        return Verdict("shared node")
    return Verdict("nothing")


def recall_class(found: str) -> str:
    return found.split(" (")[0]


def document_level(
    a: set[str], b: set[str], *, ours: dict[str, Claim], depends: set[tuple[str, str]], references: set[tuple[str, str]]
) -> str:
    """The claim-level verdict relaxed on B's side: any node of ours hosted where one of B's matches is hosted."""
    hosts = {ours[y].documents[0] for y in b}
    wide = {oid for oid, claim in ours.items() if claim.documents[0] in hosts} - a
    for relation, edges in (("depends", depends), ("references", references)):
        if any((x, y) in edges for x in a for y in wide):
            return relation
        if any((y, x) in edges for x in a for y in wide):
            return relation + " reversed"
    return "nothing"


def paper_scope(a: set[str], b: set[str], ours: dict[str, Claim]) -> str:
    """Whether some matched pair sits in one of our papers (top-level directory), where an anchor could join them."""
    if not a or not b:
        return ""
    papers = {ours[x].documents[0].split("/")[0] for x in a} & {ours[y].documents[0].split("/")[0] for y in b}
    return "same paper" if papers else "cross paper"


def pair_class(source: str, target: str, fixed: Mapping[str, str]) -> str:
    """The classes that matched an edge's two ends, in class order; an end ``fixed`` lacks counts as cosine."""
    ends = sorted((fixed.get(source, "cosine"), fixed.get(target, "cosine")), key=MATCH_CLASSES.index)
    return " + ".join(ends)


@dataclass(frozen=True)
class ClaimMatching:
    """The cascade's result: the exact classes' one-to-one matches and every reference claim's cosine ranking."""

    exact: dict[str, str]  # number class: reference claim -> node of ours
    statement: StatementMatches
    #: Every reference claim's nodes of ours by cosine, best first; keyed in reference title order.
    scored: dict[str, list[tuple[str, float]]]
    #: Each reference claim an exact class matched -> its node of ours.
    matched: dict[str, str]
    #: Each reference claim an exact class matched -> that class.
    fixed: dict[str, str]

    def ranking(self, rid: str) -> list[tuple[str, float]]:
        """``rid``'s nodes of ours by cosine, best first; for a claim left to the cosine class, less every node an
        exact class took, since those are one-to-one."""
        if rid in self.matched:
            return self.scored[rid]
        taken = set(self.matched.values())
        return [(oid, score) for oid, score in self.scored[rid] if oid not in taken]

    def mapping(self, at: float, *, top1: bool = False) -> dict[str, set[str]]:
        """Number and statement matches as they stand; every other reference claim matched by cosine at ``at``: every
        node at or above it, or with ``top1`` its best node alone where that one is."""
        mapped = {}
        for rid in self.scored:
            if rid in self.matched:
                mapped[rid] = {self.matched[rid]}
                continue
            kept = [oid for oid, score in self.ranking(rid) if score >= at]
            mapped[rid] = set(kept[:1] if top1 else kept)
        return mapped

    def match_class(self, rid: str) -> str:
        return self.fixed.get(rid, "cosine")

    def end_scores(self, rid: str, nodes: Iterable[str]) -> str:
        """``nodes`` (``rid``'s matches) as ``node=score``, by id: the statement overlap for a statement match, the
        cosine for a cosine one; a number match has no score and is the node alone."""
        if rid in self.exact:
            return " ".join(sorted(nodes))
        if rid in self.statement.matches:
            return " ".join(f"{oid}={self.statement.matches[rid][1]:.3f}" for oid in sorted(nodes))
        score_of = dict(self.ranking(rid))
        return " ".join(f"{oid}={score_of[oid]:.3f}" for oid in sorted(nodes))

    def weakest_end(self, source: str, target: str, mapped: Mapping[str, set[str]]) -> str:
        """The lower of the two ends' best cosine, banded, over the ends cosine matched; ``exact both`` where neither
        end is a cosine match; ``endpoint unmatched`` where either end has no match."""
        if not mapped[source] or not mapped[target]:
            return UNMATCHED
        tops = [self.ranking(rid)[0][1] for rid in (source, target) if rid not in self.matched]
        if not tops:
            return "exact both"
        low = min(tops)
        return "cosine < 0.40" if low < 0.40 else "cosine 0.40-0.50" if low < 0.50 else "cosine >= 0.50"


def cosine_scores(reference: Mapping[str, Claim], ours: Mapping[str, Claim]) -> dict[str, list[tuple[str, float]]]:
    """Each reference claim's nodes of ours by TF-IDF cosine over title plus text, best first, ties by id.

    Keyed in reference title order, then id; the vocabulary and its weights are shared by both sides.
    """
    bags = {("r", key): tokens(c.title + "\n" + c.text) for key, c in reference.items()}
    bags |= {("o", key): tokens(c.title + "\n" + c.text) for key, c in ours.items()}
    idf = shortlist.idf(bags)
    vecs = {key: shortlist.weigh(words, idf) for key, words in bags.items()}
    return {
        rid: sorted(
            ((oid, shortlist.cosine(vecs[("r", rid)], vecs[("o", oid)])) for oid in ours),
            key=lambda pair: (-pair[1], pair[0]),
        )
        for rid in sorted(reference, key=lambda key: (reference[key].title, key))
    }


def statement_tokens(
    *, reference: Reference, ours: Ours
) -> tuple[dict[str, list[frozenset[str]]], dict[str, frozenset[str]]]:
    """What the statement class compares: each reference claim's marked statements, and each node of ours's first
    span, as token sets."""
    statements = {
        rid: [frozenset(tokens(text)) for text in texts] for rid, texts in reference.statements.items() if rid in reference.claims
    }
    nodes = {oid: frozenset(tokens(_span_text(claim.spans[0], ours.texts))) for oid, claim in ours.claims.items() if claim.spans}
    return statements, nodes


@dataclass(frozen=True)
class Disagreement:
    """A reference claim the number class matched to one node and the statement class names another for."""

    reference: str
    number_node: str
    #: The claim's best statement overlap with the number class's node; 0.0 where that node has no span read.
    number_overlap: float
    statement_node: str
    statement_overlap: float


def number_statement_disagreements(*, reference: Reference, ours: Ours, exact: Mapping[str, str]) -> list[Disagreement]:
    """Each claim of ``exact`` (the number class's matches) that the statement class, run as though the number class
    had taken nothing, matches to another node; by claim id."""
    statements, nodes = statement_tokens(reference=reference, ours=ours)
    unnumbered = match_statements(statements, nodes, threshold=STATEMENT_THRESHOLD)
    found = []
    for rid, number_node in sorted(exact.items()):
        if rid not in unnumbered.matches or unnumbered.matches[rid][0] == number_node:
            continue
        statement_node, statement_overlap = unnumbered.matches[rid]
        number_words = nodes.get(number_node, frozenset())
        found.append(
            Disagreement(
                reference=rid,
                number_node=number_node,
                number_overlap=max(overlap(words, number_words) for words in statements[rid]),
                statement_node=statement_node,
                statement_overlap=statement_overlap,
            )
        )
    return found


def match_claims(
    *, reference: Reference, ours: Ours, exact: Mapping[str, str], scored: dict[str, list[tuple[str, float]]]
) -> ClaimMatching:
    """The cascade after the number class: ``exact`` (its matches) first, then statement overlap over the reference
    claims and nodes it left, then cosine (``ClaimMatching.mapping``) for the rest."""
    taken = set(exact.values())
    statements, nodes = statement_tokens(reference=reference, ours=ours)
    by_statement = match_statements(
        {rid: words for rid, words in statements.items() if rid not in exact},
        {oid: words for oid, words in nodes.items() if oid not in taken},
        threshold=STATEMENT_THRESHOLD,
    )
    matched = dict(exact) | {rid: oid for rid, (oid, _) in by_statement.matches.items()}
    return ClaimMatching(
        exact=dict(exact),
        statement=by_statement,
        scored=scored,
        matched=matched,
        fixed={rid: "number" if rid in exact else "statement" for rid in matched},
    )


def recall(
    *,
    matching: ClaimMatching,
    at: float,
    walked_names: Mapping[str, str],
    reference: Reference,
    ours: Ours,
    top1: bool = False,
) -> list[tuple[str, ...]]:
    """One row per reference edge, its ends matched as ``matching.mapping(at, top1=top1)``; ``walked_names`` maps each
    block node to its walked word and number (``mark_test``)."""
    mapped = matching.mapping(at, top1=top1)
    rows = []
    for source, target in reference.edges:
        a, b = mapped[source], mapped[target]
        judged = verdict(a, b, depends=ours.depends, references=ours.references, demoted=ours.demoted)
        found = judged.recall
        miss = recall_class(found) in MISSED
        a_nodes = [ours.claims[x] for x in sorted(a)]
        b_nodes = [ours.claims[y] for y in sorted(b)]
        b_reference = reference.claims[target]
        marks = (
            mark_test(a_nodes=a_nodes, b_nodes=b_nodes, walked_names=walked_names, anchors=ours.anchors, texts=ours.texts)
            if miss and a
            else []
        )
        titles = title_test(a_nodes=a_nodes, b_nodes=b_nodes, b_reference=b_reference, texts=ours.texts) if miss and a else []
        source_miss = recall_class(found) in MISSES
        evidence = "" if not miss else UNDETERMINED if not a else PRESENT if marks or titles else NONE_FOUND
        rows.append(
            (
                source,
                reference.claims[source].title,
                target,
                b_reference.title,
                found,
                judged.witness,
                "" if judged.path_length is None else str(judged.path_length),
                ("marked" if marks else UNMARKED) if source_miss else "",
                "; ".join(marks) if source_miss else "",
                " ".join(sorted(a)),
                " ".join(sorted(b)),
                paper_scope(a, b, ours.claims),
                document_level(a, b, ours=ours.claims, depends=ours.depends, references=ours.references) if source_miss else "",
                evidence,
                "; ".join(marks + titles),
                pair_class(source, target, matching.fixed),
                matching.match_class(source),
                matching.end_scores(source, a),
                matching.match_class(target),
                matching.end_scores(target, b),
            )
        )
    return rows


def tally(rows: list[tuple[str, ...]]) -> tuple[Counter, Counter]:
    split = COLUMN["mark_split"]
    return Counter(recall_class(row[COLUMN["recall"]]) for row in rows), Counter(row[split] for row in rows if row[split])


def short_paths(rows: list[tuple[str, ...]], *, longest: int = 2) -> Counter:
    """Per path class, the rows whose shortest path runs over at most ``longest`` edges."""
    return Counter(
        row[COLUMN["recall"]]
        for row in rows
        if row[COLUMN["path_length"]] and int(row[COLUMN["path_length"]]) <= longest
    )


def distinct_witnesses(rows: list[tuple[str, ...]]) -> dict[str, tuple[int, int]]:
    """Per recovering class, its row count and how many distinct witnesses those rows carry: one built edge or path
    can recover several reference edges."""
    return {
        name: (len(found), len(set(found)))
        for name in RECOVERING
        if (found := [row[COLUMN["witness"]] for row in rows if row[COLUMN["recall"]] == name])
    }


@dataclass(frozen=True)
class WitnessEdges:
    """The built edges the recovering rows' witnesses name."""

    rows: int
    #: Every edge each row's witness names, a path's edges one by one, counted once per row naming it.
    mentions: int
    distinct: int


def witness_edges(rows: list[tuple[str, ...]]) -> WitnessEdges:
    """Over the rows of every recovering class, the built edges their witnesses name. Every witness is written in
    the stored direction, a reversed one included, so its edges are read as written."""
    recovering = [row for row in rows if row[COLUMN["recall"]] in RECOVERING]
    named = [
        step
        for row in recovering
        for chain in row[COLUMN["witness"]].split()
        for step in itertools.pairwise(chain.split("->"))
    ]
    return WitnessEdges(rows=len(recovering), mentions=len(named), distinct=len(set(named)))


def sweep(
    *, matching: ClaimMatching, walked_names: Mapping[str, str], reference: Reference, ours: Ours, top1: bool = False
) -> list[tuple[float, int, Counter, Counter]]:
    """At each ``SWEEP`` threshold: the reference claims matched, and ``tally`` of the edge rows, each cosine end its
    whole fan or, with ``top1``, its best node."""
    rows = []
    for at in SWEEP:
        matched = sum(1 for kept in matching.mapping(at, top1=top1).values() if kept)
        classes, split = tally(
            recall(matching=matching, at=at, walked_names=walked_names, reference=reference, ours=ours, top1=top1)
        )
        rows.append((at, matched, classes, split))
    return rows


CLAIM_MATCH_HEADER = ("ref_id", "ref_title", "class", "score", "overlap", "ours_id", "ours_kind", "ours_title", "ours_document")


def claim_match_rows(
    *, matching: ClaimMatching, mapped: Mapping[str, set[str]], reference: Reference, ours: Ours
) -> list[tuple[str, ...]]:
    """``claim-matches.tsv``'s rows (``CLAIM_MATCH_HEADER``): each reference claim's matches in ``mapped``, best cosine
    first, ties by id, or one ``UNMATCHED`` row."""
    rows = []
    for rid, pairs in matching.scored.items():
        title = reference.claims[rid].title
        score_of = dict(pairs)
        kept = sorted(mapped[rid], key=lambda oid: (-score_of[oid], oid))
        if not kept:
            rows.append((rid, title, "", "", "", "", "", "UNMATCHED", ""))
        by_statement = matching.statement.matches.get(rid)
        overlap = "" if by_statement is None else f"{by_statement[1]:.3f}"
        for oid in kept:
            node = ours.claims[oid]
            rows.append(
                (rid, title, matching.match_class(rid), f"{score_of[oid]:.3f}", overlap, oid, node.kind, node.title, node.documents[0])
            )
    return rows


# --- the comparison ----------------------------------------------------------------------


def matching_heading(threshold: float) -> list[str]:
    """The claim-matching section's heading, naming the cascade's classes in order, and the label class's absence."""
    return [
        "## Claim matching — " + ", then ".join(MATCH_CLASS_HEADINGS.values()) + f" at threshold {threshold}",
        "",
        "label: not implemented; no class pairs a reference claim with a node of ours by a shared \\label",
    ]


# --- summary.md: one builder per section, each ending in its blank line ------------------


def overview_section(
    *,
    reference_root: Path,
    ours_root: Path,
    reference: Reference,
    ours: Ours,
    failures: Sequence[str],
    dangling: Sequence[tuple[str, str]],
) -> list[str]:
    """The inputs, each side's counts, and the read failures."""
    kinds = Counter(c.kind for c in ours.claims.values())
    relations = dict(sorted((f"{r}/{k}", n) for (r, k), n in reference.relations.items()))
    return [
        "# reference vs ours",
        "",
        f"script: {Path(__file__).resolve()}",
        f"reference: {reference_root}",
        f"ours: {ours_root}",
        "",
        f"reference: {len(reference.claims)} claims; depends-on relations {relations}",
        f"reference claim-to-claim depends edges: {len(reference.edges)}",
        f"ours: {len(ours.claims)} claim nodes {dict(sorted(kinds.items()))}; depends {len(ours.depends)}; references {len(ours.references)}; "
        f"demoted {len(ours.demoted)}",
        "",
        "## Read failures (each skipped; the rest compared)",
        "",
        *([f"  {line}" for line in sorted(failures)] or ["  none"]),
        *[f"  reference edge {s} -> {t}: an end is not a reference claim; not compared" for s, t in dangling],
        "",
    ]


def matching_section(
    *, threshold: float, matching: ClaimMatching, mapped: Mapping[str, set[str]], reference: Reference
) -> list[str]:
    matched = [rid for rid, kept in mapped.items() if kept]
    return [
        *matching_heading(threshold),
        "",
        f"matched: {len(matched)} of {len(reference.claims)}; unmatched {len(reference.claims) - len(matched)}",
        f"matched by class: {dict(sorted(Counter(matching.match_class(rid) for rid in matched).items()))}",
        "",
    ]


def number_section(
    *,
    numbered: Mapping[str, NumberOutcome],
    exact: Mapping[str, str],
    walked: Mapping[str, Numbering],
    roots: Sequence[Path],
    sources: SourceMapping | None,
    page_blocks: Sequence[PageBlock],
    reference: Reference,
) -> list[str]:
    """The number class's volumes, the page blocks no walked environment accounts for, and each numbered claim;
    with ``sources``, first how its roots map to our volumes."""
    listing = [] if sources is None else source_lines(sources)
    if not walked:
        reason = "no --volume-root or --sources given" if sources is None else "no root under --sources maps to a volume"
        return [f"### number — not attempted: {reason}", "", *listing, *([""] if listing else [])]
    environments = {volume: numbering.environments for volume, numbering in walked.items()}
    block_count = sum(1 for block in page_blocks if block.volume in walked)
    unwalked = unwalked_blocks(page_blocks, environments)

    def number_line(rid: str) -> str:
        outcome = numbered[rid]
        key = f"{outcome.key.word} {outcome.key.number}" + (f" (suffix {outcome.key.suffix})" if outcome.key.suffix else "")
        result = f"-> {outcome.node_id}" if rid in exact else outcome.reason or f"node {outcome.node_id} not read"
        return f"  {rid}  {key}  {result}  ({reference.claims[rid].title})"

    return [
        f"### number — reference claims titled by a result word and number: {len(numbered)}, matched {len(exact)}",
        "",
        *listing,
        "volumes walked (our directory <- volume root; environments, of them left without a number):",
        *[
            f"  {volume} <- {root}; {len(numbering.environments)}, {sum(1 for e in numbering.environments if e.ordinal_in_group is not None and not e.number)}"
            for (volume, numbering), root in zip(walked.items(), roots)
        ],
        f"page blocks of a walked volume whose printed word and counter name no walked environment: {len(unwalked)} of {block_count}",
        *[f"  {block.volume}: {block.printed}" for block in unwalked],
        "unmatched by reason:",
        *counted(Counter(outcome.reason or "node not read" for rid, outcome in numbered.items() if rid not in exact)),
        *[number_line(rid) for rid in sorted(numbered, key=lambda rid: (reference.claims[rid].title, rid))],
        "",
    ]


def statement_section(*, matching: ClaimMatching, reference: Reference) -> list[str]:
    by_statement = matching.statement
    after_number = [rid for rid in matching.scored if rid not in matching.exact]

    def statement_line(rid: str) -> str:
        if rid in by_statement.best:
            best = by_statement.best[rid]
            result = f"best {best:.2f}, " + ("its node taken at a higher overlap" if best >= STATEMENT_THRESHOLD else "below the threshold")
        elif rid in reference.marked:
            result = "marker found, no statement located after it"
        else:
            result = "no marker on any citing page"
        return f"  {rid}  {result}  ({reference.claims[rid].title})"

    return [
        f"### statement — token-set overlap (Jaccard) of marked statements at threshold {STATEMENT_THRESHOLD}, one-to-one: "
        f"matched {len(by_statement.matches)} of the {len(after_number)} reference claims the number class left",
        "",
        f"reference claims with a marked statement located: {sum(1 for rid in reference.claims if rid in reference.statements)} "
        f"of {len(reference.claims)}; of those the number class left: {sum(1 for rid in after_number if rid in reference.statements)}",
        f"best overlap per reference claim with a statement, sorted: {', '.join(f'{s:.2f}' for s in sorted(by_statement.best.values()))}",
        "ties (a pair taken while another at the same overlap, sharing an end, was open):",
        *(
            [f"  {tie.reference} -> {tie.node} at {tie.overlap:.2f}; passed over {tie.rivals}" for tie in by_statement.ties]
            or ["  none"]
        ),
        "the reference claims the number and statement classes left, each falling through to cosine:",
        *[statement_line(rid) for rid in after_number if rid not in by_statement.matches],
        "",
    ]


def statementless_section(*, matching: ClaimMatching, mapped: Mapping[str, set[str]], reference: Reference) -> list[str]:
    """The reference claims with no marked statement located, which only the number or cosine class can match."""
    claims = [rid for rid in matching.scored if rid not in reference.statements]

    def claim_line(rid: str) -> str:
        ranked = matching.ranking(rid)
        best = f"best {ranked[0][1]:.3f}" if ranked else "no node scored"
        result = f"matched by {matching.match_class(rid)}" if mapped[rid] else "unmatched"
        return f"  {rid}  {best}  fan {len(mapped[rid])}  {result}  ({reference.claims[rid].title})"

    return [
        f"### the reference claims with no marked statement located: {len(claims)} (only number or cosine can match them)",
        "",
        "best cosine over the nodes left to cosine, the nodes matched at the run's threshold, and by which class:",
        *([claim_line(rid) for rid in claims] or ["  none"]),
        "",
    ]


def disagreement_section(disagreements: Sequence[Disagreement] | None, *, reference: Reference) -> list[str]:
    """``disagreements`` is None where the number class was not attempted."""
    if disagreements is None:
        return ["## Number and statement disagree", "", "not attempted: no volume root walked, so no number class", ""]
    return [
        "## Number and statement disagree",
        "",
        "each reference claim the number class matched that the statement class, run as though the number class had "
        "taken nothing, matches to another node (the claim's statement overlap with each):",
        *(
            [
                f"  {found.reference}  number -> {found.number_node} at {found.number_overlap:.2f}  "
                f"statement -> {found.statement_node} at {found.statement_overlap:.2f}  ({reference.claims[found.reference].title})"
                for found in disagreements
            ]
            or ["  none"]
        ),
        "",
    ]


def cosine_section(
    *, matching: ClaimMatching, mapped: Mapping[str, set[str]], reference: Reference, ours: Ours
) -> list[str]:
    fan = Counter(len(kept) for kept in mapped.values())
    ours_matched = {oid for kept in mapped.values() for oid in kept}
    volumes = Counter(
        (
            reference.claims[rid].documents[0].split("/")[0] if reference.claims[rid].documents else "?",
            ours.claims[oid].documents[0].split("/")[0],
        )
        for rid, kept in mapped.items()
        for oid in kept
    )
    rankings = {rid: matching.ranking(rid) for rid in matching.scored}
    best_scores = sorted(ranked[0][1] for ranked in rankings.values() if ranked)
    return [
        "### cosine, and the matches as a whole",
        "",
        f"matches per reference claim: {dict(sorted(fan.items()))}",
        f"distinct nodes of ours matched: {len(ours_matched)} ({dict(sorted(Counter(ours.claims[o].kind for o in ours_matched).items()))})",
        "best score per reference claim, sorted (a claim left to cosine is scored only against the nodes no exact class "
        f"took): {', '.join(f'{s:.2f}' for s in best_scores)}",
        "unmatched reference claims (best node left to cosine; candidates.tsv ranks every node):",
        *[
            f"  {rid}  {reference.claims[rid].title}  (best {rankings[rid][0][1]:.3f})"
            if rankings[rid]
            else f"  {rid}  {reference.claims[rid].title}"
            for rid, kept in mapped.items()
            if not kept
        ],
        "",
        "matches by reference volume x our paper (top-level directory of each host document):",
        *[f"  {volume:<12} {paper:<55} {count}" for (volume, paper), count in sorted(volumes.items())],
        "",
    ]


def pair_columns(rows: Iterable[tuple[str, ...]]) -> list[str]:
    """The pair classes some row carries, in class order."""
    carried = {row[COLUMN["pair_class"]] for row in rows}
    return [pair for pair in PAIR_CLASSES if pair in carried]


def by_pair_class(
    rows: list[tuple[str, ...]],
    *,
    columns: Sequence[str],
    heading: str,
    value: Callable[[tuple[str, ...]], str],
    values: Iterable[str],
) -> list[str]:
    """A table of each row's ``value`` against the pair class of the row's edge, one column per pair class in ``columns``."""
    cells = Counter((value(row), row[COLUMN["pair_class"]]) for row in rows)
    return [
        f"| {heading} | " + " | ".join(columns) + " |",
        "|" + "---|" * (len(columns) + 1),
        *[f"| {value} | " + " | ".join(str(cells[(value, pair)]) for pair in columns) + " |" for value in values],
        "| total | " + " | ".join(str(sum(1 for row in rows if row[COLUMN["pair_class"]] == pair)) for pair in columns) + " |",
    ]


def edge_recall_section(
    *,
    threshold: float,
    matching: ClaimMatching,
    mapped: Mapping[str, set[str]],
    edge_rows: list[tuple[str, ...]],
    top1_rows: list[tuple[str, ...]],
) -> list[str]:
    classes, _ = tally(edge_rows)
    top1_classes, _ = tally(top1_rows)
    fan_short, top1_short = short_paths(edge_rows), short_paths(top1_rows)
    recall_lines = []
    for name in RECALL_CLASSES:
        recall_lines.append(f"| {name} | {classes.get(name, 0)} | {top1_classes.get(name, 0)} |")
        if name.startswith("depends path"):
            recall_lines.append(f"| {name}, length ≤ 2 | {fan_short.get(name, 0)} | {top1_short.get(name, 0)} |")
    fan_witnesses, top1_witnesses = distinct_witnesses(edge_rows), distinct_witnesses(top1_rows)
    witness_lines = [
        f"| {name} | {fan_witnesses.get(name, (0, 0))[0]} | {fan_witnesses.get(name, (0, 0))[1]} | "
        f"{top1_witnesses.get(name, (0, 0))[0]} | {top1_witnesses.get(name, (0, 0))[1]} |"
        for name in RECOVERING
    ]
    return [
        "## Edge recall over reference depends edges",
        "",
        f"each end left to cosine matched two ways at {threshold}: fan, every node at or above it; top-1, its best node "
        "alone where that one is; a path's length is the edges on its shortest path",
        "",
        "| recall | fan | top-1 |",
        "|---|---|---|",
        *recall_lines,
        "",
        "distinct witnesses per recovering class (one built edge or path can recover several reference edges):",
        "",
        "| recall | fan rows | fan distinct witnesses | top-1 rows | top-1 distinct witnesses |",
        "|---|---|---|---|---|",
        *witness_lines,
        "",
        "distinct built edges across the recovering rows (each witness's edges, a path's one by one; one built edge "
        "can sit in several witnesses):",
        *[
            f"  {view}: {found.rows} rows naming {found.mentions} edges, {found.distinct} distinct"
            for view, found in (("fan", witness_edges(edge_rows)), ("top-1", witness_edges(top1_rows)))
        ],
        "",
        "by pair class, fan (how the edge's two ends were matched; an end no class matched counts as cosine):",
        "",
        *by_pair_class(
            edge_rows,
            columns=pair_columns(edge_rows),
            heading="recall",
            value=lambda row: recall_class(row[COLUMN["recall"]]),
            values=RECALL_CLASSES,
        ),
        "",
        "by weakest end (the lower best cosine of the ends cosine matched; an edge with no cosine end is exact both; "
        "edges.tsv gives each end's class and scores):",
        *counted(
            Counter(
                f"{recall_class(row[COLUMN['recall']])} / "
                + matching.weakest_end(row[COLUMN["ref_source"]], row[COLUMN["ref_target"]], mapped)
                for row in edge_rows
            )
        ),
        "",
    ]


def mark_split_section(*, edge_rows: list[tuple[str, ...]], anchors: Sequence[inventory.Anchor]) -> list[str]:
    _, split = tally(edge_rows)
    return [
        f"## Mechanical split of misses ({' + '.join(MISSES)})",
        "",
        *counted(split),
        "",
        f"anchors of ours resolving to a document: {sum(1 for x in anchors if x.target)}; of them crossing papers: "
        f"{sum(1 for x in anchors if x.target and x.target.split('/')[0] != x.document.split('/')[0])}",
        "misses by paper scope (whether some matched pair shares one of our papers):",
        *[
            f"  {scope}: {count}"
            for scope, count in sorted(
                Counter(f"{row[COLUMN['scope']]}, {row[COLUMN['mark_split']]}" for row in edge_rows if row[COLUMN["mark_split"]]).items()
            )
        ],
        "",
        "misses re-read at document level (A's matches to any node hosted in a document hosting one of B's matches):",
        *counted(Counter(row[COLUMN["document_level"]] for row in edge_rows if row[COLUMN["mark_split"]])),
        "",
    ]


def evidence_section(edge_rows: list[tuple[str, ...]]) -> list[str]:
    missed_rows = [row for row in edge_rows if row[COLUMN["evidence"]]]
    return [
        f"## Evidence split of every miss ({', '.join(MISSED)}): {len(missed_rows)}",
        "",
        *counted(Counter(row[COLUMN["evidence"]] for row in missed_rows)),
        "by recall class:",
        *counted(Counter(f"{recall_class(row[COLUMN['recall']])} / {row[COLUMN['evidence']]}" for row in missed_rows)),
        "by pair class:",
        "",
        *by_pair_class(
            missed_rows,
            columns=pair_columns(edge_rows),
            heading="evidence",
            value=lambda row: row[COLUMN["evidence"]],
            values=(PRESENT, NONE_FOUND, UNDETERMINED),
        ),
        "",
    ]


def sweep_section(views: Mapping[str, list[tuple[float, int, Counter, Counter]]]) -> list[str]:
    """``views`` holds ``sweep``'s rows for the ``fan`` and the ``top-1`` view."""
    sweep_lines = []
    for fan_row, top1_row in zip(views["fan"], views["top-1"]):
        for view, (at, at_matched, at_classes, at_split) in (("fan", fan_row), ("top-1", top1_row)):
            sweep_lines.append(
                f"| {at:.2f} | {view} | {at_matched} | "
                + " | ".join(str(at_classes.get(name, 0)) for name in RECALL_CLASSES)
                + f" | {at_split.get('marked', 0)} |"
            )
    return [
        "## Threshold sensitivity — the cosine class only; number and statement matches held",
        "",
        "| threshold | view | matched | " + " | ".join(RECALL_CLASSES) + " | marked |",
        "|" + "---|" * (len(RECALL_CLASSES) + 4),
        *sweep_lines,
        "",
    ]


def threshold_argument(raw: str) -> float:
    try:
        value = float(raw)
    except ValueError:
        raise argparse.ArgumentTypeError(f"{raw!r} is not a number") from None
    if not 0.0 < value <= 1.0:
        raise argparse.ArgumentTypeError(f"{raw!r} is not a cosine in (0, 1]")
    return value


def argument_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ours", required=True, type=kb_root_argument, help="the built kb-root")
    parser.add_argument("--reference", required=True, type=kb_root_argument, help="the reference kb-root")
    parser.add_argument("--out", required=True, type=scratch_directory_argument, help="output directory under .claude-temp/")
    parser.add_argument("--threshold", type=threshold_argument, default=THRESHOLD, help=f"match cosine (default {THRESHOLD})")
    roots = parser.add_mutually_exclusive_group()
    roots.add_argument(
        "--volume-root",
        action="append",
        default=[],
        type=volume_argument,
        help="each volume's root .tex, repeated in the order --ours' entry-point.md lists its volumes; "
        "without it or --sources the number class is not attempted",
    )
    roots.add_argument(
        "--sources",
        type=kb_root_argument,
        help="the LaTeX sources directory: its volume roots are found and mapped to --ours' volumes by title",
    )
    return parser


def main() -> int:
    args = argument_parser().parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")
    threshold: float = args.threshold

    for flag, kb_root in (("--reference", args.reference), ("--ours", args.ours)):
        refusal = format_refusal(kb_root)
        if refusal is not None:
            print(f"{flag} {str(kb_root)!r}: {refusal}", file=sys.stderr)
            return 1

    failures: list[str] = []
    try:
        reference = read_reference(args.reference, failures)
    except Exception as exc:  # noqa: BLE001 - a kb-root no reader can open is an unusable argument
        print(f"--reference {str(args.reference)!r}: unreadable: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    try:
        ours = read_ours(args.ours, failures)
    except Exception as exc:  # noqa: BLE001 - as above
        print(f"--ours {str(args.ours)!r}: unreadable: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    sources = None if args.sources is None else map_sources(args.sources, entry_volumes(args.ours))
    if sources is not None:
        mapped = sources.mapped()
    else:
        given: list[Path] = args.volume_root
        volume_dirs = entry_volumes(args.ours) if given else []
        if len(given) != len(volume_dirs):
            print(
                f"--volume-root given {len(given)} times; --ours' entry point lists {len(volume_dirs)} volumes: "
                "give one root per volume, in the entry point's order",
                file=sys.stderr,
            )
            return 2
        mapped = dict(zip(volume_dirs, given))
    roots = list(mapped.values())
    walked = {volume: walk(root) for volume, root in mapped.items()}
    dangling = [edge for edge in reference.edges if edge[0] not in reference.claims or edge[1] not in reference.claims]
    reference = replace(reference, edges=[edge for edge in reference.edges if edge not in dangling])

    scored = cosine_scores(reference.claims, ours.claims)
    environments = {volume: numbering.environments for volume, numbering in walked.items()}
    numbered = match_numbers(
        {rid: claim.title for rid, claim in reference.claims.items()}, blocks=ours.page_blocks, walked=environments
    )
    walked_names = {
        block.node_id: f"{env.name} {env.number}"
        for block in ours.page_blocks
        if block.node_id is not None and (env := walked_environment(block, environments)) is not None and env.number
    }
    exact = {rid: outcome.node_id for rid, outcome in numbered.items() if outcome.node_id in ours.claims}
    matching = match_claims(reference=reference, ours=ours, exact=exact, scored=scored)
    mapped = matching.mapping(threshold)
    edge_rows = recall(matching=matching, at=threshold, walked_names=walked_names, reference=reference, ours=ours)
    top1_rows = recall(matching=matching, at=threshold, walked_names=walked_names, reference=reference, ours=ours, top1=True)

    candidate_rows = []
    for rid, pairs in scored.items():
        for rank, (oid, score) in enumerate(pairs[:TOP_K], start=1):
            node = ours.claims[oid]
            candidate_rows.append(
                (rid, reference.claims[rid].title, str(rank), f"{score:.3f}", oid, node.kind, node.title, node.documents[0])
            )

    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    write_tsv(
        out,
        "claim-matches.tsv",
        CLAIM_MATCH_HEADER,
        claim_match_rows(matching=matching, mapped=mapped, reference=reference, ours=ours),
    )
    write_tsv(
        out,
        "candidates.tsv",
        ("ref_id", "ref_title", "rank", "score", "ours_id", "ours_kind", "ours_title", "ours_document"),
        candidate_rows,
    )
    write_tsv(
        out,
        "ours-nodes.tsv",
        ("ours_id", "kind", "document", "spans", "tokens", "title"),
        [
            (c.id, c.kind, c.documents[0], " ".join(f"{s.start}-{s.end}" for s in c.spans), str(len(tokens(c.title + "\n" + c.text))), c.title)
            for c in sorted(ours.claims.values(), key=lambda c: (c.documents[0], c.id))
        ],
    )
    write_tsv(out, "edges.tsv", EDGE_HEADER, edge_rows)

    views = {
        view: sweep(matching=matching, walked_names=walked_names, reference=reference, ours=ours, top1=top1)
        for view, top1 in (("fan", False), ("top-1", True))
    }
    summary = [
        *overview_section(
            reference_root=args.reference, ours_root=args.ours, reference=reference, ours=ours, failures=failures, dangling=dangling
        ),
        *matching_section(threshold=threshold, matching=matching, mapped=mapped, reference=reference),
        *number_section(
            numbered=numbered, exact=exact, walked=walked, roots=roots, sources=sources, page_blocks=ours.page_blocks, reference=reference
        ),
        *statement_section(matching=matching, reference=reference),
        *cosine_section(matching=matching, mapped=mapped, reference=reference, ours=ours),
        *statementless_section(matching=matching, mapped=mapped, reference=reference),
        *disagreement_section(
            number_statement_disagreements(reference=reference, ours=ours, exact=exact) if walked else None,
            reference=reference,
        ),
        *edge_recall_section(threshold=threshold, matching=matching, mapped=mapped, edge_rows=edge_rows, top1_rows=top1_rows),
        *mark_split_section(edge_rows=edge_rows, anchors=ours.anchors),
        *evidence_section(edge_rows),
        *sweep_section(views),
    ]
    (out / "summary.md").write_text("\n".join(summary), encoding="utf-8", newline="\n")
    print(f"wrote claim-matches.tsv, candidates.tsv, ours-nodes.tsv, edges.tsv, summary.md under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
