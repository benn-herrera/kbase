package from30to40

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format30"
	"kbase/internal/formats/format40"
)

// oldIDTokenRE is a node id of any kind as 3.0.0 prose spells it: an axiom's
// label read to its last letter or digit, so a sentence's full stop is not
// taken for part of it.
var oldIDTokenRE = regexp.MustCompile(format30.IDBody() + `|` + format30.AxiomPrefix +
	`-[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?|` + format30.WorkIDPattern)

// derivedID is the nth derivation of the minted id of prefix from the 3.0.0
// id from: its body is read off the SHA-256 digest of from, and from the
// first re-derivation on of from, a NUL byte and n.
func derivedID(prefix, from string, n int) string {
	seed := from
	if n > 0 {
		seed += "\x00" + strconv.Itoa(n)
	}
	sum := sha256.Sum256([]byte(seed))
	v := binary.BigEndian.Uint64(sum[:8])
	body := make([]byte, format40.IDBodyLen)
	for i := range body {
		body[i] = format40.IDBodyAlphabet[v%uint64(len(format40.IDBodyAlphabet))]
		v /= uint64(len(format40.IDBodyAlphabet))
	}
	return prefix + "-" + string(body)
}

// mintIDs gives each 3.0.0 id of olds the minted id of prefix derive derives
// from it, in out: the ids in ascending order, each taking its first
// derivation that neither an earlier one took nor any 3.0.0 id of olds is, so
// no minted id reads as a 3.0.0 id.
func mintIDs(prefix string, olds []string, out map[string]string, derive func(prefix, from string, n int) string) {
	olds = slices.Sorted(slices.Values(olds))
	taken := map[string]bool{}
	for _, id := range olds {
		taken[id] = true
	}
	for _, id := range out {
		taken[id] = true
	}
	for _, from := range olds {
		for n := 0; ; n++ {
			if id := derive(prefix, from, n); !taken[id] {
				taken[id], out[from] = true, id
				break
			}
		}
	}
}

// mintedIDs is each work's and axiom's minted id by its 3.0.0 id: derived for
// the records old holds, and read off the records current, a record file
// already in the 4.0.0 form, holds, its 3.0.0 id its prefix and label.
func mintedIDs(old format30.Graph, current format40.Graph) map[string]string {
	out := map[string]string{}
	for _, n := range current.Works {
		out[format30.WorkPrefix+"-"+n.Label] = n.ID
	}
	for _, n := range current.Axioms {
		out[format30.AxiomPrefix+"-"+n.Label] = n.ID
	}
	var works, axioms []string
	for _, n := range old.Works {
		works = append(works, n.ID)
	}
	for _, n := range old.Axioms {
		axioms = append(axioms, n.ID)
	}
	mintIDs(format40.WorkPrefix, works, out, derivedID)
	mintIDs(format40.AxiomPrefix, axioms, out, derivedID)
	return out
}

// mintedWarnings is a warning per work and axiom of old, naming its 3.0.0 id
// and the id minted for it.
func mintedWarnings(old format30.Graph, minted map[string]string) []common.Warning {
	var out []common.Warning
	add := func(kind, from string) {
		to := minted[from]
		out = append(out, common.Warning{Path: nodesPath, List: common.ListIDs, ID: from, Text: to,
			Detail: fmt.Sprintf("the %s %s is now %s, a minted id, and %q is its label",
				kind, from, to, strings.SplitN(from, "-", 2)[1])})
	}
	for _, n := range old.Works {
		add(format40.NodeKindWork, n.ID)
	}
	for _, n := range old.Axioms {
		add(format40.NodeKindAxiom, n.ID)
	}
	return out
}

// renamed is id as 4.0.0 names it: its minted id where it is a 3.0.0 work's
// or axiom's id, else id itself.
func renamed(minted map[string]string, id string) string {
	if to, ok := minted[id]; ok {
		return to
	}
	return id
}

// renameIDs is s with every 3.0.0 work or axiom id standing bare in it as
// 4.0.0 names it.
func renameIDs(minted map[string]string, s string) string {
	tokens := format40.BareTokens(s, oldIDTokenRE)
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		s = s[:t.Start] + renamed(minted, t.ID) + s[t.End:]
	}
	return s
}
