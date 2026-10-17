package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
	"github.com/kryspal/plugin-sdk/sdk"
)

// The fanart.tv v3 API (https://fanart.tv/api-docs/):
//
//	GET {base}/movies/{tmdb or imdb id}   header api-key
//	GET {base}/tv/{tvdb id}               header api-key
//
// The response is an object whose keys are image kinds (hdmovielogo, movieposter, hdtvlogo, showbackground, …), each an array of
// {id, url, lang, likes} (likes is a string; disc images also carry disc and disc_type); 404 means the id is unknown.

const defaultLimitPerType = 10

// kindMap maps a fanart.tv key to the protocol's image type; hd marks the high-resolution variant, which sorts before its standard counterpart.
type kindSpec struct {
	key  string
	kind pluginv1.ImageType
	hd   bool
}

var movieKinds = []kindSpec{
	{"movieposter", pluginv1.ImageType_IMAGE_TYPE_PRIMARY, false},
	{"moviebackground", pluginv1.ImageType_IMAGE_TYPE_BACKDROP, false},
	{"hdmovielogo", pluginv1.ImageType_IMAGE_TYPE_LOGO, true},
	{"movielogo", pluginv1.ImageType_IMAGE_TYPE_LOGO, false},
	{"moviethumb", pluginv1.ImageType_IMAGE_TYPE_THUMB, false},
	{"moviebanner", pluginv1.ImageType_IMAGE_TYPE_BANNER, false},
	{"hdmovieclearart", pluginv1.ImageType_IMAGE_TYPE_ART, true},
	{"movieart", pluginv1.ImageType_IMAGE_TYPE_ART, false},
	{"moviedisc", pluginv1.ImageType_IMAGE_TYPE_DISC, false},
}

var seriesKinds = []kindSpec{
	{"tvposter", pluginv1.ImageType_IMAGE_TYPE_PRIMARY, false},
	{"showbackground", pluginv1.ImageType_IMAGE_TYPE_BACKDROP, false},
	{"hdtvlogo", pluginv1.ImageType_IMAGE_TYPE_LOGO, true},
	{"clearlogo", pluginv1.ImageType_IMAGE_TYPE_LOGO, false},
	{"tvthumb", pluginv1.ImageType_IMAGE_TYPE_THUMB, false},
	{"tvbanner", pluginv1.ImageType_IMAGE_TYPE_BANNER, false},
	{"hdclearart", pluginv1.ImageType_IMAGE_TYPE_ART, true},
	{"clearart", pluginv1.ImageType_IMAGE_TYPE_ART, false},
}

// flexInt reads a count that fanart.tv sends as a string ("12") or, occasionally, a number.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		n = 0
	}
	*f = flexInt(n)
	return nil
}

type fanartImage struct {
	ID    string  `json:"id"`
	URL   string  `json:"url"`
	Lang  string  `json:"lang"`
	Likes flexInt `json:"likes"`
}

// GetImages returns the images of a movie (by TMDB or IMDb ID) or a series (by TVDB ID). An unknown ID or one the plugin does not recognize
// returns an empty list (not an error).
func (p *plugin) GetImages(ctx context.Context, req *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	var kinds []kindSpec
	var path, id string
	ids := req.GetProviderIds()
	switch req.GetType() {
	case pluginv1.ItemType_ITEM_TYPE_MOVIE:
		kinds, path = movieKinds, "movies"
		if id = ids["tmdb"]; !digits(id) {
			if id = ids["imdb"]; !imdbOK(id) {
				id = ""
			}
		}
	case pluginv1.ItemType_ITEM_TYPE_SERIES:
		kinds, path = seriesKinds, "tv"
		if id = ids["tvdb"]; !digits(id) {
			id = ""
		}
	default:
		return nil, sdk.InvalidArgument("fanart: only movies and series have images")
	}
	if id == "" {
		return &pluginv1.GetImagesResponse{}, nil
	}
	key, base := p.settings()
	if key == "" {
		return nil, sdk.NeedsConfiguration("fanart.tv API key is not configured")
	}
	cacheKey := base + "|" + path + "|" + id
	body, hit := p.cache.get(cacheKey)
	if !hit {
		var raw map[string]json.RawMessage
		status, err := p.api.get(ctx, key, base+"/"+path+"/"+url.PathEscape(id), &raw)
		if errors.Is(err, errUnauthorized) {
			// The key is wrong for the whole plugin (not one member): forget it, so the plugin shows as needing configuration until a new key is saved.
			p.forgetKey(key)
			return nil, sdk.NeedsConfiguration(errUnauthorized.Error())
		}
		if err != nil {
			return nil, err
		}
		body = map[string][]fanartImage{}
		if status == http.StatusOK {
			for _, k := range kinds {
				var list []fanartImage
				if m, ok := raw[k.key]; ok && json.Unmarshal(m, &list) == nil {
					body[k.key] = list
				}
			}
		}
		// 404 (unknown id) and other 4xx are all treated as no images; cache it so it is not asked again.
		p.cache.put(cacheKey, body)
	}
	limit := int(req.GetLimitPerType())
	if limit <= 0 {
		limit = defaultLimitPerType
	}
	want := map[pluginv1.ImageType]bool{}
	for _, t := range req.GetTypes() {
		want[t] = true
	}
	order := languageOrder(req.GetLanguages())
	var out []*pluginv1.Image
	for _, kind := range []pluginv1.ImageType{pluginv1.ImageType_IMAGE_TYPE_PRIMARY, pluginv1.ImageType_IMAGE_TYPE_BACKDROP,
		pluginv1.ImageType_IMAGE_TYPE_LOGO, pluginv1.ImageType_IMAGE_TYPE_THUMB, pluginv1.ImageType_IMAGE_TYPE_BANNER,
		pluginv1.ImageType_IMAGE_TYPE_ART, pluginv1.ImageType_IMAGE_TYPE_DISC} {
		if len(want) > 0 && !want[kind] {
			continue
		}
		var cands []ranked
		for _, k := range kinds {
			if k.kind != kind {
				continue
			}
			for _, im := range body[k.key] {
				u, ok := httpsURL(im.URL)
				if !ok {
					continue
				}
				lang := imageLanguage(im.Lang)
				cands = append(cands, ranked{
					img:  &pluginv1.Image{Type: kind, Url: u, Language: lang, Likes: int32(im.Likes), SourceId: im.ID},
					rank: order.rank(lang, kind == pluginv1.ImageType_IMAGE_TYPE_BACKDROP), hd: k.hd,
				})
			}
		}
		sort.SliceStable(cands, func(i, j int) bool {
			a, b := cands[i], cands[j]
			if a.rank != b.rank {
				return a.rank < b.rank
			}
			if a.hd != b.hd {
				return a.hd
			}
			return a.img.GetLikes() > b.img.GetLikes()
		})
		if len(cands) > limit {
			cands = cands[:limit]
		}
		for _, c := range cands {
			out = append(out, c.img)
		}
	}
	resp := &pluginv1.GetImagesResponse{Images: out}
	if known := imageCount(body); known == 0 && hostShowsNotices() {
		resp.Notices = noImagesNotice() // fanart.tv answered, but has nothing for this ID (a title that has images, but none of the types asked for, says nothing)
	}
	return resp, nil
}

// imageCount is the number of images fanart.tv has for the title, of any type.
func imageCount(body map[string][]fanartImage) int {
	n := 0
	for _, l := range body {
		n += len(l)
	}
	return n
}

type ranked struct {
	img  *pluginv1.Image
	rank int
	hd   bool
}

func digits(s string) bool {
	if s == "" || len(s) > 12 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func imdbOK(s string) bool {
	return len(s) >= 9 && len(s) <= 10 && strings.HasPrefix(s, "tt") && digits(s[2:])
}

// httpsURL returns the image URL upgraded to https (fanart.tv still sends http:// for its assets); anything that is not an http(s) URL is dropped.
func httpsURL(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	u.Scheme = "https"
	return u.String(), true
}

// imageLanguage turns fanart.tv's lang ("en", "de", "00" for no text) into BCP 47 ("" when the image has no text).
func imageLanguage(l string) string {
	l = strings.ToLower(strings.TrimSpace(l))
	if l == "00" {
		return ""
	}
	return l
}

// langOrder ranks image languages: the preferred language, its fallbacks in order, then images without text (language ""), then English, then the rest.
type langOrder struct{ tags []string }

func languageOrder(l *pluginv1.Languages) langOrder {
	var tags []string
	if p := primary(l.GetLanguage()); p != "" {
		tags = append(tags, p)
	}
	for _, f := range l.GetFallbacks() {
		if p := primary(f); p != "" {
			tags = append(tags, p)
		}
	}
	if len(tags) == 0 {
		tags = append(tags, "en")
	}
	return langOrder{tags: tags}
}

// rank is the position of an image language in the order. Backgrounds put images without text first (text on a background is a flaw).
func (o langOrder) rank(lang string, backdrop bool) int {
	if lang == "" {
		if backdrop {
			return -1
		}
		return len(o.tags)
	}
	p := primary(lang)
	for i, t := range o.tags {
		if t == p {
			return i
		}
	}
	if p == "en" {
		return len(o.tags) + 1
	}
	return len(o.tags) + 2
}

func primary(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_"); i > 0 {
		tag = tag[:i]
	}
	return tag
}

// responseCache keeps parsed responses (including "no images") in memory so the same id is not asked again within ttl.
type responseCache struct {
	mu sync.Mutex
	m  map[string]cacheEntry
}

type cacheEntry struct {
	body    map[string][]fanartImage
	expires time.Time
}

const (
	cacheTTL     = 6 * time.Hour
	cacheEntries = 512
)

func (c *responseCache) get(key string) (map[string][]fanartImage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.expires) {
		delete(c.m, key)
		return nil, false
	}
	return e.body, true
}

func (c *responseCache) put(key string, body map[string][]fanartImage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]cacheEntry{}
	}
	if len(c.m) >= cacheEntries {
		now := time.Now()
		for k, e := range c.m {
			if now.After(e.expires) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= cacheEntries {
			clear(c.m)
		}
	}
	c.m[key] = cacheEntry{body: body, expires: time.Now().Add(cacheTTL)}
}
