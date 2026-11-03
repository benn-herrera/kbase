package index

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// Finding is one verify failure: the check that found it, the kb-root-relative
// file and line it is about where it is about one, what it found, and the op
// that clears it where one does; where none does, the detail guides the
// user's prose edit or git work.
type Finding struct {
	Check, Path string
	Line        int
	Detail      string
	Remedy      string
}

// UpdateFormatRemedy is the command that converts a KB in an older format,
// and so clears its index-freshness finding and a write refused while its
// conversion is pending.
var UpdateFormatRemedy = argmark.Call("update-format", nil)

// EmptyBatchRemedy is the write that clears a register-freshness finding: an
// empty batch writes no record and re-derives every summary page.
var EmptyBatchRemedy = "an empty write batch, " + argmark.Call("write", map[string]any{"entry": []any{}}) +
	", which re-derives every summary page"

// CheckIndex is the refusal class of a derived index a verb cannot read, and
// of a write while a conversion is pending.
const CheckIndex = kb.CheckIndex

// Item is the finding as a result item.
func (f Finding) Item() result.Item {
	return result.Item{Check: f.Check, Path: f.Path, Line: f.Line, Detail: f.Detail, Remedy: f.Remedy}
}

// PendingConversion is the refusal of a write over src while its conversion
// is pending — src was read at an older format — and nil where src was read at
// the current one. Only update-format writes over such a KB.
func PendingConversion(src *kb.Source) error {
	if !src.Migrated() {
		return nil
	}
	return result.Refusal{{Check: CheckIndex, Path: src.KBPath(kb.EntryPointFile), Remedy: UpdateFormatRemedy,
		Detail: fmt.Sprintf("the KB's format is %s and kbase writes %s; no write but update-format runs while the conversion is pending",
			src.Version(), kb.FormatVersion)}}
}

// Items is findings as result items.
func Items(findings []Finding) []result.Item {
	items := make([]result.Item, len(findings))
	for i, f := range findings {
		items[i] = f.Item()
	}
	return items
}

// The metadata gate's checks, besides register freshness: a frontmatter block where kbase reads none, a claim, analysis or
// experiment that carries other than one statement marker, and an axiom record
// and axioms.md's headings that disagree.
const (
	CheckFrontmatterPresence = "frontmatter"
	CheckStatementMarker     = "statement-marker"
	CheckAxiomHeading        = "axiom-heading"
)

// frontmatterRemedy clears a frontmatter block on a document kbase reads none
// on.
const frontmatterRemedy = "remove the block: kbase reads no frontmatter here"

// Verify is the standard check of the KB src reads — the dead-link and
// unknown-id gate over kb-root/, the bare-id check, then the metadata gate —
// and returns every finding. The error is a failure to read the KB, or a
// refusal: the symlink refusal of a document that is a link, or the strict
// load's refusal of the records. Never a finding.
func Verify(src *kb.Source) ([]Finding, error) {
	var all []Finding
	for _, gate := range []func(*kb.Source) ([]Finding, error){LinkFindings, BareIDFindings, MetadataFindings} {
		found, err := gate(src)
		if err != nil {
			if r, ok := symlinkRefusal(err); ok {
				return nil, r
			}
			return nil, err
		}
		all = append(all, found...)
	}
	return all, nil
}

// CheckDemoted is the check a verify finding for a demoted edge names.
const CheckDemoted = kb.RelationDemoted

// DemotedFindings is every demoted record of the KB, each an item naming the
// edge and the relation it replaced, about the register page its target is
// rendered on, or about no file where its target has no host: structure the KB
// carries, not a fault.
func DemotedFindings(src *kb.Source) ([]result.Item, error) {
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	items := []result.Item{}
	for _, e := range x.g.Demoted {
		items = append(items, result.Item{Check: CheckDemoted, Path: x.rows.RegisterPages[e.Target],
			Detail: fmt.Sprintf("%s → %s, demoted from %s: cut by cycle breaking", e.Source, e.Target, e.ReplacedRelation)})
	}
	return items, nil
}

// CheckBareID names the fault of a node id standing bare in prose, where a
// citation of the node belongs.
const CheckBareID = "bare-id"

// bareIDRemedy is the write batch whose add-citation entry clears the bare-id
// fault of id on the 1-based line of document: the id as snippet, which
// writes the naming form.
func bareIDRemedy(document string, line int, id string) string {
	entry := map[string]any{"operation": "add-citation", "document": document, "line": line, "id": id, "snippet": id}
	return "cite the node: " + argmark.Call("write", map[string]any{"entry": []map[string]any{entry}}) +
		", which replaces the id with its naming-form citation"
}

// BareIDFindings is a finding per id of a node the records carry, of any
// kind, standing in the prose of a document under kb-root/ outside code,
// statement markers and link titles and destinations; the register pages,
// which refresh renders, are not read. A KB state Derive refuses is its refusal.
func BareIDFindings(src *kb.Source) ([]Finding, error) {
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	known := GraphKinds(x.g)
	pages := x.pagePositions()
	files, err := kb.MarkdownFiles(src)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, file := range files {
		rel, err := filepath.Rel(src.Root(), file)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if slices.Contains(pages, rel) {
			continue
		}
		text, err := src.ReadText(file)
		if err != nil {
			return nil, err
		}
		for n, line := range kb.SplitLines(kb.StripCodeSplitLines(text)) {
			for _, tok := range kb.BareIDs(line) {
				if _, ok := known[tok.ID]; !ok {
					continue
				}
				out = append(out, Finding{Check: CheckBareID, Path: rel, Line: n + 1, Remedy: bareIDRemedy(rel, n+1, tok.ID),
					Detail: fmt.Sprintf("%s:%d: %s stands bare in prose, and prose refers to a node only by citing it", rel, n+1, tok.ID)})
			}
		}
	}
	return out, nil
}

// CheckIndexFreshness names the finding for a KB loaded at an older format,
// which update-format rewrites in the current one.
const CheckIndexFreshness = "index-freshness"

// MetadataFindings is the metadata gate: a frontmatter block on any document,
// a KB loaded at an older format, every statement-marker fault, every
// disagreement between the axiom records and axioms.md's headings, and every
// register page that does not stand as kbase renders it. A kb-root the
// precheck refuses is that refusal's one finding per item; a load that fails
// is Derive's refusal.
func MetadataFindings(src *kb.Source) ([]Finding, error) {
	if err := Precheck(src); err != nil {
		var r result.Refusal
		if errors.As(err, &r) {
			var out []Finding
			for _, it := range r {
				out = append(out, Finding{Check: checkMetadata, Path: it.Path, Detail: it.Detail})
			}
			return out, nil
		}
		return nil, err
	}
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	out, err := frontmatterFindings(src)
	if err != nil {
		return nil, err
	}
	if src.Migrated() {
		out = append(out, Finding{Check: CheckIndexFreshness, Path: kb.IndexDir, Remedy: UpdateFormatRemedy,
			Detail: fmt.Sprintf("the KB's format is %s and kbase writes %s; update-format rewrites the KB in %s",
				src.Version(), kb.FormatVersion, kb.FormatVersion)})
	}
	out = append(out, markerFindings(x.rows.HostFaults)...)
	axioms, err := axiomHeadingFindings(src, x.g.Axioms)
	if err != nil {
		return nil, err
	}
	out = append(out, axioms...)
	pages, err := RegisterFreshness(src)
	if err != nil {
		return nil, err
	}
	return append(out, pages...), nil
}

// frontmatterFindings is a finding per document that opens with a frontmatter
// block: no document carries one, the entry point and its superseded stamp
// among them.
func frontmatterFindings(src *kb.Source) ([]Finding, error) {
	docs, err := kb.Documents(src)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, rel := range docs {
		text, err := src.ReadText(src.KBPath(rel))
		if err != nil {
			return nil, err
		}
		if kb.FindFrontmatter(text) != nil {
			out = append(out, Finding{Check: CheckFrontmatterPresence, Path: rel, Line: 1, Remedy: frontmatterRemedy,
				Detail: rel + " opens with a frontmatter block, and no document carries one"})
		}
	}
	return out, nil
}

// markerFindings is a finding per node that carries no statement marker, and
// per node that carries several, about the first, its host's.
func markerFindings(faults []kb.HostFault) []Finding {
	var out []Finding
	for _, f := range faults {
		if len(f.Markers) == 0 {
			out = append(out, Finding{Check: CheckStatementMarker, Detail: f.ID + " carries no statement marker, so no leaf hosts it"})
			continue
		}
		sites := make([]string, len(f.Markers))
		for i, m := range f.Markers {
			sites[i] = fmt.Sprintf("%s:%d", m.Path, m.Line+1)
		}
		out = append(out, Finding{Check: CheckStatementMarker, Path: f.Markers[0].Path, Line: f.Markers[0].Line + 1,
			Detail: fmt.Sprintf("%s carries %d statement markers, at %s; it carries one, and the first is its host's",
				f.ID, len(f.Markers), strings.Join(sites, ", "))})
	}
	return out
}

// axiomHeadingFindings is a finding per axiom record that has no heading in
// axioms.md carrying its marker, or more than one; per marked heading whose
// text is not its record's label and title; and per marked heading naming no
// axiom record. An unmarked heading is the author's prose.
func axiomHeadingFindings(src *kb.Source, axioms []kb.AxiomNode) ([]Finding, error) {
	var headings []kb.AxiomHeading
	if p := src.KBPath(kb.AxiomsFile); src.IsFile(p) {
		text, err := src.ReadText(p)
		if err != nil {
			return nil, err
		}
		headings = kb.AxiomHeadings(text)
	}
	marked := map[string][]kb.AxiomHeading{}
	for _, h := range headings {
		if h.ID != "" {
			marked[h.ID] = append(marked[h.ID], h)
		}
	}
	var out []Finding
	recorded := map[string]bool{}
	for _, n := range axioms {
		recorded[n.ID] = true
		hs := marked[n.ID]
		switch {
		case len(hs) == 0:
			out = append(out, Finding{Check: CheckAxiomHeading, Path: kb.AxiomsFile,
				Detail: fmt.Sprintf("%s has no heading in %s carrying its marker; its heading reads %q",
					n.ID, kb.AxiomsFile, kb.AxiomHeadingLine(n.Label, n.Title, n.ID))})
		case len(hs) > 1:
			lines := make([]string, len(hs))
			for i, h := range hs {
				lines[i] = strconv.Itoa(h.Line)
			}
			out = append(out, Finding{Check: CheckAxiomHeading, Path: kb.AxiomsFile, Line: hs[0].Line,
				Detail: fmt.Sprintf("%s's marker ends %d headings of %s, on lines %s; keep it on one heading and remove it from the others",
					n.ID, len(hs), kb.AxiomsFile, strings.Join(lines, ", "))})
		}
		for _, h := range hs {
			if want := kb.AxiomHeadingText(n.Label, n.Title); h.Text != want {
				out = append(out, Finding{Check: CheckAxiomHeading, Path: kb.AxiomsFile, Line: h.Line,
					Detail: fmt.Sprintf("%s:%d: the heading of %s reads %q, and its record's label and title make it %q",
						kb.AxiomsFile, h.Line, n.ID, h.Text, want)})
			}
		}
	}
	for _, h := range headings {
		if h.ID != "" && !recorded[h.ID] {
			out = append(out, Finding{Check: CheckAxiomHeading, Path: kb.AxiomsFile, Line: h.Line,
				Detail: fmt.Sprintf("%s:%d: the heading %q carries the marker of %s, and no axiom record has that id",
					kb.AxiomsFile, h.Line, h.Text, h.ID)})
		}
	}
	return out, nil
}
