package distill

import (
	"reflect"
	"testing"
)

func TestDestinations(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"an inline link", "see [the guide](guide/sync.md) for more\n", []string{"guide/sync.md"}},
		{"an image", "![logo](../img/logo.png)\n", []string{"../img/logo.png"}},
		{"two on one line", "[a](a.md) and [b](b.md)\n", []string{"a.md", "b.md"}},
		{"a title after the destination", `[a](a.md "The A")` + "\n", []string{"a.md"}},
		{"an angle-bracketed destination", "[a](<a b.md>)\n", []string{"a b.md"}},
		{"a fragment", "[a](a.md#top)\n", []string{"a.md#top"}},
		{"a bare anchor", "[a](#top)\n", []string{"#top"}},
		{"a reference definition", "[label]: guide/sync.md\n", []string{"guide/sync.md"}},
		{"an indented reference definition", "   [label]: a.md\n", []string{"a.md"}},
		{"parentheses inside the destination", "[a](a(1).md)\n", []string{"a(1).md"}},
		{"an escaped bracket opens no link", `\](notalink)` + "\n", nil},
		{"a fenced block is skipped", "```\n[a](a.md)\n```\n[b](b.md)\n", []string{"b.md"}},
		{"a tilde fence is skipped", "~~~\n[a](a.md)\n~~~\n", nil},
		{"an unterminated destination on a line", "[a](\n", nil},
		{"no links at all", "plain text\nmore text\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.src)
			var got []string
			for _, d := range Destinations(src) {
				got = append(got, d.Text(src))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Destinations() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Every destination the scanner reports must be readable back out of the
// source at the offsets it reported: the whole rebase depends on the range
// naming the destination and nothing around it.
func TestDestinationOffsetsAreExact(t *testing.T) {
	src := []byte("intro [a](one.md) middle ![b](two.png) end\n\n[c]: three.md\n")
	for _, d := range Destinations(src) {
		if d.Start < 0 || d.End > len(src) || d.End <= d.Start {
			t.Fatalf("destination range [%d,%d) is not a range of %d bytes", d.Start, d.End, len(src))
		}
		text := d.Text(src)
		if text != "one.md" && text != "two.png" && text != "three.md" {
			t.Fatalf("the range [%d,%d) reads %q, which is not a destination in the source", d.Start, d.End, text)
		}
	}
}

func TestIsExternal(t *testing.T) {
	tests := []struct {
		dest string
		want bool
	}{
		{"https://example.com/x", true},
		{"http://example.com", true},
		{"mailto:a@b.example", true},
		{"//cdn.example.com/x.js", true},
		{"#section", true},
		{"", true},
		{"guide/sync.md", false},
		{"../nested.md", false},
		{"./setup.md", false},
		{"setup", false},
		{"a.md#top", false},
	}
	for _, tt := range tests {
		t.Run(tt.dest, func(t *testing.T) {
			if got := IsExternal(tt.dest); got != tt.want {
				t.Errorf("IsExternal(%q) = %t, want %t", tt.dest, got, tt.want)
			}
		})
	}
}
