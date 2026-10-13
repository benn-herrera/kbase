package write

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestDropClaimEdges drops a claim's outgoing edge bullets per class: both
// classes when edge-class is omitted, one when named, an absent list left as
// it stands, a demoted edge-class refused naming its producer.
func TestDropClaimEdges(t *testing.T) {
	seed := func(t *testing.T) (string, string) {
		t.Helper()
		root := seedKB(t)
		id := insertClaim(t, root, "Second Result",
			"  depends-on:\n  - id: "+seedClaim+"\n    context: the bound it rests on\n")
		if res := run(t, root, "add-depends-on", "entry:\n- id: "+id+"\n  references:\n  - id: "+seedClaim+"\n", false); res.Outcome != result.Done {
			t.Fatalf("add references: %+v", res)
		}
		return root, id
	}
	register := func(t *testing.T, root string) string { return snapshot(t, root)[kb.RegisterFile] }

	root, id := seed(t)
	res := run(t, root, "drop-claim-edges", "entry:\n- id: "+id+"\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop both classes: %+v", res)
	}
	text := register(t, root)
	if strings.Contains(text, "- depends-on:") || strings.Contains(text, "- references:") {
		t.Errorf("an edge list survived:\n%s", text)
	}
	if !strings.Contains(text, "## Seed Result") || !strings.Contains(text, "rationale: Seeded.") {
		t.Errorf("the seed entry moved:\n%s", text)
	}

	before := snapshot(t, root)
	again := run(t, root, "drop-claim-edges", "entry:\n- id: "+id+"\n", false)
	if again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("again: %+v, want unchanged, byte-identical", again)
	}

	root, id = seed(t)
	res = run(t, root, "drop-claim-edges", "entry:\n- id: "+id+"\n  edge-class: references\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop references: %+v", res)
	}
	text = register(t, root)
	if !strings.Contains(text, "- depends-on:\n  - "+seedClaim+" — Seed Result") {
		t.Errorf("the depends-on list did not survive:\n%s", text)
	}
	if strings.Contains(text, "- references:\n  - "+seedClaim+" — Seed Result") {
		t.Errorf("the references list did not go:\n%s", text)
	}

	res = run(t, root, "drop-claim-edges", "entry:\n- id: "+id+"\n  edge-class: demoted\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 ||
		!strings.Contains(res.Refusals[0].Detail, "resolve-demoted") {
		t.Errorf("edge-class demoted: %+v, want refused naming resolve-demoted", res)
	}
}

// TestDropClaimEdgesEmptyList drops a references header carrying no bullets:
// the header goes with the first call, and a re-issue is unchanged and
// byte-identical. No writer produces a bulletless header, so the fixture
// stages one by hand in the seed entry's Quality section.
func TestDropClaimEdgesEmptyList(t *testing.T) {
	root := seedKB(t)
	reg := filepath.Join(root, kb.RegisterFile)
	text, err := os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	staged := strings.Replace(string(text), "- rationale: Seeded.",
		"- references:\n- rationale: Seeded.", 1)
	if staged == string(text) {
		t.Fatal("no rationale line to stage the empty header beside")
	}
	if err := os.WriteFile(reg, []byte(staged), 0o644); err != nil {
		t.Fatal(err)
	}

	res := run(t, root, "drop-claim-edges", "entry:\n- id: "+seedClaim+"\n  edge-class: references\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop empty references list: %+v", res)
	}
	text, err = os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), "- "+kb.RelationReferences+":") {
		t.Errorf("the bulletless references header survived:\n%s", text)
	}

	before := snapshot(t, root)
	again := run(t, root, "drop-claim-edges", "entry:\n- id: "+seedClaim+"\n  edge-class: references\n", false)
	if again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("again: %+v, want unchanged, byte-identical", again)
	}
}

// TestDropInboundEdges drops every entry's edge bullets naming one claim:
// the naming entries lose their lists, the rest of each register and the
// unnamed entries stand byte-identical, and a claim with no inbound edges
// writes nothing.
func TestDropInboundEdges(t *testing.T) {
	root := seedKB(t)
	b := insertClaim(t, root, "Bound Result",
		"  depends-on:\n  - id: "+seedClaim+"\n    context: the bound it rests on\n")
	insertClaim(t, root, "Dependent Result",
		"  depends-on:\n  - id: "+b+"\n    context: rests on the bound result\n")
	d := insertClaim(t, root, "Isolated Result", "")

	res := run(t, root, "drop-inbound-edges", "entry:\n- id: "+b+"\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop inbound: %+v", res)
	}
	text := snapshot(t, root)[kb.RegisterFile]
	if strings.Contains(text, "- "+kb.IndexDependsOn+":\n  - "+b+" — Bound Result") {
		t.Errorf("C's depends-on bullet naming B survived:\n%s", text)
	}
	if !strings.Contains(text, "- "+kb.IndexDependsOn+":\n  - "+seedClaim+" — Seed Result") {
		t.Errorf("B's own depends-on list did not survive:\n%s", text)
	}
	if !strings.Contains(text, "## Seed Result") || !strings.Contains(text, "rationale: Seeded.") {
		t.Errorf("the seed entry moved:\n%s", text)
	}

	before := snapshot(t, root)
	again := run(t, root, "drop-inbound-edges", "entry:\n- id: "+b+"\n", false)
	if again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("again: %+v, want unchanged, byte-identical", again)
	}

	res = run(t, root, "drop-inbound-edges", "entry:\n- id: "+d+"\n", false)
	if res.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("no inbound: %+v, want unchanged, byte-identical", res)
	}
}

// seedSharedLeaf seeds a KB whose leaf claims the seed claim and a second,
// "Bound Result", inserted with extra and marked in the leaf's prose; it
// returns kb-root and the second claim's id.
func seedSharedLeaf(t *testing.T, extra string) (string, string) {
	t.Helper()
	root := seedKB(t)
	b := insertClaim(t, root, "Bound Result", extra)
	if res := run(t, root, "set-frontmatter", "entry:\n- document: leaf.md\n  kind: leaf\n  claims: ["+seedClaim+", "+b+"]\n", false); res.Outcome != result.Done {
		t.Fatalf("set-frontmatter: %+v", res)
	}
	if res := run(t, root, "mark-claim-in-leaf", "entry:\n- document: leaf.md\n  id: "+b+"\n  locator: a second paragraph names a bound\n", false); res.Outcome != result.Done {
		t.Fatalf("mark-claim-in-leaf: %+v", res)
	}
	return root, b
}

// TestDropClaimNode deletes a claim node whole: its register entry block, its
// marker and its claims entry go from the leaf claiming it, the rest of each
// file standing. An entry still naming the claim refuses, naming the entry
// and the two edge ops; a leaf claimed by it alone refuses, an empty claims
// list being no-claim's decision; a re-issue takes the resolve refusal, the
// id having gone with the entry.
func TestDropClaimNode(t *testing.T) {
	seed := func(t *testing.T) (string, string) {
		t.Helper()
		return seedSharedLeaf(t, "  depends-on:\n  - id: "+seedClaim+"\n    context: the bound it rests on\n")
	}

	root, b := seed(t)
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop: %+v", res)
	}
	register := snapshot(t, root)[kb.RegisterFile]
	if strings.Contains(register, b) || strings.Contains(register, "## Bound Result") {
		t.Errorf("B's entry survived:\n%s", register)
	}
	if !strings.Contains(register, "## Seed Result") || !strings.Contains(register, "rationale: Seeded.") {
		t.Errorf("the seed entry moved:\n%s", register)
	}
	leaf := snapshot(t, root)["leaf.md"]
	if strings.Contains(leaf, renderTier2Marker(b)) || strings.Contains(leaf, b) {
		t.Errorf("B's marker or claims entry survived:\n%s", leaf)
	}
	if !strings.Contains(leaf, "claims: ["+seedClaim+"]") || !strings.Contains(leaf, "A second paragraph names a bound.") {
		t.Errorf("the leaf lost more than B:\n%s", leaf)
	}

	before := snapshot(t, root)
	again := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n", false)
	if again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("again: %+v, want unchanged, byte-identical", again)
	}

	root, b = seed(t)
	res = run(t, root, "drop-claim-node", "entry:\n- id: "+seedClaim+"\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 ||
		!strings.Contains(res.Refusals[0].Detail, b) ||
		!strings.Contains(res.Refusals[0].Detail, "drop-claim-edges") ||
		!strings.Contains(res.Refusals[0].Detail, "drop-inbound-edges") {
		t.Errorf("named inbound: %+v, want refused naming %s and the two edge ops", res, b)
	}

	root = seedKB(t)
	solo := insertClaim(t, root, "Solo Result", "")
	if res := run(t, root, "set-frontmatter", "entry:\n- document: leaf.md\n  kind: leaf\n  claims: ["+solo+"]\n", false); res.Outcome != result.Done {
		t.Fatalf("set-frontmatter solo: %+v", res)
	}
	res = run(t, root, "drop-claim-node", "entry:\n- id: "+solo+"\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 ||
		!strings.Contains(res.Refusals[0].Detail, "leaf.md") ||
		!strings.Contains(res.Refusals[0].Detail, "no-claim") {
		t.Errorf("solo claims: %+v, want refused naming leaf.md and the no-claim decision", res)
	}
}

// lineOf is the 1-based number of text's first line containing sub; the
// fixture failing where none does.
func lineOf(t *testing.T, text, sub string) int {
	t.Helper()
	for n, line := range strings.Split(text, "\n") {
		if strings.Contains(line, sub) {
			return n + 1
		}
	}
	t.Fatalf("no line contains %q:\n%s", sub, text)
	return 0
}

// refusedNaming is whether res is refused with an item whose detail names
// every one of subs.
func refusedNaming(res Result, subs ...string) bool {
	if res.Outcome != result.Refused {
		return false
	}
	for _, it := range res.Refusals {
		if !slices.ContainsFunc(subs, func(s string) bool { return !strings.Contains(it.Detail, s) }) {
			return true
		}
	}
	return false
}

// TestDropClaimNodeRestatedLeaf: a leaf hosting one claim, restated no-claim
// with the claim's marker still in its prose, loses the marker when the node
// drops, and the register loses the entry.
func TestDropClaimNodeRestatedLeaf(t *testing.T) {
	root := seedKB(t)
	if res := run(t, root, "set-frontmatter", "entry:\n- document: leaf.md\n  kind: leaf\n  no-claim: restated before the drop\n", false); res.Outcome != result.Done {
		t.Fatalf("set-frontmatter no-claim: %+v", res)
	}
	if !strings.Contains(snapshot(t, root)["leaf.md"], renderTier2Marker(seedClaim)) {
		t.Fatal("the restatement took the marker; the fixture needs it standing")
	}
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+seedClaim+"\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop: %+v, want done", res)
	}
	files := snapshot(t, root)
	if leaf := files["leaf.md"]; strings.Contains(leaf, renderTier2Marker(seedClaim)) {
		t.Errorf("the marker survived the restated leaf:\n%s", leaf)
	} else if !strings.Contains(leaf, "The first result holds here.") {
		t.Errorf("the leaf lost more than the marker:\n%s", leaf)
	}
	if register := files[kb.RegisterFile]; strings.Contains(register, seedClaim) {
		t.Errorf("the entry survived:\n%s", register)
	}
}

// TestDropClaimNodeDemotedRow: another entry's demoted row naming the node
// goes with the node, the rest of that entry byte-identical to before the
// row was planted.
func TestDropClaimNodeDemotedRow(t *testing.T) {
	root := seedKB(t)
	b := insertClaim(t, root, "Bound Result", "")
	y := insertClaim(t, root, "Holding Result", "  depends-on:\n  - id: "+seedClaim+"\n    context: the bound it rests on\n")
	entryY := func() string {
		t.Helper()
		register := snapshot(t, root)[kb.RegisterFile]
		at := strings.Index(register, "## Holding Result")
		if at < 0 {
			t.Fatalf("no entry for Y:\n%s", register)
		}
		return register[at:] // Y is the register's last entry
	}
	before := entryY()
	plant := Run(AddBuildEdges, Options{KBRoot: root, Values: []byte("entry:\n- id: " + y + "\n  " + kb.RelationDemoted + ":\n  - id: " + b + "\n    origin: cited\n")})
	if plant.Outcome != result.Done || !strings.Contains(entryY(), b) {
		t.Fatalf("plant the demoted row: %+v\n%s", plant, entryY())
	}
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n", false)
	if res.Outcome != result.Done {
		t.Fatalf("drop: %+v, want done", res)
	}
	if after := entryY(); after != before {
		t.Errorf("Y's entry is not as before the plant:\ngot:\n%s\nwant:\n%s", after, before)
	}
}

// TestDropClaimNodeBatchEmptiesLeaf: a batch naming every claim a leaf holds
// refuses the whole call, naming the leaf and the no-claim restatement.
func TestDropClaimNodeBatchEmptiesLeaf(t *testing.T) {
	root, b := seedSharedLeaf(t, "")
	before := snapshot(t, root)
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+seedClaim+"\n- id: "+b+"\n", false)
	if !refusedNaming(res, "leaf.md", "set-frontmatter", "no-claim") {
		t.Errorf("batch emptying leaf.md: %+v, want refused naming leaf.md and set-frontmatter's no-claim", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused batch changed kb-root")
	}
}

// TestDropClaimNodeSupportDependsOn: a support entry whose depends-on names
// the node refuses the drop, naming the bullet's path:line. The support
// stands in a register of its own, so the line is the same whether read
// before or after the node's entry leaves the claim register.
func TestDropClaimNodeSupportDependsOn(t *testing.T) {
	const supports = "supports/claim-quality.md"
	root := seedKB(t)
	b := insertClaim(t, root, "Bound Result", "")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(supports)), 0o755); err != nil {
		t.Fatal(err)
	}
	if res := run(t, root, "insert-support-entry", "entry:\n- register: "+supports+"\n  title: A Support\n  rigor: \"*pending*\"\n  rationale: Rests on the bound.\n  depends-on:\n  - id: "+b+"\n", true); res.Outcome != result.Done {
		t.Fatalf("insert support: %+v", res)
	}
	at := fmt.Sprintf("%s:%d", supports, lineOf(t, snapshot(t, root)[supports], b))
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n", false)
	if !refusedNaming(res, at, b) {
		t.Errorf("support depends-on: %+v, want refused naming %s and %s", res, at, b)
	}
}

// TestDropClaimNodeProseMention: a prose mention of the node in a leaf that
// does not claim it refuses the drop, naming the mention's path:line.
func TestDropClaimNodeProseMention(t *testing.T) {
	root := seedKB(t)
	b := insertClaim(t, root, "Bound Result", "")
	leafPath := filepath.Join(root, "leaf.md")
	text, err := os.ReadFile(leafPath)
	if err != nil {
		t.Fatal(err)
	}
	staged := strings.Replace(string(text), "The bound is tight in the limit.", "The bound is tight in the limit, as "+b+" shows.", 1)
	if staged == string(text) {
		t.Fatal("no sentence to stage the mention in")
	}
	if err := os.WriteFile(leafPath, []byte(staged), 0o644); err != nil {
		t.Fatal(err)
	}
	at := fmt.Sprintf("leaf.md:%d", lineOf(t, staged, b))
	res := run(t, root, "drop-claim-node", "entry:\n- id: "+b+"\n", false)
	if !refusedNaming(res, at, b) {
		t.Errorf("prose mention: %+v, want refused naming %s and %s", res, at, b)
	}
}
