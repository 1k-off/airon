package runlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactInput_scrubsSensitiveKeys(t *testing.T) {
	in := map[string]string{
		"input":        "hello",
		"OPENAI_API_KEY": "sk-secret",
		"db_password":  "hunter2",
		"topic":        "go",
	}
	out := RedactInput(in)
	assert.Equal(t, "hello", out["input"])
	assert.Equal(t, "***REDACTED***", out["OPENAI_API_KEY"])
	assert.Equal(t, "***REDACTED***", out["db_password"])
	assert.Equal(t, "go", out["topic"])
	assert.Equal(t, "sk-secret", in["OPENAI_API_KEY"]) // original untouched
}

func TestRedactString_replacesSecretValues(t *testing.T) {
	got := RedactString("key=sk-secret end", map[string]string{"api_key": "sk-secret", "topic": "go"})
	assert.Equal(t, "key=***REDACTED*** end", got)
}
