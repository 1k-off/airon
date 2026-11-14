package executor

import (
	"fmt"

	"github.com/1k-off/airon/internal/config"
)

// SortPrompts topologically sorts prompts by DependsOn using Kahn's algorithm.
func SortPrompts(prompts []config.Prompt) ([]config.Prompt, error) {
	if len(prompts) == 0 {
		return nil, nil
	}

	byID := make(map[string]config.Prompt, len(prompts))
	inDegree := make(map[string]int, len(prompts))
	edges := make(map[string][]string, len(prompts))
	order := make(map[string]int, len(prompts))

	for i, p := range prompts {
		byID[p.ID] = p
		inDegree[p.ID] = 0
		order[p.ID] = i
	}

	for _, p := range prompts {
		for _, dep := range p.DependsOn {
			if _, ok := byID[dep]; !ok {
				return nil, fmt.Errorf("unknown dependency %q for prompt %q", dep, p.ID)
			}
			edges[dep] = append(edges[dep], p.ID)
			inDegree[p.ID]++
		}
	}

	queue := make([]string, 0, len(prompts))
	for _, p := range prompts {
		if inDegree[p.ID] == 0 {
			queue = append(queue, p.ID)
		}
	}

	result := make([]config.Prompt, 0, len(prompts))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, byID[current])

		for _, next := range edges[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				inserted := false
				for i := range queue {
					if order[next] < order[queue[i]] {
						queue = append(queue[:i], append([]string{next}, queue[i:]...)...)
						inserted = true
						break
					}
				}
				if !inserted {
					queue = append(queue, next)
				}
			}
		}
	}

	if len(result) != len(prompts) {
		return nil, fmt.Errorf("cycle detected in prompt dependencies")
	}
	return result, nil
}
