package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	pluginapi "github.com/kryspal/plugin-sdk"
	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

// An ID fanart.tv does not know is answered with an empty list. A host that keeps notices is also told why; an older host gets the plain answer.
func TestNoImagesNotice(t *testing.T) {
	unknown := newFake(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	known := newFake(t, serve(`{"movieposter":[{"id":"1","url":"https://assets.example/a.jpg","lang":"en","likes":"3"}]}`))
	ask := func(p *plugin, types ...pluginv1.ImageType) *pluginv1.GetImagesResponse {
		t.Helper()
		resp, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE,
			ProviderIds: map[string]string{"tmdb": "1"}, Types: types})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	if resp := ask(configured(t, unknown)); len(resp.GetImages()) != 0 || len(resp.GetNotices()) != 0 {
		t.Errorf("old host: %v, want an empty answer without notices", resp)
	}

	t.Setenv(pluginapi.EnvHostFeatures, pluginapi.HostFeatureScrapeNotices)
	resp := ask(configured(t, unknown))
	if len(resp.GetImages()) != 0 || len(resp.GetNotices()) != 1 {
		t.Fatalf("unknown id: %v, want one notice", resp)
	}
	n := resp.GetNotices()[0]
	if n.GetStatus() != pluginv1.NoticeStatus_NOTICE_STATUS_NOT_FOUND || n.GetSource() != "fanart.tv" || n.GetMessage() == "" || strings.Contains(n.GetMessage(), testKey) {
		t.Errorf("notice = %v", n)
	}

	// A title with images is not "not found", even when none of them is of the type asked for.
	p := configured(t, known)
	if resp := ask(p); len(resp.GetImages()) != 1 || len(resp.GetNotices()) != 0 {
		t.Errorf("known id: %v, want its image and no notice", resp)
	}
	if resp := ask(p, pluginv1.ImageType_IMAGE_TYPE_LOGO); len(resp.GetImages()) != 0 || len(resp.GetNotices()) != 0 {
		t.Errorf("known id, other type: %v, want an empty answer and no notice", resp)
	}
}
