// Package sheet draws the KB's claim-graph sheets from its index: DOT text
// rendered to SVG by Graphviz dot. The root's full sheet is every node
// clustered by volume; where two or more volumes hold nodes, the root also
// carries one box per volume and each such volume a sheet of itself and every
// node one drawn edge from it. Without dot on PATH nothing is drawn, and
// Placeholder stands in for the root's full sheet.
package sheet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"

	"kbase/internal/kb"
)

// ErrNoDot is Render's error where Graphviz dot is not on PATH.
var ErrNoDot = errors.New("no Graphviz dot on PATH")

// Sheet is one drawn sheet: its kb-root-relative slash path and its SVG.
type Sheet struct {
	Path string
	SVG  []byte
}

// Render draws every sheet of the KB src reads from its .index/ and the build
// records beside kb-root: the root's full sheet first, then, where two or
// more volumes hold nodes, the digest and each volume's in slug order. It is
// ErrNoDot where dot is not on PATH.
func Render(src *kb.Source) ([]Sheet, error) {
	dot, err := exec.LookPath("dot")
	if err != nil {
		return nil, ErrNoDot
	}
	g, err := load(src)
	if err != nil {
		return nil, err
	}
	drawn := g.sheets()
	out := make([]Sheet, len(drawn))
	errs := make([]error, len(drawn))
	slots := make(chan struct{}, renderConcurrency)
	var wg sync.WaitGroup
	for i, s := range drawn {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer func() { <-slots; wg.Done() }()
			svg, err := renderSVG(dot, s.dot)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", s.path, err)
				return
			}
			out[i] = Sheet{Path: s.path, SVG: svg}
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return out, nil
}

// renderConcurrency bounds the dot processes Render runs at once.
const renderConcurrency = 4

func renderSVG(dot, text string) ([]byte, error) {
	cmd := exec.Command(dot, "-Tsvg")
	cmd.Stdin = strings.NewReader(text)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("dot -Tsvg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return fit(stdout.Bytes()), nil
}

var svgSizeRE = regexp.MustCompile(`<svg width="[^"]*" height="[^"]*"`)

// fit is svg with the root's fixed size replaced by the viewer's width, so
// its viewBox scales to the viewer.
func fit(svg []byte) []byte {
	loc := svgSizeRE.FindIndex(svg)
	if loc == nil {
		return svg
	}
	return slices.Concat(svg[:loc[0]], []byte(`<svg width="100%"`), svg[loc[1]:])
}

// Placeholder is the root's full sheet where dot is not on PATH: the lack, over the
// first 12 hex digits of the SHA-256 of the index's files concatenated in
// sorted path order.
func Placeholder(src *kb.Source) ([]byte, error) {
	files, err := src.DirFiles(src.KBPath(kb.IndexDir))
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	for _, p := range files {
		if !strings.HasSuffix(p, kb.IndexFileExt) {
			continue
		}
		data, err := src.ReadFile(p)
		if err != nil {
			return nil, err
		}
		h.Write(data)
	}
	digest := hex.EncodeToString(h.Sum(nil))[:12]
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="80" viewBox="0 0 320 80">
<text x="160" y="36" text-anchor="middle" font-family="sans-serif" font-size="18">%s</text>
<text x="160" y="60" text-anchor="middle" font-family="monospace" font-size="12">index sha256:%s</text>
</svg>
`, ErrNoDot, digest), nil
}
