package provider

import (
	"context"
	"strings"
)

type trackingProvider struct {
	inner        Provider
	defaultModel string
	tracker      *UsageTracker
}

// NewTrackingProvider wraps a provider and records token usage on every completion.
func NewTrackingProvider(inner Provider, defaultModel string, tracker *UsageTracker) Provider {
	if inner == nil || tracker == nil {
		return inner
	}
	return &trackingProvider{
		inner:        inner,
		defaultModel: defaultModel,
		tracker:      tracker,
	}
}

func (p *trackingProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	resp, err := p.inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	if resp.Usage != nil {
		model := strings.TrimSpace(req.Model)
		if model == "" {
			model = p.defaultModel
		}
		p.tracker.Record(model, *resp.Usage)
	}
	return resp, nil
}
