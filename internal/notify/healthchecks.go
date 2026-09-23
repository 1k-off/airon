// Package notify contains lightweight run-notification clients such as
// healthchecks.io compatible pings.
package notify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

// Healthchecks is a healthchecks.io compatible ping client. It is safe to use
// with an empty URL or in disabled mode; all methods become no-ops in that case.
type Healthchecks struct {
	url      string
	disabled bool
	client   *http.Client
}

// NewHealthchecks constructs a healthchecks.io style notifier.
//
// When url is empty or disabled is true, Start/Success/Fail return nil without
// performing any HTTP requests. This makes the notifier safe to always
// construct — callers do not need to guard call sites.
func NewHealthchecks(url string, disabled bool) *Healthchecks {
	return &Healthchecks{
		url:      strings.TrimRight(strings.TrimSpace(url), "/"),
		disabled: disabled,
		client:   &http.Client{Timeout: defaultTimeout},
	}
}

// WithHTTPClient overrides the underlying http.Client. Passing nil is a no-op.
// Primarily intended for tests using httptest servers.
func (h *Healthchecks) WithHTTPClient(client *http.Client) *Healthchecks {
	if h == nil {
		return h
	}
	if client != nil {
		h.client = client
	}
	return h
}

// Enabled reports whether the notifier will actually issue HTTP requests.
func (h *Healthchecks) Enabled() bool {
	return h != nil && !h.disabled && h.url != ""
}

// Start pings the healthchecks.io style "/start" endpoint.
func (h *Healthchecks) Start(ctx context.Context) error {
	if !h.Enabled() {
		return nil
	}
	return h.do(ctx, http.MethodGet, h.url+"/start", nil)
}

// Success pings the base URL to record a successful run.
func (h *Healthchecks) Success(ctx context.Context) error {
	if !h.Enabled() {
		return nil
	}
	return h.do(ctx, http.MethodGet, h.url, nil)
}

// Fail posts to the "/fail" endpoint with an optional body describing the
// failure. Body may be empty.
func (h *Healthchecks) Fail(ctx context.Context, body string) error {
	if !h.Enabled() {
		return nil
	}
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	return h.do(ctx, http.MethodPost, h.url+"/fail", reader)
}

func (h *Healthchecks) do(ctx context.Context, method, url string, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("notify: build %s request: %w", method, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}

	client := h.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("notify: %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("notify: %s %s returned status %d", method, url, resp.StatusCode)
}
