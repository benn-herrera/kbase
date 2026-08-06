// Package version is the single source of the appliance's version identity.
// The provenance receipt (PROJECT.md §4) derives from it: app version implies
// the embedded prompt set.
package version

// Current is the appliance version.
const Current = "0.0.1-dev"

// Short returns the version string for --version output and provenance stamps.
func Short() string { return Current }
