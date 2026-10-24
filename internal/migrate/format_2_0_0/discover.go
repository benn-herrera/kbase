package format_2_0_0

import (
	"fmt"
	"strings"
)

// AnnotationFault is one malformed register bullet: the list field it was read
// from, the file and physical line it opens on, and what the reader refused.
type AnnotationFault struct {
	Field   string
	Path    string
	Line    int
	Allowed string
	Remedy  string
	Detail  string
}

// AnnotationError is every annotation fault one load found, in register order.
type AnnotationError struct{ Faults []AnnotationFault }

func (e *AnnotationError) Error() string {
	parts := make([]string, len(e.Faults))
	for i, f := range e.Faults {
		parts[i] = fmt.Sprintf("%s:%d: %s: %s", f.Path, f.Line, f.Field, f.Detail)
	}
	return strings.Join(parts, "; ")
}

// IDRecord is where one authored node id lives: the register keying its
// canonical entry and the document hosting its body, each "" where none does.
type IDRecord struct {
	Kind, RegisterPath, HostingLeaf string
}
