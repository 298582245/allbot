package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

func TestCompileOpenAPIIPRulesAndMappedIPv4(t *testing.T) {
	rules, err := compileOpenAPIIPRules([]string{"192.0.2.1", "2001:db8::/32"}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"192.0.2.1", "::ffff:192.0.2.1", "2001:db8::1234"} {
		if !rules.contains(parseOpenAPIAddress(value)) {
			t.Fatalf("expected %s to match %#v", value, rules.raw)
		}
	}
	if rules.contains(parseOpenAPIAddress("192.0.2.2")) {
		t.Fatal("unexpected exact IP match")
	}
	for _, values := range [][]string{{}, {"*", "127.0.0.1"}, {"invalid"}, {"192.0.2.1:80"}, {"example.com"}, {"192.168.*.*"}, {"192.0.2.1-192.0.2.9"}, {"fe80::1%eth0"}} {
		if _, err := compileOpenAPIIPRules(values, true, true); err == nil {
			t.Fatalf("expected invalid rules %#v to fail", values)
		}
	}
	if _, err := compileOpenAPIIPRules([]string{"*"}, false, false); err == nil {
		t.Fatal("trusted proxies must reject wildcard")
	}
}

func TestResolveOpenAPIClientIPTrustChain(t *testing.T) {
	server := NewServer("0", nil, nil, nil, nil)
	settings := config.OpenAPISettings{IPWhitelist: []string{"*"}, TrustedProxies: []string{"10.0.0.0/8", "192.0.2.10"}, RetentionDays: 30}
	access, err := compileOpenAPIAccessConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	server.openAPIAccess.Store(access)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.9, 192.0.2.10, 10.0.0.3")
	if got := server.resolveOpenAPIClientIP(request).String(); got != "198.51.100.9" {
		t.Fatalf("client IP = %s", got)
	}
	request.RemoteAddr = "203.0.113.8:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.9")
	if got := server.resolveOpenAPIClientIP(request).String(); got != "203.0.113.8" {
		t.Fatalf("untrusted remote must ignore proxy headers, got %s", got)
	}
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Del("X-Forwarded-For")
	request.Header.Set("X-Real-IP", "::ffff:198.51.100.7")
	if got := server.resolveOpenAPIClientIP(request).String(); got != "198.51.100.7" {
		t.Fatalf("X-Real-IP fallback = %s", got)
	}
}

func TestEffectiveOpenAPIIPRulesInheritanceAndOverride(t *testing.T) {
	server := NewServer("0", nil, nil, nil, nil)
	_, _, source, err := server.effectiveOpenAPIIPRules(&types.OpenAPIEndpoint{})
	if err != nil || source != "default" {
		t.Fatalf("missing global settings should use default source: source=%s err=%v", source, err)
	}
	access, err := compileOpenAPIAccessConfig(config.OpenAPISettings{IPWhitelist: []string{"192.0.2.0/24"}, RetentionDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	server.openAPIAccess.Store(access)
	endpoint := &types.OpenAPIEndpoint{}
	rules, mode, _, err := server.effectiveOpenAPIIPRules(endpoint)
	if err != nil || mode != "inherit" || !rules.contains(parseOpenAPIAddress("192.0.2.5")) {
		t.Fatalf("unexpected inherited rules: %#v mode=%s err=%v", rules.raw, mode, err)
	}
	override := []string{"203.0.113.10"}
	endpoint.IPWhitelist = &override
	rules, mode, source, err = server.effectiveOpenAPIIPRules(endpoint)
	if err != nil || mode != "custom" || source != "endpoint" || rules.contains(parseOpenAPIAddress("192.0.2.5")) || !rules.contains(parseOpenAPIAddress("203.0.113.10")) {
		t.Fatalf("unexpected endpoint rules: %#v mode=%s source=%s err=%v", rules.raw, mode, source, err)
	}
}

func TestOpenAPIIPDeniedBeforeBodyAndCallClassification(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		endpoint := types.OpenAPIEndpoint{ID: "qr", Name: "QR", Path: "qr", Method: http.MethodGet, Enabled: true, Token: "secret", Runtime: "builtin", Builtin: "qrcode"}
		whitelist := []string{"192.0.2.1"}
		endpoint.IPWhitelist = &whitelist
		if _, err := saveOpenAPIEndpoint(endpoint, nil); err != nil {
			t.Fatal(err)
		}

		request := httptest.NewRequest(http.MethodGet, "/api/open/qr?text=hello&token=secret", errorReadCloser{})
		request.RemoteAddr = "198.51.100.9:1234"
		response := httptest.NewRecorder()
		server.handleOpenAPI(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("expected IP denial, got %d: %s", response.Code, response.Body.String())
		}
		if err := server.openAPIStats.Flush(); err != nil {
			t.Fatal(err)
		}
		stats, err := server.runtimeDatabase().GetOpenAPICallStats([]string{"qr"})
		if err != nil || stats["qr"].Rejected != 1 || stats["qr"].LastOutcome != config.OpenAPICallOutcomeIPDenied {
			t.Fatalf("unexpected denied stats: %#v err=%v", stats, err)
		}

		whitelist = []string{"*"}
		endpoint.IPWhitelist = &whitelist
		if _, err := saveOpenAPIEndpoint(endpoint, nil); err != nil {
			t.Fatal(err)
		}
		request = httptest.NewRequest(http.MethodGet, "/api/open/qr?text=hello&token=secret", nil)
		request.RemoteAddr = "198.51.100.9:1234"
		response = httptest.NewRecorder()
		server.handleOpenAPI(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("expected builtin success, got %d type=%s", response.Code, response.Header().Get("Content-Type"))
		}
		if err := server.openAPIStats.Flush(); err != nil {
			t.Fatal(err)
		}
		stats, err = server.runtimeDatabase().GetOpenAPICallStats([]string{"qr"})
		if err != nil || stats["qr"].Total != 2 || stats["qr"].Success != 1 {
			t.Fatalf("unexpected success stats: %#v err=%v", stats, err)
		}
	})
}

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("body should not be read") }
func (errorReadCloser) Close() error             { return nil }

type fakeOpenAPIStatsWriter struct {
	mu       chan struct{}
	stats    []config.OpenAPICallStatDelta
	logs     []config.OpenAPICallLog
	failNext bool
}

func (w *fakeOpenAPIStatsWriter) WriteOpenAPICallBatch(stats []config.OpenAPICallStatDelta, logs []config.OpenAPICallLog) error {
	w.mu <- struct{}{}
	defer func() { <-w.mu }()
	if w.failNext {
		w.failNext = false
		return errors.New("write failed")
	}
	w.stats = append(w.stats, stats...)
	w.logs = append(w.logs, logs...)
	return nil
}

func (w *fakeOpenAPIStatsWriter) CleanupOpenAPICallLogsBatch(int, int, time.Time) (int64, error) {
	return 0, nil
}

func TestOpenAPIStatsRecorderFlushMergeAndClose(t *testing.T) {
	writer := &fakeOpenAPIStatsWriter{mu: make(chan struct{}, 1), failNext: true}
	recorder := newOpenAPIStatsRecorder(writer, func() int { return 0 })
	recorder.Record(config.OpenAPICallLog{EndpointID: "alpha", StatusCode: 200, Outcome: config.OpenAPICallOutcomeSuccess, StartedAt: time.Now()})
	if err := recorder.Flush(); err == nil {
		t.Fatal("expected first flush failure")
	}
	if err := recorder.Flush(); err != nil {
		t.Fatalf("second flush failed: %v", err)
	}
	if len(writer.stats) != 1 || writer.stats[0].Total != 1 || len(writer.logs) != 0 {
		t.Fatalf("stats should merge back while failed logs may drop: stats=%#v logs=%#v", writer.stats, writer.logs)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAPIAdminSettingsEndpointStateAndCallFilters(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		settings := map[string]interface{}{"ip_whitelist": []string{"192.0.2.1"}, "trusted_proxies": []string{"127.0.0.1"}, "call_log_retention_days": 7}
		response := performOpenAPIJSONRequest(t, server.handleOpenAPIConfigDetail, http.MethodPut, "/api/openapis/settings", settings)
		if response.Code != http.StatusOK {
			t.Fatalf("settings PUT = %d: %s", response.Code, response.Body.String())
		}
		endpointDir := filepath.Join("openapis", "state")
		if err := os.MkdirAll(endpointDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(endpointDir, "config.json"), []byte(`{"id":"state","name":"State","path":"state","method":"GET","enabled":true,"token":"secret","runtime":"builtin","builtin":"qrcode"}`), 0644); err != nil {
			t.Fatal(err)
		}
		response = performOpenAPIJSONRequest(t, server.handleOpenAPIConfigDetail, http.MethodPut, "/api/open-apis/state", map[string]interface{}{"name": "Changed"})
		if response.Code != http.StatusOK {
			t.Fatalf("endpoint PUT preserve = %d: %s", response.Code, response.Body.String())
		}
		loaded, err := loadOpenAPIEndpoint("state")
		if err != nil || loaded.IPWhitelist != nil {
			t.Fatalf("missing field must preserve inherit: %#v err=%v", loaded, err)
		}
		response = performOpenAPIJSONRequest(t, server.handleOpenAPIConfigDetail, http.MethodPut, "/api/open-apis/state", map[string]interface{}{"ip_whitelist": []string{"203.0.113.5"}})
		if response.Code != http.StatusOK {
			t.Fatalf("endpoint PUT override = %d: %s", response.Code, response.Body.String())
		}
		loaded, _ = loadOpenAPIEndpoint("state")
		if loaded.IPWhitelist == nil || len(*loaded.IPWhitelist) != 1 {
			t.Fatalf("override not persisted: %#v", loaded)
		}
		response = performOpenAPIJSONRequest(t, server.handleOpenAPIConfigDetail, http.MethodPut, "/api/open-apis/state", map[string]interface{}{"ip_whitelist": nil})
		if response.Code != http.StatusOK {
			t.Fatalf("endpoint PUT clear = %d: %s", response.Code, response.Body.String())
		}
		loaded, _ = loadOpenAPIEndpoint("state")
		if loaded.IPWhitelist != nil {
			t.Fatalf("null should clear override: %#v", loaded)
		}

		request := httptest.NewRequest(http.MethodGet, "/api/open-apis/state/calls?limit=0", nil)
		response = httptest.NewRecorder()
		server.handleOpenAPIConfigDetail(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid calls limit = %d: %s", response.Code, response.Body.String())
		}
	})
}
