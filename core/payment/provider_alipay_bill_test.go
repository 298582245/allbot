package payment

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestAlipayBillCreateOrderBuildsTransferURL(t *testing.T) {
	provider, err := NewAlipayBillProvider(testAlipayBillSettings("https://openapi.alipay.com/gateway.do"), nil)
	if err != nil {
		t.Fatalf("NewAlipayBillProvider returned error: %v", err)
	}
	order, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: "PTEST_ALIPAY", AmountCents: 123, Method: "alipay_transfer"})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if order.PayURL == "" || order.QRCode != order.PayURL || !strings.HasPrefix(order.PayURL, "alipays://platformapi/startapp") {
		t.Fatalf("unexpected transfer url: %#v", order)
	}
	if !strings.Contains(order.PayURL, "actionType=toAccount") || !strings.Contains(order.PayURL, "amount=1.23") || !strings.Contains(order.PayURL, "memo=PTEST_ALIPAY") || !strings.Contains(order.Raw, "amount_unique") {
		t.Fatalf("transfer url or raw missing expected fields: %#v", order)
	}
}

func TestAlipayBillCreateOrderPrefersReceiptQRURL(t *testing.T) {
	settings := testAlipayBillSettings("https://openapi.alipay.com/gateway.do")
	settings.ReceiptQRURL = "https://qr.alipay.com/fkx11224piym1km7a20uka5"
	provider, err := NewAlipayBillProvider(settings, nil)
	if err != nil {
		t.Fatalf("NewAlipayBillProvider returned error: %v", err)
	}
	order, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: "PTEST_QR", AmountCents: 500, Method: "alipay_transfer"})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if !strings.HasPrefix(order.PayURL, "https://qr.alipay.com/fkx11224piym1km7a20uka5?") || !strings.Contains(order.PayURL, "_s=web-other") || !strings.Contains(order.PayURL, "a=5.00") || !strings.Contains(order.PayURL, "m=%E8%AE%A2%E5%8D%95%3APTEST_QR") || !strings.Contains(order.Raw, "receipt_qr") {
		t.Fatalf("unexpected receipt qr url: %#v", order)
	}
}

func TestAlipayBillReceiptQRURLUnwrapsRenderURL(t *testing.T) {
	wrapped := "https://render.alipay.com/p/s/i?scheme=alipays://platformapi/startapp?saId=10000007&qrcode=%68%74%74%70%73%3A%2F%2F%71%72%2E%61%6C%69%70%61%79%2E%63%6F%6D%2F%66%6B%78%31%39%38%34%33%62%69%65%70%67%74%33%71%70%6E%39%76%6C%66%37%3F%5F%73%3D%77%65%62%2D%6F%74%68%65%72"
	actual := buildAlipayReceiptQRURL(wrapped, 123, "订单:PTEST_WRAP")
	if !strings.HasPrefix(actual, "https://qr.alipay.com/fkx19843biepgt3qpn9vlf7?") || !strings.Contains(actual, "_s=web-other") || !strings.Contains(actual, "a=1.23") || !strings.Contains(actual, "m=%E8%AE%A2%E5%8D%95%3APTEST_WRAP") {
		t.Fatalf("unexpected unwrapped receipt qr url: %s", actual)
	}
}

func TestAlipayBillReceiptQRURLUnwrapsDSURL(t *testing.T) {
	wrapped := "https://ds.alipay.com/?from=mobilecodec&scheme=alipays://platformapi/startapp?saId=10000007&clientVersion=3.7.0.0718&qrcode=https%3A%2F%2Fqr.alipay.com%2F2m611971st2xtigydlmpu15%3F_s%3Dweb-other"
	actual := buildAlipayReceiptQRURL(wrapped, 123, "订单:PTEST_DS")
	if !strings.HasPrefix(actual, "https://qr.alipay.com/2m611971st2xtigydlmpu15?") || !strings.Contains(actual, "_s=web-other") || !strings.Contains(actual, "a=1.23") || !strings.Contains(actual, "m=%E8%AE%A2%E5%8D%95%3APTEST_DS") {
		t.Fatalf("unexpected unwrapped ds receipt qr url: %s", actual)
	}
}

func TestAlipayBillCreateOrderUsesCashierBaseURL(t *testing.T) {
	settings := testAlipayBillSettings("https://openapi.alipay.com/gateway.do")
	settings.CashierBaseURL = "https://pay.example.com/"
	provider, err := NewAlipayBillProvider(settings, nil)
	if err != nil {
		t.Fatalf("NewAlipayBillProvider returned error: %v", err)
	}
	order, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: "PTEST_CASHIER", AmountCents: 500, Method: "alipay_transfer"})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if !strings.HasPrefix(order.PayURL, "alipays://platformapi/startapp?") || !strings.Contains(order.PayURL, "appId=20000067") || !strings.Contains(order.PayURL, "url=https%3A%2F%2Fpay.example.com%2Fapi%2Fopen%2Fpayments%2Falipay-bill%2Fcashier%2FPTEST_CASHIER") || !strings.Contains(order.Raw, "cashier") {
		t.Fatalf("unexpected cashier url: %#v", order)
	}
}

func TestAlipayBillSignContentSortsAndSkipsSign(t *testing.T) {
	content := alipaySignContent(map[string]string{"b": "2", "sign": "bad", "a": "1", "empty": ""})
	if content != "a=1&b=2" {
		t.Fatalf("unexpected sign content: %s", content)
	}
}

func TestAlipayBillQueryOrderMatchesAmount(t *testing.T) {
	var requestQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"alipay_data_bill_accountlog_query_response":{"code":"10000","detail_list":[{"trade_no":"ABILL1","trans_amount":"1.00","balance_type":"收入","memo":"OTHER","summary":"支付宝转账","trans_dt":"2026-07-06 12:00:00"}]}}`))
	}))
	defer server.Close()
	provider, err := NewAlipayBillProvider(testAlipayBillSettings(server.URL), server.Client())
	if err != nil {
		t.Fatalf("NewAlipayBillProvider returned error: %v", err)
	}
	result, err := provider.QueryOrderWithAmount("PTEST_MATCH", "", 100)
	if err != nil {
		t.Fatalf("QueryOrderWithAmount returned error: %v", err)
	}
	if result.Status != "paid" || result.ProviderOrderNo != "ABILL1" || result.AmountCents != 100 || result.Method != "alipay_transfer" {
		t.Fatalf("unexpected query result: %#v", result)
	}
	mismatch, err := provider.QueryOrderWithAmount("PTEST_MATCH", "", 200)
	if err != nil {
		t.Fatalf("QueryOrderWithAmount mismatch returned error: %v", err)
	}
	if mismatch.Status != "pending" {
		t.Fatalf("amount mismatch should stay pending: %#v", mismatch)
	}
	if !strings.Contains(requestQuery, "sign=") || !strings.Contains(requestQuery, "method=alipay.data.bill.accountlog.query") {
		t.Fatalf("request missing signed params: %s", requestQuery)
	}
}

func TestParseAlipayBillItems(t *testing.T) {
	items, err := parseAlipayBillItems([]byte(`{"alipay_data_bill_accountlog_query_response":{"code":"10000","detail_list":{"account_log_item":[{"trade_no":"ABILL2","trans_amount":"2.50","balance_type":"收入","memo":"PTEST_PARSE","summary":"到账","trans_dt":"2026-07-06 12:01:02"}]}}}`))
	if err != nil {
		t.Fatalf("parseAlipayBillItems returned error: %v", err)
	}
	if len(items) != 1 || items[0].ProviderOrderNo != "ABILL2" || items[0].AmountCents != 250 || items[0].Remark != "PTEST_PARSE" || items[0].PaidAt.IsZero() {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func testAlipayBillSettings(gatewayURL string) config.AlipayBillSettings {
	privateKey, publicKey := testRSAKeys()
	return config.AlipayBillSettings{Enabled: true, GatewayURL: gatewayURL, AppID: "app-1", PrivateKey: privateKey, AlipayPublicKey: publicKey, TransferUserID: "2088000000000000", TransferUserName: "测试", QueryMinutesBack: 30, CheckIntervalSeconds: 15, OrderTimeoutSeconds: 300, BillPageSize: 100, MatchMode: "amount_unique"}
}

func testRSAKeys() (string, string) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return string(privatePEM), string(publicPEM)
}
