// Package secret is a value that renders as "<redacted>" wherever it is
// printed, logged or marshalled; Reveal alone hands back its text.
package secret

import (
	"fmt"
	"io"
)

// Redacted is every rendering of a Key.
const Redacted = "<redacted>"

// Key is an API key. fmt's verbs, a text or JSON or YAML encoding of it, and
// a log value of it are all Redacted; Reveal returns the key itself. The zero
// Key is no key.
//
// The text is held behind a pointer because fmt cannot call a method on a
// value it reaches through an unexported struct field and prints the value's
// fields instead: there it prints the pointer's address, never the key.
// Key is not comparable, since == on that pointer would compare identity;
// IsZero says whether there is a key.
type Key struct {
	_    [0]func()
	text *string
}

// NewKey is the key s; NewKey("") is the zero Key.
func NewKey(s string) Key {
	if s == "" {
		return Key{}
	}
	return Key{text: &s}
}

// Reveal is the key's text, "" for the zero Key.
func (k Key) Reveal() string {
	if k.text == nil {
		return ""
	}
	return *k.text
}

// IsZero is whether k holds no key.
func (k Key) IsZero() bool { return k.text == nil }

func (k Key) String() string   { return Redacted }
func (k Key) GoString() string { return Redacted }

// Format writes Redacted for every verb, flags and width ignored.
func (k Key) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, Redacted) }

func (k Key) MarshalText() ([]byte, error) { return []byte(Redacted), nil }
