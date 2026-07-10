package provider

import (
	"context"
	"sync"
)

type MockProvider struct {
	mu       sync.Mutex
	Requests []CompletionRequest
	Response CompletionResponse
	Err      error
}

func (m *MockProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Requests = append(m.Requests, req)

	if m.Err != nil {
		return CompletionResponse{}, m.Err
	}
	return m.Response, nil
}
