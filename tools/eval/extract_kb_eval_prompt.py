"""Extract the KB inferential-quality eval instrument's prompt body, or one of its header fields.

Two independent modes, each with its own stdout contract, so a shell caller captures each with its own
`$(...)` and neither can corrupt the other:

  Body mode  — argv: --walk-set <walk_set_file> <brief_path> <kb_root_path> <report_path> <corpus_note>
               Stdout: the substituted prompt body.
  Field mode — argv: --field model <brief_path>
               Stdout: the dispatch contract's model name (e.g. "sonnet"), newline-terminated.

Stderr + exit 1: any refusal below, in either mode. Described in tools/eval/README.md.
"""

import re
import sys
from pathlib import Path

START_MARKER = "---- PROMPT BODY BELOW"
END_MARKER = "---- PROMPT BODY ABOVE"
DISPATCH_PREFIX = "Dispatch contract:"
MODEL_TOKEN = re.compile(r"`model:\s*([^`\s]+)`")
WALK_SET_FLAG = "--walk-set"

# The header's "<run-tag>" (angle brackets, in its report-path naming convention) is documentation only,
# never a fifth placeholder — this list is exhaustive and deliberately does not include it.
PLACEHOLDERS = ("{kb-root-path}", "{output-report-path}", "{corpus-note}", "{walk-set}")


def fail(message: str) -> None:
    sys.stderr.write(f"error: {message}\n")
    sys.exit(1)


def find_start(lines: list[str], brief_path: str) -> int:
    start_idx = next((i for i, line in enumerate(lines) if line.startswith(START_MARKER)), None)
    if start_idx is None:
        fail(f"no line matching '^{START_MARKER}' in {brief_path} — the fixture's shape is a contract; refusing.")
    return start_idx


def extract_model(lines: list[str], start_idx: int, brief_path: str) -> str:
    # Bounded to the header so a "Dispatch contract:"-shaped string inside the body is never mistaken for it.
    header = lines[:start_idx]
    dispatch_idx = next((i for i, line in enumerate(header) if line.lstrip().startswith(DISPATCH_PREFIX)), None)
    if dispatch_idx is None:
        fail(f"no line starting with '{DISPATCH_PREFIX}' before the prompt body in {brief_path} — refusing.")

    # The sentence may wrap before the model token; join a small forward window.
    window = "".join(header[dispatch_idx : dispatch_idx + 3])
    match = MODEL_TOKEN.search(window)
    if match is None:
        fail(
            f"dispatch contract has no backtick-quoted `model: ...` token in {brief_path} — "
            f"refusing. Line: {header[dispatch_idx]!r}"
        )
    return match.group(1)


def extract_body(
    lines: list[str], start_idx: int, *, kb_root: str, report_path: str, corpus_note: str, walk_set: str
) -> str:
    # Without an end marker, extraction runs to EOF. Searched only after start_idx so header prose can never
    # be mistaken for it.
    end_idx = next((i for i, line in enumerate(lines) if i > start_idx and line.startswith(END_MARKER)), None)

    body = "".join(lines[start_idx + 1 : end_idx]).lstrip("\n")

    # The scalar values are substituted in sequence, so one carrying a placeholder-shaped string could fill
    # or swallow another's slot; a stray brace in any of them refuses instead. The walk set may carry LaTeX
    # braces, so it is exempt and substituted last, where nothing rescans it.
    for name, value in (("kb-root-path", kb_root), ("output-report-path", report_path), ("corpus-note", corpus_note)):
        if "{" in value or "}" in value:
            brace = "{" if "{" in value else "}"
            fail(f"{name} value contains a stray '{brace}' — refusing. Value: {value!r}")

    missing = [p for p in PLACEHOLDERS if p not in body]
    if missing:
        fail(f"prompt body missing placeholder(s): {', '.join(missing)} — refusing rather than launch a malformed eval.")

    body = body.replace("{kb-root-path}", kb_root)
    body = body.replace("{output-report-path}", report_path)
    body = body.replace("{corpus-note}", corpus_note)
    return body.replace("{walk-set}", walk_set)


def read_brief(brief_path: str) -> tuple[list[str], int]:
    lines = Path(brief_path).read_text(encoding="utf-8").splitlines(keepends=True)
    return lines, find_start(lines, brief_path)


def main(argv: list[str]) -> int:
    if argv[:1] == ["--field"]:
        if len(argv) != 3 or argv[1] != "model":
            fail(f"--field takes exactly one field name ('model') and a brief_path, got {argv[1:]!r}")
        brief_path = argv[2]
        lines, start_idx = read_brief(brief_path)
        sys.stdout.write(extract_model(lines, start_idx, brief_path) + "\n")
        return 0

    if argv[:1] != [WALK_SET_FLAG] or len(argv) != 6:
        fail(
            f"expected {WALK_SET_FLAG} <walk_set_file> brief_path kb_root_path report_path corpus_note, "
            f"got {argv!r}"
        )
    walk_set_path = Path(argv[1])
    brief_path, kb_root, report_path, corpus_note = argv[2:]
    if not walk_set_path.is_file():
        fail(f"walk-set file {str(walk_set_path)!r} does not exist or is not a file — refusing.")
    walk_set = walk_set_path.read_text(encoding="utf-8")
    lines, start_idx = read_brief(brief_path)
    sys.stdout.write(
        extract_body(
            lines, start_idx, kb_root=kb_root, report_path=report_path, corpus_note=corpus_note, walk_set=walk_set
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
