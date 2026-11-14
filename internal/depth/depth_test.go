package depth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromContext_defaultZero(t *testing.T) {
	assert.Equal(t, 0, FromContext(context.Background()))
}

func TestWithIncrement_increasesDepth(t *testing.T) {
	ctx := WithIncrement(WithIncrement(context.Background()))
	assert.Equal(t, 2, FromContext(ctx))
}
