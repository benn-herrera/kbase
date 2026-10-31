package secret_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdlog "log"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/log"
	"kbase/internal/secret"
)

const plaintext = "sk-SECRET-MUST-NOT-RENDER-0001"

// holder is a struct carrying a key the way configuration does: exported, so
// fmt, encoders and loggers reach the Key's own methods.
type holder struct {
	Name string
	Key  secret.Key
}

// hidden carries the key in an unexported field, where fmt cannot call the
// Key's methods and prints its fields instead.
type hidden struct {
	name string
	key  secret.Key
}

func logged(t *testing.T, args ...any) string {
	t.Helper()
	var buf bytes.Buffer
	lg, closer, err := log.New(log.Options{Level: log.LevelDebug, Console: &buf})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := closer.Close(); err != nil {
			t.Error(err)
		}
	}()
	lg.Info("rendered", args...)
	return buf.String()
}

func marshalled(t *testing.T, marshal func(any) ([]byte, error), v any) string {
	t.Helper()
	b, err := marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jsonText is v's JSON decoded back, since json.Marshal escapes the <> of
// Redacted.
func jsonText(t *testing.T, v any) string {
	t.Helper()
	var back any
	if err := json.Unmarshal([]byte(marshalled(t, json.Marshal, v)), &back); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(back)
}

// TestKeyRendersRedacted: every way kbase or the standard library renders a
// value — fmt's verbs, a logger, an encoder — writes Redacted for a Key and
// for a struct holding one, never the key.
func TestKeyRendersRedacted(t *testing.T) {
	k := secret.NewKey(plaintext)
	h := holder{Name: "lab", Key: k}
	var stdBuf bytes.Buffer
	std := stdlog.New(&stdBuf, "", 0)
	std.Printf("%v %+v", k, h)

	for _, tc := range []struct {
		name, got string
		redacted  bool // whether the rendering must carry Redacted, not only omit the key
	}{
		{"%v", fmt.Sprintf("%v", k), true},
		{"%+v", fmt.Sprintf("%+v", k), true},
		{"%#v", fmt.Sprintf("%#v", k), true},
		{"%s", fmt.Sprintf("%s", k), true},
		{"%q", fmt.Sprintf("%q", k), true},
		{"%x", fmt.Sprintf("%x", k), true},
		{"%d", fmt.Sprintf("%d", k), true},
		{"Sprint", fmt.Sprint(k), true},
		{"Sprint of a pointer", fmt.Sprint(&k), true},
		{"struct %v", fmt.Sprintf("%v", h), true},
		{"struct %+v", fmt.Sprintf("%+v", h), true},
		{"struct %#v", fmt.Sprintf("%#v", h), true},
		{"struct pointer %+v", fmt.Sprintf("%+v", &h), true},
		{"map value %v", fmt.Sprintf("%v", map[string]secret.Key{"lab": k}), true},
		{"unexported field %+v", fmt.Sprintf("%+v", hidden{name: "lab", key: k}), false},
		{"unexported field %#v", fmt.Sprintf("%#v", hidden{name: "lab", key: k}), false},
		{"error wrapping", fmt.Errorf("dial %v: %w", h, fmt.Errorf("refused")).Error(), true},
		{"log value", logged(t, "key", k), true},
		{"log struct value", logged(t, "endpoint", h), true},
		{"standard log", stdBuf.String(), true},
		{"JSON", jsonText(t, k), true},
		{"JSON struct", jsonText(t, h), true},
		{"YAML", marshalled(t, yaml.Marshal, k), true},
		{"YAML struct", marshalled(t, yaml.Marshal, h), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.got, plaintext) {
				t.Fatalf("the rendering carries the key (%d bytes)", len(plaintext))
			}
			if tc.redacted && !strings.Contains(tc.got, secret.Redacted) {
				t.Errorf("the rendering %q carries no %s", tc.got, secret.Redacted)
			}
		})
	}
}

func TestKeyRevealsItsText(t *testing.T) {
	for _, tc := range []struct {
		name   string
		key    secret.Key
		want   string
		isZero bool
	}{
		{"a key", secret.NewKey(plaintext), plaintext, false},
		{"the empty text", secret.NewKey(""), "", true},
		{"the zero Key", secret.Key{}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.key.Reveal(); got != tc.want {
				t.Errorf("Reveal is %d bytes, want %d", len(got), len(tc.want))
			}
			if got := tc.key.IsZero(); got != tc.isZero {
				t.Errorf("IsZero = %t, want %t", got, tc.isZero)
			}
		})
	}
}
