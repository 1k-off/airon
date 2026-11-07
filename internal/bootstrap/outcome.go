package bootstrap

import (
	"fmt"
	"strings"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/executor"
)

// evaluateAgentOutcome applies runtime.outcome rules after a successful
// executor run. When mode is pass_fail, the agent must emit a PASS/FAIL
// verdict line; FAIL (or no PASS) becomes a run error for exit code +
// healthchecks /fail.
func evaluateAgentOutcome(mode string, steps []executor.StepResult) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", config.OutcomeNoneAlt:
		return nil
	case config.OutcomePassFail:
		return evaluatePassFail(steps)
	default:
		return fmt.Errorf("unknown runtime.outcome %q (want %q or empty)", mode, config.OutcomePassFail)
	}
}

func evaluatePassFail(steps []executor.StepResult) error {
	var (
		failLine string
		sawPass  bool
	)
	for _, step := range steps {
		for _, raw := range strings.Split(step.Output, "\n") {
			line := normalizeOutcomeLine(raw)
			if line == "" {
				continue
			}
			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "FAIL"):
				failLine = line
			case strings.HasPrefix(upper, "PASS"):
				sawPass = true
			}
		}
	}
	if failLine != "" {
		return fmt.Errorf("agent reported failure: %s", failLine)
	}
	if !sawPass {
		return fmt.Errorf("agent outcome missing: expected a PASS or FAIL line in step output")
	}
	return nil
}

func normalizeOutcomeLine(raw string) string {
	line := strings.TrimSpace(raw)
	// Strip common markdown bold wrappers: **PASS:** / **FAIL**
	line = strings.TrimPrefix(line, "**")
	line = strings.TrimSuffix(line, "**")
	line = strings.TrimSpace(line)
	return line
}
