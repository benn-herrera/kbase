// Package asks is everything kbase puts to a model: kb_tools' prompt
// templates and fragments, imported unchanged and embedded; the composer that
// fills their slots; the letter-ask seam every claim-graph decision crosses;
// the asks of each kind; and the Caller that puts a composed prompt to a
// provider under its system fragment.
package asks

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

// shelf is the imported templates and fragments, byte for byte as upstream
// wrote them. The top level holds what something dispatches, fragments/ what
// something splices into one.
//
//go:embed prompt-templates
var shelf embed.FS

const (
	shelfDir       = "prompt-templates"
	fragmentsDir   = "fragments"
	templateSuffix = ".tmpl.md"
	dynamicPrefix  = "dyn."
)

var (
	// slotRE is one slot, whole, its namespace included: @!name!@ is the
	// composer's to fill, @!dyn.name!@ the caller's.
	slotRE = regexp.MustCompile(`@!((?:dyn\.)?[a-z][a-z0-9]*(?:-[a-z0-9]+)*)!@`)
	// delimiterRE is either delimiter, read on a line its well-formed slots
	// have been taken out of.
	delimiterRE = regexp.MustCompile(`@!|!@`)
)

// The composer-resolved fragments. A template names none of them: each is a
// call's whole system prompt, rendered alone.
const (
	ReaderSystem   = "reader-system"
	OverviewSystem = "overview-system"
)

var fragmentSlots = []string{ReaderSystem, OverviewSystem}

// alternativeSlots are the caller-chosen pieces, per slot the choices a
// caller may name; "" is the empty fill, where the slot's absence is itself
// an answer.
var alternativeSlots = map[string][]string{
	"correction":         {"", "letter-correction"},
	"classify-options":   {"classify-options-three", "classify-options-two"},
	"passage-correction": {"", "overview-correction"},
}

// ComposeError is a template and its caller disagreeing: a defect in kbase,
// never an outcome of the build it stops.
type ComposeError struct{ Template, Detail string }

func (e ComposeError) Error() string { return "asks: " + e.Template + ": " + e.Detail }

func fragmentFile(name string) string { return fragmentsDir + "/" + name + templateSuffix }

func load(name string) (string, error) {
	b, err := shelf.ReadFile(path.Join(shelfDir, name))
	if err != nil {
		return "", ComposeError{name, "no such template on the shelf"}
	}
	return string(b), nil
}

// slotsOf is text's slots by spelling, in order, once each; a delimiter that
// does not parse as a slot is refused by line.
func slotsOf(text, source string) ([]string, error) {
	var stray []string
	for i, line := range strings.Split(text, "\n") {
		if delimiterRE.MatchString(slotRE.ReplaceAllString(line, "")) {
			stray = append(stray, fmt.Sprint(i+1))
		}
	}
	if len(stray) > 0 {
		return nil, ComposeError{source, "stray slot delimiter on line(s) " + strings.Join(stray, ", ") + "; every @! opens an @!slot!@ and every !@ closes one"}
	}
	var names []string
	for _, m := range slotRE.FindAllStringSubmatch(text, -1) {
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	return names, nil
}

// fill substitutes every slot once; a filled value is never rescanned.
func fill(text string, values map[string]string) string {
	return slotRE.ReplaceAllStringFunc(text, func(s string) string { return values[s[2:len(s)-2]] })
}

// Render composes the named template: every slot it and what it splices
// declare filled, every value supplied used. slots fills the dyn. slots, keyed
// by bare name; constants is the pool a bare slot draws on where it names an
// entry; alternatives names one registered choice per alternative slot the
// template declares, "" for the empty fill. A fragment or an alternative
// resolves one level deep.
func Render(name string, slots, constants, alternatives map[string]string) (string, error) {
	return render(load, name, slots, constants, alternatives)
}

// render is Render over the templates load reads by shelf path.
func render(load func(name string) (string, error), name string, slots, constants, alternatives map[string]string) (string, error) {
	text, err := load(name)
	if err != nil {
		return "", err
	}
	fields, err := slotsOf(text, name)
	if err != nil {
		return "", err
	}
	var declared []string
	for _, f := range fields {
		if _, ok := alternativeSlots[f]; ok {
			declared = append(declared, f)
		}
	}
	for _, f := range declared {
		if _, ok := alternatives[f]; !ok {
			return "", ComposeError{name, "no alternative chosen for slot " + f}
		}
	}
	for f, pick := range alternatives {
		if !slices.Contains(declared, f) {
			return "", ComposeError{name, "an alternative chosen for slot " + f + ", which the template does not declare"}
		}
		if !slices.Contains(alternativeSlots[f], pick) {
			return "", ComposeError{name, fmt.Sprintf("@!%s!@ takes one of %q, not %q", f, alternativeSlots[f], pick)}
		}
	}

	spliced := map[string]string{}
	splicedFields := map[string][]string{}
	required := map[string]bool{}
	for _, f := range fields {
		var source string
		switch {
		case slices.Contains(fragmentSlots, f):
			source = fragmentFile(f)
		case slices.Contains(declared, f):
			if alternatives[f] == "" {
				spliced[f] = ""
				continue
			}
			source = fragmentFile(alternatives[f])
		default:
			required[f] = true
			continue
		}
		body, err := load(source)
		if err != nil {
			return "", err
		}
		inner, err := slotsOf(body, source)
		if err != nil {
			return "", err
		}
		for _, s := range inner {
			if _, alt := alternativeSlots[s]; alt || slices.Contains(fragmentSlots, s) {
				return "", ComposeError{source, "@!" + s + "!@ names a slot the composer would have to expand in turn; composition expands one level"}
			}
			required[s] = true
		}
		spliced[f], splicedFields[f] = body, inner
	}

	values := map[string]string{}
	for f := range required {
		bare, dynamic := strings.CutPrefix(f, dynamicPrefix)
		if dynamic {
			v, ok := slots[bare]
			if !ok {
				return "", ComposeError{name, "the caller supplied no value for @!" + f + "!@"}
			}
			values[f] = v
			continue
		}
		if _, ok := slots[f]; ok {
			return "", ComposeError{name, "the caller supplied " + f + ", which the composer fills"}
		}
		v, ok := constants[f]
		if !ok {
			return "", ComposeError{name, "unfilled slot @!" + f + "!@"}
		}
		values[f] = v
	}
	for k := range slots {
		if !required[dynamicPrefix+k] {
			return "", ComposeError{name, "supplied slot " + k + ", which the template never uses"}
		}
	}

	filled := map[string]string{}
	for _, f := range fields {
		if body, ok := spliced[f]; ok {
			inner := map[string]string{}
			for _, s := range splicedFields[f] {
				inner[s] = values[s]
			}
			filled[f] = fill(body, inner)
			continue
		}
		filled[f] = values[f]
	}
	return fill(text, filled), nil
}

// System is the named fragment rendered whole: a call's system prompt.
func System(name string) (string, error) {
	if !slices.Contains(fragmentSlots, name) {
		return "", ComposeError{name, "no fragment of that name is a system prompt"}
	}
	return Render(fragmentFile(name), nil, nil, nil)
}

// shelfFiles is every template and fragment on the shelf, by its path under
// prompt-templates/, sorted.
func shelfFiles() ([]string, error) {
	var out []string
	err := fs.WalkDir(shelf, shelfDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		out = append(out, strings.TrimPrefix(p, shelfDir+"/"))
		return nil
	})
	slices.Sort(out)
	return out, err
}

// Lint is every finding over the shelf, in file order: a line spelling
// something prohibited names, by its label, and a delimiter that does not
// parse as a slot. Empty is a pass.
func Lint(prohibited map[string]*regexp.Regexp) ([]string, error) {
	files, err := shelfFiles()
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(prohibited))
	for l := range prohibited {
		labels = append(labels, l)
	}
	slices.Sort(labels)
	var findings []string
	for _, f := range files {
		text, err := load(f)
		if err != nil {
			return nil, err
		}
		for i, line := range strings.Split(text, "\n") {
			for _, l := range labels {
				if prohibited[l].MatchString(line) {
					findings = append(findings, fmt.Sprintf("%s:%d: names %q — prohibited in a prompt template", f, i+1, l))
				}
			}
		}
		if _, err := slotsOf(text, f); err != nil {
			findings = append(findings, err.Error())
		}
	}
	return findings, nil
}
