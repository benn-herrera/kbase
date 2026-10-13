"""The comparison's text reading and its mark test, over synthetic page text."""

import tempfile
import unittest
from pathlib import Path

from kb_tools.kb_claimgraph import inventory, prose, tree

from compare_to_pristine import Claim, Span, mark_test, marked_statement, plain_text, reference_section

SCRATCH = Path(__file__).resolve().parents[2] / ".claude-temp"

LEAF = """# Leaf A

Opening prose of the leaf.

> <span id="thm:a">**Theorem**</span>
>
> **Theorem 1** (Stability). <!-- claim-quality: clm-blk001 --> The fixed point
> is stable under every perturbation.

Some prose between.

<!-- claim-quality: clm-par001 -->
The paragraph that the marker above marks,
running over a second line.

<!-- claim-quality: clm-hdg001 -->
## Next heading

Prose under the heading.
"""


class MarkedStatementTest(unittest.TestCase):
    """The reference side's statement: the block, fence or paragraph holding the marker, else the next one to open."""

    def test_extent(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            root = Path(scratch)
            (root / "a.md").write_text(LEAF, encoding="utf-8")
            (root / "b.md").write_text("# Leaf B\n\nNothing here.\n", encoding="utf-8")
            documents = tree.read(root)
        sites = inventory.scan(documents)
        document = documents.documents["a.md"]
        readable = prose.readable(document, sites)
        lines = LEAF.splitlines()

        def statement(claim_id: str) -> list[str] | None:
            span = marked_statement(document, claim_id, sites=sites, readable=readable)
            return None if span is None else lines[span.start : span.end]

        self.assertEqual(statement("clm-blk001"), lines[4:8])
        self.assertEqual(statement("clm-par001"), lines[12:14])
        self.assertIsNone(statement("clm-hdg001"))


class ReferenceSectionTest(unittest.TestCase):
    def test_without_marker_is_the_opening_section(self):
        leaf = "---\nclaims: [clm-x]\n---\n# Title\nfirst line\nsecond line\n## Sub\nmore prose\n"
        self.assertEqual(reference_section(leaf, "clm-x"), "first line\nsecond line")


TWO_ID_LEAF = """# Leaf C

<!-- claim-quality: clm-aaaaaa clm-bbbbbb -->
The paragraph both ids mark.

## Next heading
"""


class TwoIdMarkerTest(unittest.TestCase):
    """A marker naming two claims locates the same statement for either."""

    def test_reference_section(self):
        for claim_id in ("clm-aaaaaa", "clm-bbbbbb"):
            with self.subTest(claim_id=claim_id):
                self.assertEqual(reference_section(TWO_ID_LEAF, claim_id), "The paragraph both ids mark.\n")

    def test_marked_statement(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as scratch:
            (Path(scratch) / "c.md").write_text(TWO_ID_LEAF, encoding="utf-8")
            documents = tree.read(Path(scratch))
        sites = inventory.scan(documents)
        document = documents.documents["c.md"]
        readable = prose.readable(document, sites)
        for claim_id in ("clm-aaaaaa", "clm-bbbbbb"):
            with self.subTest(claim_id=claim_id):
                self.assertEqual(marked_statement(document, claim_id, sites=sites, readable=readable), Span("c.md", 3, 4))

REF = '<a href="b.md#thm:b" data-reference-type="ref" data-reference="thm:b">3</a>'


class PlainTextTest(unittest.TestCase):
    def test_tags_and_comments_go_inequalities_stay(self):
        cases = {
            '<span id="thm:b">**Theorem**</span> holds': "Theorem holds",
            f"by Theorem {REF}.": "by Theorem 3 .",
            "x <!-- a\ncomment --> y<br/>": "x y",
            "a < b & c > d": "a < b & c > d",
            r"$x<y \text{ and } z>w$": r"$x<y \text{ and } z>w$",
            "$a<b$ and $c>d$": "$a<b$ and $c>d$",
        }
        for text, expected in cases.items():
            with self.subTest(text=text):
                self.assertEqual(plain_text(text), expected)


def anchor(*, document: str, line: int, target: str | None, label: str) -> inventory.Anchor:
    return inventory.Anchor(
        document=document,
        line=line,
        reference_type="ref",
        href=f"{target or ''}#{label}",
        target=target,
        fragment=label,
        label=label,
        hosting_environment=None,
        preceding_word="Theorem",
    )


class MarkTest(unittest.TestCase):
    """A's text names B by our page's counter, the walked number or B's label; never by the reference's number."""

    def marks(self, line: str, *, anchors=(), a_document="v/a.md") -> list[str]:
        a = Claim(id="a", title="A", kind="prose", documents=(a_document,), text=line, spans=(Span(a_document, 0, 1),))
        b = Claim(
            id="b", title="Theorem 9.9 — B", kind="block", documents=("v/b.md",), text="", printed=("Theorem 3",), label="thm:b"
        )
        return mark_test(
            a_nodes=[a],
            b_nodes=[b],
            walked_names={"b": "Theorem 5.11"},
            anchors=list(anchors),
            texts={a_document: [line]},
        )

    def test_names(self):
        cases = {
            f"by Theorem {REF}": ["name a: 'Theorem 3' (ours: Theorem 3)"],
            "as Thm. 5.11 shows": ["name a: 'Thm. 5.11' (walked: Theorem 5.11)"],
            "Theorem 9.9 of the long paper": [],
            "Theorem 5.110": [],
        }
        for line, expected in cases.items():
            with self.subTest(line=line):
                self.assertEqual(self.marks(line), expected)

    def test_label_anchor_in_b_volume(self):
        unresolved = anchor(document="v/a.md", line=0, target=None, label="thm:b")
        self.assertEqual(self.marks("see [thm:b]", anchors=[unresolved]), ["label a:ref -> thm:b"])

    def test_label_anchor_in_another_volume_is_another_result(self):
        elsewhere = anchor(document="w/a.md", line=0, target=None, label="thm:b")
        self.assertEqual(self.marks("see [thm:b]", anchors=[elsewhere], a_document="w/a.md"), [])


if __name__ == "__main__":
    unittest.main()
