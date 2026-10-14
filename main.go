// Command fanart is the official image provider plugin for fanart.tv (change request 0146, capabilities images.movie, images.series).
//
// The user enters their own personal API key from their fanart.tv account. The plugin returns clear logos, clear art, discs, banners,
// thumbs, posters and backgrounds by TMDB or IMDb ID (movies) or TVDB ID (series); the host downloads and caches the images.
package main

import "github.com/kryspal/plugin-sdk/sdk"

func main() {
	p := newPlugin()
	sdk.Main(sdk.Config{Plugin: p, ImageProvider: p})
}
