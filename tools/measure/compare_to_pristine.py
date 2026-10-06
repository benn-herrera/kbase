"""Read-only comparison of a built KB against a reference KB of the same corpus.

Every output file and column is described in tools/measure/README.md. kb_tools is imported from PYTHONPATH.
"""

import argparse
import logging
import re
import sys
from collections import Counter, deque
from collections.abc import Callable, Iterable
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from kb_tools import kb_index_lib
from kb_tools.kb_claimgraph import graph, inventory, prose, shortlist, tree
from kb_tools.kb_claimgraph.tree import strip_markers, unquote

logger = logging.getLogger(__name__)

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"
THRESHOLD = 0.30
TOP_K = 5

MARKER_RE = re.compile(r"<!-- claim-quality: (clm-[a-z0-9]+) -->")
HEADING_RE = re.compile(r"^#{1,6} ")
TAG_RE = re.compile(r"<[^>]+>")
WORD_RE = re.compile(r"[a-z]{2,}")

NAME_FORMS = {
    "proposition": r"(?:Proposition|Prop)",
    "theorem": r"(?:Theorem|Thm)",
    "corollary": r"(?:Corollary|Cor)",
    "lemma": r"Lemma",
    "conjecture": r"(?:Conjecture|Conj)",
    "definition": r"(?:Definition|Def)",
    "remark": r"Remark",
    "assumption": r"Assumption",
}
NAME_NUMBER_RE = re.compile(r"^\W*(" + "|".join(NAME_FORMS) + r")\s+([0-9A-Z][0-9.]*[a-z]?)\b", re.IGNORECASE)

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
)
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


@dataclass(frozen=True)
class Reference:
    claims: dict[str, Claim]
    edges: list[tuple[str, str]]
    relations: Counter


def read_reference(kb_root: Path, failures: list[str]) -> Reference:
    state = kb_index_lib.discover_kb(kb_root, diagnostic_stream=None)
    if not state.claim_entries:
        raise ValueError("no claim entry in its registers: nothing to compare against")
    documents = read_tree(kb_root)
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
    return Reference(claims=claims, edges=edges, relations=relations)


# --- our KB ----------------------------------------------------------------------------


@dataclass(frozen=True)
class Ours:
    claims: dict[str, Claim]
    depends: set[tuple[str, str]]
    references: set[tuple[str, str]]
    anchors: list[inventory.Anchor]
    texts: dict[str, list[str]]


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
    return Ours(claims=claims, depends=depends, references=references, anchors=list(sites.anchors), texts=texts)


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


def recall(*, mapped: dict[str, set[str]], reference: Reference, ours: Ours) -> list[tuple[str, ...]]:
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
    args = parser.parse_args()
    logging.basicConfig(level=logging.WARNING, format="%(levelname)s %(message)s")
    threshold: float = args.threshold

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
    dangling = [edge for edge in reference.edges if edge[0] not in reference.claims or edge[1] not in reference.claims]
    reference = Reference(
        claims=reference.claims,
        edges=[edge for edge in reference.edges if edge not in dangling],
        relations=reference.relations,
    )

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

    def mapping(at: float) -> dict[str, set[str]]:
        return {rid: {oid for oid, score in pairs if score >= at} for rid, pairs in scored.items()}

    mapped = mapping(threshold)
    edge_rows = recall(mapped=mapped, reference=reference, ours=ours)
    classes, split = tally(edge_rows)

    match_rows, candidate_rows = [], []
    for rid, pairs in scored.items():
        title = reference.claims[rid].title
        kept = [(oid, score) for oid, score in pairs if score >= threshold]
        if not kept:
            match_rows.append((rid, title, "", "", "", "UNMATCHED", ""))
        for oid, score in kept:
            c = ours.claims[oid]
            match_rows.append((rid, title, f"{score:.3f}", oid, c.kind, c.title, c.documents[0]))
        for rank, (oid, score) in enumerate(pairs[:TOP_K], start=1):
            c = ours.claims[oid]
            candidate_rows.append((rid, title, str(rank), f"{score:.3f}", oid, c.kind, c.title, c.documents[0]))

    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    write_tsv(out, "claim-matches.tsv", ("ref_id", "ref_title", "score", "ours_id", "ours_kind", "ours_title", "ours_document"), match_rows)
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

    sweep = []
    for at in SWEEP:
        at_mapped = mapping(at)
        at_classes, at_split = tally(recall(mapped=at_mapped, reference=reference, ours=ours))
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
        f"## Claim matching — TF-IDF cosine over title+statement, threshold {threshold}",
        "",
        f"matched: {len(matched)} of {len(reference.claims)}; unmatched {len(reference.claims) - len(matched)}",
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
        "",
        "## Threshold sensitivity",
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
