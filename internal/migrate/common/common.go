// Package common holds Files, the migrate tree's one declaration that two or
// more of its packages use and no format defines.
package common

// Files is a KB's metadata files by repository-relative slash path: kb-root/…
// and the build records beside it.
type Files map[string][]byte
