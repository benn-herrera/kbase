package main

import (
	"strings"
	"testing"
)

// TestSafeBaseURL: run.json is by design an artifact an operator shares as
// evidence, and a base URL with userinfo in it is a legal providers.toml
// value. The key never reaches the record; this is the other way a credential
// could have.
func TestSafeBaseURL(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"no userinfo", "https://host/v1", "https://host/v1"},
		{"a user and a password", "https://user:secret@host/v1", "https://host/v1"},
		{"a user alone", "https://user@host/v1", "https://host/v1"},
		{"not a URL at all", "://nonsense", "://nonsense"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeBaseURL(tc.in); got != tc.want {
				t.Errorf("safeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.Contains(safeBaseURL(tc.in), "secret") {
				t.Error("a credential survived into the recorded base URL")
			}
		})
	}
}
