package main

import (
	"go/token"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The package map's dependency rules (ARCHITECTURE §11), checked over the
// module's import graph: every production Go file is parsed for its imports,
// and a rule names who may not reach whom, directly or through any chain.

const modulePath = "kbase"

// dependencyRule binds the packages under from — every package, where from
// is empty — less those under except: none may reach a package under
// forbidden, and where allowed or allowedRoots is non-nil, none may reach a
// module package outside allowed but those allowedRoots names themselves — an
// empty, non-nil allowed allowing none. direct limits a rule to a package's
// own imports. A package matches a prefix where it is the prefix or beneath
// it.
type dependencyRule struct {
	from, except       []string
	forbidden, allowed []string
	allowedRoots       []string
	direct             bool
	why                string
}

var dependencyRules = []dependencyRule{{
	from:      []string{"internal/docgraph"},
	forbidden: []string{"internal/claimgraph"},
	why:       "docgraph knows nothing of claims; the record schema both read lives in internal/records",
}, {
	from:    []string{"internal/records"},
	allowed: []string{"internal/atomicfile"},
	why:     "the record schema imports nothing of kbase but atomicfile, to write its files",
}, {
	from:    []string{"internal/buildrecords"},
	allowed: []string{"internal/kb"},
	direct:  true,
	why:     "buildrecords imports no kbase package but kb, so claimgraph reads the build records through it and kbload stays above it",
}, {
	from:      []string{"internal/claimgraph"},
	forbidden: []string{"internal/latex"},
	why:       "the claim graph reads the tree and the records, never the reader",
}, {
	from:      []string{"internal/claimgraph"},
	forbidden: []string{"internal/model", "internal/asks/call"},
	why:       "nothing in claimgraph knows a network exists: it asks through the multichoice-ask seam build injects",
}, {
	except:    []string{"internal/build", "internal/asks/call"},
	forbidden: []string{"internal/asks/call"},
	direct:    true,
	why:       "only build imports the caller",
}, {
	from:      []string{"internal/write", "internal/index", "internal/query", "internal/sheet", "internal/kbdocs"},
	forbidden: []string{"internal/docgraph", "internal/claimgraph", "internal/build"},
	why:       "the maintenance packages stand below the build",
}, {
	from:    []string{"internal/kb"},
	allowed: []string{"internal/log", "internal/formats/format300"},
	why:     "kb imports no other kbase package but log and format300, the live format it re-exports",
}, {
	from:    []string{"internal/gitledger"},
	allowed: []string{"internal/log"},
	why:     "gitledger imports log and nothing else of kbase",
}, {
	from:    []string{"internal/model"},
	allowed: []string{"internal/model"},
	why:     "model imports no kbase package: it knows nothing of files, roles or consumers, and its test support, modeltest, imports model alone",
}, {
	from:    []string{"internal/config"},
	allowed: []string{"internal/atomicfile", "internal/result"},
	why:     "the configuration is grammar and resolution: it imports atomicfile to write its files and result to refuse, and never model or log — it opens no client, writes no output, and returns its faults as data, so each caller decides whether one is a warning or a result field",
}, {
	except:    []string{"internal/build", "internal/mcp", "cmd"},
	forbidden: []string{"internal/kbdocs"},
	direct:    true,
	why:       "only build and cmd stamp readiness documents: init at a KB's birth, a migration save whole; mcp reads the session rule alone",
}, {
	except:    []string{"internal/build", "internal/buildstore", "internal/gitledger", "cmd"},
	forbidden: []string{"internal/gitledger"},
	direct:    true,
	why:       "only build, buildstore and cmd's init verb import gitledger",
}, {
	except:    []string{"internal/build", "internal/buildstore", "cmd"},
	forbidden: []string{"internal/statedir"},
	direct:    true,
	why:       "only the build and the verbs that read a run's state open the state store",
}, {
	except:    []string{"internal/asks/call", "internal/build", "internal/model", "internal/modelclient", "cmd"},
	forbidden: []string{"internal/model"},
	direct:    true,
	why:       "only the caller, build, the client registry modelclient, and the cmd files that build, score or probe import model; modelclient is the only constructor of a model.Client",
}, {
	from:    []string{"internal/modelclient"},
	allowed: []string{"internal/model", "internal/config", "internal/result", "internal/log"},
	direct:  true,
	why:     "the client registry imports model to construct clients, config for the endpoints it takes, result to refuse an unreadable key file, and log to report a close that failed",
}, {
	except:    []string{"cmd", "internal/modelclient"},
	forbidden: []string{"internal/modelclient"},
	direct:    true,
	why:       "only cmd imports the client registry: it opens the registry a verb dials through",
}, {
	from:    []string{"internal/formats"},
	allowed: []string{"internal/formats"},
	why:     "the formats tree may import its own packages and nothing else of kbase, so maintaining a package outside it never changes what an old KB converts to (I8)",
}, {
	except:    []string{"internal/kbload", "internal/formats", "internal/kb"},
	forbidden: []string{"internal/formats"},
	direct:    true,
	why:       "outside the formats tree only the loader and kb import it: an older KB is converted where it is opened, and nowhere else, kb re-exports the live format, and no other package reaches a format (I8)",
}, {
	from:    []string{"internal/formats/format300"},
	allowed: []string{},
	why:     "the live format imports no kbase package, common included, and restates any declaration it shares with an earlier format",
}, {
	from:         []string{"internal/kbload"},
	allowed:      []string{"internal/kb", "internal/buildrecords", "internal/result"},
	allowedRoots: []string{"internal/formats"},
	direct:       true,
	why:          "the loader reaches the formats tree through its root alone, and the live format only through kb and that root",
}, {
	from:    []string{"internal/sheet"},
	allowed: []string{"internal/kb"},
	direct:  true,
	why:     "sheet reads no build record and imports no kbase package but kb",
}, {
	except:    []string{"cmd", "internal/score", "internal/mcp"},
	forbidden: []string{"internal/score"},
	direct:    true,
	why:       "only cmd and the MCP server import score: the score verb and the verbs that read a run, and the server's score launch and its kbase://build resource",
}, {
	except:    []string{"cmd", "internal/build", "internal/write", "internal/kbload"},
	forbidden: []string{"internal/kbload"},
	direct:    true,
	why:       "only the openers import the loader: a reader takes the source it is handed, and a refusal is result's",
}, {
	from:    []string{"internal/kbload"},
	allowed: []string{"internal/kb", "internal/buildrecords", "internal/formats", "internal/result", "internal/log"},
	why:     "the loader stands between kb and every reader: it imports no reader, buildrecords only for the records' paths, and the formats tree to convert an older KB",
}, {
	from:    []string{"internal/mcp/protocol"},
	allowed: []string{"internal/log"},
	why:     "the MCP wire is framing and dispatch over a table its caller injects: it imports log only",
}, {
	from:      []string{"internal/mcp"},
	forbidden: []string{"internal/model", "internal/asks/call"},
	why:       "the MCP server performs no inference in a call: nothing under internal/mcp reaches model or the caller (I9)",
}, {
	from:      []string{"internal/mcp"},
	forbidden: []string{"internal/build"},
	why:       "the MCP server reads the build's state through buildstore and never imports the runner",
}, {
	from:      []string{"internal/buildstore"},
	forbidden: []string{"internal/model", "internal/asks/call"},
	why:       "the build's state, stage vocabulary and trail are read without the runner: buildstore reaches no model and no caller, so its readers reach no inference (I9)",
}, {
	from:      []string{"internal/score"},
	forbidden: []string{"internal/model"},
	why:       "the back end sits below the score verb's Ask seam: the runner, the guard, the parse and the ledger learn nothing of what answered",
}, {
	from:      []string{"internal/score"},
	forbidden: []string{"internal/build"},
	why:       "a scoring run reads the store through buildstore and never imports the build's runner: the verb takes the run's locks",
}, {
	except:    []string{"cmd", "internal/mcp"},
	forbidden: []string{"internal/mcp"},
	direct:    true,
	why:       "only cmd imports mcp: the tool table is the command tree",
}, {
	from:    []string{"internal/export"},
	allowed: []string{"internal/kb", "internal/atomicfile"},
	direct:  true,
	why:     "export imports kb and atomicfile and nothing else of kbase: the verb opens the KB and supplies every pandoc call",
}, {
	from:    []string{"internal/auth"},
	allowed: []string{"internal/atomicfile"},
	why:     "the credentials store imports nothing of kbase but atomicfile, to write its files at their mode",
}, {
	except:    []string{"cmd"},
	forbidden: []string{"internal/auth"},
	direct:    true,
	why:       "only cmd imports the credentials store: it reads the stored keys and hands them to config as data",
}}

func underAny(pkg string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return under(pkg, p) })
}

func (r dependencyRule) binds(pkg string) bool {
	return (len(r.from) == 0 || underAny(pkg, r.from)) && !underAny(pkg, r.except)
}

func (r dependencyRule) refuses(dep string) bool {
	limited := r.allowed != nil || r.allowedRoots != nil
	return underAny(dep, r.forbidden) || (limited && !underAny(dep, r.allowed) && !slices.Contains(r.allowedRoots, dep))
}

// under is whether pkg is prefix or beneath it.
func under(pkg, prefix string) bool { return pkg == prefix || strings.HasPrefix(pkg, prefix+"/") }

// importGraph is every module package's in-module imports, from its
// production files, by module-relative directory.
func importGraph(t *testing.T) map[string][]string {
	t.Helper()
	graph := map[string][]string{}
	for _, m := range moduleGoFiles(t, token.NewFileSet()) {
		if !m.production() {
			continue
		}
		pkg := path.Dir(m.rel)
		if _, ok := graph[pkg]; !ok {
			graph[pkg] = nil
		}
		for _, spec := range m.f.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil || !strings.HasPrefix(imported, modulePath+"/") {
				continue
			}
			dep := strings.TrimPrefix(imported, modulePath+"/")
			if !slices.Contains(graph[pkg], dep) {
				graph[pkg] = append(graph[pkg], dep)
			}
		}
	}
	return graph
}

// reach is every package pkg imports, directly or through others.
func reach(graph map[string][]string, pkg string) map[string]bool {
	seen := map[string]bool{}
	pending := slices.Clone(graph[pkg])
	for len(pending) > 0 {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if !seen[next] {
			seen[next] = true
			pending = append(pending, graph[next]...)
		}
	}
	return seen
}

// dependencyViolation is one package reaching a dependency a rule refuses it.
type dependencyViolation struct {
	pkg, dep string
	rule     dependencyRule
}

// dependencyViolations is every package of graph that reaches a dependency one
// of rules refuses it, in package order.
func dependencyViolations(graph map[string][]string, rules []dependencyRule) []dependencyViolation {
	pkgs := slices.Sorted(maps.Keys(graph))
	var out []dependencyViolation
	for _, rule := range rules {
		for _, pkg := range pkgs {
			if !rule.binds(pkg) {
				continue
			}
			reached := map[string]bool{}
			if rule.direct {
				for _, d := range graph[pkg] {
					reached[d] = true
				}
			} else {
				reached = reach(graph, pkg)
			}
			for _, dep := range slices.Sorted(maps.Keys(reached)) {
				if rule.refuses(dep) {
					out = append(out, dependencyViolation{pkg: pkg, dep: dep, rule: rule})
				}
			}
		}
	}
	return out
}

// formatsRoot is the formats tree's root; beneath it stand common, a format
// package per format, format<version>, and a converter per step,
// from<version>to<version>, each version its digits.
const formatsRoot = "internal/formats"

var (
	formatPackageRE    = regexp.MustCompile(`^format([0-9]+)$`)
	converterPackageRE = regexp.MustCompile(`^from([0-9]+)to([0-9]+)$`)
)

// formatsPackage is a tree package's role — "root", "common", "format" or
// "converter" — and the version past which it may not import a format: a
// format's own, a converter's target; "" where pkg is outside the tree or
// fits no role.
func formatsPackage(pkg string) (role string, version int) {
	if pkg == formatsRoot {
		return "root", 0
	}
	name, ok := strings.CutPrefix(pkg, formatsRoot+"/")
	switch {
	case !ok || strings.Contains(name, "/"):
		return "", 0
	case name == "common":
		return "common", 0
	case formatPackageRE.MatchString(name):
		v, _ := strconv.Atoi(formatPackageRE.FindStringSubmatch(name)[1])
		return "format", v
	case converterPackageRE.MatchString(name):
		v, _ := strconv.Atoi(converterPackageRE.FindStringSubmatch(name)[2])
		return "converter", v
	}
	return "", 0
}

// formatsTreeViolations is every import inside the formats tree its one-way
// order refuses, as "pkg → dep — why": common imports nothing of the tree; a
// format imports common and earlier formats; a converter imports common and
// formats up to its target; the root imports the rest; no package but the
// root imports a converter, and none imports a later format. A tree package
// that fits no role is a violation of its own.
func formatsTreeViolations(graph map[string][]string) []string {
	var out []string
	for _, pkg := range slices.Sorted(maps.Keys(graph)) {
		role, version := formatsPackage(pkg)
		if !under(pkg, formatsRoot) {
			continue
		}
		if role == "" {
			out = append(out, pkg+" — a formats tree package is the root, common, a format or a converter")
			continue
		}
		for _, dep := range graph[pkg] {
			depRole, depVersion := formatsPackage(dep)
			why := ""
			switch {
			case !under(dep, formatsRoot):
				continue
			case depRole == "converter" && role != "root":
				why = "no package but the root imports a converter"
			case depRole == "root":
				why = "nothing in the tree imports its root"
			case role == "common":
				why = "common imports nothing of the tree"
			case depRole == "format" && role == "format" && depVersion >= version:
				why = "a format imports only earlier formats"
			case depRole == "format" && role == "converter" && depVersion > version:
				why = "a converter imports no format later than its target"
			}
			if why != "" {
				out = append(out, pkg+" → "+dep+" — "+why)
			}
		}
	}
	return out
}

func TestDependencyRules(t *testing.T) {
	graph := importGraph(t)
	if len(graph[path.Join("internal", "kb")]) == 0 && len(graph["cmd"]) == 0 {
		t.Fatal("no module packages read; the walk found nothing to check")
	}
	for _, v := range dependencyViolations(graph, dependencyRules) {
		t.Errorf("%s reaches %s — %s", v.pkg, v.dep, v.rule.why)
	}
	for _, v := range formatsTreeViolations(graph) {
		t.Error(v)
	}
}

// TestFormatsTreeRules: over a synthetic graph, the formats tree importing
// within itself, the loader importing its root and kb, and kb importing
// format300 pass the rules; kb importing another format, a live package
// importing format300, the loader importing format300 or format200 directly,
// format300 importing common, and a tree package importing internal/kb each
// fail them, at exactly the packages named. Within the tree, a format
// importing itself or a later format, a non-root package importing a
// converter, common importing a format, a converter importing a format past
// its target, a package importing the root, and a package fitting no role each
// break the tree's order.
func TestFormatsTreeRules(t *testing.T) {
	base := map[string][]string{
		"internal/kbload":               {"internal/formats", "internal/kb"},
		"internal/formats":              {"internal/formats/common", "internal/formats/format100", "internal/formats/format200", "internal/formats/from100to200"},
		"internal/formats/common":       nil,
		"internal/formats/format100":    {"internal/formats/common"},
		"internal/formats/format200":    {"internal/formats/common"},
		"internal/formats/from100to200": {"internal/formats/common", "internal/formats/format100", "internal/formats/format200"},
		"internal/formats/format300":    nil,
		"internal/kb":                   {"internal/formats/format300"},
	}
	if got := dependencyViolations(base, dependencyRules); len(got) != 0 {
		t.Errorf("the base graph breaks %v, want no rule broken", got)
	}
	for _, tc := range []struct {
		pkg, dep string
		want     []string
	}{
		{"internal/kb", "internal/formats/format200", []string{
			"internal/kb → internal/formats/common",
			"internal/kb → internal/formats/format200",
		}},
		{"internal/index", "internal/formats/format300", []string{
			"internal/index → internal/formats/format300",
		}},
		{"internal/kbload", "internal/formats/format300", []string{
			"internal/kbload → internal/formats/format300",
		}},
		{"internal/formats/format100", "internal/kb", []string{
			"internal/formats → internal/kb",
			"internal/formats/format100 → internal/kb",
			"internal/formats/from100to200 → internal/kb",
		}},
		{"internal/kbload", "internal/formats/format200", []string{
			"internal/kbload → internal/formats/format200",
		}},
		{"internal/formats/format300", "internal/formats/common", []string{
			"internal/kb → internal/formats/common",
			"internal/formats/format300 → internal/formats/common",
		}},
	} {
		graph := withImport(base, tc.pkg, tc.dep)
		var got []string
		for _, v := range dependencyViolations(graph, dependencyRules) {
			got = append(got, v.pkg+" → "+v.dep)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s importing %s breaks %q, want %q", tc.pkg, tc.dep, got, tc.want)
		}
	}
	if got := formatsTreeViolations(base); len(got) != 0 {
		t.Errorf("the base graph breaks the tree's order: %q", got)
	}
	for _, tc := range []struct{ pkg, dep, want string }{
		{"internal/formats/format200", "internal/formats/format300", "a format imports only earlier formats"},
		{"internal/formats/format200", "internal/formats/format200", "a format imports only earlier formats"},
		{"internal/formats/format200", "internal/formats/from100to200", "no package but the root imports a converter"},
		{"internal/formats/common", "internal/formats/format100", "common imports nothing of the tree"},
		{"internal/formats/from100to200", "internal/formats/format300", "a converter imports no format later than its target"},
		{"internal/formats/format100", "internal/formats", "nothing in the tree imports its root"},
	} {
		want := []string{tc.pkg + " → " + tc.dep + " — " + tc.want}
		if got := formatsTreeViolations(withImport(base, tc.pkg, tc.dep)); !slices.Equal(got, want) {
			t.Errorf("%s importing %s breaks %q, want %q", tc.pkg, tc.dep, got, want)
		}
	}
	if got := formatsTreeViolations(withImport(base, "internal/formats/legacy", "")); len(got) != 1 {
		t.Errorf("a tree package fitting no role breaks %q, want it named", got)
	}
}

// withImport is a copy of graph in which pkg also imports dep, or, where dep
// is "", stands with no import.
func withImport(graph map[string][]string, pkg, dep string) map[string][]string {
	out := map[string][]string{}
	for p, deps := range graph {
		out[p] = slices.Clone(deps)
	}
	if dep == "" {
		out[pkg] = nil
	} else {
		out[pkg] = append(out[pkg], dep)
	}
	return out
}

// cmdFileImporters names, per package whose rule reaches into cmd at file
// level, the production files of cmd that may import it — the file-level
// read of a rule whose package form exempts all of cmd.
var cmdFileImporters = map[string][]string{
	// the build verb's role closure, the score verb's ask, and the probe's one call
	"internal/model": {"cmd/build.go", "cmd/score.go", "cmd/probe.go"},
	// the init verb, the only cmd path that commits
	"internal/gitledger": {"cmd/init.go"},
	// the init verb, which stamps readiness documents at a KB's birth, and the
	// render every cmd verb that saves a migration puts in its source
	"internal/kbdocs": {"cmd/init.go", "cmd/restamp.go"},
}

// TestCmdFileImporters: within cmd, each package of cmdFileImporters is
// imported by its listed files alone, and each of them imports it.
func TestCmdFileImporters(t *testing.T) {
	files := moduleGoFiles(t, token.NewFileSet())
	for _, pkg := range slices.Sorted(maps.Keys(cmdFileImporters)) {
		owners := cmdFileImporters[pkg]
		seen := map[string]bool{}
		for _, m := range files {
			if !m.production() || path.Dir(m.rel) != "cmd" || !slices.Contains(slices.Collect(maps.Values(importedAs(m.f))), modulePath+"/"+pkg) {
				continue
			}
			seen[m.rel] = true
			if !slices.Contains(owners, m.rel) {
				t.Errorf("%s imports %s; within cmd only %s may", m.rel, pkg, strings.Join(owners, " and "))
			}
		}
		for _, owner := range owners {
			if !seen[owner] {
				t.Errorf("%s is listed as importing %s and does not; drop it from cmdFileImporters", owner, pkg)
			}
		}
	}
}

// TestModelImportsNoOS: no production file of internal/model imports os, so
// the package that reaches the wire touches no file.
func TestModelImportsNoOS(t *testing.T) {
	read := false
	for _, m := range moduleGoFiles(t, token.NewFileSet()) {
		if !m.production() || path.Dir(m.rel) != "internal/model" {
			continue
		}
		read = true
		if slices.Contains(slices.Collect(maps.Values(importedAs(m.f))), "os") {
			t.Errorf("%s imports os; internal/model touches no file", m.rel)
		}
	}
	if !read {
		t.Fatal("the walk read no production file of internal/model")
	}
}

func TestReach(t *testing.T) {
	graph := map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}, "d": nil}
	if got := reach(graph, "a"); !got["b"] || !got["c"] || !got["a"] || got["d"] {
		t.Errorf("reach(a) = %v, want the cycle a, b, c and not d", got)
	}
}
