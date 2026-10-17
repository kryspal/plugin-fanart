# fanart: fanart.tv image provider plugin

An official Kryspal Media plugin that provides artwork from fanart.tv. It implements the protocol v1 `ImageProvider` service with [`github.com/kryspal/plugin-sdk`](https://github.com/kryspal/plugin-sdk) and declares the capabilities `images.movie` and `images.series`. After the administrator enters their key and enables the plugin, it can be ticked under "Image sources" in the library settings; the server merges its candidates with those of other image providers.

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

## Notices

On a host that lists `scrape_notices` in `KRYSPAL_HOST_FEATURES`, a title for which fanart.tv has no image of any type (404 or an empty answer) gets one `not_found` notice. Errors (bad key, rate limit, server error) stay errors, as before. The sentence is fixed and never contains the key or a URL.

## Settings

| Key | Type | Description |
| --- | --- | --- |
| `api_key` | secret, required | Personal API key (32 hexadecimal characters); `Configure` checks the shape, not the network |
| `base_url` | string (advanced) | Defaults to `https://webservice.fanart.tv/v3`; use only with a trusted HTTPS proxy |

## Version and bundling

The version number is written only in `VERSION` (embedded with `go:embed`; it is also the plugin's reported version). `CATALOG.json` holds the display data for the plugin catalog. Releases are built by the CI workflows in `.github/workflows/`: a `v*` tag builds linux/amd64 and linux/arm64 packages and creates a GitHub Release. Any change that alters the binary needs a new `VERSION`; a published version is never changed.

## Tests

`go test ./...` (`-short` skips the process test): recorded-shape responses with made-up IDs (`images_test.go`: conversion, ordering, HTTPS upgrade, likes parsing, type filter and limit, IMDb fallback, cache, empty results without requests, 401/403/404/429/5xx, one request at a time and spacing) and `plugin_test.go` (`TestDescribe`, `TestConfigure`, `TestPluginProcess`). Tests never call the real API.

## Official builds and releases

This repository is the source code of the official Kryspal Media `fanart` plugin. Official versions are built by this repository's GitHub Actions workflow from a `v*` tag, attached to a GitHub Release, and then added to the Kryspal Media plugin catalog, which Kryspal signs. The Kryspal Media server installs and updates official plugins from that signed catalog and verifies the signature.

A binary you build from this source yourself is **not** signed by Kryspal and is not part of the official catalog. You are free to build, modify and run it under the Apache License 2.0, but the server treats it as a third-party plugin.

Release process (maintainers): update `VERSION` and `CHANGELOG.md` for any change that alters the binary, wait for the CI workflow to pass on `main`, then push an annotated tag `vX.Y.Z` that matches `VERSION`. The release workflow builds linux/amd64 and linux/arm64 packages, creates the GitHub Release and notifies the main repository, which verifies the packages and publishes the catalog. A published version is never changed.

## License

Apache License 2.0, see `LICENSE` and `NOTICE`.
