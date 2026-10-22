# Changelog

All notable changes to this plugin are listed here. The version in `VERSION` is the single source of the version number; a published version is never changed.

## 1.1.1

- Built with plugin SDK v0.6.0. The fanart.tv client now uses the SDK's HTTP client. No functional change.

## 1.1.0

- Tells servers that show it when fanart.tv knows no images for a title ("not found" notice), so the user can tell "nothing there" from "not asked". Older servers get exactly the answer they got before.
- Built with plugin SDK v0.2.0. Needs server 0.9.2 or newer.

## 1.0.3

- Rebuilt with Go 1.27.2 and golang.org/x/net v0.60.0 (security fixes); no functional change.

## 1.0.2

- First release from the standalone repository; no functional change.

## 1.0.1

- Image provider for movies (TMDB/IMDb ID) and series (TVDB ID): posters, backdrops, logos, thumbs, banners, art and discs.
- Development history before this release lives in the Kryspal Media main repository; this repository starts from this version.
