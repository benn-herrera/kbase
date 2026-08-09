// Package version is the single source of the appliance's version identity.
// The provenance receipt (SPEC.md §7) derives from it: app version implies
// the embedded prompt set.
package version

// Current is the appliance version — the string --version prints and every
// provenance stamp carries.
const Current = "0.0.1-dev"
