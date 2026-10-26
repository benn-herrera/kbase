package format300

import (
	"regexp"
	"strings"
)

// PyWhitespace is the body of the class Python's \s and str.isspace() share,
// for use inside a bracket expression.
const PyWhitespace = `\t\n\v\f\r\x1c-\x1f \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var pyEscapes = strings.NewReplacer(`\s`, "["+PyWhitespace+"]", `\S`, "[^"+PyWhitespace+"]", `\d`, `[0-9]`)

// PyRE compiles a pattern written in Python's spelling of \s, \S and \d,
// none of them inside a bracket expression. \d stays ASCII.
func PyRE(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pyEscapes.Replace(pattern))
}
