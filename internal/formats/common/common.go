// Package common holds the formats tree's declarations that two or more of its
// packages use and no format defines.
package common

// Files is a KB's metadata files by repository-relative slash path: kb-root/…
// and the build records beside it.
type Files map[string][]byte

// Warning is one thing a conversion step dropped or changed that its output
// does not show: the repository-relative file and 1-based line it names (0
// where it names none), and one sentence. A warning a conversion result lists
// by kind also names that list, the node id it is about and the text it
// reports; each is "" on every other warning.
type Warning struct {
	Path   string
	Line   int
	Detail string
	List   string `yaml:"list,omitempty"`
	ID     string `yaml:"id,omitempty"`
	Text   string `yaml:"text,omitempty"`
}

// The lists a conversion result groups warnings under: each node id minted
// in place of an older one, its older id under ID and its minted one under
// Text; each bare node id converted into a citation, each link or bare id not
// resolved to one node without guessing, a dropped no-edge field's text, and a
// dropped strengthen-by field's.
const (
	ListIDs                 = "ids"
	ListBareIDs             = "bare-ids"
	ListUnresolvedCitations = "unresolved-citations"
	ListNoEdge              = "no-edge"
	ListStrengthenBy        = "strengthen-by"
)
