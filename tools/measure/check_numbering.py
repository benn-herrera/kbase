"""One-off oracle for latex_numbering.py: compile a volume with tectonic in scratch and compare the .aux label numbers.

A hand-run development check, never a recipe: it needs tectonic on PATH. Outputs and exit status are described in
tools/measure/README.md.
"""

import argparse
import shutil
import subprocess
import sys
from pathlib import Path

import latex_numbering
from latex_numbering import braced

AGREE, DISAGREE, UNWALKED = "yes", "no", "no walked number"
# Some sources send the engine into a loop that never ends; the first run on a host also downloads tectonic's bundle.
COMPILE_SECONDS = 600


def _text(captured: str | bytes | None) -> str:
    """A timed-out run's partial output, which subprocess may hand back as bytes despite ``text=True``."""
    if isinstance(captured, bytes):
        return captured.decode("utf-8", errors="replace")
    return captured or ""


def aux_numbers(aux: Path, numbers: dict[str, str]) -> None:
    """Every ``\\newlabel{<label>}{{<number>}…}`` of ``aux`` and the .aux files it ``\\@input``s, into ``numbers``.

    A value that is not a braced group first (amsart's ``\\newlabel{tocindent0}{0pt}``) is no label number.
    """
    if not aux.is_file():
        return
    for line in aux.read_text(encoding="utf-8", errors="replace").splitlines():
        if line.startswith("\\@input"):
            name = braced(line, len("\\@input"))
            if name is not None:
                aux_numbers(aux.parent / name[0], numbers)
        elif line.startswith("\\newlabel"):
            label = braced(line, len("\\newlabel"))
            value = None if label is None else braced(line, label[1])
            number = None if value is None else braced(value[0], 0)
            if number is not None:
                numbers[label[0]] = number[0]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--volume", required=True, type=latex_numbering.volume_argument, help="the volume's root .tex file")
    parser.add_argument(
        "--out", required=True, type=latex_numbering.scratch_directory_argument, help="output directory under .claude-temp/"
    )
    parser.add_argument(
        "--continue-on-errors",
        action="store_true",
        help="compile past TeX errors (tectonic -Z continue-on-errors), for a paper written for another engine",
    )
    args = parser.parse_args()
    volume: Path = args.volume.resolve()
    out: Path = args.out
    if volume.parent == out or volume.parent in out.parents:
        print(f"--out {str(out)!r} lies inside the volume's directory, which is copied into it", file=sys.stderr)
        return 2
    tectonic = shutil.which("tectonic")
    if tectonic is None:
        print("tectonic is not on PATH: this oracle compiles the volume with it", file=sys.stderr)
        return 2

    build = out / "build"
    (out / "agreement.tsv").unlink(missing_ok=True)
    shutil.copytree(volume.parent, build, dirs_exist_ok=True, ignore=shutil.ignore_patterns(".git"))
    log = build / "tectonic-run.log"
    leniency = ["-Z", "continue-on-errors"] if args.continue_on_errors else []
    try:
        compiled = subprocess.run(
            [tectonic, *leniency, "--keep-intermediates", "--keep-logs", volume.name],
            cwd=build,
            capture_output=True,
            text=True,
            timeout=COMPILE_SECONDS,
        )
    except subprocess.TimeoutExpired as expired:
        log.write_text(_text(expired.stdout) + _text(expired.stderr), encoding="utf-8", newline="\n")
        print(f"tectonic did not finish in {COMPILE_SECONDS} s; its log so far is {log}", file=sys.stderr)
        return 2
    log.write_text(compiled.stdout + compiled.stderr, encoding="utf-8", newline="\n")
    if compiled.returncode != 0:
        print(f"tectonic failed with status {compiled.returncode}; its log is {log}", file=sys.stderr)
        return 2

    aux: dict[str, str] = {}
    aux_numbers(build / f"{volume.stem}.aux", aux)
    rows = []
    for environment in latex_numbering.walk(volume).environments:
        if not environment.label:
            continue
        found = aux.get(environment.label, "")
        agree = UNWALKED if not environment.number else AGREE if environment.number == found else DISAGREE
        rows.append((environment.label, environment.number, found, agree))
    counts = {state: sum(1 for row in rows if row[3] == state) for state in (AGREE, DISAGREE, UNWALKED)}
    out.mkdir(parents=True, exist_ok=True)
    latex_numbering.write_tsv(out, "agreement.tsv", ("label", "walked", "aux", "agree"), rows)
    with (out / "agreement.tsv").open("a", encoding="utf-8", newline="\n") as tsv:
        tsv.write(
            f"# agree {counts[AGREE]}, disagree {counts[DISAGREE]}, {UNWALKED} {counts[UNWALKED]}"
            + ("; compiled past TeX errors\n" if args.continue_on_errors else "\n")
        )
    print(f"wrote agreement.tsv, build/ under {out}")
    return 0 if counts[AGREE] == len(rows) else 1


if __name__ == "__main__":
    sys.exit(main())
