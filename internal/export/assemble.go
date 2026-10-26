package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
)

// GenTexDir is the out directory's subdirectory kbase owns: every file the
// export writes but the preamble lives under it.
const GenTexDir = "gen-tex"

// PreambleFile is the author's preamble, in the out directory beside
// GenTexDir; the template loads it from there.
const PreambleFile = "preamble.tex"

// The export's files, by name relative to GenTexDir.
const (
	// streamFrontFile is the stream's front matter; each volume's part of the
	// stream is a file of its own, volumeStreamFile.
	streamFrontFile = "kbase-export-front.md"
	xrefFile        = "kbase-xref.json"
	filterFile      = "kbase-export.lua"
	templateFile    = "kbase-export.latex"
	assetsDir       = "assets"

	// DefaultsFile is the pandoc defaults file naming every other input; the
	// verb runs pandoc over it.
	DefaultsFile = "defaults.yaml"
	// OutputFile is the LaTeX pandoc writes.
	OutputFile = "main.tex"
	// FindingsFile is where the filter writes its findings, a JSON list of
	// {form, path, reason}, for the verb to merge into the report.
	FindingsFile = "kbase-export-findings.json"
	// ReportFile is the export report the verb writes.
	ReportFile = "kbase-export-report.yaml"
	// ManifestFile lists every file the export owns in GenTexDir.
	ManifestFile = "kbase-export.yaml"
)

// UnknownVolumesError is a volume selection naming directories that are no
// volume the walk reached.
type UnknownVolumesError struct {
	Names []string
}

func (e UnknownVolumesError) Error() string {
	return "no volume of the entry point's list is " + strings.Join(e.Names, ", ")
}

// xrefTables is kbase-xref.json, the tables the filter reads; the schema is
// stated at the head of kbase-export.lua.
type xrefTables struct {
	Documents    map[string]xrefDocument `json:"documents"`
	Paths        map[string]string       `json:"paths"`
	Volumes      []xrefVolume            `json:"volumes"`
	Theorems     []string                `json:"theorems"`
	Bibliography map[string]string       `json:"bibliography"`
	Report       string                  `json:"report"`
	Assets       map[string]string       `json:"assets"`
	EntryPoint   string                  `json:"entry_point"`
	Streams      []string                `json:"streams"`
}

type xrefDocument struct {
	Path   string `json:"path"`
	Volume int    `json:"volume"`
	Rank   int    `json:"rank"`
}

type xrefVolume struct {
	Label string `json:"label"`
	Title string `json:"title"`
}

// pandocDefaults is defaults.yaml. Its paths are relative: pandoc runs in
// GenTexDir, for the verb as for the author.
type pandocDefaults struct {
	From             string           `yaml:"from"`
	To               string           `yaml:"to"`
	Standalone       bool             `yaml:"standalone"`
	Wrap             string           `yaml:"wrap"`
	TopLevelDivision string           `yaml:"top-level-division"`
	Template         string           `yaml:"template"`
	InputFiles       []string         `yaml:"input-files"`
	FileScope        bool             `yaml:"file-scope"`
	Filters          []string         `yaml:"filters"`
	OutputFile       string           `yaml:"output-file"`
	Metadata         defaultsMetadata `yaml:"metadata"`
	Variables        defaultsVars     `yaml:"variables"`
}

// defaultsVars is the template variables the defaults file sets: the kbase
// version the template's generation comment names.
type defaultsVars struct {
	KBaseVersion string `yaml:"kbase-version"`
}

type defaultsMetadata struct {
	KBaseXref string `yaml:"kbase-xref"`
	Title     string `yaml:"title,omitempty"`
}

// assemble is GenTexDir's inputs for the export of tree, read from src, by
// GenTexDir-relative slash path: the stream's files, the tables, the defaults
// file, the filter, the template, and every non-Markdown file under an
// exported volume's directory under assets/. Every volume of tree is exported
// (see selectVolumes); the defaults file names kbaseVersion for the template.
// The same KB and version give the same bytes.
//
// The stream is the front matter's file, then one file per volume, in order;
// pandoc reads them file-scoped, so a footnote label is local to its volume.
// The front matter is the entry point's prose, its first heading taken as the
// document's title and its volume list removed; a volume's file is its index
// and each document of its spine, navigation stripped. Each document's first
// heading outside code fences carries its label as its id; a document with
// none opens with a heading naming it by its path.
func assemble(src *kb.Source, tree Tree, kbaseVersion string) (map[string][]byte, error) {
	entry, err := src.ReadText(src.KBPath(tree.EntryPoint))
	if err != nil {
		return nil, fmt.Errorf("reading the entry point: %w", err)
	}
	front, title := frontMatter(tree.EntryPoint, entry)
	x := xrefTables{
		Documents:  map[string]xrefDocument{},
		Paths:      map[string]string{},
		Volumes:    []xrefVolume{},
		Report:     FindingsFile,
		EntryPoint: tree.EntryPoint,
		Streams:    []string{streamFrontFile},
	}
	files := map[string][]byte{streamFrontFile: []byte(joinParts([]string{front}))}
	theorems := map[string]bool{}
	addTheorems(theorems, front)
	for i, v := range tree.Volumes {
		x.Volumes = append(x.Volumes, xrefVolume{Label: v.Label, Title: v.Title})
		var parts []string
		index := Document{Path: path.Join(v.Dir, kb.IndexFile), Label: v.Label}
		for _, d := range append([]Document{index}, v.Documents...) {
			text, err := src.ReadText(src.KBPath(d.Path))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", d.Path, err)
			}
			body := Strip(d.Path, text)
			addTheorems(theorems, body)
			parts = append(parts, labelDocument(d, body))
			if d.Label != "" {
				x.Documents[d.Label] = xrefDocument{Path: d.Path, Volume: i + 1, Rank: d.Rank}
				x.Paths[d.Path] = d.Label
			}
		}
		name := volumeStreamFile(v.Label)
		x.Streams = append(x.Streams, name)
		files[name] = []byte(joinParts(parts))
	}
	x.Theorems = slices.Sorted(maps.Keys(theorems))
	if x.Theorems == nil {
		x.Theorems = []string{}
	}
	if x.Bibliography, err = bibliography(src, tree); err != nil {
		return nil, err
	}
	assets, err := volumeAssets(src, tree)
	if err != nil {
		return nil, err
	}
	x.Assets = map[string]string{}
	for p, data := range assets {
		out := assetsDir + "/" + p
		x.Assets[p] = out
		files[out] = data
	}

	var xref bytes.Buffer
	enc := json.NewEncoder(&xref)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(x); err != nil {
		return nil, fmt.Errorf("encoding %s: %w", xrefFile, err)
	}
	defaults, err := yamlBytes(pandocDefaults{
		From:             "gfm+attributes-gfm_auto_identifiers",
		To:               "latex",
		Standalone:       true,
		Wrap:             "none",
		TopLevelDivision: "chapter",
		Template:         templateFile,
		InputFiles:       x.Streams,
		FileScope:        true,
		Filters:          []string{filterFile},
		OutputFile:       OutputFile,
		Metadata:         defaultsMetadata{KBaseXref: xrefFile, Title: title},
		Variables:        defaultsVars{KBaseVersion: kbaseVersion},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding %s: %w", DefaultsFile, err)
	}
	files[xrefFile] = xref.Bytes()
	files[DefaultsFile] = defaults
	files[filterFile] = LuaFilter
	files[templateFile] = LaTeXTemplate
	return files, nil
}

// selectVolumes is tree with only the volumes whose directories names lists,
// every volume where names is empty, each in entry-point order, renumbered by
// its place among them and its labels minted again under that number. A name
// that is no volume's directory is an UnknownVolumesError naming every such
// name.
func selectVolumes(tree Tree, names []string) (Tree, error) {
	want := map[string]bool{}
	for _, name := range names {
		want[path.Clean(name)] = true
	}
	var kept []Volume
	for _, v := range tree.Volumes {
		if len(names) > 0 && !want[v.Dir] {
			continue
		}
		delete(want, v.Dir)
		n := len(kept) + 1
		v.Label = VolumeLabel(n)
		v.Documents = slices.Clone(v.Documents)
		for i, d := range v.Documents {
			if d.Label != "" {
				v.Documents[i].Label = DocumentLabel(n, v.Dir, d.Path)
			}
		}
		kept = append(kept, v)
	}
	if len(want) > 0 {
		return Tree{}, UnknownVolumesError{Names: slices.Sorted(maps.Keys(want))}
	}
	tree.Volumes = kept
	return tree, nil
}

// frontMatter is the entry point's prose, the stream's front matter, and the
// document's title, its first heading. The title's line goes, and so does the
// navigation: the up-link line, the volume list's lines, and each heading
// whose section held list lines and nothing else but blank lines and headings
// removed the same way. Every heading left is marked unnumbered, so it
// typesets before the first chapter without a number.
func frontMatter(entryPoint, text string) (prose, title string) {
	lines, uplink, children := kb.ChildListLines(entryPoint, text)
	levels := headingLevels(lines)
	drop := make([]bool, len(lines))
	for i := range lines {
		drop[i] = i == uplink || children[i] != nil
	}
	if at := slices.IndexFunc(levels, func(l int) bool { return l > 0 }); at >= 0 {
		title, drop[at] = kb.FirstHeading(lines[at]), true
	}
	for h := len(lines) - 1; h >= 0; h-- {
		if levels[h] == 0 || drop[h] {
			continue
		}
		emptied, holds := false, false
		for i := h + 1; i < len(lines) && (levels[i] == 0 || levels[i] > levels[h]); i++ {
			switch {
			case drop[i]:
				emptied = true
			case strings.TrimSpace(lines[i]) != "":
				holds = true
			}
		}
		drop[h] = emptied && !holds
	}
	var out []string
	for i, line := range lines {
		switch {
		case drop[i]:
		case levels[i] > 0:
			out = append(out, strings.TrimRight(line, " \t")+" {.unnumbered}")
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n"), title
}

// headingLevels is, for each of lines, its heading level outside code
// fences, 0 for a line that is no heading.
func headingLevels(lines []string) []int {
	levels := make([]int, len(lines))
	for i, line := range kb.SplitLines(kb.StripCodeFences(strings.Join(lines, "\n"))) {
		if kb.FirstHeading(line) != "" {
			levels[i] = len(line) - len(strings.TrimLeft(line, "#"))
		}
	}
	return levels
}

// labelDocument is body with its first heading carrying d's label as its id,
// or, where it has no heading, opening with one naming d by its path.
func labelDocument(d Document, body string) string {
	id := ""
	if d.Label != "" {
		id = " {#" + d.Label + "}"
	}
	lines, at := firstHeadingLine(body)
	if at < 0 {
		return "# " + pathTitle(d.Path) + id + "\n\n" + body
	}
	lines[at] = strings.TrimRight(lines[at], " \t") + id
	return strings.Join(lines, "\n")
}

// firstHeadingLine is body's lines and the index of its first heading line
// outside code fences, -1 where it has none.
func firstHeadingLine(body string) ([]string, int) {
	lines := kb.SplitLines(body)
	for i, line := range kb.SplitLines(kb.StripCodeFences(body)) {
		if kb.FirstHeading(line) != "" {
			return lines, i
		}
	}
	return lines, -1
}

// pathTitle names a document by its path: an index by its directory, any
// other document by its file name less the extension.
func pathTitle(p string) string {
	if path.Base(p) == kb.IndexFile {
		return path.Base(path.Dir(p))
	}
	return strings.TrimSuffix(path.Base(p), path.Ext(p))
}

// addTheorems adds every display name a label line of body carries, outside
// code fences.
func addTheorems(names map[string]bool, body string) {
	for _, line := range kb.SplitLines(kb.StripCodeFences(body)) {
		if name, ok := kb.LabelLine(line); ok {
			names[name] = true
		}
	}
}

// volumeStreamFile is the stream file of the volume labelled label.
func volumeStreamFile(label string) string {
	return "kbase-export-" + label + ".md"
}

// joinParts is a stream file: every non-blank part, blank lines between, so
// no construct of one part runs into the next.
func joinParts(parts []string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, strings.Trim(p, "\n"))
		}
	}
	return strings.Join(kept, "\n\n") + "\n"
}

// bibliography is every citation key's entry body: each exported volume's
// references.md entries, and, where a volume has none, every work record,
// each a work's title under its key. The first entry of a key stands.
func bibliography(src *kb.Source, tree Tree) (map[string]string, error) {
	out := map[string]string{}
	add := func(key, body string) {
		if _, taken := out[key]; !taken && key != "" {
			out[key] = body
		}
	}
	works := false
	for _, v := range tree.Volumes {
		p := src.KBPath(path.Join(v.Dir, ReferencesFile))
		var entries []kb.BibliographyEntry
		if src.IsFile(p) {
			text, err := src.ReadText(p)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", p, err)
			}
			entries = kb.BibliographyEntries(text)
		}
		works = works || len(entries) == 0
		for _, e := range entries {
			add(e.Key, strings.TrimSpace(e.Body))
		}
	}
	if !works {
		return out, nil
	}
	g, err := kb.LoadGraph(src)
	if err != nil {
		return nil, fmt.Errorf("reading the node records: %w", err)
	}
	for _, w := range g.Works {
		add(kb.WorkKey(w.ID), w.Title)
	}
	return out, nil
}

// volumeAssets is every non-Markdown file under an exported volume's
// directory, by kb-root-relative slash path, less the claim-graph sheets: a
// kb.ClaimGraphFile directly inside a top-level directory. The root's sheets
// lie under no volume.
func volumeAssets(src *kb.Source, tree Tree) (map[string][]byte, error) {
	all, err := kb.Files(src, func(name string) bool { return !strings.HasSuffix(name, ".md") })
	if err != nil {
		return nil, fmt.Errorf("listing the KB's files: %w", err)
	}
	out := map[string][]byte{}
	for _, p := range all {
		if !slices.ContainsFunc(tree.Volumes, func(v Volume) bool { return strings.HasPrefix(p, v.Dir+"/") }) {
			continue
		}
		if dir, name := path.Split(p); name == kb.ClaimGraphFile && !strings.Contains(strings.TrimSuffix(dir, "/"), "/") {
			continue
		}
		data, err := src.ReadFile(src.KBPath(p))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		out[p] = data
	}
	return out, nil
}

func yamlBytes(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
