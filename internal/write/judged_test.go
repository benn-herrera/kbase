package write

import (
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/index"
	"kbase/internal/kb"
)

// statementNow is node id's statement in the KB at root as it stands: the
// text its host leaf gives it, the derivation placing the host and its lifts
// edges, a cycle among them included; "" where no leaf hosts it.
func statementNow(t *testing.T, root, id string) string {
	t.Helper()
	rows, err := index.DeriveOverCycles(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	host, ok := rows.Hosts[id]
	if !ok {
		return ""
	}
	text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(host)))
	if err != nil {
		t.Fatal(err)
	}
	return kb.Statement(string(text), id, func(analysis, claim string) bool {
		for _, e := range rows.Edges {
			if e.Relation == kb.RelationLifts && e.Source == analysis && e.Target == claim {
				return true
			}
		}
		return false
	})
}

// nodeJudgedNow is the judged hash a rigor set now on node id records over the
// KB at root.
func nodeJudgedNow(t *testing.T, root, id string) kb.JudgedHash {
	t.Helper()
	kind, _ := kb.NodeIDKind(id)
	h, err := asks.NodeJudgedHash(kind, statementNow(t, root, id))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// edgeJudgedNow is the judged hash a bearing set now on a scoring edge from
// source to target records over the KB at root.
func edgeJudgedNow(t *testing.T, root, source, target string) kb.JudgedHash {
	t.Helper()
	h, err := asks.EdgeJudgedHash(statementNow(t, root, source), statementNow(t, root, target))
	if err != nil {
		t.Fatal(err)
	}
	return h
}
