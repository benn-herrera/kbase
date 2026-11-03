package main

import "testing"

// TestTheSortingQueriesShareOneLimitDescription: find's and trace's --limit
// take the same rule, so they carry one description, and it names the --sort
// it stands beside as the command line spells it.
func TestTheSortingQueriesShareOneLimitDescription(t *testing.T) {
	var first string
	for i, name := range []string{"find", "trace"} {
		c, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		usage := atTheCommandLine(c.Flags().Lookup("limit").Usage)
		if !cliSpelled("sort").MatchString(usage) {
			t.Errorf("%s's --limit description does not spell --sort at the command line: %q", name, usage)
		}
		if i == 0 {
			first = usage
		} else if usage != first {
			t.Errorf("%s's --limit description is %q, find's %q: want one description", name, usage, first)
		}
	}
}
