package format300

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// FormatFile is the metadata-format stamp file, kb-root-relative: one block
// mapping holding one key, FormatKey, whose value names the format the tree
// stands at. Its shape is version-agnostic, so a reader of any format can
// read the stamp of any format, which is why the loader reads it first.
const FormatFile = IndexDir + "/format" + IndexFileExt

// EncodeFormatStamp is the stamp file's content holding version.
func EncodeFormatStamp(version string) []byte {
	return []byte(FormatKey + ": " + QuoteRecordString(version) + "\n")
}

// DecodeFormatStamp is the version the stamp file's bytes data hold. The file
// is one block mapping holding the one key FormatKey with a non-empty string
// value; an unparseable file, another key, a repeated key, a missing key and
// a non-empty string value's absence are each an error naming what it found.
func DecodeFormatStamp(data []byte) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("%s is not one block mapping", FormatFile)
	}
	mapping := root.Content[0]
	if mapping.Style&yaml.FlowStyle != 0 && len(mapping.Content) > 0 {
		return "", fmt.Errorf("%s is not one block mapping", FormatFile)
	}
	stamp := ""
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		switch {
		case key.Value != FormatKey:
			return "", fmt.Errorf("%s holds the key %q, and no key but %s", FormatFile, key.Value, FormatKey)
		case stamp != "":
			return "", fmt.Errorf("%s repeats %s", FormatFile, FormatKey)
		case value.Kind != yaml.ScalarNode || value.Tag != "!!str" || value.Value == "":
			return "", fmt.Errorf("%s in %s is not a version", FormatKey, FormatFile)
		}
		stamp = value.Value
	}
	if stamp == "" {
		return "", fmt.Errorf("%s carries no %s", FormatFile, FormatKey)
	}
	return stamp, nil
}
