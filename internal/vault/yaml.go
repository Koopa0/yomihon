package vault

import (
	"errors"
	"strings"

	"go.yaml.in/yaml/v3"
)

// unsupportedYAML classifies only the library's non-scalar-key refusal. The
// syntax tree retains authored merge tags without changing parser evidence.
func unsupportedYAML(content []byte, failure error) bool {
	if !strings.HasPrefix(failure.Error(), "yaml: invalid map key:") {
		return false
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return false
	}
	return unsupportedYAMLNode(&document, true)
}

// unsupportedYAMLNode follows the decoder's first fatal failure, rather than
// letting a later unsupported mapping replace an earlier ordinary error.
// Type errors accumulate in the decoder and do not stop its later entries.
func unsupportedYAMLNode(node *yaml.Node, stringKeys bool) bool {
	if node.Kind == yaml.AliasNode {
		return unsupportedYAMLNode(node.Alias, stringKeys)
	}
	if node.Kind != yaml.MappingNode {
		for _, child := range node.Content {
			if yamlNodeFails(child) {
				return unsupportedYAMLNode(child, stringKeys)
			}
		}
		return false
	}
	merge := false
	for i := 0; i < len(node.Content); i += 2 {
		merge = merge || yamlMergeKey(node.Content[i])
	}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if yamlMergeKey(key) {
			continue
		}
		var decoded any
		var keyErr error
		if stringKeys {
			// The root is decoded into map[string]any, so a complex root
			// key is an accumulating type error, not a fatal map key.
			var text string
			keyErr = key.Decode(&text)
			decoded = text
		} else {
			keyErr = key.Decode(&decoded)
		}
		if keyErr != nil {
			if _, ok := errors.AsType[*yaml.TypeError](keyErr); ok {
				continue
			}
			return unsupportedYAMLNode(key, false)
		}
		switch decoded.(type) {
		case []any, map[string]any, map[any]any:
			return merge
		}
		if yamlNodeFails(value) {
			return unsupportedYAMLNode(value, false)
		}
	}
	// Merge values are decoded only after the mapping's own entries.
	for i := 0; i < len(node.Content); i += 2 {
		if yamlMergeKey(node.Content[i]) && yamlNodeFails(node.Content[i+1]) {
			return unsupportedYAMLNode(node.Content[i+1], stringKeys)
		}
	}
	return false
}

func yamlNodeFails(node *yaml.Node) bool {
	var decoded any
	if err := node.Decode(&decoded); err != nil {
		_, accumulated := errors.AsType[*yaml.TypeError](err)
		return !accumulated
	}
	return false
}

func yamlMergeKey(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Value == "<<" &&
		(node.Tag == "" || node.Tag == "!" || node.ShortTag() == "!!merge")
}
