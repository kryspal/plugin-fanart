package main

import (
	"context"
	_ "embed"
	"net/url"
	"regexp"
	"strings"
	"sync"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

const (
	pluginID = "fanart"

	keyAPIKey  = "api_key"
	keyBaseURL = "base_url"

	defaultBaseURL  = "https://webservice.fanart.tv/v3"
	attributionText = "Images provided by fanart.tv"
	attributionLink = "https://fanart.tv"
)

// apiKeyRE is the shape of a fanart.tv key: 32 hexadecimal characters.
var apiKeyRE = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// versionFile is the single source of the plugin version: the manifest's version.
//
//go:embed VERSION
var versionFile string

func version() string { return strings.TrimSpace(versionFile) }

type plugin struct {
	pluginv1.UnimplementedPluginServer
	pluginv1.UnimplementedImageProviderServer

	mu      sync.Mutex
	apiKey  string
	baseURL string
	api     *api // replaceable in tests
	cache   responseCache
}

func newPlugin() *plugin {
	return &plugin{baseURL: defaultBaseURL, api: newAPI()}
}

// Describe declares images.movie and images.series. The key is required: without it the plugin is in the needs-configuration state.
func (p *plugin) Describe(_ context.Context, req *pluginv1.DescribeRequest) (*pluginv1.Manifest, error) {
	t := messages.For(req.GetLanguage())
	return &pluginv1.Manifest{
		Id: pluginID, Name: "fanart.tv", Description: t.T("desc"), Version: version(), ProtocolVersion: pluginv1.ProtocolVersion,
		Capabilities: []string{pluginv1.CapabilityImagesMovie, pluginv1.CapabilityImagesSeries},
		ConfigGroups: []*pluginv1.ConfigGroup{{Key: "advanced", Title: t.T("groupAdvanced")}},
		ConfigFields: []*pluginv1.ConfigField{
			{Key: keyAPIKey, Type: pluginv1.ConfigFieldType_CONFIG_FIELD_TYPE_SECRET, Required: true, Label: t.T("keyLabel"),
				Description: t.T("keyHelp"), Link: "https://fanart.tv/get-an-api-key/", LinkLabel: t.T("keyLink")},
			{Key: keyBaseURL, Type: pluginv1.ConfigFieldType_CONFIG_FIELD_TYPE_STRING, Group: "advanced", Label: t.T("baseURLLabel"),
				Description: t.T("baseURLHelp"), Placeholder: defaultBaseURL},
		},
		Attribution:          &pluginv1.Attribution{Text: attributionText, Link: attributionLink},
		SuggestedConcurrency: 1,
	}, nil
}

// Configure validates the settings: the key is required and must be 32 hexadecimal characters; the URL must be https. It does not verify the key
// over the network (a rejected key is reported by the first call as an authentication failure).
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	t := messages.For(req.GetLanguage())
	v := req.GetValues()
	resp := &pluginv1.ConfigureResponse{FieldErrors: map[string]string{}}
	key := strings.TrimSpace(v[keyAPIKey])
	switch {
	case key == "":
		resp.FieldErrors[keyAPIKey] = t.T("errRequired")
	case !apiKeyRE.MatchString(key):
		resp.FieldErrors[keyAPIKey] = t.T("errKeyFormat")
	}
	base := strings.TrimSpace(v[keyBaseURL])
	if base == "" {
		base = defaultBaseURL
	}
	if u, err := url.Parse(base); err != nil || u.Scheme != "https" || u.Host == "" {
		resp.FieldErrors[keyBaseURL] = t.T("errHTTPS")
	}
	if len(resp.FieldErrors) > 0 {
		return resp, nil
	}
	p.mu.Lock()
	p.apiKey, p.baseURL = key, strings.TrimRight(base, "/")
	p.mu.Unlock()
	return resp, nil
}

func (p *plugin) settings() (key, base string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.apiKey, p.baseURL
}

// forgetKey clears the key after fanart.tv rejected it, unless a new one was saved in the meantime.
func (p *plugin) forgetKey(rejected string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.apiKey == rejected {
		p.apiKey = ""
	}
}
