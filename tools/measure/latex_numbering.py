"""The author's theorem numbers recovered from LaTeX source: a counter walker over amsthm's declarations.

Every output file and column is described in tools/measure/README.md. Stdlib only; it reads source and runs nothing.
"""

import argparse
import re
import sys
from collections import Counter
from collections.abc import Callable, Iterable
from dataclasses import dataclass, field, replace
from pathlib import Path

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"

CHAPTERED_CLASSES = {"book", "report", "amsbook", "scrbook", "scrreprt", "memoir"}
SECTION_LEVELS = {"chapter": 0, "section": 1, "subsection": 2, "subsubsection": 3}

# A % preceded by an even run of backslashes opens a comment; \% is a percent sign.
COMMENT_RE = re.compile(r"(?<!\\)((?:\\\\)*)%.*")
TOKEN_RE = re.compile(
    "|".join(
        (
            r"\\(?P<begin_end>begin|end)\s*\{(?P<env>[^}]*)\}",
            r"\\newtheorem(?P<star>\*?)\s*\{(?P<thm_env>[^}]*)\}\s*(?:\[(?P<thm_shared>[^\]]*)\]\s*)?"
            r"\{(?P<thm_name>(?:[^{}]|\{[^{}]*\})*)\}(?:\s*\[(?P<thm_within>[^\]]*)\])?",
            r"\\numberwithin\s*\{(?P<nw_counter>[^}]*)\}\s*\{(?P<nw_parent>[^}]*)\}",
            r"\\setcounter\s*\{(?P<sc_counter>[^}]*)\}\s*\{(?P<sc_value>[^}]*)\}",
            r"\\(?P<sectioning>chapter|section|subsection|subsubsection)\b(?P<sec_star>\s*\*)?",
            r"\\(?P<appendix>appendix)\b",
            r"\\(?:input|include)\s*\{(?P<include>[^}]*)\}",
            r"\\input\s+(?P<include_bare>[^\s{}\\]+)",
            r"\\label\s*\{(?P<label>[^}]*)\}",
            r"\\documentclass\s*(?:\[[^\]]*\]\s*)?\{(?P<doc_class>[^}]*)\}",
            r"\\(?:(?:renew|provide|new)command\*?|g?def)\s*\{?\s*\\the(?P<the>[A-Za-z]+)\b",
            r"\\(?P<counter_op>addtocounter|stepcounter|refstepcounter|counterwithin\*?|counterwithout\*?)"
            r"\s*\{(?P<op_counter>[^}]*)\}",
            r"\\(?P<new_counter>newcounter|newaliascnt)\s*\{(?P<new_name>[^}]*)\}(?:\s*\{(?P<alias_of>[^}]*)\})?",
            r"\\(?P<other_decl>declaretheorem|spnewtheorem\*?)\s*(?:\[[^\]]*\]\s*)?\{(?P<decl_env>[^}]*)\}",
            # Text TeX never reads: a macro defined to discard its arguments, \iffalse, the comment environment.
            r"\\(?:renew|provide|new)command\*?\s*\{?\s*\\(?P<discarder>[A-Za-z]+)\s*\}?\s*\[(?P<discarded>\d)\]\s*\{\s*\}",
            r"\\(?P<iffalse>iffalse)\b",
            r"\\(?P<macro>[A-Za-z]+)",
        )
    )
)
CONDITIONAL_RE = re.compile(r"\\(?:if[A-Za-z@]*|else|or|fi)\b")
SPACE_RE = re.compile(r"\s*")
END_COMMENT = "\\end{comment}"


def braced(text: str, start: int) -> tuple[str, int] | None:
    """The balanced group opening at ``text[start]`` and the index after it, or None if none opens there."""
    if start >= len(text) or text[start] != "{":
        return None
    depth = 0
    for index in range(start, len(text)):
        if text[index] == "{":
            depth += 1
        elif text[index] == "}":
            depth -= 1
            if depth == 0:
                return text[start + 1 : index], index + 1
    return None


@dataclass(frozen=True)
class Environment:
    ordinal: int
    env: str
    name: str  # the printed word \newtheorem declared; empty where no \newtheorem named it
    group: str
    ordinal_in_group: int | None
    number: str
    label: str
    file: str
    line: int


@dataclass
class _Counter:
    value: int = 0
    within: str | None = None
    letters: bool = False
    outside: str = ""  # why its value or format lies outside the grammar; numbers drawn through it stay empty


def _render(counter: _Counter) -> str | None:
    if not counter.letters:
        return str(counter.value)
    return chr(ord("A") + counter.value - 1) if 1 <= counter.value <= 26 else None


@dataclass
class _Walk:
    root: Path
    load: Callable[[Path], str | None]
    counters: dict[str, _Counter] = field(
        default_factory=lambda: {
            "section": _Counter(),
            "subsection": _Counter(within="section"),
            "subsubsection": _Counter(within="subsection"),
        }
    )
    groups: dict[str, str | None] = field(default_factory=dict)  # environment -> counter; None when unnumbered
    names: dict[str, str] = field(default_factory=dict)  # environment -> its \newtheorem Name
    secnumdepth: int = 3
    in_document: bool = False
    rows: list[Environment] = field(default_factory=list)
    open: list[tuple[str, int | None]] = field(default_factory=list)
    in_group: Counter = field(default_factory=Counter)
    redefined_the: dict[str, str] = field(default_factory=dict)
    discarders: dict[str, int] = field(default_factory=dict)  # macro -> how many arguments it throws away
    notes: list[str] = field(default_factory=list)
    reading: list[Path] = field(default_factory=list)

    def relative(self, path: Path) -> str:
        try:
            return path.relative_to(self.root.parent).as_posix()
        except ValueError:
            return path.as_posix()

    def note(self, where: str, message: str) -> None:
        self.notes.append(f"{where}: {message}")

    def mark_outside(self, name: str, reason: str) -> None:
        if not self.counters[name].outside:
            self.counters[name].outside = reason

    def define(self, name: str, counter: _Counter, where: str) -> None:
        self.counters[name] = counter
        if name in self.redefined_the:
            self.note(self.redefined_the[name], f"\\the{name} redefined: numbers from counter {name!r} left empty")
            self.mark_outside(name, f"\\the{name} redefined at {self.redefined_the[name]}")

    def number(self, name: str) -> str | None:
        counter = self.counters.get(name)
        if counter is None or counter.outside:
            return None
        own = _render(counter)
        if own is None or counter.within is None:
            return own
        prefix = self.number(counter.within)
        return None if prefix is None else f"{prefix}.{own}"

    def step(self, name: str) -> None:
        self.counters[name].value += 1
        self.reset_within(name)

    def reset_within(self, name: str) -> None:
        for child, counter in self.counters.items():
            if counter.within == name:
                counter.value = 0
                self.reset_within(child)

    def resolve(self, raw: str, including: Path) -> Path | None:
        name = raw.strip()
        for base in (including.parent, self.root.parent):
            path = base / name
            candidates = (path,) if path.suffix == ".tex" else (path.with_name(path.name + ".tex"), path)
            for candidate in candidates:
                if self.load(candidate) is not None:
                    return candidate
        return None

    def read(self, path: Path) -> bool:
        """Walk one file, following its includes; True once ``\\end{document}`` is reached."""
        text = self.load(path)
        if text is None:
            return False
        text = COMMENT_RE.sub(r"\1", text)
        self.reading.append(path)
        line, last, position = 1, 0, 0
        try:
            while (match := TOKEN_RE.search(text, position)) is not None:
                line += text.count("\n", last, match.start())
                last = match.start()
                where = f"{self.relative(path)}:{line}"
                position = self.unread_end(match, text, where=where)
                if position is None:
                    position = match.end()
                    if self.token(match, path=path, where=where, line=line):
                        return True
        finally:
            self.reading.pop()
        return False

    def unread_end(self, match: re.Match[str], text: str, *, where: str) -> int | None:
        """Where the text TeX never reads, opening at ``match``, ends; None when ``match`` opens no such text."""
        g = match.group
        if g("iffalse"):
            closing = CONDITIONAL_RE.search(text, match.end())
            if closing is not None and closing.group() == "\\fi":
                return closing.end()
            self.note(where, "\\iffalse not closed by a plain \\fi: its text is read, every number after it left empty")
            for name in self.counters:
                self.mark_outside(name, f"unpaired \\iffalse at {where}")
            return match.end()
        if g("begin_end") == "begin" and g("env").strip() == "comment":
            end = text.find(END_COMMENT, match.end())
            return len(text) if end < 0 else end + len(END_COMMENT)
        if g("macro") in self.discarders:
            position = match.end()
            for _ in range(self.discarders[g("macro")]):
                argument = braced(text, SPACE_RE.match(text, position).end())
                if argument is None:
                    break
                position = argument[1]
            return position
        return None

    def token(self, match: re.Match[str], *, path: Path, where: str, line: int) -> bool:
        g = match.group
        if g("begin_end"):
            return self.begin_end(g("begin_end"), g("env").strip(), path=path, line=line)
        if g("thm_env") is not None:
            self.names[g("thm_env").strip()] = g("thm_name").strip()
            self.newtheorem(
                g("thm_env").strip(),
                starred=bool(g("star")),
                shared=g("thm_shared"),
                within=g("thm_within"),
                where=where,
            )
        elif g("nw_counter") is not None:
            counter, parent = g("nw_counter").strip(), g("nw_parent").strip()
            if counter in self.counters and parent in self.counters:
                self.counters[counter].within = parent
            elif counter in self.counters:
                self.note(where, f"\\numberwithin{{{counter}}}{{{parent}}}: {parent!r} is not a counter the walker tracks")
                self.mark_outside(counter, f"\\numberwithin an untracked counter at {where}")
        elif g("sc_counter") is not None:
            self.setcounter(g("sc_counter").strip(), g("sc_value").strip(), where=where)
        elif g("sectioning"):
            name = g("sectioning")
            if self.in_document and not g("sec_star") and name in self.counters and SECTION_LEVELS[name] <= self.secnumdepth:
                self.step(name)
        elif g("appendix"):
            if self.in_document:
                lettered = "chapter" if "chapter" in self.counters else "section"
                self.counters[lettered].value = 0
                self.counters[lettered].letters = True
                for child in ("section", "subsection"):
                    self.counters[child].value = 0
        elif g("include") is not None or g("include_bare") is not None:
            raw = g("include") if g("include") is not None else g("include_bare")
            found = self.resolve(raw, path)
            if found is None:
                self.note(where, f"include {raw.strip()!r} not found: skipped")
            elif found in self.reading:
                self.note(where, f"include {raw.strip()!r} is already being read: skipped")
            else:
                return self.read(found)
        elif g("label") is not None:
            # Only a label at the environment's own level names it; one inside a nested list or equation names that.
            index = self.open[-1][1] if self.open else None
            if index is not None and not self.rows[index].label:
                self.rows[index] = replace(self.rows[index], label=g("label").strip())
        elif g("doc_class") is not None:
            if g("doc_class").strip() in CHAPTERED_CLASSES:
                self.counters["chapter"] = _Counter()
                self.counters["section"].within = "chapter"
                self.secnumdepth = 2
        elif g("the") is not None:
            name = g("the")
            self.redefined_the.setdefault(name, where)
            if name in self.counters:
                self.note(where, f"\\the{name} redefined: numbers from counter {name!r} left empty")
                self.mark_outside(name, f"\\the{name} redefined at {where}")
        elif g("counter_op"):
            name = g("op_counter").strip()
            if name in self.counters:
                self.note(where, f"\\{g('counter_op')}{{{name}}}: numbers from counter {name!r} left empty")
                self.mark_outside(name, f"\\{g('counter_op')} at {where}")
        elif g("new_counter"):
            name = g("new_name").strip()
            self.define(name, _Counter(outside=f"\\{g('new_counter')} at {where}"), where)
            aliased = (g("alias_of") or "").strip()
            if g("new_counter") == "newaliascnt" and aliased in self.counters:
                # Every environment drawing on the alias steps the aliased counter too.
                self.note(where, f"\\newaliascnt{{{name}}}{{{aliased}}}: numbers from counter {aliased!r} left empty")
                self.mark_outside(aliased, f"\\newaliascnt at {where}")
        elif g("discarder"):
            self.discarders[g("discarder")] = int(g("discarded"))
        elif g("other_decl"):
            for env in (part.strip() for part in g("decl_env").split(",")):
                self.note(where, f"\\{g('other_decl')}{{{env}}}: environment {env!r} counted, its numbers left empty")
                self.groups[env] = env
                self.define(env, _Counter(outside=f"\\{g('other_decl')} at {where}"), where)
        return False

    def newtheorem(self, env: str, *, starred: bool, shared: str | None, within: str | None, where: str) -> None:
        if starred:
            self.groups[env] = None
            return
        if shared is not None:
            shared = shared.strip()
            self.groups[env] = shared
            if shared not in self.counters:
                self.note(where, f"\\newtheorem{{{env}}}[{shared}]: {shared!r} is not a counter the walker tracks")
                self.define(shared, _Counter(outside=f"untracked shared counter at {where}"), where)
            elif self.counters[shared].outside:
                self.note(where, f"\\newtheorem{{{env}}}[{shared}]: counter {shared!r} is outside the grammar "
                          f"({self.counters[shared].outside})")
            return
        self.groups[env] = env
        if within is None:
            self.define(env, _Counter(), where)
            return
        within = within.strip()
        if within in self.counters:
            self.define(env, _Counter(within=within), where)
        else:
            self.note(where, f"\\newtheorem{{{env}}}{{…}}[{within}]: {within!r} is not a counter the walker tracks")
            self.define(env, _Counter(outside=f"numbered within untracked {within!r} at {where}"), where)

    def setcounter(self, name: str, value: str, *, where: str) -> None:
        integer = re.fullmatch(r"-?\d+", value)
        if name == "secnumdepth" and integer:
            self.secnumdepth = int(value)
        elif name in self.counters:
            if integer:
                self.counters[name].value = int(value)
            else:
                self.note(where, f"\\setcounter{{{name}}}{{{value}}}: not an integer; numbers from {name!r} left empty")
                self.mark_outside(name, f"\\setcounter to {value!r} at {where}")

    def begin_end(self, which: str, env: str, *, path: Path, line: int) -> bool:
        if env == "document":
            self.in_document = which == "begin"
            return which == "end"
        if not self.in_document:
            return False
        if which == "end":
            if any(name == env for name, _ in self.open):
                while self.open.pop()[0] != env:
                    pass
            return False
        if env not in self.groups:
            self.open.append((env, None))
            return False
        group = self.groups[env]
        number, ordinal_in_group = "", None
        if group is not None:
            self.step(group)
            self.in_group[group] += 1
            ordinal_in_group = self.in_group[group]
            number = self.number(group) or ""
        index = len(self.rows)
        self.rows.append(
            Environment(
                ordinal=index + 1,
                env=env,
                name=self.names.get(env, ""),
                group=group or "",
                ordinal_in_group=ordinal_in_group,
                number=number,
                label="",
                file=self.relative(path),
                line=line,
            )
        )
        self.open.append((env, index))
        return False


@dataclass(frozen=True)
class Numbering:
    environments: list[Environment]
    notes: list[str]


def walk_source(root: Path, load: Callable[[Path], str | None]) -> Numbering:
    """Number every theorem-like environment reachable from ``root``, reading each file through ``load``."""
    walk = _Walk(root=root, load=load)
    if load(root) is None:
        walk.note(walk.relative(root), "volume root not found")
    walk.read(root)
    return Numbering(environments=walk.rows, notes=walk.notes)


def read_source(path: Path) -> str | None:
    """A source file's text, or None where there is no such file; bytes that are not UTF-8 are replaced."""
    if not path.is_file():
        return None
    return path.read_text(encoding="utf-8", errors="replace")


def walk(root: Path) -> Numbering:
    return walk_source(root.resolve(), read_source)


# --- arguments and output, the same rules as compare_to_pristine.py ---------------------------------


def volume_argument(raw: str) -> Path:
    path = Path(raw)
    if not path.is_file():
        raise argparse.ArgumentTypeError(f"{raw!r} is not a file")
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


def write_tsv(out: Path, name: str, header: Iterable[str], rows: Iterable[Iterable[str]]) -> None:
    def clean(cell: str) -> str:
        return cell.replace("\t", " ").replace("\r", " ").replace("\n", " ")

    body = ["\t".join(header)] + ["\t".join(clean(cell) for cell in row) for row in rows]
    (out / name).write_text("\n".join(body) + "\n", encoding="utf-8", newline="\n")


ENVIRONMENT_HEADER = ("ordinal", "env", "name", "group", "ordinal_in_group", "number", "label", "file", "line")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--volume", required=True, type=volume_argument, help="the volume's root .tex file")
    parser.add_argument("--out", required=True, type=scratch_directory_argument, help="output directory under .claude-temp/")
    args = parser.parse_args()
    numbering = walk(args.volume)
    out: Path = args.out
    out.mkdir(parents=True, exist_ok=True)
    write_tsv(
        out,
        "environments.tsv",
        ENVIRONMENT_HEADER,
        (
            (str(e.ordinal), e.env, e.name, e.group, "" if e.ordinal_in_group is None else str(e.ordinal_in_group), e.number,
             e.label, e.file, str(e.line))
            for e in numbering.environments
        ),
    )
    (out / "notes.txt").write_text("".join(f"{note}\n" for note in numbering.notes), encoding="utf-8", newline="\n")
    print(f"wrote environments.tsv, notes.txt under {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
