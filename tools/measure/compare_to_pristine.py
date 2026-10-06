"""Read-only comparison of a built KB against a reference KB of the same corpus.

Every output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import logging
import re
import sys
from collections import Counter, deque
from collections.abc import Callable, Iterable, Mapping
from dataclasses import dataclass, replace
from pathlib import Path
from typing import Any

from kb_tools import kb_index_lib, kb_load, kb_schema
from kb_tools.kb_claimgraph import graph, inventory, prose, shortlist, tree
from kb_tools.kb_claimgraph.tree import strip_markers, unquote
from latex_numbering import volume_argument, walk
from number_match import NAME_FORMS, NAME_NUMBER_RE, PageBlock, match_numbers, unwalked_blocks
from statement_match import match_statements

logger = logging.getLogger(__name__)

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"
THRESHOLD = 0.30
STATEMENT_THRESHOLD = 0.6
TOP_K = 5

MARKER_RE = re.compile(r"<!-- claim-quality: (clm-[a-z0-9]+) -->")
HEADING_RE = re.compile(r"^#{1,6} ")
TAG_RE = re.compile(r"<[^>]+>")
WORD_RE = re.compile(r"[a-z]{2,}")
LABEL_SPAN_RE = re.compile(r'<span id="[^"]*"')
ENTRY_VOLUME_RE = re.compile(r"^- \[.*\]\(([^/()]+)/index\.md\)$", re.MULTILINE)

MISSES = ("references only", "nothing")
UNMATCHED = "endpoint unmatched"
MISSED = (*MISSES, UNMATCHED)
UNMARKED = "unmarked — needs reading"
PRESENT, NONE_FOUND, UNDETERMINED = "present, unused", "none found", "undetermined"
SWEEP = (0.20, 0.25, 0.30, 0.35, 0.40, 0.50)
RECALL_CLASSES = (
    "depends",
    "depends reversed",
    "depends path",
    "depends path reversed",
    "shared node",
    "references only",
    "nothing",
    UNMATCHED,
)
EDGE_HEADER = (
    "ref_source",
    "ref_source_title",
    "ref_target",
    "ref_target_title",
    "recall",
    "mark_split",
    "marks",
    "ours_sources",
    "ours_targets",
    "scope",
    "document_level",
    "evidence",
    "evidence_basis",
    "pair_class",
)
MATCH_CLASSES = ("number", "label", "statement", "cosine")
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


def reference_section(text: str, claim_id: str) -> str:
    """The claim's statement: from its marker to the next heading or marker, else the leaf's opening section."""
    lines = kb_index_lib.strip_frontmatter(text).splitlines()
    marker = f"<!-- claim-quality: {claim_id} -->"
    start = next((number + 1 for number, line in enumerate(lines) if marker in line), None)
    if start is None:
        start = next((number + 1 for number, line in enumerate(lines) if line.startswith("# ")), 0)
    end = start
    while end < len(lines) and not HEADING_RE.match(lines[end]) and not MARKER_RE.search(lines[end]):
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
    anchors: list[inventory.Anchor]
    texts: dict[str, list[str]]
    page_blocks: list[PageBlock]


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
    if node.equation is not None:
        kind = "equation"
        fence = next((f for f in sites.fences if f.document == node.document and node.equation in f.labels), None)
        if fence is None:
            raise ValueError(f"no math fence carries the label {node.equation!r}")
        spans = (Span(node.document, fence.start, fence.end),)
    elif block is not None:
        kind = "block"
        spans = (Span(node.document, block.start, block.end),) + tuple(
            Span(proof.document, proof.start, proof.end)
            for proof in sites.proofs
            if any(subject.document == block.document and subject.start == block.start for subject in proof.subjects)
        )
        name = inventory._printed_name(block.display)
        printed = (plain_text(name).rstrip("."),) if name else ()
    else:
        kind = "prose"
        marker = f"<!-- claim-quality: {node.id} -->"
        line = next((n for n, text in enumerate(lines) if marker in text), None)
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
    return Claim(id=node.id, title=node.title, kind=kind, documents=(node.document,), text=text, spans=spans, printed=printed)


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

    state = kb_index_lib.discover_kb(kb_root, diagnostic_stream=None)
    depends = {
        (edge.source, edge.target)
        for entry in state.claim_entries
        for edge in entry.depends_on
        if edge.relation == "depends" and edge.target_kind == "claim"
    }
    references = {(edge.source, edge.target) for entry in state.claim_entries for edge in entry.references}
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
        anchors=list(sites.anchors),
        texts=texts,
        page_blocks=page_blocks,
    )


def entry_volumes(kb_root: Path) -> list[str]:
    """Our volume directories in the order the entry point lists them: the build's volume order."""
    text = (kb_root / kb_index_lib.ENTRY_POINT_FILENAME).read_text(encoding="utf-8")
    return ENTRY_VOLUME_RE.findall(text)


# --- edges ----------------------------------------------------------------------------


def reaches(edges: set[tuple[str, str]], sources: set[str], targets: set[str]) -> bool:
    following: dict[str, set[str]] = {}
    for source, target in edges:
        following.setdefault(source, set()).add(target)
    seen = set(sources)
    pending = deque(sorted(sources))
    while pending:
        for node in sorted(following.get(pending.popleft(), ())):
            if node in targets:
                return True
            if node not in seen:
                seen.add(node)
                pending.append(node)
    return False


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
    b_reference: Claim,
    anchors: list[inventory.Anchor],
    texts: dict[str, list[str]],
) -> list[str]:
    """Every mark of B inside the text our pipeline reads for A's matched claims."""
    b_hosts = {doc for node in b_nodes for doc in node.documents}
    names = {name: "ours" for node in b_nodes for name in node.printed}
    names.setdefault(b_reference.title, "reference title")
    patterns = [(name, origin, found) for name, origin in sorted(names.items()) if (found := name_pattern(name)) is not None]
    marks = []
    for a in a_nodes:
        for span in a.spans:
            for anchor in anchors:
                if anchor.document == span.document and span.start <= anchor.line < span.end and anchor.target in b_hosts:
                    marks.append(f"anchor {a.id}:{anchor.reference_type} -> {anchor.href}")
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


def verdict(a: set[str], b: set[str], *, depends: set[tuple[str, str]], references: set[tuple[str, str]]) -> str:
    if not a or not b:
        return f"{UNMATCHED} (" + " and ".join(end for end, s in (("source", a), ("target", b)) if not s) + ")"
    if a & b:
        return "shared node"
    if any((x, y) in depends for x in a for y in b):
        return "depends"
    if any((y, x) in depends for x in a for y in b):
        return "depends reversed"
    if reaches(depends, a, b):
        return "depends path"
    if reaches(depends, b, a):
        return "depends path reversed"
    if any((x, y) in references or (y, x) in references for x in a for y in b):
        return "references only"
    return "nothing"


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


def recall(*, mapped: dict[str, set[str]], fixed: Mapping[str, str], reference: Reference, ours: Ours) -> list[tuple[str, ...]]:
    """One row per reference edge; ``fixed`` holds the class of each reference claim matched by a class before cosine."""
    rows = []
    for source, target in reference.edges:
        a, b = mapped[source], mapped[target]
        found = verdict(a, b, depends=ours.depends, references=ours.references)
        miss = recall_class(found) in MISSED
        a_nodes = [ours.claims[x] for x in sorted(a)]
        b_nodes = [ours.claims[y] for y in sorted(b)]
        b_reference = reference.claims[target]
        marks = mark_test(a_nodes=a_nodes, b_nodes=b_nodes, b_reference=b_reference, anchors=ours.anchors, texts=ours.texts) if miss and a else []
        titles = title_test(a_nodes=a_nodes, b_nodes=b_nodes, b_reference=b_reference, texts=ours.texts) if miss and a else []
        source_miss = found in MISSES
        evidence = "" if not miss else UNDETERMINED if not a else PRESENT if marks or titles else NONE_FOUND
        rows.append(
            (
                source,
                reference.claims[source].title,
                target,
                b_reference.title,
                found,
                ("marked" if marks else UNMARKED) if source_miss else "",
                "; ".join(marks) if source_miss else "",
                " ".join(sorted(a)),
                " ".join(sorted(b)),
                paper_scope(a, b, ours.claims),
                document_level(a, b, ours=ours.claims, depends=ours.depends, references=ours.references) if source_miss else "",
                evidence,
                "; ".join(marks + titles),
                pair_class(source, target, fixed),
            )
        )
    return rows


def tally(rows: list[tuple[str, ...]]) -> tuple[Counter, Counter]:
    split = COLUMN["mark_split"]
    return Counter(recall_class(row[COLUMN["recall"]]) for row in rows), Counter(row[split] for row in rows if row[split])


# --- the comparison ----------------------------------------------------------------------


def threshold_argument(raw: str) -> float:
    try:
        value = float(raw)
    except ValueError:
        raise argparse.ArgumentTypeError(f"{raw!r} is not a number") from None
    if not 0.0 < value <= 1.0:
        raise argparse.ArgumentTypeError(f"{raw!r} is not a cosine in (0, 1]")
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ours", required=True, type=kb_root_argument, help="the built kb-root")
    parser.add_argument("--reference", required=True, type=kb_root_argument, help="the reference kb-root")
    parser.add_argument("--out", required=True, type=scratch_directory_argument, help="output directory under .claude-temp/")
    parser.add_argument("--threshold", type=threshold_argument, default=THRESHOLD, help=f"match cosine (default {THRESHOLD})")
    parser.add_argument(
        "--volume-root",
        action="append",
        default=[],
        type=volume_argument,
        help="each volume's root .tex, repeated in the order --ours' entry-point.md lists its volumes; "
        "without it the number class is not attempted",
    )
    args = parser.parse_args()
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
    roots: list[Path] = args.volume_root
    volume_dirs = entry_volumes(args.ours) if roots else []
    if len(roots) != len(volume_dirs):
        print(
            f"--volume-root given {len(roots)} times; --ours' entry point lists {len(volume_dirs)} volumes: "
            "give one root per volume, in the entry point's order",
            file=sys.stderr,
        )
        return 2
    walked = {volume: walk(root) for volume, root in zip(volume_dirs, roots)}
    dangling = [edge for edge in reference.edges if edge[0] not in reference.claims or edge[1] not in reference.claims]
    reference = replace(reference, edges=[edge for edge in reference.edges if edge not in dangling])

    bags = {("r", key): tokens(c.title + "\n" + c.text) for key, c in reference.claims.items()}
    bags |= {("o", key): tokens(c.title + "\n" + c.text) for key, c in ours.claims.items()}
    idf = shortlist.idf(bags)
    vecs = {key: shortlist.weigh(words, idf) for key, words in bags.items()}
    scored = {
        rid: sorted(
            ((oid, shortlist.cosine(vecs[("r", rid)], vecs[("o", oid)])) for oid in ours.claims),
            key=lambda pair: (-pair[1], pair[0]),
        )
        for rid in sorted(reference.claims, key=lambda key: (reference.claims[key].title, key))
    }

    numbered = match_numbers(
        {rid: claim.title for rid, claim in reference.claims.items()},
        blocks=ours.page_blocks,
        walked={volume: numbering.environments for volume, numbering in walked.items()},
    )
    exact = {rid: outcome.node_id for rid, outcome in numbered.items() if outcome.node_id in ours.claims}
    taken = set(exact.values())
    by_statement = match_statements(
        {
            rid: [frozenset(tokens(text)) for text in texts]
            for rid, texts in reference.statements.items()
            if rid in reference.claims and rid not in exact
        },
        {
            oid: frozenset(tokens(_span_text(claim.spans[0], ours.texts)))
            for oid, claim in ours.claims.items()
            if claim.spans and oid not in taken
        },
        threshold=STATEMENT_THRESHOLD,
    )
    matched_at = exact | {rid: oid for rid, (oid, _) in by_statement.matches.items()}
    fixed = {rid: "number" if rid in exact else "statement" for rid in matched_at}
    match_class = {rid: fixed.get(rid, "cosine") for rid in scored}

    def mapping(at: float) -> dict[str, set[str]]:
        """Number and statement matches as they stand; every other reference claim matched by cosine at ``at``."""
        return {
            rid: {matched_at[rid]} if rid in matched_at else {oid for oid, score in pairs if score >= at}
            for rid, pairs in scored.items()
        }

    mapped = mapping(threshold)
    edge_rows = recall(mapped=mapped, fixed=fixed, reference=reference, ours=ours)
    classes, split = tally(edge_rows)

    match_rows, candidate_rows = [], []
    for rid, pairs in scored.items():
        title = reference.claims[rid].title
        score_of = dict(pairs)
        kept = [(oid, score_of[oid]) for oid in sorted(mapped[rid], key=lambda oid: (-score_of[oid], oid))]
        if not kept:
            match_rows.append((rid, title, "", "", "", "", "UNMATCHED", ""))
        for oid, score in kept:
            c = ours.claims[oid]
            match_rows.append((rid, title, match_class[rid], f"{score:.3f}", oid, c.kind, c.title, c.documents[0]))
        for rank, (oid, score) in enumerate(pairs[:TOP_K], start=1):
            c = ours.claims[oid]
            candidate_rows.append((rid, title, str(rank), f"{score:.3f}", oid, c.kind, c.title, c.documents[0]))

    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    write_tsv(
        out,
        "claim-matches.tsv",
        ("ref_id", "ref_title", "class", "score", "ours_id", "ours_kind", "ours_title", "ours_document"),
        match_rows,
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
            (c.id, c.kind, c.documents[0], " ".join(f"{s.start}-{s.end}" for s in c.spans), str(len(bags[("o", c.id)])), c.title)
            for c in sorted(ours.claims.values(), key=lambda c: (c.documents[0], c.id))
        ],
    )
    write_tsv(out, "edges.tsv", EDGE_HEADER, edge_rows)

    matched = [rid for rid, kept in mapped.items() if kept]
    kinds = Counter(c.kind for c in ours.claims.values())
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
    top1 = sorted(pairs[0][1] for pairs in scored.values() if pairs)
    missed_rows = [row for row in edge_rows if row[COLUMN["evidence"]]]
    pair_columns = [pair for pair in PAIR_CLASSES if any(row[COLUMN["pair_class"]] == pair for row in edge_rows)]

    def by_pair_class(
        rows: list[tuple[str, ...]], *, heading: str, value: Callable[[tuple[str, ...]], str], values: Iterable[str]
    ) -> list[str]:
        """A table of each row's ``value`` against the pair class of the row's edge, over the pair classes edges carry."""
        cells = Counter((value(row), row[COLUMN["pair_class"]]) for row in rows)
        return [
            f"| {heading} | " + " | ".join(pair_columns) + " |",
            "|" + "---|" * (len(pair_columns) + 1),
            *[f"| {value} | " + " | ".join(str(cells[(value, pair)]) for pair in pair_columns) + " |" for value in values],
            "| total | " + " | ".join(str(sum(1 for row in rows if row[COLUMN["pair_class"]] == pair)) for pair in pair_columns) + " |",
        ]

    block_count = sum(1 for block in ours.page_blocks if block.volume in walked)
    unwalked = unwalked_blocks(ours.page_blocks, {volume: numbering.environments for volume, numbering in walked.items()})

    def number_line(rid: str) -> str:
        outcome = numbered[rid]
        key = f"{outcome.key.word} {outcome.key.number}" + (f" (suffix {outcome.key.suffix})" if outcome.key.suffix else "")
        result = f"-> {outcome.node_id}" if rid in exact else outcome.reason or f"node {outcome.node_id} not read"
        return f"  {rid}  {key}  {result}  ({reference.claims[rid].title})"

    if walked:
        number_section = [
            f"### number — reference claims titled by a result word and number: {len(numbered)}, matched {len(exact)}",
            "",
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
        ]
    else:
        number_section = ["### number — not attempted: no --volume-root given"]
    label_spans = (
        sum(len(LABEL_SPAN_RE.findall(c.text)) for c in reference.claims.values()),
        sum(len(LABEL_SPAN_RE.findall(c.text)) for c in ours.claims.values()),
    )
    after_number = [rid for rid in scored if rid not in exact]

    def statement_line(rid: str) -> str:
        if rid in by_statement.matches:
            oid, score = by_statement.matches[rid]
            result = f"-> {oid} at {score:.2f}"
        elif rid in by_statement.best:
            best = by_statement.best[rid]
            result = f"best {best:.2f}, " + ("its node taken at a higher overlap" if best >= STATEMENT_THRESHOLD else "below the threshold")
        elif rid in reference.marked:
            result = "marker found, no statement located after it"
        else:
            result = "no marker on any citing page"
        return f"  {rid}  {result}  ({reference.claims[rid].title})"

    statement_section = [
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
        "the reference claims the number class left (no statement: falls through to cosine):",
        *[statement_line(rid) for rid in after_number],
    ]

    sweep = []
    for at in SWEEP:
        at_mapped = mapping(at)
        at_classes, at_split = tally(recall(mapped=at_mapped, fixed=fixed, reference=reference, ours=ours))
        sweep.append(
            f"| {at:.2f} | {sum(1 for kept in at_mapped.values() if kept)} | "
            + " | ".join(str(at_classes.get(name, 0)) for name in RECALL_CLASSES)
            + f" | {at_split.get('marked', 0)} |"
        )

    relations = dict(sorted((f"{r}/{k}", n) for (r, k), n in reference.relations.items()))
    summary = [
        "# reference vs ours",
        "",
        f"script: {Path(__file__).resolve()}",
        f"reference: {args.reference}",
        f"ours: {args.ours}",
        "",
        f"reference: {len(reference.claims)} claims; depends-on relations {relations}",
        f"reference claim-to-claim depends edges: {len(reference.edges)}",
        f"ours: {len(ours.claims)} claim nodes {dict(sorted(kinds.items()))}; depends {len(ours.depends)}; references {len(ours.references)}",
        "",
        "## Read failures (each skipped; the rest compared)",
        "",
        *([f"  {line}" for line in sorted(failures)] or ["  none"]),
        *[f"  reference edge {s} -> {t}: an end is not a reference claim; not compared" for s, t in dangling],
        "",
        "## Claim matching — number, then label, then statement overlap, "
        f"then TF-IDF cosine over title+statement at threshold {threshold}",
        "",
        f"matched: {len(matched)} of {len(reference.claims)}; unmatched {len(reference.claims) - len(matched)}",
        f"matched by class: {dict(sorted(Counter(match_class[rid] for rid in matched).items()))}",
        "",
        *number_section,
        "",
        f"### label — not attempted: reference claim statements carry {label_spans[0]} label spans (<span id=…>), "
        f"ours {label_spans[1]}; the class needs both sides",
        "",
        *statement_section,
        "",
        "### cosine, and the matches as a whole",
        "",
        f"matches per reference claim: {dict(sorted(fan.items()))}",
        f"distinct nodes of ours matched: {len(ours_matched)} ({dict(sorted(Counter(ours.claims[o].kind for o in ours_matched).items()))})",
        f"best score per reference claim, sorted: {', '.join(f'{s:.2f}' for s in top1)}",
        "unmatched reference claims (best candidate in candidates.tsv):",
        *[
            f"  {rid}  {reference.claims[rid].title}  (best {scored[rid][0][1]:.3f})" if scored[rid] else f"  {rid}  {reference.claims[rid].title}"
            for rid, kept in mapped.items()
            if not kept
        ],
        "",
        "matches by reference volume x our paper (top-level directory of each host document):",
        *[f"  {volume:<12} {paper:<55} {count}" for (volume, paper), count in sorted(volumes.items())],
        "",
        "## Edge recall over reference depends edges",
        "",
        *counted(classes),
        "",
        "by pair class (how the edge's two ends were matched; an end no class matched counts as cosine):",
        "",
        *by_pair_class(edge_rows, heading="recall", value=lambda row: recall_class(row[COLUMN["recall"]]), values=RECALL_CLASSES),
        "",
        "## Mechanical split of misses (references only + nothing)",
        "",
        *counted(split),
        "",
        f"anchors of ours resolving to a document: {sum(1 for x in ours.anchors if x.target)}; of them crossing papers: "
        f"{sum(1 for x in ours.anchors if x.target and x.target.split('/')[0] != x.document.split('/')[0])}",
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
        f"## Evidence split of every miss ({', '.join(MISSED)}): {len(missed_rows)}",
        "",
        *counted(Counter(row[COLUMN["evidence"]] for row in missed_rows)),
        "by recall class:",
        *counted(Counter(f"{recall_class(row[COLUMN['recall']])} / {row[COLUMN['evidence']]}" for row in missed_rows)),
        "by pair class:",
        "",
        *by_pair_class(
            missed_rows, heading="evidence", value=lambda row: row[COLUMN["evidence"]], values=(PRESENT, NONE_FOUND, UNDETERMINED)
        ),
        "",
        "## Threshold sensitivity — the cosine class only; number and statement matches held",
        "",
        "| threshold | matched | " + " | ".join(RECALL_CLASSES) + " | marked |",
        "|" + "---|" * (len(RECALL_CLASSES) + 3),
        *sweep,
        "",
    ]
    (out / "summary.md").write_text("\n".join(summary), encoding="utf-8", newline="\n")
    print(f"wrote claim-matches.tsv, candidates.tsv, ours-nodes.tsv, edges.tsv, summary.md under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
