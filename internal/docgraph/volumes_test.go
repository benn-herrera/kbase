package docgraph

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
	"kbase/internal/records"
)

func TestJoinRecords(t *testing.T) {
	block := func(doc string, order int) records.Record {
		return records.Record{Kind: records.KindBlock, Order: order, Document: doc}
	}
	ref := func(doc string, order, within int) records.Record {
		return records.Record{Kind: records.KindReference, Order: order, Document: doc, Within: within}
	}
	got := joinRecords([][]records.Record{
		{block("second/x.md", 1), ref("second/x.md", 2, 1), ref("second/x.md", 3, 0)},
		{block("first/y.md", 1), ref("first/y.md", 2, 1)},
	})
	want := []records.Record{
		block("first/y.md", 1), ref("first/y.md", 2, 1),
		block("second/x.md", 3), ref("second/x.md", 4, 3), ref("second/x.md", 5, 0),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("joinRecords = %+v\nwant %+v", got, want)
	}
}

// twoVolumes are two small papers, each its own volume root in its own
// directory, each with a labelled block a reference inside another block
// points at.
var twoVolumes = map[string]string{
	"one/one.tex": `\documentclass{article}
\newtheorem{theorem}{Theorem}
\newtheorem{lemma}{Lemma}
\title{First Paper}
\begin{document}
\maketitle
\section{Results}
\begin{lemma}\label{lem:a}
Every widget is a gadget.
\end{lemma}
\begin{theorem}\label{thm:a}
By Lemma~\ref{lem:a}, every gadget is a widget.
\end{theorem}
\end{document}
`,
	"two/two.tex": `\documentclass{article}
\newtheorem{theorem}{Theorem}
\title{Second Paper}
\begin{document}
\maketitle
\section{Setting}
Some prose.
\section{Claims}
\begin{theorem}\label{thm:b}
Every gizmo is a gadget, as Section~\ref{sec:none} does not say.
\end{theorem}
\end{document}
`,
}

func TestBuildTwoVolumesIntoOneTree(t *testing.T) {
	if _, err := pandoc.Preflight(context.Background()); err != nil {
		t.Skipf("pandoc is not usable here: %v", err)
	}
	dir := t.TempDir()
	var roots []string
	for _, name := range []string{"one/one.tex", "two/two.tex"} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(twoVolumes[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, p)
	}
	kbRoot := filepath.Join(dir, "kb-root")
	report, err := Build(context.Background(), log.Discard(), Options{VolumeRoots: roots, KBRoot: kbRoot})
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed() {
		t.Errorf("the build's checks failed: %+v", report.Findings)
	}
	entry, err := os.ReadFile(filepath.Join(kbRoot, "entry-point.md"))
	if want := "# Knowledge Base\n\n- [First Paper](first-paper/index.md)\n- [Second Paper](second-paper/index.md)\n"; err != nil || string(entry) != want {
		t.Errorf("entry-point.md = %q, %v; want every volume listed in the order given, %q", entry, err, want)
	}

	volumes := map[string]int{}
	orders := map[int]records.Record{}
	for i, r := range report.Records {
		if r.Order != i+1 {
			t.Errorf("record %d has order %d; want the records numbered as the tree is read", i, r.Order)
		}
		if i > 0 && r.Document < report.Records[i-1].Document {
			t.Errorf("record %d (%s) follows %s; want documents by path", i, r.Document, report.Records[i-1].Document)
		}
		volume, _, _ := strings.Cut(r.Document, "/")
		volumes[volume]++
		orders[r.Order] = r
	}
	if volumes["first-paper"] == 0 || volumes["second-paper"] == 0 || len(volumes) != 2 {
		t.Errorf("records by volume directory = %v; want both volumes and nothing else", volumes)
	}
	within := 0
	for _, r := range report.Records {
		if r.Within == 0 {
			continue
		}
		within++
		if b := orders[r.Within]; b.Kind != records.KindBlock || b.Document != r.Document {
			t.Errorf("record %d is within %d, which is %+v; want a block of the same document", r.Order, r.Within, b)
		}
	}
	if within < 2 {
		t.Errorf("%d records sit within a block; want each volume's reference inside its theorem", within)
	}

	var censused []string
	for _, c := range report.Censuses {
		censused = append(censused, c.Volume)
	}
	if !slices.Contains(censused, roots[0]) || !slices.Contains(censused, roots[1]) {
		t.Errorf("censuses name volumes %q; want both roots", censused)
	}
}
