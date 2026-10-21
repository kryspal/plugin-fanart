package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/kryspal/plugin-sdk/sdk"
	"github.com/kryspal/plugin-sdk/sdk/httpclient"
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
type api struct{ c *httpclient.Client }

func newAPI() *api { return newAPIWith(minInterval, nil) }

// newAPIWith is newAPI with another interval and transport (tests).
func newAPIWith(interval time.Duration, rt http.RoundTripper) *api {
	return &api{c: httpclient.New(httpclient.Config{Name: "fanart", UserAgent: "KryspalMedia-fanart/" + version(), Timeout: requestTimeout,
		MinInterval: interval, MaxBodyBytes: maxResponseBytes, RetryAfter: defaultRetryAfter, Transport: rt})}
}

// get sends the request and returns the status code; on 200 it parses the JSON into out. A 429 returns sdk.RateLimited; 401 and 403 return
// errUnauthorized; 5xx and connection errors return sdk.Unavailable; other statuses (404 means no images) are returned for the caller to decide.
func (a *api) get(ctx context.Context, key, rawURL string, out any) (int, error) {
	resp, err := a.c.Do(ctx, httpclient.Request{URL: rawURL, Header: http.Header{"Api-Key": {key}}})
	if err != nil {
		return 0, err
	}
	switch resp.Status {
	case http.StatusTooManyRequests:
		return resp.Status, resp.RateLimit("fanart")
	case http.StatusUnauthorized, http.StatusForbidden:
		return resp.Status, errUnauthorized
	case http.StatusOK:
		if out != nil {
			if err := json.Unmarshal(resp.Body, out); err != nil {
				return resp.Status, sdk.Unavailable("fanart: bad response")
			}
		}
	}
	return resp.Status, nil
}
