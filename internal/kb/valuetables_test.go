package kb

import (
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// scoringPagesDir is the SPEC tree of the rigor-and-bearing scoring pages,
// from this package's directory.
const scoringPagesDir = "../../PROJECT_DOCS/SPEC/scoring/rigor-and-bearing"

// scoringValuePages pairs each fault table with the vocabulary page that lists
// its faults and the band-outcome page that lists the values its charges reach.
var scoringValuePages = []struct {
	vocabulary, bands string
	table             FaultTable
}{
	{"the-fault-vocabulary/the-statement-faults.md", "the-band-outcomes/the-statement-table.md", RigorStatement},
	{"the-fault-vocabulary/the-method-faults.md", "the-band-outcomes/the-method-table.md", RigorMethod},
	{"the-fault-vocabulary/the-bearing-faults.md", "the-band-outcomes/the-bearing-values.md", BearingFaults},
}

// countWords are the words a page counts faults and charges in.
var countWords = map[string]int{"none": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10}

// pageFigureRE is a figure as the pages write one: backticked, in 0.00 form.
var pageFigureRE = regexp.MustCompile("`([01])\\.([0-9]{2})`")

// faultCountRE is the vocabulary page's prose line naming its fault count and
// maximum.
var faultCountRE = regexp.MustCompile("^(\\w+) faults, maximum (`[01]\\.[0-9]{2}`)")

// pageFigure is the first figure in s, in hundredths; ok is false where s
// holds none.
func pageFigure(s string) (int, bool) {
	m := pageFigureRE.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1] + m[2])
	return n, err == nil
}

// tableRow is one body row of a page's pipe table: its 1-based line and cells,
// trimmed, under the table's header cells.
type tableRow struct {
	line          int
	header, cells []string
}

// pipeTables is every body row of every pipe table in text whose header's
// first cell is first.
func pipeTables(text, first string) []tableRow {
	var rows []tableRow
	var header []string
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "|") {
			header = nil
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}
		switch {
		case i == 0 || !strings.HasPrefix(lines[i-1], "|"):
			header = nil
			if cells[0] == first {
				header = cells
			}
		case strings.HasPrefix(cells[0], "---"):
		case header != nil:
			rows = append(rows, tableRow{i + 1, header, cells})
		}
	}
	return rows
}

// weightsOf is each of table's fault ids with what one occurrence charges, in
// hundredths, read through Price.
func weightsOf(table FaultTable) map[string]int {
	weights := map[string]int{}
	for _, id := range table.IDs() {
		weights[id] = priceHundredths(table, nil) - priceHundredths(table, []string{id})
	}
	return weights
}

// priceHundredths is the value table prices ids at, in hundredths.
func priceHundredths(table FaultTable, ids []string) int {
	return int(math.Round(table.Price(ids).Value * 100))
}

// vocabularyPageFaults lists every figure on a fault vocabulary page that
// table does not produce: a fault id it lacks or one it holds that the page
// lacks, a weight, the stated fault count or the stated maximum.
func vocabularyPageFaults(text string, table FaultTable) []string {
	var faults []string
	weights := weightsOf(table)
	rows := pipeTables(text, "fault id")
	listed := map[string]bool{}
	for _, r := range rows {
		if r.header[len(r.header)-1] != "weight per occurrence" {
			faults = append(faults, fmt.Sprintf("line %d: the table's last column is %q, not the weight per occurrence", r.line, r.header[len(r.header)-1]))
			continue
		}
		id := strings.Trim(r.cells[0], "`")
		listed[id] = true
		weight, ok := pageFigure(r.cells[len(r.cells)-1])
		want, known := weights[id]
		switch {
		case !known:
			faults = append(faults, fmt.Sprintf("line %d: %s is no fault of the table", r.line, id))
		case !ok:
			faults = append(faults, fmt.Sprintf("line %d: %s's weight is no figure", r.line, id))
		case weight != want:
			faults = append(faults, fmt.Sprintf("line %d: %s weighs %d hundredths, the table %d", r.line, id, weight, want))
		}
	}
	if len(rows) == 0 {
		return append(faults, "no row under a `| fault id | … | weight per occurrence |` header")
	}
	for _, id := range table.IDs() {
		if !listed[id] {
			faults = append(faults, fmt.Sprintf("the table's %s is not on the page", id))
		}
	}
	for i, line := range strings.Split(text, "\n") {
		m := faultCountRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if n, ok := countWords[strings.ToLower(m[1])]; !ok || n != len(rows) {
			faults = append(faults, fmt.Sprintf("line %d: %q faults, and the table lists %d", i+1, m[1], len(rows)))
		}
		if maximum, _ := pageFigure(m[2]); maximum != priceHundredths(table, nil) {
			faults = append(faults, fmt.Sprintf("line %d: maximum %d hundredths, the table's %d", i+1, maximum, priceHundredths(table, nil)))
		}
		return faults
	}
	return append(faults, "no line naming the fault count and maximum")
}

// bandPageFaults lists every figure on a band-outcome page that table does not
// produce: each row's value for its count of charges, and its sum where the
// page gives one as a figure.
func bandPageFaults(text string, table FaultTable) []string {
	var faults []string
	weights := slices.Sorted(maps.Values(weightsOf(table)))
	if weights[0] != weights[len(weights)-1] {
		return []string{fmt.Sprintf("the table's weights differ (%v); a band row's charge count does not decide its value", weights)}
	}
	weight, id := weights[0], table.IDs()[0]
	rows := pipeTables(text, "charges")
	for _, r := range rows {
		value, sum := -1, -1
		for j, h := range r.header {
			switch h {
			case "statement rigor", "method rigor", "bearing":
				value = j
			case "sum":
				sum = j
			}
		}
		if value < 0 {
			faults = append(faults, fmt.Sprintf("line %d: no value column in the table's header", r.line))
			continue
		}
		if strings.Trim(r.cells[value], "`") == "*pending*" {
			continue
		}
		word := strings.TrimRight(strings.Fields(strings.ReplaceAll(r.cells[0], "`", ""))[0], ",")
		count, ok := countWords[word]
		if !ok {
			faults = append(faults, fmt.Sprintf("line %d: %q opens with no count word", r.line, r.cells[0]))
			continue
		}
		want := priceHundredths(table, slices.Repeat([]string{id}, count))
		if got, ok := pageFigure(r.cells[value]); !ok || got != want {
			faults = append(faults, fmt.Sprintf("line %d: %d charges read %s, the table prices them at %d hundredths", r.line, count, r.cells[value], want))
		}
		if sum < 0 {
			continue
		}
		if got, ok := pageFigure(r.cells[sum]); ok && got != count*weight {
			faults = append(faults, fmt.Sprintf("line %d: %d charges sum to %s, the table to %d hundredths", r.line, count, r.cells[sum], count*weight))
		}
	}
	if len(rows) == 0 {
		faults = append(faults, "no row under a header whose first cell is charges")
	}
	return faults
}

// TestScoringValueTablesAreDerived reads every figure on the rigor-and-bearing
// scoring pages' value tables and recomputes it from the fault tables, so a
// page left stale by a change of weight or maximum fails. The synthetic pages
// show each figure read: one altered figure fails.
func TestScoringValueTablesAreDerived(t *testing.T) {
	for _, p := range scoringValuePages {
		for page, check := range map[string]func(string, FaultTable) []string{p.vocabulary: vocabularyPageFaults, p.bands: bandPageFaults} {
			t.Run(page, func(t *testing.T) {
				text, err := os.ReadFile(filepath.Join(scoringPagesDir, page))
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range check(string(text), p.table) {
					t.Errorf("%s: %s", page, f)
				}
			})
		}
	}

	synthetic := FaultTable{maximum: 100, faults: []fault{{"flt-a", 40}, {"flt-b", 40}, {"flt-c", 40}}}
	const vocabulary = "Three faults, maximum `1.00`.\n\n" +
		"| fault id | the fault | one occurrence is | weight per occurrence |\n|---|---|---|---|\n" +
		"| `flt-a` | a | one | `0.40` |\n| `flt-b` | b | one | `0.40` |\n| `flt-c` | c | one | `0.40` |\n"
	const bands = "| charges | sum | bearing |\n|---|---|---|\n" +
		"| `none` | — | `1.00` |\n| one | `0.40` | `0.60` |\n| two | `0.80` | `0.20` |\n" +
		"| three, or any sum of `1.00` or more | `≥ 1.00` | `0.00` |\n" +
		"| a recorded `flt-pending` | — | `*pending*` |\n"
	for _, tc := range []struct {
		name, page, from, to string
		check                func(string, FaultTable) []string
		fails                bool
	}{
		{"synthetic vocabulary as derived", vocabulary, "", "", vocabularyPageFaults, false},
		{"synthetic vocabulary, a weight altered", vocabulary, "| b | one | `0.40` |", "| b | one | `0.45` |", vocabularyPageFaults, true},
		{"synthetic vocabulary, the maximum altered", vocabulary, "maximum `1.00`", "maximum `0.90`", vocabularyPageFaults, true},
		{"synthetic vocabulary, the count altered", vocabulary, "Three faults", "Four faults", vocabularyPageFaults, true},
		{"synthetic vocabulary, a fault dropped", vocabulary, "| `flt-b` | b | one | `0.40` |\n", "", vocabularyPageFaults, true},
		{"synthetic vocabulary, a fault the table lacks", vocabulary, "`flt-b`", "`flt-d`", vocabularyPageFaults, true},
		{"synthetic vocabulary reshaped", vocabulary, "| fault id |", "| id |", vocabularyPageFaults, true},
		{"synthetic bands as derived", bands, "", "", bandPageFaults, false},
		{"synthetic bands, a value altered", bands, "| `0.80` | `0.20` |", "| `0.80` | `0.25` |", bandPageFaults, true},
		{"synthetic bands, a sum altered", bands, "| `0.80` | `0.20` |", "| `0.70` | `0.20` |", bandPageFaults, true},
		{"synthetic bands, the floor altered", bands, "| `≥ 1.00` | `0.00` |", "| `≥ 1.00` | `0.10` |", bandPageFaults, true},
		{"synthetic bands, no count word", bands, "| two |", "| a pair |", bandPageFaults, true},
		{"synthetic bands reshaped", bands, "| charges |", "| findings |", bandPageFaults, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := tc.page
			if tc.from != "" {
				if !strings.Contains(page, tc.from) {
					t.Fatalf("the synthetic page holds no %q to alter", tc.from)
				}
				page = strings.Replace(page, tc.from, tc.to, 1)
			}
			faults := tc.check(page, synthetic)
			if (len(faults) > 0) != tc.fails {
				t.Errorf("faults = %q, want failing %v", faults, tc.fails)
			}
		})
	}
}
