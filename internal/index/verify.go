package index

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// Finding is one verify failure: the check that found it, the kb-root-relative
// file and line it is about where it is about one, what it found, the command
// that clears it where one does, and whether a refresh clears it.
type Finding struct {
	Check, Path    string
	Line           int
	Detail         string
	Remedy         string
	RefreshFixable bool
}

// RefreshRemedy is the command that clears a refresh-fixable finding.
const RefreshRemedy = "kbase refresh"

// CheckIndex is the refusal class of a derived index a verb cannot read.
const CheckIndex = kb.CheckIndex

// Item is the finding as a result item.
func (f Finding) Item() result.Item {
	it := result.Item{Check: f.Check, Path: f.Path, Line: f.Line, Detail: f.Detail, Remedy: f.Remedy}
	if f.RefreshFixable {
		it.Remedy = RefreshRemedy
	}
	return it
}

// Items is findings as result items.
func Items(findings []Finding) []result.Item {
	items := make([]result.Item, len(findings))
	for i, f := range findings {
		items[i] = f.Item()
	}
	return items
}

// The metadata gate's checks, besides register freshness and the degraded
// load's: a frontmatter block where kbase reads none, a claim, analysis or
// experiment that carries other than one statement marker, and an axiom record
// and axioms.md's headings that disagree.
const (
	CheckFrontmatterPresence = "frontmatter"
	CheckStatementMarker     = "statement marker"
	CheckAxiomHeading        = "axiom heading"
)

// frontmatterRemedy clears a frontmatter block on a document kbase reads none
// on.
const frontmatterRemedy = "remove the block: kbase reads no frontmatter here"

// Verify is the standard check of the KB src reads — the dead-link and
// unknown-id gate over kb-root/, then the metadata gate — and returns every
// finding. The error is a failure to read the KB, or a refusal: the symlink
// refusal of a document that is a link, or the strict load's refusal of the
// records. Never a finding.
func Verify(src *kb.Source) ([]Finding, error) {
	links, err := LinkFindings(src)
	if err != nil {
		if r, ok := symlinkRefusal(err); ok {
			return nil, r
		}
		return nil, err
	}
	meta, err := MetadataFindings(src)
	if err != nil {
		if r, ok := symlinkRefusal(err); ok {
			return nil, r
		}
		return nil, err
	}
	return slices.Concat(links, meta), nil
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

// BuildVerify is the build-time check: Verify, then the citation gate, every
// gate run whatever the earlier found, findings in that order.
func BuildVerify(src *kb.Source) ([]Finding, error) {
	standard, err := Verify(src)
	if err != nil {
		return nil, err
	}
	cites, err := CitationFindings(src)
	if err != nil {
		return nil, err
	}
	return slices.Concat(standard, cites), nil
}

// CheckIndexFreshness names the finding for a KB loaded at an older format,
// which a refresh rewrites in the current one.
const CheckIndexFreshness = "index freshness"

// MetadataFindings is the metadata gate: a frontmatter block on any document,
// a KB loaded at an older format, every statement-marker fault, every
// disagreement between the axiom records and axioms.md's headings, every fault
// a degraded load resolved, and every register page that does not stand as
// refresh renders it. A kb-root the precheck refuses is that refusal's one
// finding per item; records the strict load refuses are its refusal.
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
		out = append(out, Finding{Check: CheckIndexFreshness, Path: kb.IndexDir, RefreshFixable: true,
			Detail: fmt.Sprintf("the KB's metadata format is %s and kbase writes %s; a refresh rewrites the KB in %s",
				src.Version(), kb.FormatVersion, kb.FormatVersion)})
	}
	out = append(out, markerFindings(x.rows.HostFaults)...)
	axioms, err := axiomHeadingFindings(src, x.g.Axioms)
	if err != nil {
		return nil, err
	}
	out = append(out, axioms...)
	for _, w := range x.rows.Degraded {
		out = append(out, Finding{Check: kb.CheckDegraded, Path: w.Path, Detail: w.Detail, RefreshFixable: true})
	}
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

// axiomHeadingFindings is a finding per axiom record axioms.md carries no
// heading of its label and title for, and per axiom heading of axioms.md that
// names no record of its label and title.
func axiomHeadingFindings(src *kb.Source, axioms []kb.AxiomNode) ([]Finding, error) {
	var headings []kb.AxiomHeading
	if p := src.KBPath(kb.AxiomsFile); src.IsFile(p) {
		text, err := src.ReadText(p)
		if err != nil {
			return nil, err
		}
		headings = kb.AxiomHeadings(text)
	}
	declared := map[[2]string]bool{}
	for _, h := range headings {
		declared[[2]string{h.Label, h.Title}] = true
	}
	recorded := map[[2]string]bool{}
	var out []Finding
	for _, n := range axioms {
		label := strings.TrimPrefix(n.ID, kb.AxiomPrefix+"-")
		recorded[[2]string{label, n.Title}] = true
		if !declared[[2]string{label, n.Title}] {
			out = append(out, Finding{Check: CheckAxiomHeading, Path: kb.AxiomsFile,
				Detail: fmt.Sprintf("%s, label %s and title %q, has no axiom heading in %s", n.ID, label, n.Title, kb.AxiomsFile)})
		}
	}
	for _, h := range headings {
		if !recorded[[2]string{h.Label, h.Title}] {
			out = append(out, Finding{Check: CheckAxiomHeading, Path: kb.AxiomsFile, Line: h.Line,
				Detail: fmt.Sprintf("%s:%d: the heading of label %s and title %q names no axiom record", kb.AxiomsFile, h.Line, h.Label, h.Title)})
		}
	}
	return out, nil
}
