package auth

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"kbase/internal/secret"
)

// sentinelKey is the key every test stores. No failure message formats it, or
// a value that would hold it: a message says whether it appeared or matched.
const sentinelKey = "sk-test-SENTINEL-4f1c9a7e2b"

// storeFor is a store in a fresh directory, sealed for user.
func storeFor(t *testing.T, user string) Store {
	t.Helper()
	return Store{dir: t.TempDir(), username: func() (string, error) { return user, nil }}
}

func addSentinel(t *testing.T, s Store, name string) string {
	t.Helper()
	if err := s.Add(name, secret.NewKey(sentinelKey)); err != nil {
		t.Fatalf("Add(%q) failed with a %T", name, err)
	}
	return filepath.Join(s.dir, name+FileSuffix)
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestRoundTrip: a key added is the key read back, and a second add of the
// name replaces it.
func TestRoundTrip(t *testing.T) {
	s := storeFor(t, "alice")
	addSentinel(t, s, "cloud")
	got, err := s.Read("cloud")
	if err != nil {
		t.Fatalf("Read failed with a %T", err)
	}
	if got.Reveal() != sentinelKey {
		t.Errorf("the key read back differs from the one added (length %d, want %d)", len(got.Reveal()), len(sentinelKey))
	}
	if err := s.Add("cloud", secret.NewKey("replacement")); err != nil {
		t.Fatalf("Add over a stored key failed with a %T", err)
	}
	if got, err := s.Read("cloud"); err != nil || got.Reveal() != "replacement" {
		t.Errorf("after a replace, the key read back is not the replacement (length %d, error %T)", len(got.Reveal()), err)
	}
}

// TestEachWriteSealsAfresh: two writes of one key leave different bytes.
func TestEachWriteSealsAfresh(t *testing.T) {
	s := storeFor(t, "alice")
	first := readBytes(t, addSentinel(t, s, "cloud"))
	second := readBytes(t, addSentinel(t, s, "cloud"))
	if bytes.Equal(first, second) {
		t.Error("two writes of one key left identical bytes; each write must take a fresh nonce")
	}
}

// TestFileModeIsOwnerOnly: a key file is 0600 when it is created, and when it
// replaces a file that was 0644.
func TestFileModeIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	s := storeFor(t, "alice")
	created := addSentinel(t, s, "cloud")
	wide := filepath.Join(s.dir, "lab"+FileSuffix)
	if err := os.WriteFile(wide, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(wide, 0o644); err != nil {
		t.Fatal(err)
	}
	replaced := addSentinel(t, s, "lab")
	for _, path := range []string{created, replaced} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != FileMode {
			t.Errorf("%s has mode %#o, want %#o", path, got, FileMode)
		}
	}
}

// TestFileHoldsNoPlaintext: the key's bytes appear nowhere in its file.
func TestFileHoldsNoPlaintext(t *testing.T) {
	s := storeFor(t, "alice")
	if bytes.Contains(readBytes(t, addSentinel(t, s, "cloud")), []byte(sentinelKey)) {
		t.Error("the key file holds the key in plain text")
	}
}

// TestRenamedFileOpensUnderItsNewName: nothing binds a file to its provider,
// so a file copied to another provider's name reads as that provider's key.
func TestRenamedFileOpensUnderItsNewName(t *testing.T) {
	s := storeFor(t, "alice")
	sealed := readBytes(t, addSentinel(t, s, "old-name"))
	if err := os.WriteFile(filepath.Join(s.dir, "new-name"+FileSuffix), sealed, FileMode); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Read("new-name"); err != nil || got.Reveal() != sentinelKey {
		t.Errorf("the copied file read as new-name: key matches %t, error %T; want the key", got.Reveal() == sentinelKey, err)
	}
}

// TestUnusableNamesRefused: a name that breaks the provider-name rule is
// refused by every operation, by the rule alone and never the name, and
// nothing is written.
func TestUnusableNamesRefused(t *testing.T) {
	s := storeFor(t, "alice")
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "a\x00b", "../escape",
		"dotted.name", "with space", "naïve", "-lead", "colon:name"} {
		var refused *NameError
		if err := s.Add(name, secret.NewKey(sentinelKey)); !errors.As(err, &refused) {
			t.Errorf("Add(%q) returned a %T, want a name refusal", name, err)
		} else if err.Error() != NameRule {
			t.Errorf("Add(%q) refused with %q, want the rule alone", name, err)
		}
		if _, err := s.Read(name); !errors.As(err, &refused) {
			t.Errorf("Read(%q) returned a %T, want a name refusal", name, err)
		}
		if _, err := s.Remove(name); !errors.As(err, &refused) {
			t.Errorf("Remove(%q) = %v, want a name refusal", name, err)
		}
	}
	if entries, err := os.ReadDir(s.dir); err != nil || len(entries) != 0 {
		t.Errorf("the store directory holds %d entries (err %v), want none", len(entries), err)
	}
}

// TestListAndRemove: List names the stored providers in name order and no
// other file; Remove reports whether a key stood, and a removed key reads as
// absent. A store whose directory does not stand lists none.
func TestListAndRemove(t *testing.T) {
	s := storeFor(t, "alice")
	if names, err := (Store{dir: filepath.Join(s.dir, "absent")}).List(); err != nil || len(names) != 0 {
		t.Errorf("List over no directory = %q, %v; want none", names, err)
	}
	for _, name := range []string{"zeta", "alpha", "mid-key"} {
		addSentinel(t, s, name)
	}
	for _, stray := range []string{"notes.txt", FileSuffix, ".alpha" + FileSuffix + ".x1.tmp", "Mixed" + FileSuffix} {
		if err := os.WriteFile(filepath.Join(s.dir, stray), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(s.dir, "dir"+FileSuffix), 0o755); err != nil {
		t.Fatal(err)
	}
	if names, err := s.List(); err != nil || !slices.Equal(names, []string{"alpha", "mid-key", "zeta"}) {
		t.Errorf("List = %q, %v; want [alpha mid-key zeta]", names, err)
	}
	for _, want := range []bool{true, false} {
		if removed, err := s.Remove("alpha"); err != nil || removed != want {
			t.Errorf("Remove(alpha) = %t, %v; want %t", removed, err, want)
		}
	}
	if _, err := s.Read("alpha"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read of a removed key returned a %T, want not-exist", err)
	}
}

// TestNameCaseSharesOneFile: a key added under a name in any case is stored in
// the file of its lowercase, and read and removed under any other case of it.
func TestNameCaseSharesOneFile(t *testing.T) {
	s := storeFor(t, "alice")
	if err := s.Add("Reaper", secret.NewKey(sentinelKey)); err != nil {
		t.Fatalf("Add(Reaper) failed with a %T", err)
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	if !slices.Equal(files, []string{"reaper" + FileSuffix}) {
		t.Fatalf("the store holds %q, want reaper%s alone", files, FileSuffix)
	}
	if got, err := s.Read("reaper"); err != nil || got.Reveal() != sentinelKey {
		t.Errorf("Read(reaper) after Add(Reaper): key matches %t, error %T; want the key", got.Reveal() == sentinelKey, err)
	}
	if names, err := s.List(); err != nil || !slices.Equal(names, []string{"reaper"}) {
		t.Errorf("List = %q, %v; want [reaper]", names, err)
	}
	if removed, err := s.Remove("REAPER"); err != nil || !removed {
		t.Errorf("Remove(REAPER) = %t, %v; want the key removed", removed, err)
	}
	if entries, err := os.ReadDir(s.dir); err != nil || len(entries) != 0 {
		t.Errorf("after Remove the store holds %d entries (err %v), want none", len(entries), err)
	}
}

// TestStoreErrorsCarryNoKey: every failure the store reports is told apart by
// errors.Is or errors.As, names the file where one is involved, and never
// carries the key.
func TestStoreErrorsCarryNoKey(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fail    func(t *testing.T) error
		is      error
		file    bool
		refusal bool
	}{
		{name: "tampered", is: ErrDoesNotOpen, file: true, fail: func(t *testing.T) error {
			s := storeFor(t, "alice")
			path := addSentinel(t, s, "cloud")
			sealed := readBytes(t, path)
			sealed[len(sealed)-1] ^= 0x01
			if err := os.WriteFile(path, sealed, FileMode); err != nil {
				t.Fatal(err)
			}
			_, err := s.Read("cloud")
			return err
		}},
		{name: "truncated", is: ErrTruncated, file: true, fail: func(t *testing.T) error {
			s := storeFor(t, "alice")
			if err := os.Truncate(addSentinel(t, s, "cloud"), 8); err != nil {
				t.Fatal(err)
			}
			_, err := s.Read("cloud")
			return err
		}},
		{name: "another user's", is: ErrDoesNotOpen, file: true, fail: func(t *testing.T) error {
			s := storeFor(t, "alice")
			addSentinel(t, s, "cloud")
			bob := Store{dir: s.dir, username: func() (string, error) { return "bob", nil }}
			_, err := bob.Read("cloud")
			return err
		}},
		{name: "no username", is: ErrNoUsername, fail: func(t *testing.T) error {
			s := Store{dir: t.TempDir(), username: func() (string, error) { return "", nil }}
			return s.Add("cloud", secret.NewKey(sentinelKey))
		}},
		{name: "unusable name", refusal: true, fail: func(t *testing.T) error {
			return storeFor(t, "alice").Add("a/b", secret.NewKey(sentinelKey))
		}},
		{name: "key passed as the name", refusal: true, fail: func(t *testing.T) error {
			return storeFor(t, "alice").Add(sentinelKey+"/x", secret.NewKey("k"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fail(t)
			if err == nil {
				t.Fatal("no error, want a failure")
			}
			if strings.Contains(err.Error(), sentinelKey) {
				t.Error("the error's text carries the key")
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("the error is not %q", tc.is)
			}
			var fileErr *FileError
			if tc.file && (!errors.As(err, &fileErr) || !strings.HasSuffix(fileErr.Path, "cloud"+FileSuffix)) {
				t.Error("the error names no key file, want the file that failed")
			}
			var refused *NameError
			if tc.refusal != errors.As(err, &refused) {
				t.Errorf("the error is a name refusal: %t, want %t", !tc.refusal, tc.refusal)
			}
		})
	}
}
