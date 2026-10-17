import os
import tempfile
import unittest

import export_forms as ef

XREF = {
    'documents': {'d1-a': {'path': 'v/a.md'}, 'd1-b': {'path': 'v/sub/b.md'}},
    'paths': {'v/a.md': 'd1-a', 'v/sub/b.md': 'd1-b'},
    'assets': {'v/fig.png': 'assets/v/fig.png'},
    'entry_point': 'entry-point.md',
}

FRONT = 'See [entry](missing.md).\n'

STREAM = '''# V {#vol1}

# A {#d1-a}

Cite <span class="citation" data-cites="k1 k2">(k1)</span>, see
<a href="sub/b.md" data-reference-type="ref"
data-reference="eq:x">1</a> and [b](sub/b.md#h), [gone](nowhere.md), [web](https://x.org),
![f](fig.png), ![g](nofig.png), `[code](nowhere.md)`.
<span id="L1">**Lemma**</span> <figure id="fig:a"></figure>
<a href="#h" data-reference-type="ref+label" data-reference="s1,s2">2</a>

> ``` math
> a \\label{eq:x} \\eqref{eq:x}
> ```

```python
<span id="not-a-label">
```

# B {#d1-b}

Row text
{#tab:t}

[wrapped <a href="a.md" data-reference="q">1</a>](#frag)
'''


class Forms(unittest.TestCase):
    def setUp(self):
        self.seg = ef.segments([FRONT, STREAM], XREF['documents'], XREF['entry_point'])
        self.a = ef.scan('v/a.md', *self.seg['v/a.md'], XREF)

    def test_segments_split_at_document_headings_and_keep_fences_apart(self):
        self.assertEqual(sorted(self.seg), ['entry-point.md', 'v/a.md', 'v/sub/b.md'])
        self.assertIn('\\label{eq:x}', self.seg['v/a.md'][1])
        self.assertNotIn('not-a-label', self.seg['v/a.md'][0])
        self.assertIn('entry', self.seg['entry-point.md'][0])

    def test_each_stream_file_starts_in_the_entry_point(self):
        seg = ef.segments([STREAM, '# V2 {#vol2}\n\nvolume text\n'], XREF['documents'], XREF['entry_point'])
        self.assertIn('volume text', seg['entry-point.md'][0])
        self.assertNotIn('volume text', seg['v/sub/b.md'][0])

    def test_scan_reads_each_contract_form_once(self):
        self.assertEqual(self.a['cites'], [['k1', 'k2']])
        self.assertEqual([x['data-reference'] for x in self.a['anchors']], ['eq:x', 's1,s2'])
        self.assertEqual(self.a['labels'], ['L1', 'fig:a'])
        self.assertEqual([(c, t) for c, t in self.a['braced'] if c in ('label', 'eqref')],
                         [('label', 'eq:x'), ('eqref', 'eq:x')])

    def test_scan_classifies_links_and_images_as_the_filter_does(self):
        self.assertEqual(self.a['links'], [('document', 'v/sub/b.md'), ('degraded', 'v/nowhere.md')])
        self.assertEqual(self.a['images'], [('asset', 'v/fig.png'), ('degraded', 'v/nofig.png')])
        b = ef.scan('v/sub/b.md', *self.seg['v/sub/b.md'], XREF)
        self.assertEqual(b['labels'], ['tab:t'])
        self.assertEqual(b['links'], [])  # a link around an anchor is its anchor's

    def test_entry_point_links_resolve_from_the_root(self):
        e = ef.scan('entry-point.md', *self.seg['entry-point.md'], XREF)
        self.assertEqual(e['links'], [('degraded', 'missing.md')])

    def test_parse_report_reads_plain_quoted_and_folded_scalars(self):
        text = ("- form: link\n  path: v/a.md\n  reason: the link names x.md, which is\n    not an exported document\n"
                "- form: preamble\n  path: /p/preamble.tex\n  reason: 'the author''s'\n"
                "- form: residue\n  path: \"\"\n  reason: \"raw \\\"html\\\"\"\n")
        r = ef.parse_report(text)
        self.assertEqual([i['form'] for i in r], ['link', 'preamble', 'residue'])
        self.assertEqual(ef.reported_key(r[0]), 'x.md')
        self.assertEqual(r[1]['reason'], "the author's")
        self.assertEqual((r[2]['path'], r[2]['reason']), ('', 'raw "html"'))

    def test_truth_refuses_a_line_naming_a_form_its_path_lacks(self):
        with tempfile.TemporaryDirectory() as kb:
            os.makedirs(os.path.join(kb, 'v'))
            with open(os.path.join(kb, 'v/a.md'), 'w') as fh:
                fh.write(STREAM)
            good = [{'form': 'link', 'path': 'v/a.md', 'reason': 'the link names v/nowhere.md, which is not an exported document'},
                    {'form': 'citation', 'path': 'v/a.md', 'reason': 'the citation key k2 has no bibliography entry; the citation is its text'}]
            good.append({'form': 'anchor', 'path': 'v/a.md', 'reason': 'the anchor names eq:x, which no exported document of volume 1 defines'})
            good.append({'form': 'anchor', 'path': 'v/a.md', 'reason': 'the anchor names s1, s2, which no exported document of volume 1 defines'})
            good.append({'form': 'anchor', 'path': 'v/a.md', 'reason': 'the anchor names v/sub/b.md, which is not an exported document'})
            self.assertEqual(ef.truth(kb, good, XREF), [])
            bad = [{'form': 'link', 'path': 'v/a.md', 'reason': 'the link names v/other.md, which is not an exported document'},
                   {'form': 'citation', 'path': 'v/a.md', 'reason': 'the citation key k9 has no bibliography entry; the citation is its text'},
                   {'form': 'link', 'path': 'entry-point.md', 'reason': 'the link names a.md, which is not an exported document'}]
            self.assertEqual(len(ef.truth(kb, bad, XREF)), 3)

    def test_footnotes_read_references_in_order_and_multi_line_definitions(self):
        text = ('Body[^b] and more[^a] `[^code]`.\n\n```\n[^fenced]\n```\n\n'
                '[^a]: Alpha note\n    continued here.\n\n[^b]: <span class="citation" data-cites="k">Beta</span>.\n')
        refs, defs = ef.footnotes(text)
        self.assertEqual(refs, ['b', 'a'])
        self.assertEqual(defs, {'a': 'Alpha note continued here.', 'b': '<span class="citation" data-cites="k">Beta</span>.'})

    def test_tex_footnotes_balance_braces_and_skip_escaped_ones(self):
        notes = ef.tex_footnotes('x\\footnote{a \\{ \\emph{b} c} y \\footnote{d}')
        self.assertEqual([b for _, b in notes], ['a \\{ \\emph{b} c', 'd'])

    VOLS = {1: 'One[^1]\n\n[^1]: Grant sponsored the first author.\n',
            2: 'Two[^1]\n\n[^1]: <span class="citation" data-cites="hacking1975">Hacking</span> argued otherwise.\n'}

    def tex(self, first, second):
        return ('\\chapter{A}\\label{vol1}text\\footnote{%s}\n\\chapter{B}\\label{vol2}text\\footnote{%s}\n'
                '\\begin{thebibliography}{1}\\footnote{ignored}\\end{thebibliography}' % (first, second))

    def test_each_volumes_note_is_its_own_definition(self):
        good = self.tex('Grant sponsored the first author.', '\\cite{hacking1975} argued otherwise.')
        self.assertEqual(ef.note_problems(self.VOLS, good), {1: [], 2: []})

    def test_a_note_taken_from_another_volume_is_refused(self):
        bad = self.tex('Grant sponsored the first author.', 'Grant sponsored the first author.')
        problems = ef.note_problems(self.VOLS, bad)
        self.assertEqual(problems[1], [])
        self.assertEqual(len(problems[2]), 1)
        self.assertIn('[^1]', problems[2][0])

    def test_a_missing_note_is_refused(self):
        problems = ef.note_problems(self.VOLS, self.tex('Grant sponsored the first author.', '').replace('\\footnote{}', ''))
        self.assertIn('1 footnote references, main.tex 0', problems[2][0])

    def test_unresolved_names_the_volume_of_the_label_family(self):
        tex = ('\\label{v1-a}\\label{d2-x}\\ref{v1-a}\\cref{v1-a,v1-gone}\\hyperref[d2-x]{t}'
               '\\eqref{d2-lost}\\ref{plain}')
        bad, used = ef.unresolved(tex)
        self.assertEqual(bad, {1: ['v1-gone'], 2: ['d2-lost'], 0: ['plain']})
        self.assertEqual((used[1], used[2], used[0]), (3, 2, 1))

    def test_scan_log_counts_wrapped_warnings_and_missing_characters(self):
        log = ("LaTeX Warning: Reference `eq:x' on page 3\nundefined on input line 9.\n\n"
               "LaTeX Warning: Citation `k' on page 1 undefined on input line 2.\n\n"
               "Missing character: There is no x in font y!\n\n"
               "LaTeX Warning: There were multiply-defined labels.\n")
        c = ef.scan_log(log)
        self.assertEqual((c['undefined references'], c['undefined citations'], c['missing characters']), (1, 1, 1))


if __name__ == '__main__':
    unittest.main()
