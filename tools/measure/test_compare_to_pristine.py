"""The comparison's text reading and its mark test, over synthetic page text."""

import unittest

from kb_tools.kb_claimgraph import inventory

from compare_to_pristine import Claim, Span, mark_test, plain_text

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
