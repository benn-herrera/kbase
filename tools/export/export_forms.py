#!/usr/bin/env python3
"""Check a kbase export-latex project against the contract forms of its own Markdown stream.

    export_forms.py counts   GEN_TEX_DIR [--kb-root KB_ROOT]
    export_forms.py log      LOG...

`counts` reads the stream files the tables' `streams` list names, in order (the exported documents,
navigation stripped; each file's footnotes its own), the tables kbase-xref.json, the report
kbase-export-report.yaml and main.tex, and compares what the streams hold with what main.tex emitted
and the report degraded. The forms are the KB's contract: a `data-cites` span, a `data-reference`
anchor, a `<span id>` label, a maths `\\label` or reference, a Markdown link, a footnote. Each volume's
footnote references must come out in main.tex as the text of that volume's own definitions, and every
reference macro must name a label main.tex defines. With --kb-root it also checks the report against
the KB files: complete (every degraded link, image and citation has its line) and true (every line
names a form its path holds).
`log` counts undefined references, undefined citations and missing characters in compile logs.

Prints one `check: want N got M` line per check and exits 1 where any fails. Python 3.11+, stdlib only.
"""
import json
import os
import posixpath
import re
import sys
from collections import Counter
from urllib.parse import unquote

FENCE = re.compile(r'^\s*(`{3,}|~{3,})\s*([^\s`]*)')
QUOTE = re.compile(r'^(?:\s*>\s?)+')
HEADING_ID = re.compile(r'^#+\s.*\{#([^}\s]+)\}\s*$')
COMMENT = re.compile(r'<!--.*?-->', re.S)
INLINE_CODE = re.compile(r'`[^`\n]*`')
TAG = re.compile(r'<(a|span|figure|div|table)\b([^>]*)>')
ATTRIBUTE_ID = re.compile(r'(?m)^(?!#)(?:.*[^\\\w])?\{#(?!\d)([^}\s]+)[^}]*\}')
ATTR = re.compile(r'([A-Za-z][\w:-]*)\s*=\s*"([^"]*)"')
LINK = re.compile(r'(!?)\[((?:[^\[\]]|\[[^\]]*\])*)\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)')
BRACED = re.compile(r'\\([A-Za-z]+)\{([^{}]*)\}')
EXTERNAL = re.compile(r'^(?:[A-Za-z][\w+.-]*:|//)')
REFERENCE_COMMANDS = ('ref', 'eqref', 'cref', 'Cref')

CITE_EMITTED = re.compile(r'\\cite\{')
LABEL_EMITTED = re.compile(r'\\label\{v\d+-')
REF_EMITTED = re.compile(r'\\(?:eqref|ref|cref|Cref)\{')
HYPERREF_EMITTED = re.compile(r'\\hyperref\[')
LABEL_DEFINED = re.compile(r'\\label\{([^{}]*)\}')
REFERENCE_USED = re.compile(r'\\(?:eqref|ref|cref|Cref)\{([^{}]*)\}|\\hyperref\[([^\]]*)\]')
VOLUME_STREAM = re.compile(r'vol(\d+)\.md$')
VOLUME_TARGET = re.compile(r'^(?:vol|v|d)(\d+)(?!\d)')
FOOTNOTE_DEF = re.compile(r'^\[\^([^\]\s]+)\]:\s?(.*)$')
FOOTNOTE_REF = re.compile(r'\[\^([^\]\s]+)\](?!:)')
FOOTNOTE_OPEN = re.compile(r'\\footnote(?:text)?\{')
TAG_ANY = re.compile(r'<[^>]*>')
LATEX_COMMAND = re.compile(r'\\[A-Za-z]+')
WORD = re.compile(r'[a-z]{4,}')


def walk(stream):
    """(line, kind) per line of one stream file, block-quote markers dropped, fence lines skipped:
    kind is None outside a code fence, else 'math' or 'code'."""
    fence = None  # (char, length, kind)
    for raw in stream.split('\n'):
        line = QUOTE.sub('', raw, count=1)
        m = FENCE.match(line)
        if fence:
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= fence[1] and not m.group(2):
                fence = None
            else:
                yield line, fence[2]
        elif m:
            fence = (m.group(1)[0], len(m.group(1)), 'math' if m.group(2) == 'math' else 'code')
        else:
            yield line, None


def segments(streams, documents, entry_point):
    """Per exported document path, (prose, maths): the streams' text outside code fences, and the
    bodies of their math fences. `streams` is the stream files' texts, each starting in the entry
    point's text; `documents` is the xref table of heading id to path."""
    out = {entry_point: ([], [])}
    for stream in streams:
        current = entry_point
        for line, kind in walk(stream):
            if kind:
                if kind == 'math':
                    out[current][1].append(line)
                continue
            h = HEADING_ID.match(line)
            if h and h.group(1) in documents:
                current = documents[h.group(1)]['path']
                out.setdefault(current, ([], []))
            out[current][0].append(line)
    return {p: ('\n'.join(prose), '\n'.join(maths)) for p, (prose, maths) in out.items()}


def footnotes(stream):
    """(refs, defs) of one stream file: the footnote labels in order of reference, and each
    label's definition text."""
    refs, defs, label = [], {}, None
    for line, kind in walk(stream):
        if kind:
            continue
        d = FOOTNOTE_DEF.match(line)
        if d:
            label = d.group(1)
            defs[label] = [d.group(2)]
        elif label and (line[:1] in (' ', '\t') or not line.strip()):
            defs[label].append(line.strip())
        else:
            label = None
            refs += FOOTNOTE_REF.findall(INLINE_CODE.sub('', line))
    return refs, {k: ' '.join(v).strip() for k, v in defs.items()}


def tex_footnotes(tex):
    """(offset, body) of every footnote in main.tex, in order."""
    out = []
    for m in FOOTNOTE_OPEN.finditer(tex):
        depth, i = 1, m.end()
        while depth and i < len(tex):
            if tex[i] == '\\':
                i += 1
            else:
                depth += (tex[i] == '{') - (tex[i] == '}')
            i += 1
        out.append((m.start(), tex[m.end():i - 1]))
    return out


def words(text):
    """The words of four or more letters a note's Markdown or LaTeX carries, markup and command
    names removed, so a note reads the same in the stream and in main.tex."""
    return set(WORD.findall(LATEX_COMMAND.sub(' ', TAG_ANY.sub(' ', text)).lower()))


def note_problems(volume_streams, tex):
    """Per volume number, the footnotes of main.tex's volume section that are not the text of the
    definition its reference names in the volume's own stream file, as messages. A note matches
    where it holds at least 80% of its definition's words."""
    starts = {n: tex.find('\\label{vol%d}' % n) for n in volume_streams}
    back = tex.find('\\begin{thebibliography}')
    bounds = sorted(p for p in starts.values() if p >= 0) + [back if back >= 0 else len(tex)]
    notes = tex_footnotes(tex)
    problems = {}
    for n, stream in volume_streams.items():
        bad = problems[n] = []
        refs, defs = footnotes(stream)
        if starts[n] < 0:
            bad.append('main.tex has no \\label{vol%d}' % n)
            continue
        end = min(p for p in bounds if p > starts[n])
        got = [body for pos, body in notes if starts[n] <= pos < end]
        if len(got) != len(refs):
            bad.append('the stream has %d footnote references, main.tex %d footnotes' % (len(refs), len(got)))
        for i, (label, body) in enumerate(zip(refs, got), 1):
            if label not in defs:
                bad.append('reference %d, [^%s], has no definition in its volume' % (i, label))
                continue
            want = words(defs[label])
            have = words(body)
            if len(want & have) < 0.8 * len(want):
                bad.append('footnote %d, [^%s], reads %r, not its definition %r' % (i, label, body[:60], defs[label][:60]))
    return problems


def unresolved(tex):
    """The reference targets main.tex uses that no \\label defines, grouped by the volume number
    their label family names (0 for none), and the count of references per volume."""
    defined = set(LABEL_DEFINED.findall(tex))
    bad, used = {}, Counter()
    for plain, hyper in REFERENCE_USED.findall(tex):
        for target in (plain or hyper).split(','):
            m = VOLUME_TARGET.match(target)
            n = int(m.group(1)) if m else 0
            used[n] += 1
            if target not in defined:
                bad.setdefault(n, []).append(target)
    return bad, used


def resolve(path, target):
    """The kb-root-relative path a link target names from the document at path: fragment and :line
    suffix dropped, percent-decoded, joined to the document's directory and cleaned."""
    target = unquote(re.sub(r':\d+$', '', target.split('#', 1)[0]))
    if target == '':
        return path
    return posixpath.normpath(posixpath.join(posixpath.dirname(path), target))


def scan(path, prose, maths, xref):
    """The contract forms one document holds. Links and images carry their resolved target and
    what the export does with it: document, asset or degraded."""
    prose = COMMENT.sub('', prose)
    f = {'cites': [], 'anchors': [], 'labels': [], 'links': [], 'images': [], 'braced': []}
    for tag, attrs in TAG.findall(prose):
        a = dict(ATTR.findall(attrs))
        if tag == 'a':
            if a.get('data-reference'):
                f['anchors'].append(a)
        elif tag != 'span':
            if a.get('id'):
                f['labels'].append(a['id'])
        elif 'citation' in a.get('class', '').split() and a.get('data-cites'):
            f['cites'].append(a['data-cites'].split())
        elif a.get('id'):
            f['labels'].append(a['id'])
    f['labels'] += ATTRIBUTE_ID.findall(prose)
    f['braced'] = BRACED.findall(prose + '\n' + maths)
    for bang, text, target in LINK.findall(INLINE_CODE.sub('', prose)):
        if EXTERNAL.match(target) or (not bang and 'data-reference' in text):
            continue
        resolved = resolve(path, target)
        kind = 'document' if resolved in xref['paths'] else 'asset' if resolved in xref['assets'] else 'degraded'
        f['images' if bang else 'links'].append((kind, resolved))
    return f


def parse_report(text):
    """The report's lines as dicts of form, path and reason. The report is a YAML list of three
    plain, single-quoted or double-quoted scalars, possibly folded over several lines."""
    items = []
    key = None
    for line in text.split('\n'):
        m = re.match(r'^(?:- |  )(form|path|reason): ?(.*)$', line)
        if m:
            if line.startswith('- '):
                items.append({})
            key = m.group(1)
            items[-1][key] = m.group(2)
        elif key and line.startswith('    '):
            items[-1][key] += ' ' + line.strip()
    for item in items:
        for k, v in item.items():
            if v.startswith("'") and v.endswith("'"):
                item[k] = v[1:-1].replace("''", "'")
            elif v.startswith('"') and v.endswith('"'):
                item[k] = json.loads(v)
    return items


def expected_degraded(per_document):
    """The (form, path, key) a complete report holds for the links, images and citations the
    stream's documents carry: key is the resolved target, or the keys of a citation span."""
    want = Counter()
    for path, f in per_document.items():
        for form, entries in (('link', f['links']), ('image', f['images'])):
            for kind, target in entries:
                if kind == 'degraded':
                    want[(form, path, target)] += 1
    return want


def reported_key(item):
    """The key a report line names, parsed from its reason; None for a form not keyed here."""
    m = {'link': r'^the link names (.+), which is not an exported document$',
         'image': r'^the image names (.+), which is not an exported asset$'}.get(item.get('form'))
    m = m and re.match(m, item.get('reason', ''))
    return m.group(1) if m else None


def truth(kb_root, report, xref):
    """Every report line that names a form its path does not hold, as a message."""
    bad = []
    cache = {}

    def forms(path):
        if path not in cache:
            with open(os.path.join(kb_root, path), encoding='utf-8') as fh:
                text = '\n'.join(QUOTE.sub('', l, count=1) for l in fh.read().split('\n'))
            cache[path] = (text, scan(path, text, '', xref))
        return cache[path]

    for item in report:
        form, path, reason = item.get('form'), item.get('path', ''), item.get('reason', '')
        if form in ('preamble', 'walk'):
            continue
        if not os.path.isfile(os.path.join(kb_root, path)):
            bad.append('%s line names the path %r, which is no file of the KB' % (form, path))
            continue
        text, f = forms(path)
        if form in ('link', 'image'):
            key = reported_key(item)
            held = [t for k, t in f['links' if form == 'link' else 'images'] if k == 'degraded']
            if key not in held:
                bad.append('%s line names %s, which %s does not hold as a degraded %s' % (form, key, path, form))
        elif form == 'citation':
            m = re.match(r'^the citation key (.+) has no bibliography entry', reason)
            names = m.group(1).split(', ') if m else []
            if not any(set(names) <= set(keys) for keys in f['cites']):
                bad.append('citation line names %s, which no citation span of %s holds' % (names, path))
        elif form == 'anchor':
            m = re.match(r'^the anchor names (.+), which (?:is not an exported document|no exported document of volume)', reason)
            named = m.group(1) if m else ''
            if not any(named in (a['data-reference'], resolve(path, a.get('href', '').split('#')[0]))
                       or set(named.split(', ')) <= set(re.split(r'[,\s]+', a['data-reference'])) for a in f['anchors']):
                bad.append('anchor line names %s, which no anchor of %s holds' % (named, path))
        elif form == 'math-reference':
            m = re.match(r'^the maths reference (\\[A-Za-z]+\{[^{}]*\})', reason)
            if not m or m.group(1) not in text:
                bad.append('math-reference line names %s, which %s does not hold' % (m and m.group(1), path))
    return bad


def check_counts(gen, kb_root=None):
    """The (check, want, got) rows of the count-matched forms, then, with kb_root, the links and
    the report's completeness and truth as rows whose want and got are both lists; with them the
    (name, count) facts the rows were computed from."""
    read = lambda name: open(os.path.join(gen, name), encoding='utf-8').read()
    xref = json.loads(read('kbase-xref.json'))
    streams = {name: read(name) for name in xref['streams']}
    per_document = {p: scan(p, prose, maths, xref)
                    for p, (prose, maths) in segments(list(streams.values()), xref['documents'], xref['entry_point']).items()}
    report = parse_report(read('kbase-export-report.yaml'))
    by_form = Counter(item['form'] for item in report)
    tex = '\n'.join(l for l in read('main.tex').split('\n') if not l.lstrip().startswith('%'))
    volume_streams = {int(VOLUME_STREAM.search(name).group(1)): text
                      for name, text in streams.items() if VOLUME_STREAM.search(name)}
    note_bad = note_problems(volume_streams, tex)
    ref_bad, ref_used = unresolved(tex)
    total = lambda key: sum(len(f[key]) for f in per_document.values())
    braced = Counter(c for f in per_document.values() for c, _ in f['braced'])
    facts = [('documents', len(per_document)),
             ('citation spans', total('cites')), ('citation report lines', by_form['citation']),
             ('label ids', total('labels')), ('maths labels', braced['label']),
             ('reference anchors', total('anchors')), ('anchor report lines', by_form['anchor']),
             ('maths references', sum(braced[c] for c in REFERENCE_COMMANDS)),
             ('math-reference report lines', by_form['math-reference']),
             ('report lines', len(report))]
    rows = [
        ('citations: data-cites spans less citation lines = \\cite', total('cites') - by_form['citation'], len(CITE_EMITTED.findall(tex))),
        ('labels: ids (span, figure, div, table, attribute) + maths \\label = \\label{vN-', total('labels') + braced['label'], len(LABEL_EMITTED.findall(tex))),
        ('references: anchors + maths refs less anchor and math-reference lines = reference macros',
         total('anchors') + sum(braced[c] for c in REFERENCE_COMMANDS) - by_form['anchor'] - by_form['math-reference'],
         len(REF_EMITTED.findall(tex))),
    ]
    for n in sorted(volume_streams):
        rows.append(('footnotes: volume %d, each note the text of its own volume\'s definition' % n, [], note_bad[n]))
        rows.append(('references: volume %d, every reference macro naming its labels has a \\label' % n, [], ref_bad.get(n, [])))
        facts.append(('volume %d footnote references' % n, len(footnotes(volume_streams[n])[0])))
        facts.append(('volume %d reference macros naming it' % n, ref_used[n]))
    rows.append(('references: every other reference macro has a \\label', [],
                 [t for n, targets in ref_bad.items() if n not in volume_streams for t in targets]))
    if kb_root:
        links = Counter(k for f in per_document.values() for k, _ in f['links'])
        rows.append(('links: links to exported documents = \\hyperref', links['document'], len(HYPERREF_EMITTED.findall(tex))))
        rows.append(('links: degraded links = link lines', links['degraded'], by_form['link']))
        images = sum(1 for f in per_document.values() for k, _ in f['images'] if k == 'degraded')
        rows.append(('images: degraded images = image lines', images, by_form['image']))
        want = expected_degraded(per_document)
        got = Counter((i['form'], i['path'], reported_key(i)) for i in report if i['form'] in ('link', 'image'))
        rows.append(('complete: degraded links and images, each with its line',
                     [], sorted('%s %s %s' % k for k in (want - got))))
        rows.append(('true: link and image lines, each a degraded form of its path',
                     [], sorted('%s %s %s' % k for k in (got - want))))
        rows.append(('true: every report line names a form its path holds', [], truth(kb_root, report, xref)))
        facts += [('links to exported documents', links['document']), ('degraded links', links['degraded']),
                  ('degraded images', images)]
    return rows, facts


def scan_log(text):
    """Counts of the warnings a compile log holds that the check refuses."""
    out = Counter()
    for block in re.split(r'\n\s*\n', text):
        flat = ' '.join(block.split())
        if 'Missing character:' in flat:
            out['missing characters'] += flat.count('Missing character:')
        if 'Warning' in flat and re.search(r'\bundefined\b', flat) and 'multiply' not in flat:
            out['undefined citations' if 'Citation' in flat else 'undefined references'] += 1
    for k in ('missing characters', 'undefined citations', 'undefined references'):
        out[k] += 0
    return out


def main(argv):
    if len(argv) >= 3 and argv[1] == 'counts':
        kb_root = argv[argv.index('--kb-root') + 1] if '--kb-root' in argv else None
        rows, facts = check_counts(argv[2], kb_root)
        for name, n in facts:
            print('info: %s: %d' % (name, n))
    elif len(argv) >= 3 and argv[1] == 'log':
        rows = []
        for path in argv[2:]:
            with open(path, encoding='utf-8', errors='replace') as fh:
                rows += [('%s: %s' % (os.path.basename(os.path.dirname(path)) + '/' + os.path.basename(path), k), 0, n)
                         for k, n in sorted(scan_log(fh.read()).items())]
    else:
        sys.stderr.write(__doc__)
        return 2
    failed = False
    for name, want, got in rows:
        ok = want == got
        failed |= not ok
        print('%s: %s: want %s got %s' % ('ok' if ok else 'FAIL', name, want, got))
    return 1 if failed else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv))
