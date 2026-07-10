package depth

import "context"

type contextKey struct{}

// FromContext returns the current nested tool-call depth.
func FromContext(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	raw := ctx.Value(contextKey{})
	depth, ok := raw.(int)
	if !ok || depth < 0 {
		return 0
	}
	return depth
}

// WithIncrement returns a child context with depth incremented by one.
func WithIncrement(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, FromContext(ctx)+1)
}
