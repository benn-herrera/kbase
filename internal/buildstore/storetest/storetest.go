// Package storetest is test support the build and buildstore tests share: a
// repository made as git init makes one, git's identity and the state store's
// home under a test's own directory, and the reading of a result document's
// fields. It imports no build package, so buildstore's in-package tests can
// import it; nothing outside a test does.
package storetest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/result"
)

// WriteFile writes text at path, making its directory.
func WriteFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// GitEnv is git's identity, its global configuration and the state store's
// home, all under base.
func GitEnv(t *testing.T, base string) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(base, "gitconfig"), "XDG_STATE_HOME": filepath.Join(base, "xdg"),
	} {
		t.Setenv(k, v)
	}
}

// MakeRepo is the repository git init makes, without starting a process.
func MakeRepo(t *testing.T, repo string) {
	t.Helper()
	for _, d := range []string{".git/objects", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	WriteFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
}

// Field is the value fields hold under key, nil where they hold none.
func Field(fields []result.Field, key string) any {
	for _, f := range fields {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// Carries is whether the document holds key at all, an absent key told apart
// from one holding an empty value.
func Carries(fields []result.Field, key string) bool {
	return slices.ContainsFunc(fields, func(f result.Field) bool { return f.Key == key })
}

// FieldKeys is the keys of a field list or a record, in document order.
func FieldKeys(fields []result.Field) []string {
	keys := make([]string, len(fields))
	for i, f := range fields {
		keys[i] = f.Key
	}
	return keys
}
