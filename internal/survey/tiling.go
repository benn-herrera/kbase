package survey

import "fmt"

// verifyTiling checks that a file's inventory covers its source exactly once:
// front matter, then the preamble, then the heading tree in document order,
// with no gap, no overlap, and nothing past the end.
//
// This is the survey's half of the tiling discipline the dissector enforces
// again at stage 4 (ARCHITECTURE.md §5). It runs on every file, in
// production as well as in tests, because it is a handful of integer
// comparisons and because the failure it catches — a heading whose offset was
// computed against the wrong buffer, say — is otherwise invisible until a
// leaf is quietly missing a paragraph several stages later. A violation is a
// defect in this package, so it fails the run rather than being logged.
func verifyTiling(f File, size int) error {
	cursor := 0
	if r := f.FrontMatter; r != nil {
		if r.Start != 0 || r.End < r.Start {
			return fmt.Errorf("front matter range [%d,%d) must start the file", r.Start, r.End)
		}
		cursor = r.End
	}
	if p := f.Preamble; p != nil {
		if p.Start != cursor {
			return fmt.Errorf("preamble starts at %d, want %d", p.Start, cursor)
		}
		if len(p.Children) != 0 {
			return fmt.Errorf("preamble must not have children")
		}
		if p.End < p.Start {
			return fmt.Errorf("preamble range [%d,%d) is inverted", p.Start, p.End)
		}
		cursor = p.End
	}
	for _, s := range f.Sections {
		next, err := tileSection(s, cursor)
		if err != nil {
			return err
		}
		cursor = next
	}
	if cursor != size {
		return fmt.Errorf("sections cover %d bytes, file is %d", cursor, size)
	}
	return nil
}

// tileSection checks one section subtree starting at cursor and returns the
// offset immediately after it. A section's own body is the bytes between its
// heading and its first child; its children tile the rest.
func tileSection(s Section, cursor int) (int, error) {
	if s.Start != cursor {
		return 0, fmt.Errorf("section %q (level %d) starts at %d, want %d", s.Title, s.Level, s.Start, cursor)
	}
	if s.End < s.Start {
		return 0, fmt.Errorf("section %q range [%d,%d) is inverted", s.Title, s.Start, s.End)
	}
	if len(s.Children) > 0 {
		at := s.Children[0].Start
		if at < s.Start {
			return 0, fmt.Errorf("section %q has a child starting at %d, before the section at %d", s.Title, at, s.Start)
		}
		for _, c := range s.Children {
			next, err := tileSection(c, at)
			if err != nil {
				return 0, err
			}
			at = next
		}
		if at != s.End {
			return 0, fmt.Errorf("section %q children end at %d, section ends at %d", s.Title, at, s.End)
		}
	}
	return s.End, nil
}
