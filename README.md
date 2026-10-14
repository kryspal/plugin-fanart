# fanart: fanart.tv image provider plugin

Owner: A4. Official plugin (closed source, change request 0117), implementing the protocol v1 `ImageProvider` service with `github.com/kryspal/plugin-sdk/sdk` (change request 0146, phase 1). It declares the capabilities `images.movie` and `images.series` and no feature code (free plan). It is bundled in the image and reserved as an official ID; after the administrator enters their key and enables it on the plugin page, they tick it under "Image sources" in the library settings. The scraper (A2) merges its candidates through `domain.ImageProviders`.

## Data source and terms

fanart.tv serves artwork by TMDB/IMDb ID (movies) and TVDB ID (series). Each user enters **their own personal API key** (from their fanart.tv account); the plugin sends it only to fanart.tv, in the `api-key` header, never in a URL or log.

- **One request at a time**, at least 300 ms apart (`SuggestedConcurrency` 1); fanart.tv publishes no hard limit. A 429 returns `RESOURCE_EXHAUSTED` (with `Retry-After`), and the host pauses the plugin.
- **Cached in memory only** (6 hours, including "no images"); the plugin never stores images, it returns HTTPS URLs and the host downloads and caches them.
- **Attribution**: "Images provided by fanart.tv" goes in the manifest's attribution (the source line on the plugin page, with a link to fanart.tv).
- A rejected key (401/403) makes the plugin forget the key and report the needs-configuration state until a new key is saved.

## API

- Movies: `GET {base}/movies/{tmdb id}` (the IMDb ID `tt…` when there is no TMDB ID).
- Series: `GET {base}/tv/{tvdb id}`; a series without a TVDB ID returns an empty list with no request. Season images are not returned (the protocol has no season request).
- 404 (unknown id) means no images.

| fanart.tv key | Image type |
| --- | --- |
| `movieposter`, `tvposter` | primary (poster) |
| `moviebackground`, `showbackground` | backdrop |
| `hdmovielogo`, `movielogo`, `hdtvlogo`, `clearlogo` | logo |
| `moviethumb`, `tvthumb` | thumb |
| `moviebanner`, `tvbanner` | banner |
| `hdmovieclearart`, `movieart`, `hdclearart`, `clearart` | art |
| `moviedisc` | disc |

Order within each type: the preferred language, its fallbacks in order, images without text, English, the rest (backdrops put images without text first); then the HD variant before the standard one; then more likes first. `lang` `00` becomes an empty language. `http://` asset URLs are upgraded to `https://`; anything else is dropped. Default 10 images per type (`limit_per_type` overrides).

## Settings

| Key | Type | Description |
| --- | --- | --- |
| `api_key` | secret, required | Personal API key (32 hexadecimal characters); `Configure` checks the shape, not the network |
| `base_url` | string (advanced) | Defaults to `https://webservice.fanart.tv/v3`; use only with a trusted HTTPS proxy |

## Version and bundling

The version number is written only in `VERSION` (`go:embed`, also the `<version>` of the bundled directory). `deploy/plugins/build.sh` builds it; `CATALOG.json` is the display data for the plugin catalog.

## Tests

`go test ./plugins/fanart` (`-short` skips the process test): recorded-shape responses with made-up IDs (`images_test.go`: conversion, ordering, HTTPS upgrade, likes parsing, type filter and limit, IMDb fallback, cache, empty results without requests, 401/403/404/429/5xx, one request at a time and spacing) and `plugin_test.go` (`TestDescribe`, `TestConfigure`, `TestPluginProcess`). Tests never call the real API.
