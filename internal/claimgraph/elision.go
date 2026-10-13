package claimgraph

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/write"
)

// culledNoClaim is the no-claim reason a leaf takes when the co-linearity cull
// culls the claim it hosts alone.
const culledNoClaim = "This document's claim states nothing that another claim in the KB does not already state; that claim stands for both in the claim graph."

// soloClaimLeaf is a leaf whose claims list holds one claim alone, with its
// current frontmatter as a set-frontmatter values entry.
type soloClaimLeaf struct {
	Document string
	FM       map[string]any
}

// leafOf resolves a claim node to every leaf whose claims list holds the id
// alone, in path order, read fresh from t's kb-root on each call. A leaf whose
// frontmatter does not read is skipped; drop-claim-node names it if its claims
// list would empty.
func leafOf(t *Tree, lg log.Logger) func(id string) ([]soloClaimLeaf, error) {
	return func(id string) ([]soloClaimLeaf, error) {
		src := builtTree(t.Root)
		st, err := kb.Discover(src, lg)
		if err != nil {
			return nil, err
		}
		var out []soloClaimLeaf
		for _, leaf := range st.Leaves {
			if !slices.Equal(leaf.Claims, []string{id}) {
				continue
			}
			text, err := src.ReadText(src.KBPath(leaf.Path))
			if err != nil {
				continue
			}
			fm, err := write.FrontmatterValues(text, leaf.Path)
			if err != nil || fm == nil {
				continue
			}
			out = append(out, soloClaimLeaf{Document: leaf.Path, FM: fm})
		}
		return out, nil
	}
}

// IssueCulls issues, for each id in order, drop-claim-edges, drop-inbound-edges
// and drop-claim-node through run, and returns the outcomes. A run whose
// outcome is neither done nor unchanged stops the issue: no retry, no
// re-planning.
//
// A cull whose hosting leaf holds it alone would leave that leaf an empty
// claims list, which drop-claim-node refuses. Each leaf leafOf returns for the
// cull is restated as no-claim, with reason, through set-frontmatter, in the
// order returned, before the cull's drops; a refusal there stops the issue as
// any other.
func IssueCulls(ids []string, reason string, run func(op string, values []byte) write.Result, leafOf func(id string) ([]soloClaimLeaf, error)) ([]write.Result, error) {
	var out []write.Result
	for _, id := range ids {
		leaves, err := leafOf(id)
		if err != nil {
			return out, fmt.Errorf("resolving the leaves hosting %s: %w", id, err)
		}
		for _, leaf := range leaves {
			e := maps.Clone(leaf.FM)
			e["kind"] = kb.DocumentLeaf
			delete(e, "claims")
			e["no-claim"] = reason
			res, err := issue(run, "set-frontmatter", e)
			if err != nil {
				return out, fmt.Errorf("set-frontmatter %s over %s: %w", leaf.Document, id, err)
			}
			out = append(out, res)
		}
		for _, op := range []string{"drop-claim-edges", "drop-inbound-edges", "drop-claim-node"} {
			res, err := issue(run, op, map[string]any{"id": id})
			if err != nil {
				return out, fmt.Errorf("%s %s: %w", op, id, err)
			}
			out = append(out, res)
		}
	}
	return out, nil
}

// issue lands one op over one values entry, refusing an outcome the caller
// cannot proceed from.
func issue(run func(op string, values []byte) write.Result, op string, e map[string]any) (write.Result, error) {
	data, err := json.Marshal(map[string]any{"entry": []map[string]any{e}})
	if err != nil {
		return write.Result{}, err
	}
	res := run(op, data)
	switch res.Outcome {
	case result.Done, result.Unchanged:
		return res, nil
	default:
		return res, fmt.Errorf("%s ended %s: %s", op, res.Outcome, refusalText(res))
	}
}
