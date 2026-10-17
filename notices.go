package main

import (
	"os"

	pluginapi "github.com/kryspal/plugin-sdk"
	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

// hostShowsNotices reports whether the host keeps notices (host feature scrape_notices). A host without it ignores them, so nothing is sent to it.
func hostShowsNotices() bool {
	return pluginapi.HostFeatures(os.Getenv)[pluginapi.HostFeatureScrapeNotices]
}

// noImagesNotice tells the user that fanart.tv knows no images for the title. The sentence is fixed: it never carries the key or a URL.
func noImagesNotice() []*pluginv1.Notice {
	return []*pluginv1.Notice{{
		Source:  "fanart.tv",
		Status:  pluginv1.NoticeStatus_NOTICE_STATUS_NOT_FOUND,
		Message: "fanart.tv has no images for this title.",
	}}
}
