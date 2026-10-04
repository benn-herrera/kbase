package prepass

import (
	"maps"
	"slices"
	"testing"
)

func TestTheoremDisplayNames(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         map[string]string
	}{
		{"plain", `\newtheorem{claimbox}{Result}`, map[string]string{"claimbox": "Result"}},
		{"starred", `\newtheorem*{lem*}{Lemma}`, map[string]string{"lem*": "Lemma"}},
		{"shared counter", `\newtheorem{corollary}[theorem]{Corollary}`, map[string]string{"corollary": "Corollary"}},
		{"subordinate counter", `\newtheorem{theorem}{Theorem}[section]`, map[string]string{"theorem": "Theorem"}},
		{"translation macro", `\newtheorem{theorem}{\protect\theoremname}`, map[string]string{"theorem": "Theorem"}},
		{"bare translation macro", `\newtheorem{lem}{\lemmaname}`, map[string]string{"lem": "Lemma"}},
		{"unresolvable markup", `\newtheorem{thm}{\textbf{Theorem}}`, map[string]string{}},
		{"space in name", `\newtheorem{tex}{Test example}`, map[string]string{"tex": "Test example"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Apply(tc.source)
			if !maps.Equal(got.TheoremNames, tc.want) {
				t.Errorf("names = %v, want %v", got.TheoremNames, tc.want)
			}
		})
	}
}

func TestStripEnvironmentDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		found              int
		unread             []int
	}{
		{"simple", "a\\newenvironment{foo}{\\begin{x}}{\\end{x}}b", "ab", 1, nil},
		{"renew with args", "a\\renewenvironment{foo}[1][x]{B #1}{E}b", "ab", 1, nil},
		{"starred", "a\\newenvironment*{foo}{B}{E}b", "ab", 1, nil},
		{"multi-line nested braces", "a\\newenvironment{foo}\n  {\\par{\\bf X}}\n  {\\par}b", "ab", 1, nil},
		{"comment between parts", "a\\newenvironment{elabeling}[2][]%\n{B}{E}b", "ab", 1, nil},
		{"brace in comment", "a\\newenvironment{foo}{B % }\n}{E}b", "ab", 1, nil},
		{"escaped brace", "a\\newenvironment{foo}{\\{}{E}b", "ab", 1, nil},
		{"unreadable left as written", "x\n\\newenvironment{foo}{B}\nrest", "x\n\\newenvironment{foo}{B}\nrest", 0, []int{2}},
		{"none", "plain text", "plain text", 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Apply(tc.source)
			if got.Source != tc.want {
				t.Errorf("source = %q, want %q", got.Source, tc.want)
			}
			strip := got.Censuses[1]
			if strip.Found != tc.found {
				t.Errorf("found = %d, want %d", strip.Found, tc.found)
			}
			var lines []int
			for _, u := range strip.Unread {
				lines = append(lines, u.Line)
			}
			if !slices.Equal(lines, tc.unread) {
				t.Errorf("unread lines = %v, want %v", lines, tc.unread)
			}
		})
	}
}
