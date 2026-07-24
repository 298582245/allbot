package payment

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEpayV1CreateNotifyAndQuery(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/mapi.php":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			params := valuesToStringMap(r.PostForm)
			if params["out_trade_no"] != "PTEST_V1" || params["sign"] != epayMD5Sign(params, "secret") {
				t.Fatalf("unexpected create params: %#v", params)
			}
			return jsonHTTPResponse(`{"code":1,"trade_no":"T100","payurl":"https://pay.example.com/pay/T100","qrcode":"QR100"}`), nil
		case "/api.php":
			query := r.URL.Query()
			if query.Get("act") != "order" || query.Get("pid") != "1000" || query.Get("key") != "secret" || query.Get("trade_no") != "T100" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			return jsonHTTPResponse(`{"status":1,"out_trade_no":"PTEST_V1","trade_no":"T100","type":"alipay","money":"1.23"}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return nil, nil
	})}
	provider, err := NewEpayProvider(config.EpaySettings{Version: "v1", APIURL: "https://pay.example.com/", PID: "1000", Key: "secret"}, client)
	if err != nil {
		t.Fatal(err)
	}
	created, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: "PTEST_V1", Subject: "测试", AmountCents: 123, Method: "alipay", NotifyURL: "https://app.test/api/open/payments/notify/epay", ReturnURL: "https://app.test/api/open/payments/return/epay"})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if created.ProviderOrderNo != "T100" || created.PayURL == "" || created.QRCode != "QR100" {
		t.Fatalf("unexpected created order: %#v", created)
	}

	callback := map[string]string{"pid": "1000", "type": "alipay", "out_trade_no": "PTEST_V1", "trade_no": "T100", "money": "1.23", "trade_status": "TRADE_SUCCESS"}
	notifyReq := httptest.NewRequest(http.MethodGet, "/notify?"+epayMD5SignedParams(callback, "secret").Encode(), nil)
	notify, err := provider.VerifyNotify(notifyReq)
	if err != nil {
		t.Fatalf("VerifyNotify returned error: %v", err)
	}
	if notify.Status != "paid" || notify.AmountCents != 123 || notify.OrderNo != "PTEST_V1" {
		t.Fatalf("unexpected notify result: %#v", notify)
	}
	badValues := epayMD5SignedParams(callback, "secret")
	badValues.Set("sign", "bad")
	if _, err := provider.VerifyNotify(httptest.NewRequest(http.MethodGet, "/notify?"+badValues.Encode(), nil)); err == nil {
		t.Fatal("expected invalid sign to fail")
	}
	badPID := map[string]string{"pid": "2000", "type": "alipay", "out_trade_no": "PTEST_V1", "trade_no": "T100", "money": "1.23", "trade_status": "TRADE_SUCCESS"}
	if _, err := provider.VerifyNotify(httptest.NewRequest(http.MethodGet, "/notify?"+epayMD5SignedParams(badPID, "secret").Encode(), nil)); err == nil {
		t.Fatal("expected pid mismatch to fail")
	}

	query, err := provider.QueryOrder("PTEST_V1", "T100")
	if err != nil {
		t.Fatalf("QueryOrder returned error: %v", err)
	}
	if query.Status != "paid" || query.AmountCents != 123 || query.ProviderOrderNo != "T100" {
		t.Fatalf("unexpected query result: %#v", query)
	}
}

func TestEpayV2CreateNotifyAndQuery(t *testing.T) {
	merchantKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	platformKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	merchantPrivate := privateKeyPEM(merchantKey)
	merchantPublic := publicKeyPEM(&merchantKey.PublicKey)
	platformPrivate := privateKeyPEM(platformKey)
	platformPublic := publicKeyPEM(&platformKey.PublicKey)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		requestFields := valuesToStringMap(r.PostForm)
		verifier := &EpayProvider{settings: config.EpaySettings{PlatformPublicKey: merchantPublic}}
		if err := verifier.verifyRSAFields(requestFields); err != nil {
			t.Fatalf("request sign invalid: %v fields=%#v", err, requestFields)
		}
		switch r.URL.Path {
		case "/api/pay/create":
			return signedJSONResponse(t, platformPrivate, map[string]string{"code": "0", "msg": "ok", "timestamp": nowText(), "out_trade_no": "PTEST_V2", "trade_no": "T200", "payurl": "https://pay.example.com/pay/T200", "qrcode": "QR200", "money": "2.34", "status": "1"}), nil
		case "/api/pay/query":
			return signedJSONResponse(t, platformPrivate, map[string]string{"code": "0", "msg": "ok", "timestamp": nowText(), "out_trade_no": "PTEST_V2", "trade_no": "T200", "type": "wxpay", "money": "2.34", "status": "1"}), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return nil, nil
	})}
	provider, err := NewEpayProvider(config.EpaySettings{Version: "v2", APIURL: "https://pay.example.com/", PID: "1000", PlatformPublicKey: platformPublic, MerchantPrivateKey: merchantPrivate}, client)
	if err != nil {
		t.Fatal(err)
	}
	created, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: "PTEST_V2", Subject: "测试 V2", AmountCents: 234, Method: "wxpay", NotifyURL: "https://app.test/api/open/payments/notify/epay", ReturnURL: "https://app.test/api/open/payments/return/epay"})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if created.ProviderOrderNo != "T200" || created.PayURL == "" || created.QRCode != "QR200" {
		t.Fatalf("unexpected created order: %#v", created)
	}
	callback := signedValues(t, platformPrivate, map[string]string{"pid": "1000", "type": "wxpay", "out_trade_no": "PTEST_V2", "trade_no": "T200", "money": "2.34", "trade_status": "TRADE_SUCCESS", "timestamp": nowText()})
	notify, err := provider.VerifyNotify(httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(callback.Encode())))
	if err != nil {
		t.Fatalf("VerifyNotify returned error: %v", err)
	}
	if notify.Status != "paid" || notify.AmountCents != 234 || notify.Method != "wxpay" {
		t.Fatalf("unexpected notify result: %#v", notify)
	}
	expired := signedValues(t, platformPrivate, map[string]string{"pid": "1000", "type": "wxpay", "out_trade_no": "PTEST_V2", "trade_no": "T200", "money": "2.34", "trade_status": "TRADE_SUCCESS", "timestamp": "1"})
	if _, err := provider.VerifyNotify(httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(expired.Encode()))); err == nil {
		t.Fatal("expected expired timestamp to fail")
	}
	badPID := signedValues(t, platformPrivate, map[string]string{"pid": "2000", "type": "wxpay", "out_trade_no": "PTEST_V2", "trade_no": "T200", "money": "2.34", "trade_status": "TRADE_SUCCESS", "timestamp": nowText()})
	if _, err := provider.VerifyNotify(httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(badPID.Encode()))); err == nil {
		t.Fatal("expected pid mismatch to fail")
	}
	query, err := provider.QueryOrder("PTEST_V2", "T200")
	if err != nil {
		t.Fatalf("QueryOrder returned error: %v", err)
	}
	if query.Status != "paid" || query.AmountCents != 234 || query.ProviderOrderNo != "T200" {
		t.Fatalf("unexpected query result: %#v", query)
	}
}

func TestEpayV2NestedDataIsCoveredBySignature(t *testing.T) {
	platformKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := privateKeyPEM(platformKey)
	provider := &EpayProvider{settings: config.EpaySettings{PlatformPublicKey: publicKeyPEM(&platformKey.PublicKey)}}
	originalData := `{"status":"1","money":"2.34","out_trade_no":"PTEST_V2","trade_no":"T200","type":"wxpay"}`
	signed := signedValues(t, privateKey, map[string]string{"code": "0", "timestamp": nowText(), "data": originalData})
	fields := valuesToStringMap(signed)
	if err := provider.verifyRSAFields(fields); err != nil {
		t.Fatalf("valid nested data signature failed: %v", err)
	}
	mutations := []string{
		strings.Replace(originalData, `"status":"1"`, `"status":"0"`, 1),
		strings.Replace(originalData, `"money":"2.34"`, `"money":"9.99"`, 1),
		strings.Replace(originalData, `"out_trade_no":"PTEST_V2"`, `"out_trade_no":"POTHER"`, 1),
	}
	for _, mutatedData := range mutations {
		mutated := map[string]string{}
		for key, value := range fields {
			mutated[key] = value
		}
		mutated["data"] = mutatedData
		if err := provider.verifyRSAFields(mutated); err == nil {
			t.Fatalf("tampered nested data should fail signature: %s", mutatedData)
		}
	}
}

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func signedJSONResponse(t *testing.T, privateKey string, fields map[string]string) *http.Response {
	t.Helper()
	values := signedValues(t, privateKey, fields)
	bodyMap := map[string]string{}
	for key, items := range values {
		if len(items) > 0 {
			bodyMap[key] = items[0]
		}
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		t.Fatal(err)
	}
	return jsonHTTPResponse(string(body))
}

func signedValues(t *testing.T, privateKey string, fields map[string]string) url.Values {
	t.Helper()
	params := map[string]string{}
	for key, value := range fields {
		params[key] = value
	}
	signer := &EpayProvider{settings: config.EpaySettings{MerchantPrivateKey: privateKey}}
	sign, err := signer.rsaPrivateSign(epaySignContent(params, false))
	if err != nil {
		t.Fatal(err)
	}
	params["sign"] = sign
	params["sign_type"] = "RSA"
	return stringMapValues(params)
}

func nowText() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func privateKeyPEM(key *rsa.PrivateKey) string {
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func publicKeyPEM(key *rsa.PublicKey) string {
	der, _ := x509.MarshalPKIXPublicKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}
