package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	Method string
	Path   string
	Body   string
}

type recordingHandler struct {
	mu       sync.Mutex
	requests []recordedRequest
	status   int
}

func (r *recordingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.requests = append(r.requests, recordedRequest{
		Method: req.Method,
		Path:   req.URL.Path,
		Body:   string(body),
	})
	r.mu.Unlock()

	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
}

func (r *recordingHandler) snapshot() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedRequest, len(r.requests))
	copy(out, r.requests)
	return out
}

func TestHealthchecks_NoopWhenURLEmpty(t *testing.T) {
	h := NewHealthchecks("", false)
	ctx := context.Background()

	assert.False(t, h.Enabled())
	require.NoError(t, h.Start(ctx))
	require.NoError(t, h.Success(ctx))
	require.NoError(t, h.Fail(ctx, "body"))
}

func TestHealthchecks_NoopWhenDisabled(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	h := NewHealthchecks(server.URL, true).WithHTTPClient(server.Client())
	ctx := context.Background()

	assert.False(t, h.Enabled())
	require.NoError(t, h.Start(ctx))
	require.NoError(t, h.Success(ctx))
	require.NoError(t, h.Fail(ctx, "body"))

	assert.Empty(t, handler.snapshot(), "no HTTP requests should be made when disabled")
}

func TestHealthchecks_PingsStartSuccessAndFail(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	h := NewHealthchecks(server.URL, false).WithHTTPClient(server.Client())
	require.True(t, h.Enabled())

	ctx := context.Background()
	require.NoError(t, h.Start(ctx))
	require.NoError(t, h.Success(ctx))
	require.NoError(t, h.Fail(ctx, "boom"))

	requests := handler.snapshot()
	require.Len(t, requests, 3)

	assert.Equal(t, http.MethodGet, requests[0].Method)
	assert.Equal(t, "/start", requests[0].Path)
	assert.Empty(t, requests[0].Body)

	assert.Equal(t, http.MethodGet, requests[1].Method)
	assert.Equal(t, "/", requests[1].Path)
	assert.Empty(t, requests[1].Body)

	assert.Equal(t, http.MethodPost, requests[2].Method)
	assert.Equal(t, "/fail", requests[2].Path)
	assert.Equal(t, "boom", requests[2].Body)
}

func TestHealthchecks_FailWithEmptyBody(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	h := NewHealthchecks(server.URL, false).WithHTTPClient(server.Client())
	require.NoError(t, h.Fail(context.Background(), ""))

	requests := handler.snapshot()
	require.Len(t, requests, 1)
	assert.Equal(t, http.MethodPost, requests[0].Method)
	assert.Equal(t, "/fail", requests[0].Path)
	assert.Empty(t, requests[0].Body)
}

func TestHealthchecks_TrimsTrailingSlashInURL(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	h := NewHealthchecks(server.URL+"/", false).WithHTTPClient(server.Client())
	require.NoError(t, h.Start(context.Background()))

	requests := handler.snapshot()
	require.Len(t, requests, 1)
	assert.Equal(t, "/start", requests[0].Path)
}

func TestHealthchecks_ReturnsErrorOnNon2xxResponse(t *testing.T) {
	handler := &recordingHandler{status: http.StatusInternalServerError}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	h := NewHealthchecks(server.URL, false).WithHTTPClient(server.Client())
	err := h.Start(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestHealthchecks_NilReceiverIsSafe(t *testing.T) {
	var h *Healthchecks
	ctx := context.Background()

	assert.False(t, h.Enabled())
	require.NoError(t, h.Start(ctx))
	require.NoError(t, h.Success(ctx))
	require.NoError(t, h.Fail(ctx, "body"))
}
