"""The comparison's `number` class: a reference claim titled by a result word and the author's number, matched to
the block of ours whose page shows that word at the counter value the author's numbering walks to that number.

Stdlib only, so the rule is testable without kb_tools; compare_to_pristine.py reads both sides and calls it.
"""

import re
from collections import Counter
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import dataclass

from latex_numbering import Environment

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
# Only the word folds case: a number opens with a digit or a capital ("C.5.1"), never a lower-case word.
NAME_NUMBER_RE = re.compile(r"^\W*((?i:" + "|".join(NAME_FORMS) + r"))\s+([0-9A-Z][0-9.]*[a-z]?)\b")
SUFFIXED_RE = re.compile(r"(.*\d)([a-z])")
PRINTED_RE = re.compile(r"(\S.*?)\s+(\d+)")

NO_SUCH_WORD = "no environment of that word in the source"
NO_WALKED_NUMBER = "no walked number"
NUMBER_DIFFERS = "the block's number differs"
NO_BLOCK = "no block with that word and k"
NO_NODE = "no block node at all for that environment"
AMBIGUOUS = "ambiguous"


@dataclass(frozen=True)
class ResultKey:
    word: str  # a NAME_FORMS key
    number: str  # the numeric part, matched against the walked number
    suffix: str  # a trailing letter the author added by hand ("5.10c"); not part of the match


@dataclass(frozen=True)
class PageBlock:
    """A block of ours as its page shows it."""

    volume: str  # our top-level directory hosting it
    printed: str  # its display line's printed word and pandoc counter, plain ("Proposition 8")
    node_id: str | None  # the claim node it is, or None where the graph carries none


@dataclass(frozen=True)
class NumberOutcome:
    key: ResultKey
    node_id: str  # empty when unmatched
    reason: str  # empty when matched


def result_key(title: str) -> ResultKey | None:
    parsed = NAME_NUMBER_RE.match(title)
    if parsed is None:
        return None
    written = parsed.group(2).rstrip(".")
    suffixed = SUFFIXED_RE.fullmatch(written)
    number, suffix = suffixed.groups() if suffixed else (written, "")
    return ResultKey(word=parsed.group(1).lower(), number=number, suffix=suffix)


def printed_word_counter(printed: str) -> tuple[str, int] | None:
    """``("proposition", 8)`` from ``"Proposition 8"``; None where the printed name ends in no counter."""
    parsed = PRINTED_RE.fullmatch(printed.strip())
    return None if parsed is None else (parsed.group(1).lower(), int(parsed.group(2)))


def _page_index(blocks: Iterable[PageBlock]) -> dict[tuple[str, str, int], list[str | None]]:
    page: dict[tuple[str, str, int], list[str | None]] = {}
    for block in blocks:
        parsed = printed_word_counter(block.printed)
        if parsed is not None:
            page.setdefault((block.volume, *parsed), []).append(block.node_id)
    return page


def _match_one(
    key: ResultKey, *, page: Mapping[tuple[str, str, int], list[str | None]], walked: Mapping[str, Sequence[Environment]]
) -> NumberOutcome:
    def miss(reason: str) -> NumberOutcome:
        return NumberOutcome(key=key, node_id="", reason=reason)

    same_word = [(volume, env) for volume, envs in walked.items() for env in envs if env.name.lower() == key.word]
    hits = [(volume, env) for volume, env in same_word if env.number == key.number]
    if not hits:
        if not same_word:
            return miss(NO_SUCH_WORD)
        if any(env.ordinal_in_group is not None and not env.number for _, env in same_word):
            return miss(NO_WALKED_NUMBER)
        return miss(NUMBER_DIFFERS)
    if len(hits) > 1:
        return miss(f"{AMBIGUOUS}: {len(hits)} environments walk to that number")
    volume, env = hits[0]
    nodes = page.get((volume, key.word, env.ordinal_in_group), [])
    if not nodes:
        return miss(NO_BLOCK)
    if len(nodes) > 1:
        return miss(f"{AMBIGUOUS}: {len(nodes)} blocks print that word and k")
    if nodes[0] is None:
        return miss(NO_NODE)
    return NumberOutcome(key=key, node_id=nodes[0], reason="")


def match_numbers(
    titles: Mapping[str, str], *, blocks: Iterable[PageBlock], walked: Mapping[str, Sequence[Environment]]
) -> dict[str, NumberOutcome]:
    """Every reference claim whose title names a result word and number, with its node of ours or why it has none.

    ``titles`` maps reference claim id to title; ``walked`` maps our volume directory to the walker's environments
    for that volume's source. A match is one-to-one: a node two reference claims would both reach matches neither.
    """
    page = _page_index(blocks)
    outcomes = {
        rid: _match_one(key, page=page, walked=walked)
        for rid, title in titles.items()
        if (key := result_key(title)) is not None
    }
    claimed = Counter(outcome.node_id for outcome in outcomes.values() if outcome.node_id)
    return {
        rid: outcome
        if claimed[outcome.node_id] <= 1
        else NumberOutcome(key=outcome.key, node_id="", reason=f"{AMBIGUOUS}: {claimed[outcome.node_id]} reference claims reach one block")
        for rid, outcome in outcomes.items()
    }


def unwalked_blocks(blocks: Iterable[PageBlock], walked: Mapping[str, Sequence[Environment]]) -> list[PageBlock]:
    """The blocks of a walked volume whose printed word and counter name no environment the walker counted there.

    Non-empty means the page's counter and the walk disagree — a volume root given for the wrong directory, or
    pandoc numbering a group otherwise than the walker counts it — and number matches in that volume are suspect.
    """
    counted = {
        (volume, env.name.lower(), env.ordinal_in_group)
        for volume, envs in walked.items()
        for env in envs
        if env.ordinal_in_group is not None
    }
    return [
        block
        for block in blocks
        if block.volume in walked
        and (parsed := printed_word_counter(block.printed)) is not None
        and (block.volume, *parsed) not in counted
    ]
