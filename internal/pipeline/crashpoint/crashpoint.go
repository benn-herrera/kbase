// Package crashpoint is the deterministic kill-point seam the resume design
// (ARCHITECTURE.md §12) tests against.
//
// Resume claims that a run killed at any moment leaves a state a later run
// can either prove valid or redo. That claim is only worth what it is tested
// at, and "kill the process at a random instant" tests nothing reproducibly.
// So the seam is explicit: every place a kill would be interesting names
// itself, and a test arms exactly one name and drives recovery against the
// on-disk state that results.
//
// # Production cost
//
// Unarmed — always, in production, since nothing but a test calls Arm — At
// is one atomic load and a return: no map lookup, no lock, no allocation. That is why the seam is a runtime opt-in rather than a
// //go:build-gated one: build-tagged code is excluded from the ordinary
// compile and rots silently, while a hook on the always-compiled path fails
// the build the moment a refactor breaks its call site.
//
// # Crash shape
//
// An armed At(name) panics with a *Crash sentinel; the harness recovers it,
// drops all in-memory state, and re-enters through the resume path — which
// models a process kill at that instruction. In-process limitation: Go runs
// deferred functions as the panic unwinds, so a cleanup defer between the
// crashpoint and the recover WILL run, where a real kill would leave its
// residue behind. A call site that must leave forensic residue therefore
// hands its staging to AtStaging, which runs it on the crossing that kills
// and before the panic (see the pre-rename point in the store's Put).
//
// # Coverage
//
// Every call site registers its name, so RegisteredNames enumerates the
// points a resume harness is expected to cover — a point with no scenario is
// then a mechanical finding rather than an oversight nobody notices.
//
// The armed set is process-global state, which is what a kill-point seam
// inherently is. It is fenced in its own package for that reason, and tests
// that Arm must not run in parallel with each other.
package crashpoint

import (
	"slices"
	"sync"
	"sync/atomic"
)

// Crash is the sentinel an armed crashpoint panics with. A harness recovers
// it and treats it as "the process died at Point".
type Crash struct {
	Point string
}

func (c *Crash) Error() string { return "crashpoint: simulated crash at " + c.Point }

var (
	mu sync.Mutex
	// armed maps an armed point to its remaining hit countdown: n means
	// "fire on the nth At from now". The countdown never drops below 1, so
	// once a point fires it keeps firing until it is disarmed; the default
	// single-hit behaviour is the degenerate n=1 case rather than a mode of
	// its own.
	armed = map[string]int{}
	// registry holds every Register'd name, for the coverage enumeration.
	registry = map[string]struct{}{}
	// anyArmed is the fast path: false means no point is armed, so a
	// crossing returns without taking mu. Written under mu, read locklessly.
	anyArmed atomic.Bool
)

// Register records name as a known crashpoint and returns it, so a site can
// declare its name inline as a package-level var:
//
//	var cpPreRename = crashpoint.Register("pipeline.store.put.pre-rename")
//
// Registering the same name twice is idempotent. A point that is never
// registered is invisible to RegisteredNames, and therefore to any coverage
// gate built on it.
func Register(name string) string {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = struct{}{}
	return name
}

// RegisteredNames returns every registered point, sorted, so an enumeration
// over them is deterministic.
func RegisteredNames() []string {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// At is the kill hook. When name is armed and its countdown is exhausted it
// panics with a *Crash; otherwise it does nothing (consuming one hit of a
// countdown armed for later). Unarmed cost is one atomic load.
func At(name string) { AtStaging(name, nil) }

// AtStaging is At for the call site that must leave behind the forensic
// residue a real kill would have left — see the package comment on the
// in-process limitation. stage, when non-nil, runs exactly once and only on
// the crossing that kills, immediately before the panic.
//
// Deciding and staging in ONE call is the point. The call site used to ask
// Armed and then call At, and between those two questions a sibling
// goroutine could consume the deciding hit — so the site that arranged no
// residue was the one that died, and the mechanism erased the evidence it
// exists to preserve. Here the countdown is consumed once, under the lock,
// and the answer is the same answer the panic acts on.
func AtStaging(name string, stage func()) {
	if !anyArmed.Load() {
		return
	}
	if !fire(name) {
		return
	}
	if stage != nil {
		stage()
	}
	panic(&Crash{Point: name})
}

// fire consumes one hit of name's countdown and reports whether this hit is
// the one that crashes.
func fire(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	n, ok := armed[name]
	if !ok {
		return false
	}
	if n > 1 {
		armed[name] = n - 1
		return false
	}
	return true
}

// ArmOption customizes Arm. No options means "fire on the next At".
type ArmOption func(*armConfig)

type armConfig struct{ hit int }

// ArmOnHit arms the point to fire on the nth At from now (1-based; n < 1 is
// clamped to 1). A point crossed once per unit — every artifact write is one
// — is otherwise only killable on the first unit, and "died halfway through
// a stage" is the case per-unit resume exists for.
func ArmOnHit(n int) ArmOption {
	return func(c *armConfig) { c.hit = n }
}

// Arm marks name so a later At(name) panics — the next one by default, the
// nth with ArmOnHit. It is test-only by convention: nothing in production
// arms a point. The returned disarm closure should be deferred, so an arm
// never leaks into a sibling test.
func Arm(name string, opts ...ArmOption) (disarm func()) {
	cfg := armConfig{hit: 1}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.hit < 1 {
		cfg.hit = 1
	}
	mu.Lock()
	armed[name] = cfg.hit
	anyArmed.Store(true)
	mu.Unlock()
	return func() { Disarm(name) }
}

// Disarm removes name from the armed set. Disarming an unarmed name is a
// no-op.
func Disarm(name string) {
	mu.Lock()
	delete(armed, name)
	anyArmed.Store(len(armed) > 0)
	mu.Unlock()
}
