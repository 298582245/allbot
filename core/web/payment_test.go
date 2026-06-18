package web

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/payment"
)

func TestHandlePaymentSettingsGetDefault(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/payments/settings", nil)

	server.handlePaymentSettings(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response config.PaymentSettings
	decodePaymentResponse(t, recorder, &response)
	if response.PointsPerRMB != 100 || response.CurrencyUnit != "RMB" || response.HidePayURL || len(response.Methods) != 1 || response.Methods[0].Code != "points" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if !strings.Contains(recorder.Body.String(), "\"hide_pay_url\"") {
		t.Fatalf("response should include hide_pay_url field: %s", recorder.Body.String())
	}
}

func TestHandlePaymentSettingsPutSuccess(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	payload := config.DefaultPaymentSettings()
	payload.PointsPerRMB = 66
	payload.CurrencyUnit = "元"
	payload.EpayQueryIntervalSeconds = 2
	payload.HidePayURL = true
	payload.QRCodeBaseURL = "https://qr.example.com/base"
	payload.EpaySubmitSubject = "后台伪造标题"
	payload.Methods = append(payload.Methods, config.PaymentMethodSetting{Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true})
	payload.Epay = config.EpaySettings{Enabled: true, Version: "v1", APIURL: "https://pay.example.com/", PID: "1000", Key: "secret"}
	recorder := performPaymentJSONRequest(t, server.handlePaymentSettings, http.MethodPut, "/api/payments/settings", payload)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response config.PaymentSettings
	decodePaymentResponse(t, recorder, &response)
	if response.PointsPerRMB != 66 || response.CurrencyUnit != "元" || response.EpayQueryIntervalSeconds != 2 || !response.HidePayURL || response.QRCodeBaseURL != "https://qr.example.com/base" || response.EpaySubmitSubject != "后台伪造标题" || !response.Epay.HasKey || response.Epay.Key != "" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestHandlePaymentSettingsPutRejectsInvalidPointsPerRMB(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	payload := config.DefaultPaymentSettings()
	payload.PointsPerRMB = 0
	recorder := performPaymentJSONRequest(t, server.handlePaymentSettings, http.MethodPut, "/api/payments/settings", payload)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentOrdersList(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_LIST")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/payments/orders?union_id=union-web", nil)

	server.handlePaymentOrders(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []config.PaymentOrder `json:"items"`
		Total int64                 `json:"total"`
	}
	decodePaymentResponse(t, recorder, &response)
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].OrderNo != "PWEB_LIST" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestHandlePaymentOrderDetail(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_DETAIL")
	if err := server.adapterManager.GetDatabase().AppendPaymentEvent("PWEB_DETAIL", "created", "订单创建", nil); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/payments/orders/PWEB_DETAIL", nil)

	server.handlePaymentOrderDetail(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Order  config.PaymentOrder   `json:"order"`
		Events []config.PaymentEvent `json:"events"`
	}
	decodePaymentResponse(t, recorder, &response)
	if response.Order.OrderNo != "PWEB_DETAIL" || len(response.Events) != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestHandlePaymentOrderDetailNotFound(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/payments/orders/missing", nil)

	server.handlePaymentOrderDetail(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentOrderDeleteSuccess(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_DELETE")
	if err := server.adapterManager.GetDatabase().AppendPaymentEvent("PWEB_DELETE", "created", "订单创建", nil); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/payments/orders/PWEB_DELETE", nil)

	server.handlePaymentOrderDetail(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Message string `json:"message"`
	}
	decodePaymentResponse(t, recorder, &response)
	if response.Message != "支付订单已删除" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if _, err := server.adapterManager.GetDatabase().GetPaymentOrder("PWEB_DELETE"); err != sql.ErrNoRows {
		t.Fatalf("expected deleted order to return sql.ErrNoRows, got %v", err)
	}
	events, err := server.adapterManager.GetDatabase().ListPaymentEvents("PWEB_DELETE")
	if err != nil {
		t.Fatalf("ListPaymentEvents returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected payment events deleted, got %#v", events)
	}
}

func TestHandlePaymentOrderDeleteNotFound(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/payments/orders/missing", nil)

	server.handlePaymentOrderDetail(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentOrderDeleteRejectsInvalidOrderNo(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	paths := []string{
		"/api/payments/orders/",
		"/api/payments/orders/PWEB_DELETE/extra",
	}
	for _, path := range paths {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, path, nil)

		server.handlePaymentOrderDetail(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s expected 400, got %d: %s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestHandlePaymentNotifyEpaySuccessAndDuplicate(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	configureWebEpay(t, server)
	createWebPaymentTestOrder(t, server, "PWEB_NOTIFY")
	query := webEpayNotifyQuery(map[string]string{"pid": "1000", "type": "alipay", "out_trade_no": "PWEB_NOTIFY", "trade_no": "TWEB1", "money": "1.00", "trade_status": "TRADE_SUCCESS"})
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/open/payments/notify/epay?"+query, nil)
		server.handlePaymentNotifyEpay(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Body.String() != "success" {
			t.Fatalf("expected success, got %d %s", recorder.Code, recorder.Body.String())
		}
	}
	order, err := server.adapterManager.GetDatabase().GetPaymentOrder("PWEB_NOTIFY")
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "paid" || order.ProviderOrderNo != "TWEB1" {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestHandlePaymentReturnEpayRequiresLocalOrderMatch(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	configureWebEpay(t, server)
	query := webEpayNotifyQuery(map[string]string{"pid": "1000", "type": "alipay", "out_trade_no": "PWEB_RETURN_MISSING", "trade_no": "TRETURN", "money": "1.00", "trade_status": "TRADE_SUCCESS"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/return/epay?"+query, nil)
	server.handlePaymentReturnEpay(recorder, request)
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "支付成功") {
		t.Fatalf("missing local order should not show success: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentNotifyEpayRejectsAmountMismatch(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	configureWebEpay(t, server)
	createWebPaymentTestOrder(t, server, "PWEB_NOTIFY_BAD")
	query := webEpayNotifyQuery(map[string]string{"pid": "1000", "type": "alipay", "out_trade_no": "PWEB_NOTIFY_BAD", "trade_no": "TWEBBAD", "money": "1.01", "trade_status": "TRADE_SUCCESS"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/notify/epay?"+query, nil)
	server.handlePaymentNotifyEpay(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "fail" {
		t.Fatalf("expected fail, got %d %s", recorder.Code, recorder.Body.String())
	}
	order, err := server.adapterManager.GetDatabase().GetPaymentOrder("PWEB_NOTIFY_BAD")
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "pending" {
		t.Fatalf("mismatch should not mark paid: %#v", order)
	}
}

func TestHandlePaymentOrderQueryRejectsIncompletePaidResult(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	configureWebEpay(t, server)
	order := createWebPaymentTestOrder(t, server, "PWEB_QUERY_INCOMPLETE")
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":1,"out_trade_no":"PWEB_QUERY_INCOMPLETE","trade_no":"TQUERY"}`))
	}))
	defer stub.Close()
	settings, err := server.adapterManager.GetDatabase().GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Epay.APIURL = stub.URL + "/"
	if err := server.adapterManager.GetDatabase().SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := server.adapterManager.GetDatabase().UpdatePaymentOrderProviderInfo(order.OrderNo, "TQUERY", "", "", "{}"); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/payments/orders/PWEB_QUERY_INCOMPLETE/query", nil)
	server.handlePaymentOrderDetail(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentOrderQueryRejectsNonEpayOrder(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	_, err := server.adapterManager.GetDatabase().CreatePaymentOrder(&config.PaymentOrder{OrderNo: "PWEB_POINTS", PluginID: "plugin-web", UnionID: "union-web", Subject: "积分订单", AmountCents: 100, PointsAmount: 100, Provider: "points", Method: "points", Status: "pending", Metadata: "{}", ExpiredAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/payments/orders/PWEB_POINTS/query", nil)
	server.handlePaymentOrderDetail(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleOpenAPIQRCodeReturnsPNGWithToken(t *testing.T) {
	withTempOpenAPIWorkdir(t, func() {
		writeTestQRCodeOpenAPI(t, "qrcode")
		server, cleanup := newPaymentTestServer(t)
		defer cleanup()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/open/qrcode?text=https%3A%2F%2Fexample.com%2Fpay&token=qrcode", nil)

		server.handleOpenAPI(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(recorder.Body.Bytes(), []byte("\x89PNG")) {
			t.Fatalf("expected png response, content-type=%s len=%d", recorder.Header().Get("Content-Type"), recorder.Body.Len())
		}
	})
}

func TestHandleOpenAPIQRCodeRejectsMissingToken(t *testing.T) {
	withTempOpenAPIWorkdir(t, func() {
		writeTestQRCodeOpenAPI(t, "qrcode")
		server, cleanup := newPaymentTestServer(t)
		defer cleanup()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/open/qrcode?text=https%3A%2F%2Fexample.com%2Fpay", nil)

		server.handleOpenAPI(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHandleOpenAPIQRCodeRejectsEmptyText(t *testing.T) {
	withTempOpenAPIWorkdir(t, func() {
		writeTestQRCodeOpenAPI(t, "qrcode")
		server, cleanup := newPaymentTestServer(t)
		defer cleanup()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/open/qrcode?token=qrcode", nil)

		server.handleOpenAPI(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHandleOpenAPIOldQRCodePathNotFound(t *testing.T) {
	withTempOpenAPIWorkdir(t, func() {
		writeTestQRCodeOpenAPI(t, "qrcode")
		server, cleanup := newPaymentTestServer(t)
		defer cleanup()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/open/qrcode.png?text=https%3A%2F%2Fexample.com%2Fpay&token=qrcode", nil)

		server.handleOpenAPI(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHandlePaymentQRCodeReturnsPNG(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_QRCODE")
	if err := server.adapterManager.GetDatabase().UpdatePaymentOrderProviderInfo("PWEB_QRCODE", "TQR", "https://pay.example.com/pay", "QR-CONTENT", "{}"); err != nil {
		t.Fatal(err)
	}
	token := payment.PaymentQRCodeToken("PWEB_QRCODE", "QR-CONTENT")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/qrcode/PWEB_QRCODE/"+token+".png", nil)
	server.handlePaymentQRCode(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(recorder.Body.Bytes(), []byte("\x89PNG")) {
		t.Fatalf("expected png response, content-type=%s len=%d", recorder.Header().Get("Content-Type"), recorder.Body.Len())
	}
}

func TestHandlePaymentQRCodeFallsBackToPayURL(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_QRCODE_PAYURL")
	payURL := "https://pay.example.com/pay-only"
	if err := server.adapterManager.GetDatabase().UpdatePaymentOrderProviderInfo("PWEB_QRCODE_PAYURL", "TQR", payURL, "", "{}"); err != nil {
		t.Fatal(err)
	}
	token := payment.PaymentQRCodeToken("PWEB_QRCODE_PAYURL", payURL)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/qrcode/PWEB_QRCODE_PAYURL/"+token+".png", nil)
	server.handlePaymentQRCode(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentQRCodeRejectsInvalidToken(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_QRCODE_BAD_TOKEN")
	if err := server.adapterManager.GetDatabase().UpdatePaymentOrderProviderInfo("PWEB_QRCODE_BAD_TOKEN", "TQR", "https://pay.example.com/pay", "QR-CONTENT", "{}"); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/qrcode/PWEB_QRCODE_BAD_TOKEN/bad.png", nil)
	server.handlePaymentQRCode(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentQRCodeRejectsMissingOrder(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/qrcode/PWEB_QRCODE_MISSING/bad.png", nil)
	server.handlePaymentQRCode(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePaymentQRCodeRejectsMissingContent(t *testing.T) {
	server, cleanup := newPaymentTestServer(t)
	defer cleanup()
	createWebPaymentTestOrder(t, server, "PWEB_QRCODE_EMPTY")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/payments/qrcode/PWEB_QRCODE_EMPTY/bad.png", nil)
	server.handlePaymentQRCode(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func withTempOpenAPIWorkdir(t *testing.T, fn func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func writeTestQRCodeOpenAPI(t *testing.T, token string) {
	t.Helper()
	apiDir := filepath.Join("openapis", "qrcode")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatal(err)
	}
	config := `{
  "id": "qrcode",
  "name": "二维码图片接口",
  "path": "qrcode",
  "method": "GET",
  "enabled": true,
  "token": "` + token + `",
  "runtime": "builtin",
  "entry": "",
  "builtin": "qrcode",
  "description": "根据 text 或 content 参数生成二维码 PNG。"
}`
	if err := os.WriteFile(filepath.Join(apiDir, "config.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
}

func newPaymentTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	adapterManager := config.NewAdapterManager(db)
	server := NewServer("0", nil, nil, adapterManager, nil)
	return server, func() { _ = db.Close() }
}

func configureWebEpay(t *testing.T, server *Server) {
	t.Helper()
	settings := config.DefaultPaymentSettings()
	settings.ThirdPartyEnabled = true
	settings.Methods = append(settings.Methods, config.PaymentMethodSetting{Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true})
	settings.Epay = config.EpaySettings{Enabled: true, Version: "v1", APIURL: "https://pay.example.com/", PID: "1000", Key: "secret", ReturnURL: "https://app.example.com/api/open/payments/return/epay"}
	if err := server.adapterManager.GetDatabase().SavePaymentSettings(&settings); err != nil {
		t.Fatal(err)
	}
}

func webEpayNotifyQuery(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	values.Set("sign", webEpayMD5Sign(params, "secret"))
	values.Set("sign_type", "MD5")
	return values.Encode()
}

func webEpayMD5Sign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for name, value := range params {
		if name != "sign" && name != "sign_type" && strings.TrimSpace(value) != "" {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, name := range keys {
		parts = append(parts, name+"="+params[name])
	}
	digest := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(digest[:])
}

func createWebPaymentTestOrder(t *testing.T, server *Server, orderNo string) *config.PaymentOrder {
	t.Helper()
	order, err := server.adapterManager.GetDatabase().CreatePaymentOrder(&config.PaymentOrder{
		OrderNo:      orderNo,
		PluginID:     "plugin-web",
		UnionID:      "union-web",
		Platform:     "test",
		UserID:       "user-web",
		Subject:      "Web 测试支付",
		AmountCents:  100,
		PointsAmount: 100,
		Provider:     "epay",
		Method:       "alipay",
		Status:       "pending",
		Metadata:     "{}",
		ExpiredAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder returned error: %v", err)
	}
	return order
}

func performPaymentJSONRequest(t *testing.T, handler func(http.ResponseWriter, *http.Request), method, path string, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler(recorder, request)
	return recorder
}

func decodePaymentResponse(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, recorder.Body.String())
	}
}
