package runlog

import (
	"regexp"
	"strings"
)

var sensitiveKeyPattern = regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token|authorization|passwd|private[_-]?key)`)

const redacted = "***REDACTED***"

// RedactInput returns a copy of input with sensitive-looking keys scrubbed.
func RedactInput(input map[string]string) map[string]string {
	if len(input) == 0 {
		return input
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		if sensitiveKeyPattern.MatchString(key) {
			out[key] = redacted
			continue
		}
		out[key] = value
	}
	return out
}

// RedactString replaces known secret values from input into redacted markers.
// Keys that look sensitive contribute their raw values as scrub targets.
func RedactString(text string, input map[string]string) string {
	if text == "" || len(input) == 0 {
		return text
	}
	out := text
	for key, value := range input {
		if value == "" || !sensitiveKeyPattern.MatchString(key) {
			continue
		}
		out = strings.ReplaceAll(out, value, redacted)
	}
	return out
}
