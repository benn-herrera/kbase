package main

import (
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// fileWriters are the calls that create, write, rename or link a path, by
// import path and function name. kbwrite.Inside is the write under kb-root/,
// over atomicfile.WriteIn.
var fileWriters = map[string]map[string]bool{
	"kbase/internal/atomicfile": {"Write": true, "WriteIn": true, "WriteMode": true},
	"kbase/internal/kbwrite":    {"Inside": true},
	"os": {"WriteFile": true, "Create": true, "CreateTemp": true, "OpenFile": true, "Mkdir": true, "MkdirAll": true,
		"MkdirTemp": true, "Rename": true, "Symlink": true, "Link": true},
}

// writeOwners are the production files that write a path, each with where it
// writes. Every location is one the contract documents name; a temporary file
// an atomic write renames onto its target counts as that target.
var writeOwners = map[string]string{
	"cmd/init.go":                                "kb-root/ itself, its .index/ and the skeleton init writes into it",
	"internal/asks/call/call.go":                 "the answer cache and a build's captures, in the state store",
	"internal/asks/multichoice.go":               "a build stage's multichoice-ask group records, in the state store",
	"internal/atomicfile/atomicfile.go":          "the atomic writer's temporary file, in kb-root/.index/kb-temp/ (with its .gitignore) or beside a target outside kb-root/, and the rename onto the target",
	"internal/auth/auth.go":                      "the credentials store's directory and key files",
	"internal/build/build.go":                    "the build's state directory",
	"internal/build/replay.go":                   "a build's development replay directory, in the state store",
	"internal/build/rows.go":                     "the tree under kb-root/ and the overview document",
	"internal/build/seed.go":                     "kb-root/.index/ and its format stamp, at a fresh build",
	"internal/build/store.go":                    "the state store's stage reports, its progress records and its scratch trail directories",
	"internal/build/storeformat.go":              "the state store's format stamp",
	"internal/buildstore/storetest/storetest.go": "a test's fixture files, under the test's own directory",
	"internal/claimgraph/buildrecords.go":        "the build records, kb-root/.build/*.yaml",
	"internal/config/write.go":                   "the configuration files the configure commands write",
	"internal/docgraph/tree.go":                  "the document tree under kb-root/",
	"internal/export/write.go":                   "the export's gen-tex/, and a starter preamble.tex where none stands",
	"internal/filelock/filelock.go":              "the lock files",
	"internal/gitledger/ledger.go":               "a moved file's directory, under the repository a git mv moves it within",
	"internal/index/refresh.go":                  "the derived views under kb-root/",
	"internal/kbdocs/kbdocs.go":                  "kb-root/'s stamped readiness documents and the agents redirect",
	"internal/kbwrite/kbwrite.go":                "a file under kb-root/, its temporary in kb-root/.index/kb-temp/",
	"internal/log/log.go":                        "the log file",
	"internal/mcp/launch.go":                     "a detached build's or score run's captured stdout and stderr, in the state store's reports",
	"internal/records/records.go":                "the document graph's records and the pre-pass census, in the state store",
	"internal/score/invoke.go":                   "a scoring run's runs directory, in the state store",
	"internal/score/ledger.go":                   "a scoring run's directory and its files, in the state store",
	"internal/score/replay.go":                   "a scoring run's development replay directory, in the state store",
}

// kbRootWriters are the writeOwners whose writes land under kb-root/. Each
// writes there only through kbRootWrites, so every file under kb-root/ is
// written atomically, its temporary in kb-root/.index/kb-temp/ and its path
// refused where it is a symbolic link or leaves kb-root/.
var kbRootWriters = map[string]bool{
	"cmd/init.go":                         true,
	"internal/build/rows.go":              true,
	"internal/build/seed.go":              true,
	"internal/claimgraph/buildrecords.go": true,
	"internal/docgraph/tree.go":           true,
	"internal/index/refresh.go":           true,
	"internal/kbdocs/kbdocs.go":           true,
}

// kbRootWrites are the calls a kbRootWriters file may write with.
var kbRootWrites = map[string]bool{"kbase/internal/kbwrite.Inside": true, "os.Mkdir": true, "os.MkdirAll": true}

// TestWritePolicy scans every production Go file and fails on a file that
// writes a path and is not in writeOwners, on an entry whose file writes
// none, and on a kbRootWriters file writing by a call outside kbRootWrites:
// every path kbase writes is a location the contract names, so a new writer is
// a new location to name.
func TestWritePolicy(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() {
			continue
		}
		for _, r := range pkgRefs(m.f, fileWriters) {
			seen[m.rel] = true
			if _, ok := writeOwners[m.rel]; !ok {
				t.Errorf("%s: %s writes a path; name where this file writes in writeOwners, a location SPEC or ARCHITECTURE names", fset.Position(r.pos), r.name)
			}
			if kbRootWriters[m.rel] && !kbRootWrites[r.name] {
				t.Errorf("%s: %s writes under kb-root/; write there through kbwrite.Inside, making a directory with os.Mkdir or os.MkdirAll", fset.Position(r.pos), r.name)
			}
		}
	}
	for owner := range writeOwners {
		if !seen[owner] {
			t.Errorf("%s is listed as writing and writes nothing; drop it from writeOwners", owner)
		}
	}
	for owner := range kbRootWriters {
		if _, ok := writeOwners[owner]; !ok {
			t.Errorf("%s is listed as writing under kb-root/ and not in writeOwners; name where it writes there", owner)
		}
	}
}

func TestFileWriters(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"a read", `package p; import "os"; func f() { os.ReadFile("x") }`, nil},
		{"os.WriteFile", `package p; import "os"; func f() { os.WriteFile("x", nil, 0) }`, []string{"os.WriteFile"}},
		{"aliased os.Rename", `package p; import o "os"; func f() { o.Rename("a", "b") }`, []string{"os.Rename"}},
		{"os.MkdirAll and os.Create", `package p; import "os"; func f() { os.MkdirAll("d", 0); os.Create("d/x") }`, []string{"os.MkdirAll", "os.Create"}},
		{"os.OpenFile as a value", `package p; import "os"; var open = os.OpenFile`, []string{"os.OpenFile"}},
		{"atomicfile.WriteMode", `package p; import "kbase/internal/atomicfile"; func f() { atomicfile.WriteMode("x", nil, 0) }`, []string{"kbase/internal/atomicfile.WriteMode"}},
		{"kbwrite.Inside", `package p; import "kbase/internal/kbwrite"; func f() { kbwrite.Inside("r", "r/x", nil) }`, []string{"kbase/internal/kbwrite.Inside"}},
		{"another package's Write", `package p; import "bytes"; func f(b *bytes.Buffer) { b.Write(nil) }`, nil},
		{"a method named Rename", `package p; func f(r interface{ Rename(a, b string) }) { r.Rename("a", "b") }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range pkgRefs(f, fileWriters) {
				got = append(got, r.name)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("writes = %q, want %q", got, tc.want)
			}
		})
	}
}
