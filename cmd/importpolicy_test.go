package main

import (
	"go/token"
	"maps"
	"path"
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
// forbidden, and where allowed is set, none may reach a module package
// outside it. direct limits a rule to a package's own imports. A package
// matches a prefix where it is the prefix or beneath it.
type dependencyRule struct {
	from, except       []string
	forbidden, allowed []string
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
	why:     "buildrecords imports no kbase package but kb, so sheet reads the build records without claimgraph and kbload stays above it",
}, {
	from:      []string{"internal/claimgraph"},
	forbidden: []string{"internal/latex"},
	why:       "the claim graph reads the tree and the records, never the reader",
}, {
	from:      []string{"internal/claimgraph"},
	forbidden: []string{"internal/model", "internal/asks/call"},
	why:       "nothing in claimgraph knows a network exists: it asks through the letter seam build injects",
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
	allowed: []string{"internal/log"},
	why:     "kb imports no other kbase package but log",
}, {
	from:    []string{"internal/ledger"},
	allowed: []string{"internal/log"},
	why:     "ledger imports log and nothing else of kbase",
}, {
	from:    []string{"internal/config"},
	allowed: []string{"internal/atomicfile"},
	why:     "the configuration is grammar and resolution: it imports atomicfile to write its files, and never model, result or log — it opens no client and names no verb",
}, {
	except:    []string{"internal/build", "cmd"},
	forbidden: []string{"internal/kbdocs"},
	direct:    true,
	why:       "only build and cmd's init verb stamp readiness documents",
}, {
	except:    []string{"internal/build", "internal/ledger", "cmd"},
	forbidden: []string{"internal/ledger"},
	direct:    true,
	why:       "only build and cmd's init verb import ledger",
}, {
	except:    []string{"internal/build", "cmd"},
	forbidden: []string{"internal/statedir"},
	direct:    true,
	why:       "only the build and the verbs that read a run's state open the state store",
}, {
	except:    []string{"internal/asks/call", "internal/build", "internal/model", "cmd"},
	forbidden: []string{"internal/model"},
	direct:    true,
	why:       "only the caller, build, and cmd's provider verbs import model",
}, {
	from:    []string{"internal/migrate"},
	allowed: []string{"internal/migrate"},
	why:     "the migrate tree may import its own packages and nothing else of kbase, so maintaining a live package never changes what an old KB converts to (I8)",
}, {
	except:    []string{"internal/kbload", "internal/migrate"},
	forbidden: []string{"internal/migrate"},
	direct:    true,
	why:       "outside the migrate tree only the loader imports it: an older KB is converted where it is opened, and nowhere else, and no live package reaches a frozen format (I8)",
}, {
	except:    []string{"cmd", "internal/build", "internal/write", "internal/kbload"},
	forbidden: []string{"internal/kbload"},
	direct:    true,
	why:       "only the openers import the loader: a reader takes the source it is handed, and a refusal is result's",
}, {
	from:    []string{"internal/kbload"},
	allowed: []string{"internal/kb", "internal/buildrecords", "internal/migrate", "internal/result", "internal/log"},
	why:     "the loader stands between kb and every reader: it imports no reader, and buildrecords only for the records' paths",
}, {
	from:    []string{"internal/mcp"},
	allowed: []string{"internal/log"},
	why:     "the MCP server is framing and dispatch over a table cmd injects: it imports log only, so it reaches no KB package, no model, no caller (I9)",
}, {
	from:      []string{"internal/score"},
	forbidden: []string{"internal/model"},
	why:       "the back end sits below the score verb's Ask seam: the runner, the guard, the parse and the ledger learn nothing of what answered",
}, {
	except:    []string{"cmd", "internal/mcp"},
	forbidden: []string{"internal/mcp"},
	direct:    true,
	why:       "only cmd imports mcp: the tool table is the command tree",
}, {
	from:    []string{"internal/export"},
	allowed: []string{"internal/kb", "internal/atomicfile"},
	direct:  true,
	why:     "export imports kb and atomicfile and nothing else of kbase: the verb opens the KB and makes every pandoc call",
}}

func underAny(pkg string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return under(pkg, p) })
}

func (r dependencyRule) binds(pkg string) bool {
	return (len(r.from) == 0 || underAny(pkg, r.from)) && !underAny(pkg, r.except)
}

func (r dependencyRule) refuses(dep string) bool {
	return underAny(dep, r.forbidden) || (len(r.allowed) > 0 && !underAny(dep, r.allowed))
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

func TestDependencyRules(t *testing.T) {
	graph := importGraph(t)
	if len(graph[path.Join("internal", "kb")]) == 0 && len(graph["cmd"]) == 0 {
		t.Fatal("no module packages read; the walk found nothing to check")
	}
	for _, v := range dependencyViolations(graph, dependencyRules) {
		t.Errorf("%s reaches %s — %s", v.pkg, v.dep, v.rule.why)
	}
}

// TestMigrateTreeRules: over a synthetic graph, a package of the migrate tree
// importing another passes the rules, and one importing internal/kb fails
// them, at itself and at every tree package that reaches it.
func TestMigrateTreeRules(t *testing.T) {
	graph := map[string][]string{
		"internal/kbload":                      {"internal/migrate"},
		"internal/migrate":                     {"internal/migrate/common", "internal/migrate/format_1_0_0", "internal/migrate/from_1_0_0_to_2_0_0"},
		"internal/migrate/common":              nil,
		"internal/migrate/format_1_0_0":        {"internal/migrate/common"},
		"internal/migrate/from_1_0_0_to_2_0_0": {"internal/migrate/common", "internal/migrate/format_1_0_0"},
		"internal/kb":                          nil,
	}
	if got := dependencyViolations(graph, dependencyRules); len(got) != 0 {
		t.Errorf("the tree importing within itself breaks %v, want no rule broken", got)
	}
	graph["internal/migrate/format_1_0_0"] = append(graph["internal/migrate/format_1_0_0"], "internal/kb")
	var got []string
	for _, v := range dependencyViolations(graph, dependencyRules) {
		got = append(got, v.pkg+" → "+v.dep)
	}
	want := []string{
		"internal/migrate → internal/kb",
		"internal/migrate/format_1_0_0 → internal/kb",
		"internal/migrate/from_1_0_0_to_2_0_0 → internal/kb",
	}
	if !slices.Equal(got, want) {
		t.Errorf("a tree package importing internal/kb breaks %q, want %q", got, want)
	}
}

// cmdFileImporters names, per package whose rule reaches into cmd at file
// level, the production files of cmd that may import it — the file-level
// read of a rule whose package form exempts all of cmd.
var cmdFileImporters = map[string][]string{
	// the build verb, the provider dial the models and configure verbs share, the
	// score verb's ask, and the probe's one call
	"internal/model": {"cmd/build.go", "cmd/provider.go", "cmd/score.go", "cmd/probe.go"},
	// the init verb, the only cmd path that commits
	"internal/ledger": {"cmd/init.go"},
	// the init verb, the only cmd path that stamps readiness documents
	"internal/kbdocs": {"cmd/init.go"},
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

func TestReach(t *testing.T) {
	graph := map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}, "d": nil}
	if got := reach(graph, "a"); !got["b"] || !got["c"] || !got["a"] || got["d"] {
		t.Errorf("reach(a) = %v, want the cycle a, b, c and not d", got)
	}
}
