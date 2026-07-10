package template

import (
	"fmt"
	"regexp"
)

var variablePattern = regexp.MustCompile(`\{\{(\w+)\}\}`)
var filePattern = regexp.MustCompile(`\{\{file:([a-zA-Z0-9_./\-]+)\}\}`)

// FileReader loads file contents for {{file:path}} placeholders.
type FileReader func(path string) (string, error)

// Render replaces {{variable}} placeholders with values from vars.
// Variable names must contain word characters only. Missing variables return an error.
func Render(template string, vars map[string]string) (string, error) {
	return RenderWithFiles(template, vars, nil)
}

// RenderWithFiles expands {{file:path}} placeholders then {{variable}} substitutions.
func RenderWithFiles(template string, vars map[string]string, readFile FileReader) (string, error) {
	expanded := template
	if readFile != nil {
		var fileErr error
		expanded = filePattern.ReplaceAllStringFunc(template, func(match string) string {
			if fileErr != nil {
				return match
			}
			sub := filePattern.FindStringSubmatch(match)
			if len(sub) < 2 {
				fileErr = fmt.Errorf("invalid file template: %s", match)
				return match
			}
			content, err := readFile(sub[1])
			if err != nil {
				fileErr = fmt.Errorf("read template file %q: %w", sub[1], err)
				return match
			}
			return content
		})
		if fileErr != nil {
			return "", fileErr
		}
	}

	return renderVariables(expanded, vars)
}

func renderVariables(template string, vars map[string]string) (string, error) {
	var missing string

	out := variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		name := variablePattern.FindStringSubmatch(match)[1]
		value, ok := vars[name]
		if !ok {
			missing = name
			return match
		}
		return value
	})

	if missing != "" {
		return "", fmt.Errorf("missing template variable: %s", missing)
	}

	return out, nil
}
