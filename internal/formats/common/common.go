// Package common holds the formats tree's declarations that two or more of its
// packages use and no format defines.
package common

// Files is a KB's metadata files by repository-relative slash path: kb-root/…
// and the build records beside it.
type Files map[string][]byte

// Warning is one thing a conversion step dropped or changed that its output
// does not show: the repository-relative file and 1-based line it names (0
// where it names none), and one sentence.
type Warning struct {
	Path   string
	Line   int
	Detail string
}
