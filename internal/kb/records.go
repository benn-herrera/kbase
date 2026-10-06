package kb

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// RecordYAML is a build record as its file holds it: v's JSON encoding — keys
// in field order, map keys sorted — as one YAML document, strings written as
// FrontmatterString writes them, numbers as JSON wrote them.
func RecordYAML(v any) ([]byte, error) {
	var js bytes.Buffer
	enc := json.NewEncoder(&js)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(&js)
	dec.UseNumber()
	n, err := recordNode(dec)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	ye := yaml.NewEncoder(&out)
	ye.SetIndent(2)
	if err := ye.Encode(n); err != nil {
		return nil, err
	}
	if err := ye.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func recordNode(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := FrontmatterList(nil)
		if t == '{' {
			n = FrontmatterMapping()
		}
		for dec.More() {
			if n.Kind == yaml.MappingNode {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				n.Content = append(n.Content, FrontmatterString(k.(string)))
			}
			v, err := recordNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return n, nil
	case string:
		return FrontmatterString(t), nil
	case json.Number:
		return FrontmatterNumber(t.String()), nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprint(t)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	}
	return nil, fmt.Errorf("unexpected JSON token %v", tok)
}
