package export

import "fmt"

// Finding is one line of the export report: the form it was raised for, the
// source path it was found in, and the reason.
type Finding struct {
	Form   string `json:"form" yaml:"form"`
	Path   string `json:"path" yaml:"path"`
	Reason string `json:"reason" yaml:"reason"`
}

// Report accumulates the export's findings, in the order they were raised. Its
// zero value is ready to use.
type Report struct {
	findings []Finding
}

// Add records one finding: a form, the source path it was found in, and a
// reason, formatted from a format string and its arguments.
func (r *Report) Add(form, srcPath, reason string, args ...any) {
	r.findings = append(r.findings, Finding{Form: form, Path: srcPath, Reason: fmt.Sprintf(reason, args...)})
}

// Findings is every finding, in the order it was raised.
func (r *Report) Findings() []Finding { return r.findings }

// Empty is whether nothing was reported.
func (r *Report) Empty() bool { return len(r.findings) == 0 }
