package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func pointer(doc any, fragment string) (any, error) {
	value := doc
	fragment = strings.TrimPrefix(strings.TrimPrefix(fragment, "#"), "/")
	for _, part := range strings.Split(fragment, "/") {
		if part == "" {
			continue
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		mapping, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot follow OpenAPI reference token %q", part)
		}
		var exists bool
		value, exists = mapping[part]
		if !exists {
			return nil, fmt.Errorf("OpenAPI reference token %q does not exist", part)
		}
	}
	return value, nil
}

func readYAML(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func resolve(value any, source, root string, stack map[string]bool) (any, error) {
	switch v := value.(type) {
	case []any:
		for i := range v {
			resolved, err := resolve(v[i], source, root, stack)
			if err != nil {
				return nil, err
			}
			v[i] = resolved
		}
		return v, nil
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok {
			file, fragment, hasFile := strings.Cut(ref, "#")
			if !hasFile {
				file, fragment = ref, ""
			}
			target := source
			if file != "" {
				target = filepath.Clean(filepath.Join(filepath.Dir(source), file))
			}
			if target == root && file == "" { // Root-local references stay references.
				return v, nil
			}
			if target == root && file != "" {
				return map[string]any{"$ref": "#" + fragment}, nil
			}
			marker := target + "#" + fragment
			if stack[marker] {
				return nil, fmt.Errorf("cyclic local reference %s", ref)
			}
			doc, err := readYAML(target)
			if err != nil {
				return nil, err
			}
			selected, err := pointer(doc, fragment)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ref, err)
			}
			stack[marker] = true
			resolved, err := resolve(selected, target, root, stack)
			delete(stack, marker)
			return resolved, err
		}
		for key := range v {
			resolved, err := resolve(v[key], source, root, stack)
			if err != nil {
				return nil, err
			}
			v[key] = resolved
		}
		return v, nil
	default:
		return value, nil
	}
}

func main() {
	input := flag.String("input", "api/qbittorrent.yaml", "root OpenAPI document")
	output := flag.String("output", "", "bundled OpenAPI output")
	flag.Parse()
	if *output == "" {
		fmt.Fprintln(os.Stderr, "--output is required")
		os.Exit(2)
	}
	root, err := filepath.Abs(*input)
	if err != nil {
		panic(err)
	}
	out, err := filepath.Abs(*output)
	if err != nil {
		panic(err)
	}
	doc, err := readYAML(root)
	if err != nil {
		panic(err)
	}
	doc, err = resolve(doc, root, root, map[string]bool{})
	if err != nil {
		panic(err)
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(out, data, 0o600); err != nil {
		panic(err)
	}
}
