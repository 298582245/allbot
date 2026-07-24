package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

func TestJSONResponseUsesUnifiedShape(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	server.jsonResponse(recorder, map[string]interface{}{"value": "ok"})

	assertJSONContentType(t, recorder)
	var response struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}
	decodeResponseJSON(t, recorder, &response)
	if response.Code != http.StatusOK || response.Msg != "成功" || response.Data["value"] != "ok" {
		t.Fatalf("unexpected success response: %#v", response)
	}
}

func TestJSONResponsePreservesNilAndEmptyData(t *testing.T) {
	server := &Server{}
	tests := []struct {
		name  string
		data  interface{}
		check func(t *testing.T, value interface{})
	}{
		{name: "nil", data: nil, check: func(t *testing.T, value interface{}) {
			if value != nil {
				t.Fatalf("data = %#v, expected nil", value)
			}
		}},
		{name: "empty slice", data: []string{}, check: func(t *testing.T, value interface{}) {
			items, ok := value.([]interface{})
			if !ok || len(items) != 0 {
				t.Fatalf("data = %#v, expected empty array", value)
			}
		}},
		{name: "object", data: map[string]interface{}{"ok": true}, check: func(t *testing.T, value interface{}) {
			object, ok := value.(map[string]interface{})
			if !ok || object["ok"] != true {
				t.Fatalf("data = %#v, expected object", value)
			}
		}},
		{name: "string", data: "text", check: func(t *testing.T, value interface{}) {
			if value != "text" {
				t.Fatalf("data = %#v, expected string", value)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.jsonResponse(recorder, test.data)
			var response map[string]interface{}
			decodeResponseJSON(t, recorder, &response)
			test.check(t, response["data"])
		})
	}
}

func TestJSONErrorUsesHTTPStatusAndNullData(t *testing.T) {
	server := &Server{}
	tests := []struct {
		status int
		msg    string
	}{
		{http.StatusBadRequest, "请求无效"},
		{http.StatusUnauthorized, "未授权"},
		{http.StatusNotFound, "不存在"},
		{http.StatusMethodNotAllowed, "方法不允许"},
		{http.StatusInternalServerError, "服务器错误"},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.jsonError(recorder, test.msg, test.status)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, expected %d", recorder.Code, test.status)
			}
			assertJSONContentType(t, recorder)
			var response struct {
				Code int         `json:"code"`
				Msg  string      `json:"msg"`
				Data interface{} `json:"data"`
			}
			decodeResponseJSON(t, recorder, &response)
			if response.Code != test.status || response.Msg != test.msg || response.Data != nil {
				t.Fatalf("unexpected error response: %#v", response)
			}
		})
	}
}

func TestRepresentativeAdminHandlerUsesUnifiedResponse(t *testing.T) {
	server := NewServer("0", nil, nil, nil, fstest.MapFS{"index.html": {Data: []byte("index")}})
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{name: "adapter platforms method", handler: server.handleAdapterPlatforms, method: http.MethodPost},
		{name: "settings method", handler: server.handleSettings, method: http.MethodPatch},
		{name: "plugin list method", handler: server.handlePlugins, method: http.MethodPatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.handler(recorder, httptest.NewRequest(test.method, "/api/test", nil))
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, expected 405", recorder.Code)
			}
			assertUnifiedErrorBody(t, recorder, http.StatusMethodNotAllowed)
		})
	}
	server.logManager.Stop()
}

func TestAuthMiddlewareReturnsUnifiedUnauthorizedResponse(t *testing.T) {
	server := &Server{sessions: map[string]adminSession{}}
	handler := server.authMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("authenticated handler should not be called")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/settings", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, expected 401", recorder.Code)
	}
	assertUnifiedErrorBody(t, recorder, http.StatusUnauthorized)
}

func TestOptionsMiddlewareReturnsEmpty200WithoutJSONEnvelope(t *testing.T) {
	server := &Server{}
	handler := server.corsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("OPTIONS should not reach the next handler")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodOptions, "/api/settings", nil))

	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
		t.Fatalf("OPTIONS response = status %d body %q, expected empty 200", recorder.Code, recorder.Body.String())
	}
}

func TestRawProtocolBoundariesRemainUnwrapped(t *testing.T) {
	server := &Server{}
	tests := []struct {
		name string
		call func(http.ResponseWriter)
		want string
	}{
		{name: "open api custom writer", call: func(w http.ResponseWriter) {
			writeOpenAPIResponse(w, types.OpenAPIResponse{Status: http.StatusAccepted, Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"}, Body: "open-api"})
		}, want: "open-api"},
		{name: "open api raw error", call: func(w http.ResponseWriter) {
			server.handleOpenAPI(w, httptest.NewRequest(http.MethodGet, "/api/open/", nil))
		}, want: `{"error":"Open API 路径不能为空"}`},
		{name: "plugin web custom writer", call: func(w http.ResponseWriter) {
			writePluginWebResponse(w, plugincore.PluginWebResponse{JSON: map[string]bool{"ok": true}})
		}, want: `{"ok":true}`},
		{name: "payment callback method error", call: func(w http.ResponseWriter) {
			server.handlePaymentNotifyEpay(w, httptest.NewRequest(http.MethodPut, "/api/open/payments/notify/epay", nil))
		}, want: "Method not allowed"},
		{name: "platform callback", call: func(w http.ResponseWriter) {
			server.handleFeishuAdapterCallback(w, httptest.NewRequest(http.MethodGet, "/api/open/adapters/feishu/1/event", nil))
		}, want: "Method not allowed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.call(recorder)
			body := strings.TrimSpace(recorder.Body.String())
			if body != test.want {
				t.Fatalf("body = %q, expected %q", body, test.want)
			}
			if strings.Contains(body, `"code":`) || strings.Contains(body, `"msg":`) || strings.Contains(body, `"data":`) {
				t.Fatalf("raw protocol response was wrapped: %q", body)
			}
		})
	}
}

func assertJSONContentType(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, expected application/json; charset=utf-8", got)
	}
}

func assertUnifiedErrorBody(t *testing.T, recorder *httptest.ResponseRecorder, status int) {
	t.Helper()
	var response struct {
		Code int         `json:"code"`
		Msg  string      `json:"msg"`
		Data interface{} `json:"data"`
	}
	decodeResponseJSON(t, recorder, &response)
	if response.Code != status || response.Msg == "" || response.Data != nil {
		t.Fatalf("unexpected unified error response: %#v", response)
	}
}

func decodeResponseJSON(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response failed: %v, body=%q", err, recorder.Body.String())
	}
}

func decodeUnifiedResponseData(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	decodeResponseJSON(t, recorder, &response)
	if len(response.Data) == 0 {
		t.Fatalf("response missing data: %s", recorder.Body.String())
	}
	if err := json.Unmarshal(response.Data, target); err != nil {
		t.Fatalf("decode response data failed: %v, body=%q", err, recorder.Body.String())
	}
}
