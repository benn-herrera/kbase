package migrate

// The 1.0.0 writer's scalar rules and frontmatter locator as the converter
// holds them, for the test that pins them to internal/kb's.
var (
	YAMLFrontmatter = yamlFrontmatter
	StringNode      = stringNode
	NumberTag       = numberTag
	EncodeYAML      = encodeYAML
	SplitLines      = splitLines
	IsSpace         = isSpace
	Strip           = strip
	RStrip          = rstrip
)
