package bootstrap

import (
	"strings"
	"testing"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/executor"
	"github.com/stretchr/testify/require"
)

func TestEvaluateAgentOutcome_disabled(t *testing.T) {
	t.Parallel()
	steps := []executor.StepResult{{Output: "FAIL: boom"}}
	require.NoError(t, evaluateAgentOutcome("", steps))
	require.NoError(t, evaluateAgentOutcome(config.OutcomeNoneAlt, steps))
}

func TestEvaluateAgentOutcome_passFail(t *testing.T) {
	t.Parallel()

	require.NoError(t, evaluateAgentOutcome(config.OutcomePassFail, []executor.StepResult{
		{Output: "PASS: all good"},
	}))
	require.NoError(t, evaluateAgentOutcome(config.OutcomePassFail, []executor.StepResult{
		{Output: "**PASS:** markdown ok"},
	}))

	err := evaluateAgentOutcome(config.OutcomePassFail, []executor.StepResult{
		{Output: "FAIL: restore exploded"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent reported failure")
	require.Contains(t, err.Error(), "restore exploded")

	err = evaluateAgentOutcome(config.OutcomePassFail, []executor.StepResult{
		{Output: "I think it worked"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "outcome missing")
}

func TestEvaluateAgentOutcome_failWinsOverPass(t *testing.T) {
	t.Parallel()
	err := evaluateAgentOutcome(config.OutcomePassFail, []executor.StepResult{
		{Output: "PASS: earlier\nFAIL: later verdict"},
	})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "later verdict"))
}

func TestEvaluateAgentOutcome_unknownMode(t *testing.T) {
	t.Parallel()
	err := evaluateAgentOutcome("yolo", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown runtime.outcome")
}
