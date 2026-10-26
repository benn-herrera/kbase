package format300

import (
	"strings"
	"testing"
)

func TestEncodeFormatStamp(t *testing.T) {
	if got, want := string(EncodeFormatStamp("3.0.0")), "kb-format: \"3.0.0\"\n"; got != want {
		t.Errorf("EncodeFormatStamp = %q, want %q", got, want)
	}
}

func TestDecodeFormatStamp(t *testing.T) {
	for _, tc := range []struct {
		name, data, want, refused string // refused: a substring of the error
	}{
		{name: "quoted", data: "kb-format: \"3.0.0\"\n", want: "3.0.0"},
		{name: "plain", data: "kb-format: 3.0.0\n", want: "3.0.0"},
		{name: "no trailing break", data: "kb-format: \"3.0.0\"", want: "3.0.0"},
		{name: "another key", data: "kb-format: \"3.0.0\"\nkind: entry-point\n", refused: `the key "kind"`},
		{name: "only another key", data: "kind: entry-point\n", refused: `the key "kind"`},
		{name: "a repeated key", data: "kb-format: \"3.0.0\"\nkb-format: \"3.0.1\"\n", refused: "repeats"},
		{name: "no document", data: "", refused: "one block mapping"},
		{name: "an empty mapping", data: "{}\n", refused: "carries no kb-format"},
		{name: "a list value", data: "kb-format: [1]\n", refused: "is not a version"},
		{name: "a bool value", data: "kb-format: true\n", refused: "is not a version"},
		{name: "an empty value", data: "kb-format: \"\"\n", refused: "is not a version"},
		{name: "not a mapping", data: "- 3.0.0\n", refused: "one block mapping"},
		{name: "a flow mapping", data: "{kb-format: \"3.0.0\"}\n", refused: "one block mapping"},
		{name: "unparseable", data: "kb-format: [1\n", refused: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeFormatStamp([]byte(tc.data))
			if tc.want != "" {
				if err != nil || got != tc.want {
					t.Errorf("DecodeFormatStamp = %q, %v; want %q", got, err, tc.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refused) {
				t.Errorf("DecodeFormatStamp(%q) = %q, %v; want an error naming %q", tc.data, got, err, tc.refused)
			}
		})
	}
}

func TestFormatStampRoundTrip(t *testing.T) {
	for _, version := range []string{"1.0.0", "2.0.1", "3.0.0"} {
		if got, err := DecodeFormatStamp(EncodeFormatStamp(version)); err != nil || got != version {
			t.Errorf("round trip of %s = %q, %v", version, got, err)
		}
	}
}
