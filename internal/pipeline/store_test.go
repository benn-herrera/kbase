package pipeline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kbase/internal/log/logtest"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/version"
)

// newStore returns a store over a fresh temp job dir plus its capture logger.
func newStore(t *testing.T) (*Store, *logtest.Capture) {
	t.Helper()
	lg := &logtest.Capture{}
	return NewStore(t.TempDir(), lg), lg
}

// put writes a unit and fails the test if the write does.
func put(t *testing.T, s *Store, rel string, data string, inputs ...Input) {
	t.Helper()
	if err := s.Put(rel, []byte(data), inputs); err != nil {
		t.Fatalf("Put(%s): %v", rel, err)
	}
}

// verify runs Verify and fails the test on a refusal, which is a distinct
// outcome from a verdict and never what a verdict assertion means to check.
func verify(t *testing.T, s *Store, rel string, inputs ...Input) (Verdict, string) {
	t.Helper()
	v, reason, err := s.verify(rel, inputs)
	if err != nil {
		t.Fatalf("verify(%s): refused: %v", rel, err)
	}
	return v, reason
}

func TestPutVerifyRoundTrip(t *testing.T) {
	s, _ := newStore(t)
	in := []Input{{Name: "source", Hash: HashBytes([]byte("corpus"))}}

	put(t, s, "stage1/survey.json", "artifact bytes", in...)

	if v, reason := verify(t, s, "stage1/survey.json", in...); v != VerdictValid {
		t.Fatalf("verdict %s (%s), want %s", v, reason, VerdictValid)
	}

	stamp, err := s.readStamp("stage1/survey.json")
	if err != nil {
		t.Fatalf("readStamp: %v", err)
	}
	if stamp.Version != version.Current {
		t.Errorf("stamp version %q, want %q", stamp.Version, version.Current)
	}
	if stamp.Path != "stage1/survey.json" {
		t.Errorf("stamp path %q, want the artifact's own path", stamp.Path)
	}
	if stamp.Output != HashBytes([]byte("artifact bytes")) {
		t.Error("stamp output hash is not the hash of what was written")
	}
}

// TestVerifyDoubtIsInvalid walks the ways an artifact can fail to prove
// itself. They are one test because they are one rule: reuse requires
// affirmative proof, so everything short of it lands on the same verdict.
func TestVerifyDoubtIsInvalid(t *testing.T) {
	const rel = "stage1/unit.md"
	good := []Input{{Name: "source", Hash: "aaa"}, {Name: "skeleton", Hash: "bbb"}}

	cases := []struct {
		name    string
		damage  func(t *testing.T, s *Store)
		check   []Input
		want    Verdict
		wantMsg string
	}{
		{
			name:  "untouched",
			check: good,
			want:  VerdictValid,
		},
		{
			name:    "nothing written at all",
			damage:  func(t *testing.T, s *Store) { rm(t, s, rel); rm(t, s, rel+StampSuffix) },
			check:   good,
			want:    VerdictAbsent,
			wantMsg: "no artifact",
		},
		{
			name:    "artifact with no stamp",
			damage:  func(t *testing.T, s *Store) { rm(t, s, rel+StampSuffix) },
			check:   good,
			want:    VerdictInvalid,
			wantMsg: "no stamp",
		},
		{
			name:    "stamp with no artifact",
			damage:  func(t *testing.T, s *Store) { rm(t, s, rel) },
			check:   good,
			want:    VerdictInvalid,
			wantMsg: "no artifact",
		},
		{
			name:    "unparseable stamp",
			damage:  func(t *testing.T, s *Store) { write(t, s, rel+StampSuffix, "{not json") },
			check:   good,
			want:    VerdictInvalid,
			wantMsg: "unusable",
		},
		{
			name:    "artifact edited underneath its stamp",
			damage:  func(t *testing.T, s *Store) { write(t, s, rel, "tampered") },
			check:   good,
			want:    VerdictInvalid,
			wantMsg: "do not match",
		},
		{
			name:    "an input hash changed",
			check:   []Input{{Name: "source", Hash: "aaa"}, {Name: "skeleton", Hash: "CHANGED"}},
			want:    VerdictInvalid,
			wantMsg: `"skeleton" changed`,
		},
		{
			name:    "an input the stamp never carried",
			check:   append(append([]Input{}, good...), Input{Name: "zeta", Hash: "ccc"}),
			want:    VerdictInvalid,
			wantMsg: "missing input",
		},
		{
			name:    "one fewer input than the stamp carries",
			check:   good[:1],
			want:    VerdictInvalid,
			wantMsg: `where "source" was expected`,
		},
		{
			name:    "written by another app version",
			damage:  func(t *testing.T, s *Store) { restamp(t, s, rel, func(st *Stamp) { st.Version = "9.9.9-other" }) },
			check:   good,
			want:    VerdictInvalid,
			wantMsg: "running",
		},
		{
			name:    "written under another stamp schema",
			damage:  func(t *testing.T, s *Store) { restamp(t, s, rel, func(st *Stamp) { st.Schema = stampSchema + 1 }) },
			check:   good,
			want:    VerdictInvalid,
			wantMsg: "schema",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			put(t, s, rel, "body", good...)
			if tc.damage != nil {
				tc.damage(t, s)
			}
			v, reason := verify(t, s, rel, tc.check...)
			if v != tc.want {
				t.Fatalf("verdict %s (%s), want %s", v, reason, tc.want)
			}
			if tc.wantMsg != "" && !strings.Contains(reason, tc.wantMsg) {
				t.Errorf("reason %q does not mention %q", reason, tc.wantMsg)
			}
			if v == VerdictValid && reason != "" {
				t.Errorf("a valid verdict carried a reason: %q", reason)
			}
		})
	}
}

// TestVerifyRefusesIncoherentStore covers the cases redoing a unit cannot
// fix, which are refusals rather than verdicts and must name --fresh.
func TestVerifyRefusesIncoherentStore(t *testing.T) {
	t.Run("stamp belonging to another artifact", func(t *testing.T) {
		s, _ := newStore(t)
		put(t, s, "a.md", "body")
		restamp(t, s, "a.md", func(st *Stamp) { st.Path = "somewhere/else.md" })

		_, _, err := s.verify("a.md", nil)
		assertIncoherent(t, err)
	})

	t.Run("a directory where an artifact belongs", func(t *testing.T) {
		s, _ := newStore(t)
		if err := os.MkdirAll(filepath.Join(s.root, "a.md"), artifactDirMode); err != nil {
			t.Fatal(err)
		}
		_, _, err := s.verify("a.md", nil)
		assertIncoherent(t, err)
	})
}

func TestStoreRejectsUnusablePaths(t *testing.T) {
	s, _ := newStore(t)
	for _, rel := range []string{"", "../escape.md", "/absolute.md", "unit.md" + StampSuffix, LockFileName} {
		t.Run(rel, func(t *testing.T) {
			err := s.Put(rel, []byte("x"), nil)
			var bad BadPathError
			if !errors.As(err, &bad) {
				t.Fatalf("Put(%q) = %v, want a BadPathError", rel, err)
			}
		})
	}
}

// TestStampIsDeterministic pins the property the whole verdict scheme rests
// on: the same facts produce the same bytes, whatever order the caller
// happened to list its inputs in.
func TestStampIsDeterministic(t *testing.T) {
	a, _ := newStore(t)
	b, _ := newStore(t)
	in := []Input{{Name: "zeta", Hash: "222"}, {Name: "alpha", Hash: "111"}}
	reversed := []Input{in[1], in[0]}

	put(t, a, "x.md", "body", in...)
	put(t, b, "x.md", "body", reversed...)

	first := read(t, a, "x.md"+StampSuffix)
	second := read(t, b, "x.md"+StampSuffix)
	if first != second {
		t.Fatalf("stamps differ:\n%s\n%s", first, second)
	}
	if in[0].Name != "zeta" {
		t.Error("Put reordered the caller's own input slice")
	}
}

func TestPutOverwritesInPlace(t *testing.T) {
	s, _ := newStore(t)
	old := []Input{{Name: "source", Hash: "111"}}
	put(t, s, "x.md", "first", old...)

	fresh := []Input{{Name: "source", Hash: "222"}}
	put(t, s, "x.md", "second", fresh...)

	if v, reason := verify(t, s, "x.md", fresh...); v != VerdictValid {
		t.Fatalf("verdict %s (%s), want %s", v, reason, VerdictValid)
	}
	if v, _ := verify(t, s, "x.md", old...); v != VerdictInvalid {
		t.Fatalf("the superseded inputs still verify as %s", v)
	}
	if got := read(t, s, "x.md"); got != "second" {
		t.Errorf("artifact holds %q, want the second write", got)
	}
}

func TestPutFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	s, _ := newStore(t)
	put(t, s, "nested/x.md", "body")

	for _, rel := range []string{"nested/x.md", "nested/x.md" + StampSuffix} {
		info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != artifactFileMode {
			t.Errorf("%s mode %o, want %o", rel, got, artifactFileMode)
		}
	}
	dir, err := os.Stat(filepath.Join(s.root, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	if got := dir.Mode().Perm(); got != artifactDirMode {
		t.Errorf("directory mode %o, want %o", got, artifactDirMode)
	}
}

// TestPutCrashpointSeams is the interruption proof: each registered write
// point stops the run at exactly its seam, and the on-disk state that
// results carries the verdict the resume design says it must. The middle
// case is the one that matters — an artifact whose stamp never landed is
// present, plausible, and unproven, and must be redone.
func TestPutCrashpointSeams(t *testing.T) {
	cases := []struct {
		name        string
		point       string
		wantArtifat bool
		wantStamp   bool
		wantVerdict Verdict
	}{
		{"killed before the artifact is renamed into place", cpPutPreRename, false, false, VerdictAbsent},
		{"killed after the artifact lands, before its stamp", cpPutPreStamp, true, false, VerdictInvalid},
		{"killed after both land", cpPutDone, true, true, VerdictValid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			defer crashpoint.Arm(tc.point)()

			crashed := crashedAt(t, tc.point, func() {
				_ = s.Put("stage1/unit.md", []byte("body"), nil)
			})
			if !crashed {
				t.Fatal("Put ran to completion with its point armed")
			}

			if got := exists(t, s, "stage1/unit.md"); got != tc.wantArtifat {
				t.Errorf("artifact present = %v, want %v", got, tc.wantArtifat)
			}
			if got := exists(t, s, "stage1/unit.md"+StampSuffix); got != tc.wantStamp {
				t.Errorf("stamp present = %v, want %v", got, tc.wantStamp)
			}
			if v, _ := verify(t, s, "stage1/unit.md"); v != tc.wantVerdict {
				t.Errorf("verdict %s, want %s", v, tc.wantVerdict)
			}
		})
	}
}

// TestPutPreRenameLeavesResidue states the modelling limitation the
// crashpoint package documents: a real kill leaves the temporary file behind,
// so the armed simulation must too, or the harness would be testing a tidier
// world than the one it claims to test.
func TestPutPreRenameLeavesResidue(t *testing.T) {
	s, _ := newStore(t)
	defer crashpoint.Arm(cpPutPreRename)()

	if !crashedAt(t, cpPutPreRename, func() { _ = s.Put("unit.md", []byte("body"), nil) }) {
		t.Fatal("Put ran to completion with its point armed")
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), tempSuffix) {
			found = true
		}
	}
	if !found {
		t.Errorf("no temporary file survived the crash; directory holds %v", entries)
	}
}

// crashedAt runs fn and reports whether it crashed at the named point,
// failing the test if it panicked with anything else.
func crashedAt(t *testing.T, point string, fn func()) (crashed bool) {
	t.Helper()
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}
		crash, ok := rec.(*crashpoint.Crash)
		if !ok {
			t.Fatalf("recovered %v, want a *crashpoint.Crash", rec)
		}
		if crash.Point != point {
			t.Fatalf("crashed at %q, want %q", crash.Point, point)
		}
		crashed = true
	}()
	fn()
	return false
}

func assertIncoherent(t *testing.T, err error) {
	t.Helper()
	var inc IncoherentStoreError
	if !errors.As(err, &inc) {
		t.Fatalf("err = %v, want an IncoherentStoreError", err)
	}
	if !strings.Contains(err.Error(), "--fresh") {
		t.Errorf("refusal %q does not name --fresh", err)
	}
}

func read(t *testing.T, s *Store, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func write(t *testing.T, s *Store, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.root, filepath.FromSlash(rel)), []byte(content), artifactFileMode); err != nil {
		t.Fatal(err)
	}
}

func rm(t *testing.T, s *Store, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(s.root, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func exists(t *testing.T, s *Store, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
	return err == nil
}

// restamp rewrites an artifact's stamp sidecar through edit, which is how a
// test produces a stamp this build would never write (another app version,
// another schema, another artifact's path).
func restamp(t *testing.T, s *Store, rel string, edit func(*Stamp)) {
	t.Helper()
	stamp, err := s.readStamp(rel)
	if err != nil {
		t.Fatal(err)
	}
	edit(&stamp)
	b, err := json.MarshalIndent(stamp, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, s, rel+StampSuffix, string(b)+"\n")
}
