"""The comparison's cascade, edge sets and edge verdict, over synthetic claims, entries and edge sets."""

import unittest
from collections import Counter
from collections.abc import Iterable

from kb_tools import kb_index_lib

from compare_to_pristine import (
    CLAIM_MATCH_HEADER,
    COLUMN,
    MATCH_CLASSES,
    UNMATCHED,
    Claim,
    ClaimMatching,
    Disagreement,
    Ours,
    Reference,
    Span,
    Verdict,
    WitnessEdges,
    claim_match_rows,
    cosine_scores,
    distinct_witnesses,
    edge_sets,
    match_claims,
    matching_heading,
    number_statement_disagreements,
    recall,
    recall_class,
    statement_section,
    statementless_section,
    sweep,
    tally,
    verdict,
    witness_edges,
)
from statement_match import StatementMatches

DOCUMENT = "v/a.md"


def claim(claim_id: str, text: str, line: int = 0) -> Claim:
    return Claim(
        id=claim_id, title=claim_id, kind="prose", documents=(DOCUMENT,), text=text, spans=(Span(DOCUMENT, line, line + 1),)
    )


def ours_of(texts: dict[str, str], *, depends: Iterable[tuple[str, str]] = ()) -> Ours:
    """One node per entry of ``texts``, each its own line of one document."""
    claims = {oid: claim(oid, text, line) for line, (oid, text) in enumerate(texts.items())}
    return Ours(
        claims=claims,
        depends=set(depends),
        references=set(),
        demoted={},
        anchors=[],
        texts={DOCUMENT: list(texts.values())},
        page_blocks=[],
    )


def reference_of(
    titles: dict[str, str], *, edges: Iterable[tuple[str, str]] = (), statements: dict[str, tuple[str, ...]] | None = None
) -> Reference:
    if statements is None:
        statements = {}
    return Reference(
        claims={rid: Claim(id=rid, title=title, kind="reference", documents=("r/x.md",), text="") for rid, title in titles.items()},
        edges=list(edges),
        relations=Counter(),
        statements=statements,
        marked=frozenset(statements),
    )


def cosine_only(scored: dict[str, list[tuple[str, float]]]) -> ClaimMatching:
    return ClaimMatching(exact={}, statement=StatementMatches({}, {}, []), scored=scored, matched={}, fixed={})


class CosineFanTest(unittest.TestCase):
    def test_fan_keeps_every_node_at_or_above_threshold_and_top1_is_first(self):
        matching = cosine_only({"r": [("a", 0.5), ("b", 0.3), ("c", 0.29)]})
        self.assertEqual(matching.mapping(0.3), {"r": {"a", "b"}})
        self.assertEqual(matching.mapping(0.31), {"r": {"a"}})
        self.assertEqual(matching.mapping(0.3, top1=True), {"r": {"a"}})
        self.assertEqual(matching.mapping(0.51, top1=True), {"r": set()})

    def test_cosine_withholds_nodes_an_exact_class_took(self):
        matching = ClaimMatching(
            exact={},
            statement=StatementMatches({"r1": ("n1", 1.0)}, {"r1": 1.0}, []),
            scored={"r1": [("n1", 1.0)], "r2": [("n1", 0.9), ("n2", 0.45)]},
            matched={"r1": "n1"},
            fixed={"r1": "statement"},
        )
        for top1 in (False, True):
            with self.subTest(top1=top1):
                self.assertEqual(matching.mapping(0.3, top1=top1), {"r1": {"n1"}, "r2": {"n2"}})
        mapped = matching.mapping(0.3)
        self.assertEqual(matching.end_scores("r2", mapped["r2"]), "n2=0.450")
        self.assertEqual(matching.weakest_end("r1", "r2", mapped), "cosine 0.40-0.50")

    def test_top1_is_first_ties_by_id(self):
        scored = cosine_scores(
            {"r": Claim(id="r", title="", kind="reference", documents=(), text="lyapunov stability")},
            {"n2": claim("n2", "lyapunov stability"), "n1": claim("n1", "lyapunov stability"), "n3": claim("n3", "lyapunov")},
        )
        self.assertEqual([oid for oid, _ in scored["r"]], ["n1", "n2", "n3"])
        self.assertAlmostEqual(scored["r"][0][1], 1.0)


def judge(a: set[str], b: set[str], *, depends=(), references=(), demoted=None) -> Verdict:
    return verdict(a, b, depends=set(depends), references=set(references), demoted=demoted or {})


def entry(claim_id: str, *, depends_on=(), references=(), demoted=()) -> kb_index_lib.ClaimEntry:
    return kb_index_lib.ClaimEntry(
        id=claim_id,
        title=claim_id,
        canonical_path="a.md",
        canonical_anchor="",
        confidence=None,
        solidity=None,
        build_status=None,
        rationale="",
        depends_on=tuple(depends_on),
        strengthen_by=(),
        references=tuple(references),
        demoted=tuple(demoted),
    )


def edge(source: str, target: str, relation: str, *, target_kind: str = "claim", origin: str | None = None):
    return kb_index_lib.DependsOnEdge(
        source=source,
        target=target,
        relation=relation,
        target_kind=target_kind,
        target_solidity_recorded=None,
        strength=None,
        context=None,
        origin=origin,
    )


class EdgeSetsTest(unittest.TestCase):
    def test_each_relation_in_its_own_set_and_demoted_is_its_own_class(self):
        entries = [
            entry(
                "x",
                depends_on=[edge("x", "y", "depends"), edge("x", "inv-1", "depends", target_kind="invariant")],
                references=[edge("x", "z", "references")],
                demoted=[edge("x", "w", "demoted", origin="inferred")],
            ),
            entry("v", demoted=[edge("v", "x", "demoted")]),
        ]
        depends, references, demoted = edge_sets(entries)
        self.assertEqual(depends, {("x", "y")})
        self.assertEqual(references, {("x", "z")})
        self.assertEqual(demoted, {("x", "w"): "inferred", ("v", "x"): ""})
        self.assertEqual(judge({"x"}, {"w"}, demoted=demoted), Verdict("demoted (origin inferred)", "x->w"))
        self.assertEqual(judge({"x"}, {"v"}, demoted=demoted), Verdict("demoted (origin not named, reversed)", "v->x"))
        self.assertEqual(recall_class(judge({"x"}, {"w"}, demoted=demoted).recall), "demoted")


class VerdictTest(unittest.TestCase):
    def test_wrong_match_is_not_endpoint_unmatched(self):
        self.assertEqual(judge({"x"}, {"z"}, depends={("x", "y")}).recall, "nothing")
        self.assertEqual(judge({"x"}, set(), depends={("x", "y")}).recall, f"{UNMATCHED} (target)")
        self.assertEqual(judge(set(), set()).recall, f"{UNMATCHED} (source and target)")

    def test_direction(self):
        self.assertEqual(judge({"x"}, {"y"}, depends={("x", "y")}), Verdict("depends", "x->y"))
        self.assertEqual(judge({"y"}, {"x"}, depends={("x", "y")}), Verdict("depends reversed", "x->y"))

    def test_path_runs_over_depends_only_and_reports_length(self):
        chain = {("x", "m1"), ("m1", "m2"), ("m2", "m3"), ("m3", "y")}
        self.assertEqual(judge({"x"}, {"y"}, depends=chain), Verdict("depends path", "x->m1->m2->m3->y", 4))
        self.assertEqual(judge({"y"}, {"x"}, depends=chain), Verdict("depends path reversed", "x->m1->m2->m3->y", 4))
        shortcut = chain | {("m1", "y")}
        self.assertEqual(judge({"x"}, {"y"}, depends=shortcut), Verdict("depends path", "x->m1->y", 2))
        self.assertEqual(judge({"x"}, {"y"}, references={("x", "r"), ("r", "y")}).recall, "nothing")
        self.assertEqual(judge({"x"}, {"y"}, references={("y", "x")}), Verdict("references only", "y->x"))

    def test_verdict_edge_classes_precede_shared_node(self):
        self.assertEqual(judge({"x", "s"}, {"s", "y"}, depends={("x", "y")}), Verdict("depends", "x->y"))
        self.assertEqual(judge({"s"}, {"s"}), Verdict("shared node"))
        self.assertEqual(judge({"x", "s"}, {"s", "y"}, depends={("s", "y")}), Verdict("depends", "s->y"))

    def test_an_edge_into_the_shared_node_is_read(self):
        self.assertEqual(judge({"x", "s"}, {"s"}, depends={("x", "s")}), Verdict("depends", "x->s"))
        self.assertEqual(judge({"s"}, {"x", "s"}, depends={("x", "s")}), Verdict("depends reversed", "x->s"))
        self.assertEqual(judge({"x", "s"}, {"s"}, depends={("x", "m"), ("m", "s")}), Verdict("depends path", "x->m->s", 2))
        self.assertEqual(judge({"s", "t"}, {"s", "t"}, depends={("s", "t")}), Verdict("shared node"))


class WitnessTest(unittest.TestCase):
    def test_one_built_edge_witnessing_two_reference_edges_is_reported(self):
        reference = reference_of({"r1": "a", "r2": "b", "r3": "c"}, edges=[("r1", "r2"), ("r1", "r3")])
        ours = ours_of({"m": "alpha", "n": "beta"}, depends={("m", "n")})
        matching = cosine_only({"r1": [("m", 0.9)], "r2": [("n", 0.9)], "r3": [("n", 0.9)]})
        rows = recall(matching=matching, at=0.3, walked_names={}, reference=reference, ours=ours)
        self.assertEqual([(row[COLUMN["recall"]], row[COLUMN["witness"]]) for row in rows], [("depends", "m->n")] * 2)
        self.assertEqual(distinct_witnesses(rows), {"depends": (2, 1)})

    def test_a_path_witness_shares_its_edges_with_a_depends_witness(self):
        reference = reference_of({"r1": "a", "r2": "b", "r3": "c"}, edges=[("r1", "r2"), ("r3", "r2")])
        ours = ours_of({"m": "alpha", "n": "beta", "x": "gamma"}, depends={("m", "n"), ("x", "m")})
        matching = cosine_only({"r1": [("m", 0.9)], "r2": [("n", 0.9)], "r3": [("x", 0.9)]})
        rows = recall(matching=matching, at=0.3, walked_names={}, reference=reference, ours=ours)
        self.assertEqual(
            [(row[COLUMN["recall"]], row[COLUMN["witness"]]) for row in rows], [("depends", "m->n"), ("depends path", "x->m->n")]
        )
        self.assertEqual(witness_edges(rows), WitnessEdges(rows=2, mentions=3, distinct=2))


class ClaimMatchRowsTest(unittest.TestCase):
    def test_claim_matches_carries_overlap_for_statement_rows(self):
        matching = ClaimMatching(
            exact={},
            statement=StatementMatches({"r": ("n", 0.94)}, {"r": 0.94}, []),
            scored={"r": [("n", 0.22)]},
            matched={"r": "n"},
            fixed={"r": "statement"},
        )
        [row] = claim_match_rows(
            matching=matching, mapped=matching.mapping(0.3), reference=reference_of({"r": "a"}), ours=ours_of({"n": "beta"})
        )
        cells = dict(zip(CLAIM_MATCH_HEADER, row))
        self.assertEqual((cells["class"], cells["score"], cells["overlap"]), ("statement", "0.220", "0.940"))


class MatchClaimsTest(unittest.TestCase):
    STATEMENT = "lyapunov stability bifurcation threshold"

    def matching(self) -> ClaimMatching:
        reference = reference_of(
            {"r1": "numbered", "r2": "stated", "r3": "phase portrait"},
            statements={"r2": (self.STATEMENT,), "r3": ("phase portrait",)},
        )
        ours = ours_of({"n1": self.STATEMENT, "n2": "phase portrait", "n3": "unrelated words"})
        scored = cosine_scores(reference.claims, ours.claims)
        return match_claims(reference=reference, ours=ours, exact={"r1": "n1"}, scored=scored)

    def test_number_nodes_are_not_offered_to_statement(self):
        matching = self.matching()
        self.assertNotIn("r2", matching.statement.matches)
        self.assertEqual(matching.statement.best["r2"], 0.0)
        self.assertEqual(matching.statement.matches["r3"], ("n2", 1.0))

    def test_heading_names_the_classes_the_cascade_assigns(self):
        matching = self.matching()
        assigned = {matching.match_class(rid) for rid in matching.scored}
        self.assertEqual(assigned, set(MATCH_CLASSES))
        heading, _, label = matching_heading(0.3)
        self.assertEqual(heading.count(", then "), len(MATCH_CLASSES) - 1)
        self.assertNotIn("label", heading)
        self.assertIn("not implemented", label)


class StatementSectionsTest(unittest.TestCase):
    STATEMENT = "lyapunov stability bifurcation threshold"

    def setUp(self):
        self.reference = reference_of({"r1": "stated", "r2": "unstated"}, statements={"r1": (self.STATEMENT,)})
        ours = ours_of({"n1": self.STATEMENT, "n2": "phase portrait"})
        scored = {"r1": [("n1", 0.8), ("n2", 0.1)], "r2": [("n1", 0.6), ("n2", 0.35)]}
        self.matching = match_claims(reference=self.reference, ours=ours, exact={}, scored=scored)

    def test_a_statement_matched_claim_is_not_listed_as_falling_to_cosine(self):
        self.assertEqual(self.matching.statement.matches, {"r1": ("n1", 1.0)})
        section = "\n".join(statement_section(matching=self.matching, reference=self.reference))
        self.assertNotIn("r1", section)
        self.assertIn("r2", section)

    def test_the_statementless_group_lists_exactly_the_claims_without_a_statement(self):
        mapped = self.matching.mapping(0.3)
        section = statementless_section(matching=self.matching, mapped=mapped, reference=self.reference)
        listed = [line.split()[0] for line in section if line.startswith("  ")]
        self.assertEqual(listed, ["r2"])
        self.assertIn("best 0.350  fan 1  matched by cosine", section[3])


class NumberStatementDisagreementTest(unittest.TestCase):
    def test_a_number_match_another_node_restates_is_reported(self):
        reference = reference_of({"r": "Theorem 1"}, statements={"r": ("lyapunov stability bifurcation",)})
        ours = ours_of({"n1": "phase portrait", "n2": "lyapunov stability bifurcation"})
        found = number_statement_disagreements(reference=reference, ours=ours, exact={"r": "n1"})
        self.assertEqual(
            found,
            [Disagreement(reference="r", number_node="n1", number_overlap=0.0, statement_node="n2", statement_overlap=1.0)],
        )

    def test_agreement_is_not_reported(self):
        reference = reference_of({"r": "Theorem 1"}, statements={"r": ("lyapunov stability bifurcation",)})
        ours = ours_of({"n1": "lyapunov stability bifurcation"})
        self.assertEqual(number_statement_disagreements(reference=reference, ours=ours, exact={"r": "n1"}), [])


class SweepTest(unittest.TestCase):
    def test_sweep_row_equals_a_run_at_the_same_threshold(self):
        reference = reference_of({"r1": "a", "r2": "b", "r3": "c"}, edges=[("r1", "r2"), ("r2", "r3")])
        ours = ours_of({"n1": "alpha", "n2": "beta", "n3": "gamma"}, depends={("n1", "n2")})
        matching = cosine_only(
            {"r1": [("n1", 0.45)], "r2": [("n2", 0.35), ("n3", 0.32)], "r3": [("n3", 0.5)]}
        )
        for top1 in (False, True):
            with self.subTest(top1=top1):
                rows = {
                    at: (matched, classes)
                    for at, matched, classes, _ in sweep(matching=matching, walked_names={}, reference=reference, ours=ours, top1=top1)
                }
                for at in (0.30, 0.40):
                    run = recall(matching=matching, at=at, walked_names={}, reference=reference, ours=ours, top1=top1)
                    classes, _ = tally(run)
                    matched = sum(1 for kept in matching.mapping(at, top1=top1).values() if kept)
                    self.assertEqual(rows[at], (matched, classes))
                self.assertNotEqual(rows[0.30], rows[0.40])


if __name__ == "__main__":
    unittest.main()
