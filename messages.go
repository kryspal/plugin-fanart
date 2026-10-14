package main

import "github.com/kryspal/plugin-sdk/i18n"

// messages is the declared string table: en, zh-TW and zh-CN; other languages follow the i18n fallback chain, ending in English.
// The attribution (attribution.text) is the English original, the same in every language, and is not here.
var messages = i18n.New(messageTables)

var messageTables = map[string]map[string]string{
	"en": {
		"desc":          "Fetches high-quality artwork for movies and series from fanart.tv: clear logos, clear art, discs, banners, thumbs, posters and backgrounds, chosen by the language you prefer and the likes from the fanart.tv community. Movies are matched by TMDB or IMDb ID and series by TVDB ID. Choose this plugin under \"Image sources\" in a library's settings to use it.",
		"groupAdvanced": "Advanced",
		"keyLabel":      "fanart.tv API key",
		"keyHelp":       "Your own personal API key from your fanart.tv account (32 characters). It is sent only to fanart.tv.",
		"keyLink":       "Get an API key",
		"baseURLLabel":  "API base URL",
		"baseURLHelp":   "Leave empty to use the official API. Only for a trusted HTTPS proxy.",
		"errRequired":   "Enter your fanart.tv API key.",
		"errKeyFormat":  "A fanart.tv API key is 32 letters and digits (0-9, a-f).",
		"errHTTPS":      "Must be an https:// address.",
	},
	"zh-TW": {
		"desc":          "從 fanart.tv 取得電影與劇集的高品質美術圖：透明標誌、透明美術圖、光碟、橫幅、縮圖、海報與背景，依你偏好的語言與 fanart.tv 社群的喜愛數挑選。電影以 TMDB 或 IMDb ID 配對，劇集以 TVDB ID 配對。要使用，請在媒體庫設定的「圖片來源」勾選這個外掛。",
		"groupAdvanced": "進階",
		"keyLabel":      "fanart.tv API 金鑰",
		"keyHelp":       "你自己的 fanart.tv 帳號的個人 API 金鑰（32 個字元），只會送給 fanart.tv。",
		"keyLink":       "取得 API 金鑰",
		"baseURLLabel":  "API 網址",
		"baseURLHelp":   "留空表示使用官方 API。只用於可信任的 HTTPS 代理。",
		"errRequired":   "請填入你的 fanart.tv API 金鑰。",
		"errKeyFormat":  "fanart.tv 的 API 金鑰是 32 個英數字（0-9、a-f）。",
		"errHTTPS":      "必須是 https:// 網址。",
	},
	"zh-CN": {
		"desc":          "从 fanart.tv 获取电影和剧集的高品质美术图：透明标志、透明美术图、光盘、横幅、缩略图、海报与背景，按你偏好的语言和 fanart.tv 社区的喜爱数挑选。电影用 TMDB 或 IMDb ID 匹配，剧集用 TVDB ID 匹配。要使用，请在媒体库设置的“图片来源”里勾选这个插件。",
		"groupAdvanced": "高级",
		"keyLabel":      "fanart.tv API 密钥",
		"keyHelp":       "你自己的 fanart.tv 账号的个人 API 密钥（32 个字符），只会发送给 fanart.tv。",
		"keyLink":       "获取 API 密钥",
		"baseURLLabel":  "API 网址",
		"baseURLHelp":   "留空表示使用官方 API。只用于可信任的 HTTPS 代理。",
		"errRequired":   "请填入你的 fanart.tv API 密钥。",
		"errKeyFormat":  "fanart.tv 的 API 密钥是 32 个字母和数字（0-9、a-f）。",
		"errHTTPS":      "必须是 https:// 网址。",
	},
}
