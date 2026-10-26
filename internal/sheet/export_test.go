package sheet

import "kbase/internal/kb"

// Composed is one sheet as the emitter composes it: its path and its DOT,
// with or without straight-line splines.
type Composed struct {
	Path string
	DOT  func(lineSplines bool) string
}

// Compose is every sheet Render draws from the rows, in Render's order, as DOT.
func Compose(src *kb.Source, nodes []kb.NodeRow, edges []kb.EdgeRow, cites []kb.CiteRow) ([]Composed, error) {
	g, err := load(src, nodes, edges, cites)
	if err != nil {
		return nil, err
	}
	var out []Composed
	for _, s := range g.sheets() {
		out = append(out, Composed{Path: s.path, DOT: s.dot})
	}
	return out, nil
}

// RenderSVG is renderSVG.
var RenderSVG = renderSVG
