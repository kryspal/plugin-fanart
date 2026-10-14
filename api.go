package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/kryspal/plugin-sdk/sdk"
)

// errUnauthorized means fanart.tv rejected the key.
var errUnauthorized = errors.New("fanart.tv rejected the API key")

const (
	maxResponseBytes = 4 << 20
	requestTimeout   = 15 * time.Second
	// minInterval is the minimum interval between two requests: fanart.tv publishes no hard limit, so the plugin goes one request at a time, a few per second.
	minInterval = 300 * time.Millisecond
	// defaultRetryAfter is the pause time when a 429 carries no Retry-After.
	defaultRetryAfter = time.Minute
)

// api is the fanart.tv HTTP client: one request at a time, at least interval between two requests. The key goes in the api-key header, never in the URL.
type api struct {
	http     *http.Client
	interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

func newAPI() *api {
	return &api{http: &http.Client{Timeout: requestTimeout}, interval: minInterval}
}

// get sends the request and returns the status code; on 200 it parses the JSON into out. A 429 returns sdk.RateLimited; 401 and 403 return
// errUnauthorized; 5xx and connection errors return sdk.Unavailable; other statuses (404 means no images) are returned for the caller to decide.
func (a *api) get(ctx context.Context, key, rawURL string, out any) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if d := time.Until(a.last.Add(a.interval)); d > 0 {
		t := time.NewTimer(d)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return 0, ctx.Err()
		}
	}
	a.last = time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, sdk.Unavailable("fanart: bad request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "KryspalMedia-fanart/"+version())
	req.Header.Set("api-key", key)
	resp, err := a.http.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // url.Error carries the full URL; keep only the cause
		}
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, sdk.Unavailable("fanart: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		d := defaultRetryAfter
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			d = time.Duration(s) * time.Second
		}
		return resp.StatusCode, sdk.RateLimited(d, "fanart: rate limit reached")
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return resp.StatusCode, errUnauthorized
	case resp.StatusCode >= 500:
		return resp.StatusCode, sdk.Unavailable(fmt.Sprintf("fanart: %s", resp.Status))
	case resp.StatusCode != http.StatusOK:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return resp.StatusCode, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return resp.StatusCode, sdk.Unavailable("fanart: " + err.Error())
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, sdk.Unavailable("fanart: bad response")
		}
	}
	return resp.StatusCode, nil
}
