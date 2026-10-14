package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
	"github.com/kryspal/plugin-sdk/sdk"
)

const testKey = "0123456789abcdef0123456789abcdef"

// movieBody is a recorded-shape movie response (made-up ids and image names): likes are strings, lang "00" means no text, assets are http://.
const movieBody = `{
 "name": "Made Up Movie", "tmdb_id": "1001", "imdb_id": "tt0000001",
 "hdmovielogo": [
  {"id": "10", "url": "http://assets.fanart.tv/fanart/movies/1001/hdmovielogo/a.png", "lang": "en", "likes": "3"},
  {"id": "11", "url": "http://assets.fanart.tv/fanart/movies/1001/hdmovielogo/b.png", "lang": "de", "likes": "9"}
 ],
 "movielogo": [
  {"id": "12", "url": "http://assets.fanart.tv/fanart/movies/1001/movielogo/c.png", "lang": "en", "likes": "50"}
 ],
 "movieposter": [
  {"id": "20", "url": "http://assets.fanart.tv/fanart/movies/1001/movieposter/a.jpg", "lang": "en", "likes": "5"},
  {"id": "21", "url": "http://assets.fanart.tv/fanart/movies/1001/movieposter/b.jpg", "lang": "zh", "likes": "1"},
  {"id": "22", "url": "http://assets.fanart.tv/fanart/movies/1001/movieposter/c.jpg", "lang": "", "likes": "7"},
  {"id": "23", "url": "ftp://assets.fanart.tv/fanart/movies/1001/movieposter/d.jpg", "lang": "zh", "likes": "99"}
 ],
 "moviebackground": [
  {"id": "30", "url": "http://assets.fanart.tv/fanart/movies/1001/moviebackground/a.jpg", "lang": "en", "likes": "8"},
  {"id": "31", "url": "http://assets.fanart.tv/fanart/movies/1001/moviebackground/b.jpg", "lang": "00", "likes": "2"}
 ],
 "moviedisc": [
  {"id": "40", "url": "http://assets.fanart.tv/fanart/movies/1001/moviedisc/a.png", "lang": "en", "likes": "1", "disc": "1", "disc_type": "bluray"}
 ],
 "moviebanner": [{"id": "50", "url": "http://assets.fanart.tv/x/b.jpg", "lang": "en", "likes": 4}],
 "moviethumb": [], "hdmovieclearart": [{"id": "60", "url": "http://assets.fanart.tv/x/ca.png", "lang": "en", "likes": "x"}]
}`

const tvBody = `{
 "name": "Made Up Series", "thetvdb_id": "2002",
 "hdtvlogo": [{"id": "1", "url": "http://assets.fanart.tv/fanart/tv/2002/hdtvlogo/a.png", "lang": "en", "likes": "4"}],
 "clearlogo": [{"id": "2", "url": "http://assets.fanart.tv/fanart/tv/2002/clearlogo/a.png", "lang": "en", "likes": "40"}],
 "tvposter": [{"id": "3", "url": "http://assets.fanart.tv/fanart/tv/2002/tvposter/a.jpg", "lang": "en", "likes": "4"}],
 "showbackground": [{"id": "4", "url": "http://assets.fanart.tv/fanart/tv/2002/showbackground/a.jpg", "lang": "en", "likes": "4", "season": "1"}],
 "seasonposter": [{"id": "5", "url": "http://assets.fanart.tv/fanart/tv/2002/seasonposter/a.jpg", "lang": "en", "likes": "4", "season": "1"}]
}`

// fakeServer serves the recorded bodies and counts requests; seen collects the request paths and the api-key header.
type fakeServer struct {
	*httptest.Server
	hits  atomic.Int32
	paths []string
	keys  []string
}

func newFake(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.paths = append(f.paths, r.URL.RequestURI())
		f.keys = append(f.keys, r.Header.Get("api-key"))
		h(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func configured(t *testing.T, f *fakeServer) *plugin {
	t.Helper()
	p := newPlugin()
	p.api = &api{http: f.Client(), interval: 0}
	resp, err := p.Configure(context.Background(), &pluginv1.ConfigureRequest{Values: map[string]string{keyAPIKey: testKey, keyBaseURL: f.URL}})
	if err != nil || len(resp.GetFieldErrors()) != 0 {
		t.Fatalf("Configure = %v, %v", resp, err)
	}
	return p
}

func serve(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func byType(imgs []*pluginv1.Image, kind pluginv1.ImageType) (out []*pluginv1.Image) {
	for _, im := range imgs {
		if im.GetType() == kind {
			out = append(out, im)
		}
	}
	return out
}

func ids(imgs []*pluginv1.Image) string {
	var s []string
	for _, im := range imgs {
		s = append(s, im.GetSourceId())
	}
	return strings.Join(s, ",")
}

func TestMovieImages(t *testing.T) {
	f := newFake(t, serve(movieBody))
	p := configured(t, f)
	resp, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{
		Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": "1001", "imdb": "tt0000001"},
		Languages: &pluginv1.Languages{Language: "zh-TW", Fallbacks: []string{"en"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	imgs := resp.GetImages()
	if f.paths[0] != "/movies/1001" || f.keys[0] != testKey {
		t.Errorf("request %q key %q (the key must be in the api-key header, the ID is the TMDB ID)", f.paths[0], f.keys[0])
	}
	// Posters: preferred language zh, then the fallback en, then no-text, then the rest; the ftp:// one is dropped.
	if got := ids(byType(imgs, pluginv1.ImageType_IMAGE_TYPE_PRIMARY)); got != "21,20,22" {
		t.Errorf("posters = %s, want 21,20,22", got)
	}
	// Backgrounds: no-text first, then by language.
	if got := ids(byType(imgs, pluginv1.ImageType_IMAGE_TYPE_BACKDROP)); got != "31,30" {
		t.Errorf("backgrounds = %s, want 31,30", got)
	}
	// Logos: language first (en before de), and within a language the HD one before the standard one even with fewer likes.
	if got := ids(byType(imgs, pluginv1.ImageType_IMAGE_TYPE_LOGO)); got != "10,12,11" {
		t.Errorf("logos = %s, want 10,12,11", got)
	}
	for _, im := range imgs {
		if !strings.HasPrefix(im.GetUrl(), "https://assets.fanart.tv/") {
			t.Errorf("image URL %q is not https", im.GetUrl())
		}
	}
	posters := byType(imgs, pluginv1.ImageType_IMAGE_TYPE_PRIMARY)
	if posters[0].GetLanguage() != "zh" || posters[2].GetLanguage() != "" || posters[1].GetLikes() != 5 {
		t.Errorf("language/likes not carried: %v", posters)
	}
	if d := byType(imgs, pluginv1.ImageType_IMAGE_TYPE_DISC); len(d) != 1 || d[0].GetSourceId() != "40" {
		t.Errorf("disc = %v", d)
	}
	if b := byType(imgs, pluginv1.ImageType_IMAGE_TYPE_BANNER); len(b) != 1 || b[0].GetLikes() != 4 {
		t.Errorf("a numeric likes must be read too: %v", b)
	}
	if a := byType(imgs, pluginv1.ImageType_IMAGE_TYPE_ART); len(a) != 1 || a[0].GetLikes() != 0 {
		t.Errorf("an unreadable likes counts as 0: %v", a)
	}
}

func TestMovieFallsBackToIMDbAndAnswersFromTheCache(t *testing.T) {
	f := newFake(t, serve(movieBody))
	p := configured(t, f)
	req := &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"imdb": "tt0000001"}}
	for range 3 {
		if _, err := p.GetImages(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if f.hits.Load() != 1 || f.paths[0] != "/movies/tt0000001" {
		t.Errorf("hits %d paths %v: want one request by IMDb ID, then the cache", f.hits.Load(), f.paths)
	}
}

func TestTypesAndLimit(t *testing.T) {
	f := newFake(t, serve(movieBody))
	p := configured(t, f)
	resp, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{
		Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": "1001"},
		Types: []pluginv1.ImageType{pluginv1.ImageType_IMAGE_TYPE_PRIMARY, pluginv1.ImageType_IMAGE_TYPE_LOGO}, LimitPerType: 1,
		Languages: &pluginv1.Languages{Language: "en"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(resp.GetImages()); got != "20,10" {
		t.Errorf("images = %s, want one poster then one logo (20,10)", got)
	}
}

func TestSeriesImages(t *testing.T) {
	f := newFake(t, serve(tvBody))
	p := configured(t, f)
	resp, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{
		Type: pluginv1.ItemType_ITEM_TYPE_SERIES, ProviderIds: map[string]string{"tmdb": "7", "tvdb": "2002"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.paths[0] != "/tv/2002" {
		t.Errorf("path %q, want /tv/2002 (series are looked up by TVDB ID)", f.paths[0])
	}
	if got := ids(resp.GetImages()); got != "3,4,1,2" {
		t.Errorf("images = %s, want poster, background, hd logo, logo (season images are not returned)", got)
	}
}

func TestNothingToAskIsAnEmptyListWithoutARequest(t *testing.T) {
	f := newFake(t, serve(movieBody))
	p := configured(t, f)
	for name, req := range map[string]*pluginv1.GetImagesRequest{
		"series without a TVDB id": {Type: pluginv1.ItemType_ITEM_TYPE_SERIES, ProviderIds: map[string]string{"tmdb": "7"}},
		"movie without ids":        {Type: pluginv1.ItemType_ITEM_TYPE_MOVIE},
		"malformed ids":            {Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": "../x", "imdb": "nm0000001"}},
	} {
		resp, err := p.GetImages(context.Background(), req)
		if err != nil || len(resp.GetImages()) != 0 {
			t.Errorf("%s: %v, %v", name, resp, err)
		}
	}
	if f.hits.Load() != 0 {
		t.Errorf("%d requests were sent", f.hits.Load())
	}
	if _, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("episode: %v, want InvalidArgument", err)
	}
}

func TestUpstreamStatuses(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		header map[string]string
		code   codes.Code
	}{
		{"unknown id is no images", 404, nil, codes.OK},
		{"bad key", 401, nil, codes.FailedPrecondition},
		{"forbidden key", 403, nil, codes.FailedPrecondition},
		{"rate limit", 429, map[string]string{"Retry-After": "90"}, codes.Unknown},
		{"server error", 503, nil, codes.Unavailable},
	} {
		f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
			for k, v := range c.header {
				w.Header().Set(k, v)
			}
			w.WriteHeader(c.status)
		})
		p := configured(t, f)
		resp, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": "1"}})
		if status.Code(err) != c.code {
			t.Errorf("%s: code %v (%v), want %v", c.name, status.Code(err), err, c.code)
		}
		if err == nil && len(resp.GetImages()) != 0 {
			t.Errorf("%s: images %v", c.name, resp.GetImages())
		}
		if rl, ok := errors.AsType[*sdk.RateLimitError](err); c.status == 429 && (!ok || rl.RetryAfter != 90*time.Second) {
			t.Errorf("%s: %v, want a rate limit of 90s", c.name, err)
		}
		// Only "no images" is remembered; errors are asked again, except that a rejected key is forgotten, so no further request goes out until a new key is saved.
		_, _ = p.GetImages(context.Background(), &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": "1"}})
		want := int32(2)
		if c.code == codes.OK || c.status == 401 || c.status == 403 {
			want = 1
		}
		if f.hits.Load() != want {
			t.Errorf("%s: %d requests, want %d", c.name, f.hits.Load(), want)
		}
	}
}

func TestWithoutAKeyTheCallNeedsConfiguration(t *testing.T) {
	f := newFake(t, serve(movieBody))
	p := newPlugin()
	p.api = &api{http: f.Client()}
	_, err := p.GetImages(context.Background(), &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": "1"}})
	if status.Code(err) != codes.FailedPrecondition || f.hits.Load() != 0 {
		t.Errorf("err %v, %d requests", err, f.hits.Load())
	}
}

func TestRequestsAreSpacedAndOneAtATime(t *testing.T) {
	var inflight, peak atomic.Int32
	f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		n := inflight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inflight.Add(-1)
		_, _ = w.Write([]byte(`{}`))
	})
	p := configured(t, f)
	p.api.interval = 60 * time.Millisecond
	start := time.Now()
	done := make(chan struct{})
	for i := range 3 {
		go func() {
			_, _ = p.GetImages(context.Background(), &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, ProviderIds: map[string]string{"tmdb": string(rune('1' + i))}})
			done <- struct{}{}
		}()
	}
	for range 3 {
		<-done
	}
	if peak.Load() != 1 || time.Since(start) < 120*time.Millisecond {
		t.Errorf("peak %d in flight, took %v", peak.Load(), time.Since(start))
	}
}

func TestImageLanguageOrder(t *testing.T) {
	o := languageOrder(&pluginv1.Languages{Language: "zh-TW", Fallbacks: []string{"ja"}})
	for _, c := range []struct {
		lang     string
		backdrop bool
		want     int
	}{{"zh", false, 0}, {"ja", false, 1}, {"", false, 2}, {"en", false, 3}, {"de", false, 4}, {"", true, -1}} {
		if got := o.rank(c.lang, c.backdrop); got != c.want {
			t.Errorf("rank(%q, backdrop %v) = %d, want %d", c.lang, c.backdrop, got, c.want)
		}
	}
	if got := languageOrder(nil).tags; len(got) != 1 || got[0] != "en" {
		t.Errorf("default order = %v", got)
	}
}
