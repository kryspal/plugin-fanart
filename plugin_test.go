package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kryspal/plugin-sdk"
	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

var han = regexp.MustCompile(`\p{Han}`)

func TestDescribe(t *testing.T) {
	p := newPlugin()
	for _, lang := range []string{"en", "zh-TW", "zh-CN", "", "ja"} {
		m, err := p.Describe(context.Background(), &pluginv1.DescribeRequest{Language: lang})
		if err != nil {
			t.Fatal(err)
		}
		if m.GetId() != "fanart" || m.GetName() != "fanart.tv" || m.GetDescription() == "" || m.GetVersion() != version() ||
			!slices.Equal(m.GetCapabilities(), []string{pluginv1.CapabilityImagesMovie, pluginv1.CapabilityImagesSeries}) ||
			len(m.GetFeatures()) != 0 || m.GetSuggestedConcurrency() != 1 || len(m.GetConfigFields()) != 2 {
			t.Errorf("Describe(%q) = %v", lang, m)
		}
		if !m.GetConfigFields()[0].GetRequired() || m.GetConfigFields()[0].GetType() != pluginv1.ConfigFieldType_CONFIG_FIELD_TYPE_SECRET {
			t.Errorf("the key must be a required secret: %v", m.GetConfigFields()[0])
		}
		if m.GetAttribution().GetText() != "Images provided by fanart.tv" || m.GetAttribution().GetLink() != "https://fanart.tv" {
			t.Errorf("attribution = %v", m.GetAttribution())
		}
		if (lang == "en" || lang == "" || lang == "ja") && han.MatchString(m.GetDescription()) {
			t.Errorf("Describe(%q): Han characters in the English text", lang)
		}
	}
	for _, tag := range messages.Tags() {
		for key := range messageTables["en"] {
			if messages.For(tag).T(key) == "" {
				t.Errorf("%s: missing %s", tag, key)
			}
		}
	}
	if strings.ContainsAny(messageTables["zh-TW"]["desc"], "么识断读网为标志") {
		t.Error("the zh-TW text has simplified characters")
	}
	if strings.ContainsAny(messageTables["zh-CN"]["desc"], "麼識斷讀網為標誌") {
		t.Error("the zh-CN text has traditional characters")
	}
}

func TestConfigure(t *testing.T) {
	p := newPlugin()
	for name, c := range map[string]struct {
		values map[string]string
		field  string // the field expected to error; empty means it passes
	}{
		"valid key":      {map[string]string{keyAPIKey: testKey}, ""},
		"upper case key": {map[string]string{keyAPIKey: strings.ToUpper(testKey)}, ""},
		"empty":          {map[string]string{}, keyAPIKey},
		"short key":      {map[string]string{keyAPIKey: "abc123"}, keyAPIKey},
		"not hex":        {map[string]string{keyAPIKey: strings.Repeat("g", 32)}, keyAPIKey},
		"http base url":  {map[string]string{keyAPIKey: testKey, keyBaseURL: "http://proxy.example"}, keyBaseURL},
		"https base url": {map[string]string{keyAPIKey: testKey, keyBaseURL: "https://proxy.example/v3/"}, ""},
	} {
		resp, err := p.Configure(context.Background(), &pluginv1.ConfigureRequest{Values: c.values})
		if err != nil {
			t.Fatal(err)
		}
		if c.field == "" && len(resp.GetFieldErrors()) != 0 || c.field != "" && resp.GetFieldErrors()[c.field] == "" {
			t.Errorf("%s: field errors = %v, want %q", name, resp.GetFieldErrors(), c.field)
		}
	}
	_, _ = p.Configure(context.Background(), &pluginv1.ConfigureRequest{Values: map[string]string{keyAPIKey: " " + testKey + " ", keyBaseURL: "https://proxy.example/v3/"}})
	if k, b := p.settings(); k != testKey || b != "https://proxy.example/v3" {
		t.Errorf("settings = %q %q", k, b)
	}
}

// TestPluginProcess is the conformance check of the plugin process (the host's way): it compiles the executable and starts it with the host's environment
// variables, checking the handshake, health check, Describe, GetImages (no usable ID returns an empty list, no network) and graceful exit after SIGTERM.
func TestPluginProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the plugin binary")
	}
	exe := filepath.Join(t.TempDir(), "fanart")
	if out, err := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	sockDir, err := os.MkdirTemp("", "fan-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(sockDir) }()
	cmd := exec.Command(exe)
	cmd.Env = []string{pluginapi.EnvCookie + "=" + pluginapi.CookieValue, pluginapi.EnvProtocols + "=1", pluginapi.EnvSocketDir + "=" + sockDir, "HOME=" + t.TempDir()}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	hs, err := pluginapi.ParseHandshake(strings.TrimSpace(line))
	if err != nil || hs.Protocol != 1 || hs.Network != pluginapi.NetworkUnix {
		t.Fatalf("handshake %q: %+v, %v", line, hs, err)
	}
	conn, err := grpc.NewClient("unix:"+hs.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	hc := healthpb.NewHealthClient(conn)
	for _, svc := range []string{"", pluginv1.HealthServiceImageProvider} {
		if r, err := hc.Check(ctx, &healthpb.HealthCheckRequest{Service: svc}); err != nil || r.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("health %q = %v, %v", svc, r, err)
		}
	}
	pc := pluginv1.NewPluginClient(conn)
	m, err := pc.Describe(ctx, &pluginv1.DescribeRequest{Language: "zh-TW"})
	if err != nil || m.GetId() != "fanart" || m.GetProtocolVersion() != pluginv1.ProtocolVersion {
		t.Fatalf("Describe = %v, %v", m, err)
	}
	resp, err := pluginv1.NewImageProviderClient(conn).GetImages(ctx, &pluginv1.GetImagesRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE})
	if err != nil || len(resp.GetImages()) != 0 {
		t.Errorf("GetImages without an ID = %v, %v", resp, err)
	}
	if _, err := pluginv1.NewMetadataProviderClient(conn).Search(ctx, &pluginv1.SearchRequest{}); err == nil {
		t.Error("the metadata service must not be registered")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	stopped = true
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("exit after SIGTERM: %v", err)
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Error("the plugin did not exit after SIGTERM")
	}
}
