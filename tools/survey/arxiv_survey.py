"""Measure the mechanical structure arXiv papers carry in their LaTeX source, before building them.

What it measures, why, and every output field are described in tools/survey/README.md. The claim-site
vocabulary is kb_tools' own, imported from PYTHONPATH.
"""

import argparse
import json
import re
import sys
import tarfile
import time
import urllib.error
import urllib.request
from collections import Counter
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path

from kb_tools.kb_claimgraph.inventory import CLAIM_BEARING, CLASSIFIED

API = "http://export.arxiv.org/api/query"
EPRINT = "https://arxiv.org/e-print/"

REPO_ROOT = Path(__file__).resolve().parents[2]
JUSTFILE = REPO_ROOT / "justfile"
STAGE = REPO_ROOT / "test_data" / "transient" / "arxiv"
SCRATCH = REPO_ROOT / ".claude-temp"
TARBALLS = SCRATCH / "survey"

#: arXiv's published courtesy spacing between requests.
COURTESY_DELAY = 3.0

_ID = re.compile(r"<id>http://arxiv\.org/abs/([^<]+)</id>")

_NEWTHEOREM = re.compile(r"\\newtheorem\*?\{([^}]*)\}(?:\[[^\]]*\])?\{([^}]*)\}")
_LABEL = re.compile(r"\\label\{([^}]*)\}")
_REF = re.compile(r"\\(ref|cref|Cref|autoref|eqref)\{([^}]*)\}")
_CITE = re.compile(r"\\(cite[a-zA-Z]*)\{([^}]*)\}")
_DOCUMENTCLASS = re.compile(r"\\documentclass(?:\[[^\]]*\])?\{([^}]*)\}")
_PROOF = re.compile(r"\\begin\{proof\}")

#: `\begin{proof}[Proof of Theorem 3]` — the optional argument belongs to the opening delimiter, not the
#: body. Bounded to one line so a stray `[` a paragraph later cannot be swallowed.
_OPTIONAL_ARGUMENT = re.compile(r"[ \t]*\[[^\]\n]*\]")

#: `\cref{a,b}` names two targets in one command. Commands are counted, not targets; this splits only to
#: ask whether a command reaches a claim.
_TARGET_SEPARATOR = re.compile(r"\s*,\s*")

#: A `.bbl` from bibtex is a `thebibliography` environment, spliceable into the source; one from biblatex
#: is that package's internal format, which nothing else renders.
_BBL_BIBTEX = re.compile(r"\\begin\{thebibliography\}")
_BBL_BIBLATEX = re.compile(r"\\entry\{")

#: A variable whose value is a list of versioned arXiv ids is a category; the justfile's other `ARXIV_*`
#: variables (delay, directories) are not, and need no naming here.
_ARXIV_ID = re.compile(r"^(?:\d{4}\.\d{4,5}|[a-z-]+(?:\.[A-Z]{2})?/\d{7})v\d+$")
_JUST_STRING = re.compile(r'^ARXIV_([A-Z0-9_]+)\s*:=\s*"([^"]*)"', re.MULTILINE)


@dataclass(frozen=True)
class Vocabulary:
    """The claim-bearing display names, and every name anybody has classified."""

    claim_bearing: frozenset[str]
    classified: frozenset[str]


VOCABULARY = Vocabulary(claim_bearing=CLAIM_BEARING, classified=CLASSIFIED)


def _category_name(variable: str) -> str:
    """``CS_GR`` → ``cs.GR``. The archive is lowercase; the subject class is not, except under ``physics``.

    The archive is the first underscore-separated token, which a hyphenated archive (``q-bio``) breaks:
    adding one means teaching this function about it.
    """
    archive, _, subject = variable.partition("_")
    archive = archive.lower()
    return f"{archive}.{subject.lower() if archive == 'physics' else subject.upper()}"


def load_grouping(justfile: Path = JUSTFILE) -> dict[str, list[str]]:
    """Category → arXiv ids, off the justfile's own ``ARXIV_<CATEGORY> := "..."`` variables."""
    grouping: dict[str, list[str]] = {}
    for variable, value in _JUST_STRING.findall(justfile.read_text(encoding="utf-8")):
        ids = value.split()
        if ids and all(_ARXIV_ID.match(one) for one in ids):
            grouping[_category_name(variable)] = ids
    if not grouping:
        raise LookupError(f"{justfile}: no ARXIV_* variable holds a list of arXiv ids")
    return grouping


def fetch_ids(category: str, count: int) -> list[str]:
    """Recent arXiv ids in one category, newest first, version suffix kept: e-print serves that version."""
    query = (
        f"{API}?search_query=cat:{category}&start=0&max_results={count}"
        "&sortBy=submittedDate&sortOrder=descending"
    )
    with urllib.request.urlopen(query, timeout=60) as response:
        feed = response.read().decode("utf-8", errors="replace")
    return _ID.findall(feed)


def fetch_source(arxiv_id: str, into: Path) -> Path | None:
    """The e-print tarball for one id, or None where arXiv serves no source (a PDF-only submission)."""
    target = into / f"{arxiv_id.replace('/', '_')}.tar.gz"
    if target.exists():
        return target
    try:
        with urllib.request.urlopen(EPRINT + arxiv_id, timeout=120) as response:
            payload = response.read()
    except urllib.error.HTTPError:
        return None
    target.write_bytes(payload)
    return target


def _decode(raw: bytes) -> str:
    """The member's text; source encodings are undeclared, so Latin-1, which cannot fail, is the fallback."""
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        return raw.decode("latin-1")


def _sources(tarball: Path) -> dict[str, str]:
    """Every file member of the tarball under 8 MB, by name."""
    found: dict[str, str] = {}
    try:
        with tarfile.open(tarball) as archive:
            for member in archive.getmembers():
                if not member.isfile() or member.size > 8_000_000:
                    continue
                handle = archive.extractfile(member)
                if handle is None:
                    continue
                found[member.name] = _decode(handle.read())
    except (tarfile.TarError, EOFError):
        return {}
    return found


def staged_sources(paper: Path) -> dict[str, str]:
    """The same map as :func:`_sources`, off an unpacked paper directory, keyed on the path relative to it."""
    found: dict[str, str] = {}
    for path in sorted(paper.rglob("*")):
        if not path.is_file() or path.is_symlink() or path.stat().st_size > 8_000_000:
            continue
        found[path.relative_to(paper).as_posix()] = _decode(path.read_bytes())
    return found


def _prefix(label: str) -> str:
    """A label's taxonomy prefix, or `<none>` where the author uses none."""
    head, sep, _ = label.partition(":")
    return head if sep else "<none>"


def _bodies(text: str, environment: str) -> list[tuple[int, int]]:
    """``(start, end)`` of each ``\\begin{env}`` … ``\\end{env}`` body, depth-counted so a nested block of
    the same name yields one span rather than a truncated one."""
    marker = re.compile(rf"\\(begin|end)\{{{re.escape(environment)}\}}")
    spans: list[tuple[int, int]] = []
    depth = 0
    start = 0
    for match in marker.finditer(text):
        if match.group(1) == "begin":
            if depth == 0:
                optional = _OPTIONAL_ARGUMENT.match(text, match.end())
                start = optional.end() if optional else match.end()
            depth += 1
        elif depth:
            depth -= 1
            if depth == 0:
                spans.append((start, match.start()))
    return spans


def _within(spans: Sequence[tuple[int, int]], position: int) -> bool:
    return any(start <= position < end for start, end in spans)


def _targets(raw: str) -> list[str]:
    return [target for target in _TARGET_SEPARATOR.split(raw.strip()) if target]


def _fraction(part: int, whole: int) -> float | None:
    """None rather than zero where there is nothing to take a fraction of."""
    return round(part / whole, 4) if whole else None


def measure(arxiv_id: str, members: Mapping[str, str], *, vocabulary: Vocabulary) -> dict:
    """Everything the source declares, without rendering any of it."""
    tex = {name: text for name, text in members.items() if name.endswith(".tex") or "." not in name}
    body = "\n".join(tex.values())

    environments = {name: display for name, display in _NEWTHEOREM.findall(body)}
    labels = _LABEL.findall(body)
    refs = _REF.findall(body)
    cites = _CITE.findall(body)
    ref_targets = Counter(_prefix(target) for _, target in refs)

    # Classification runs on the display name `\newtheorem` declares, case-folded: the handle is arbitrary.
    # `proof` is amsthm's, declared by nobody, and matched literally.
    claim_handles = [
        handle for handle, display in environments.items() if display.casefold() in vocabulary.claim_bearing
    ]
    claim_spans = [span for handle in claim_handles for span in _bodies(body, handle)]
    proof_spans = _bodies(body, "proof")

    claim_labels = {
        match.group(1) for match in _LABEL.finditer(body) if _within(claim_spans, match.start())
    }
    ref_sites = list(_REF.finditer(body))
    in_proof = sum(1 for match in ref_sites if _within(proof_spans, match.start()))
    to_claim = sum(1 for match in ref_sites if claim_labels.intersection(_targets(match.group(2))))

    bbl = [text for name, text in members.items() if name.endswith(".bbl")]
    bbl_flavour = None
    if bbl:
        joined = "\n".join(bbl)
        if _BBL_BIBTEX.search(joined):
            bbl_flavour = "bibtex"
        elif _BBL_BIBLATEX.search(joined):
            bbl_flavour = "biblatex"
        else:
            bbl_flavour = "unknown"

    return {
        "id": arxiv_id,
        "tex_files": len(tex),
        "documentclass": sorted(set(_DOCUMENTCLASS.findall(body))),
        "newtheorem": environments,
        "labels": len(labels),
        "label_prefixes": dict(Counter(_prefix(label) for label in labels).most_common()),
        "refs": len(refs),
        "ref_commands": dict(Counter(command for command, _ in refs).most_common()),
        "ref_target_prefixes": dict(ref_targets.most_common()),
        "cites": len(cites),
        "proofs": len(_PROOF.findall(body)),
        "claim_bodies": len(claim_spans),
        "proof_bodies": len(proof_spans),
        "labels_in_claim_body": len(claim_labels),
        "refs_in_proof": in_proof,
        "refs_in_proof_fraction": _fraction(in_proof, len(refs)),
        "refs_to_claim_label": to_claim,
        "refs_to_claim_label_fraction": _fraction(to_claim, len(refs)),
        "unclassified_display_names": sorted(
            {display for display in environments.values() if display.casefold() not in vocabulary.classified}
        ),
        "has_bib": any(name.endswith(".bib") for name in members),
        "bib_count": sum(1 for name in members if name.endswith(".bib")),
        "has_bbl": bool(bbl),
        "bbl_flavour": bbl_flavour,
    }


def _row(arxiv_id: str, category: str, members: Mapping[str, str], *, vocabulary: Vocabulary) -> dict:
    """One measured paper, or one recorded failure; never a raise, since an unparseable paper is a finding."""
    try:
        row = measure(arxiv_id, members, vocabulary=vocabulary)
        row["source"] = "present"
    except Exception as exc:  # noqa: BLE001 - the failure is the datum
        row = {"id": arxiv_id, "source": "unparseable", "error": f"{type(exc).__name__}: {exc}"}
        print(f"{category} {arxiv_id}: unparseable ({type(exc).__name__}: {exc})", file=sys.stderr)
    row["category"] = category
    if row["source"] == "present":
        print(
            f"{category} {arxiv_id}: "
            f"{len(row['newtheorem'])} env, {row['labels']} labels, "
            f"{row['refs']} refs ({row['refs_in_proof']} in proof, "
            f"{row['refs_to_claim_label']} → claim), bbl={row['bbl_flavour']}",
            file=sys.stderr,
        )
    return row


def survey(categories: list[str], per_category: int, scratch: Path, *, vocabulary: Vocabulary) -> list[dict]:
    scratch.mkdir(parents=True, exist_ok=True)
    rows: list[dict] = []
    for index, category in enumerate(categories):
        if index:
            time.sleep(COURTESY_DELAY)
        try:
            ids = fetch_ids(category, per_category)
        except (urllib.error.URLError, TimeoutError) as exc:
            print(f"{category}: query failed ({exc})", file=sys.stderr)
            continue
        for arxiv_id in ids:
            time.sleep(COURTESY_DELAY)
            tarball = fetch_source(arxiv_id, scratch)
            if tarball is None:
                rows.append({"id": arxiv_id, "category": category, "source": "absent"})
                print(f"{category} {arxiv_id}: no source", file=sys.stderr)
                continue
            rows.append(_row(arxiv_id, category, _sources(tarball), vocabulary=vocabulary))
    return rows


def survey_staged(stage: Path, grouping: Mapping[str, Sequence[str]], *, vocabulary: Vocabulary) -> list[dict]:
    """The same measurement over papers already unpacked, with no network access."""
    rows: list[dict] = []
    for category, ids in grouping.items():
        for arxiv_id in ids:
            # The staging recipe maps the pre-2007 scheme's `/` to `_`; the id keeps its own spelling.
            paper = stage / arxiv_id.replace("/", "_")
            if not paper.is_dir():
                rows.append({"id": arxiv_id, "category": category, "source": "absent"})
                print(f"{category} {arxiv_id}: not staged at {paper}", file=sys.stderr)
                continue
            rows.append(_row(arxiv_id, category, staged_sources(paper), vocabulary=vocabulary))
    return rows


def aggregate(rows: Iterable[dict]) -> dict:
    """Both containment readings over a set of papers, pooled and as a per-paper mean.

    ``papers_with_refs`` is the mean's denominator: a paper with no cross-reference has no fraction to
    average, and counting it as zero would report a measurement nobody took.
    """
    rows = list(rows)
    measured = [row for row in rows if row["source"] == "present"]
    total_refs = sum(row["refs"] for row in measured)
    summary: dict = {
        "papers": len(rows),
        "measured": len(measured),
        "papers_with_refs": sum(1 for row in measured if row["refs"]),
        "unmeasured": {row["id"]: row["source"] for row in rows if row["source"] != "present"},
        "refs": total_refs,
    }
    for reading in ("refs_in_proof", "refs_to_claim_label"):
        total = sum(row[reading] for row in measured)
        seen = [row[f"{reading}_fraction"] for row in measured if row[f"{reading}_fraction"] is not None]
        summary[reading] = total
        summary[f"{reading}_fraction_pooled"] = _fraction(total, total_refs)
        summary[f"{reading}_fraction_paper_mean"] = round(sum(seen) / len(seen), 4) if seen else None

    census: Counter[str] = Counter()
    for row in measured:
        census.update(row["unclassified_display_names"])
    summary["unclassified_display_names"] = dict(census.most_common())
    return summary


def report(rows: Sequence[dict]) -> dict:
    """The aggregates the rows alone do not answer, per category and overall."""
    categories = dict.fromkeys(row["category"] for row in rows)
    return {
        "categories": {
            category: aggregate(row for row in rows if row["category"] == category) for category in categories
        },
        "overall": aggregate(rows),
    }


def scratch_directory_argument(raw: str) -> Path:
    """An output directory strictly inside the project's .claude-temp/: survey output is never committed."""
    out = Path(raw).resolve()
    if not SCRATCH.is_dir():
        raise argparse.ArgumentTypeError(f"{SCRATCH} does not exist; survey output lands only under it")
    if SCRATCH not in out.parents:
        raise argparse.ArgumentTypeError(
            f"{raw!r} is not a directory under {SCRATCH}: survey output lands only in a directory under the "
            "project's .claude-temp/, never where a recipe could commit it"
        )
    if out.exists() and not out.is_dir():
        raise argparse.ArgumentTypeError(f"{raw!r} exists and is not a directory")
    return out


def _dumps(value: object) -> str:
    return json.dumps(value, sort_keys=True, ensure_ascii=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument(
        "--fetch",
        action="append",
        metavar="CATEGORY",
        help=f"fetch and survey recent papers in this arXiv category over the network; repeatable. "
        f"Tarballs land in {TARBALLS}",
    )
    source.add_argument(
        "--staged",
        action="store_true",
        help=f"survey the papers under {STAGE}, grouped by the justfile's ARXIV_* variables; no network",
    )
    parser.add_argument("--per-category", type=int, default=5, help="papers per --fetch category (default 5)")
    parser.add_argument("--out", required=True, type=scratch_directory_argument, help="output directory under .claude-temp/")
    args = parser.parse_args()

    if args.staged:
        try:
            grouping = load_grouping()
        except (OSError, LookupError) as exc:
            print(f"cannot read the category grouping: {exc}", file=sys.stderr)
            return 1
        rows = survey_staged(STAGE, grouping, vocabulary=VOCABULARY)
    else:
        rows = survey(args.fetch, args.per_category, TARBALLS, vocabulary=VOCABULARY)
    rows.sort(key=lambda row: (row["category"], row["id"]))

    args.out.mkdir(parents=True, exist_ok=True)
    papers = args.out / "papers.jsonl"
    summary = args.out / "summary.json"
    papers.write_text("".join(_dumps(row) + "\n" for row in rows), encoding="utf-8")
    summary.write_text(json.dumps(report(rows), indent=2, sort_keys=True, ensure_ascii=False) + "\n", encoding="utf-8")
    print(papers)
    print(summary)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
