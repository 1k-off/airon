package executor

import (
	"testing"

	"github.com/1node/airon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortPrompts_linearChain(t *testing.T) {
	prompts := []config.Prompt{
		{ID: "c", DependsOn: []string{"b"}},
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
	}

	sorted, err := SortPrompts(prompts)
	require.NoError(t, err)
	require.Len(t, sorted, 3)
	assert.Equal(t, []string{"a", "b", "c"}, []string{sorted[0].ID, sorted[1].ID, sorted[2].ID})
}

func TestSortPrompts_parallelBranches(t *testing.T) {
	prompts := []config.Prompt{
		{ID: "report", DependsOn: []string{"research", "draft"}},
		{ID: "draft", DependsOn: []string{"research"}},
		{ID: "research"},
	}

	sorted, err := SortPrompts(prompts)
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	pos := map[string]int{}
	for i, p := range sorted {
		pos[p.ID] = i
	}

	assert.Less(t, pos["research"], pos["draft"])
	assert.Less(t, pos["research"], pos["report"])
	assert.Less(t, pos["draft"], pos["report"])
}

func TestSortPrompts_cycleDetection(t *testing.T) {
	prompts := []config.Prompt{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	_, err := SortPrompts(prompts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}
