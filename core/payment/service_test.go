package payment

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

func TestParseRMBToCents(t *testing.T) {
	cases := map[string]int64{
		`"1"`:    100,
		`"1.2"`:  120,
		`"1.23"`: 123,
		`1`:      100,
		`0.01`:   1,
	}
	for raw, expected := range cases {
		actual, err := ParseRMBToCents(json.RawMessage(raw))
		if err != nil {
			t.Fatalf("ParseRMBToCents(%s) returned error: %v", raw, err)
		}
		if actual != expected {
			t.Fatalf("ParseRMBToCents(%s) = %d, expected %d", raw, actual, expected)
		}
	}
}

func TestParseRMBToCentsRejectsInvalid(t *testing.T) {
	for _, raw := range []string{``, `null`, `0`, `"0"`, `-1`, `"1.234"`, `"abc"`, `1e2`, `"1e2"`, `"9223372036854775808"`} {
		if _, err := ParseRMBToCents(json.RawMessage(raw)); err == nil {
			t.Fatalf("expected %s to fail", raw)
		}
	}
}

func TestPaymentQRCodeURLSupportsContentTemplate(t *testing.T) {
	content := "https://pay.example.com/submit.php?money=0.10&name=星韵"
	cases := map[string]string{
		"https://api.example.com/qrcode?apiKey=secret&data={content}": "https://api.example.com/qrcode?apiKey=secret&data=https%3A%2F%2Fpay.example.com%2Fsubmit.php%3Fmoney%3D0.10%26name%3D%E6%98%9F%E9%9F%B5",
		"https://api.example.com/qrcode?apiKey=secret&qr=":            "https://api.example.com/qrcode?apiKey=secret&qr=https%3A%2F%2Fpay.example.com%2Fsubmit.php%3Fmoney%3D0.10%26name%3D%E6%98%9F%E9%9F%B5",
	}
	for baseURL, expected := range cases {
		actual := PaymentQRCodeURL(baseURL, "PTEST", content)
		if actual != expected {
			t.Fatalf("unexpected qrcode url:\nactual:   %s\nexpected: %s", actual, expected)
		}
	}
}

func TestEnabledPointMethodsOnlyReturnsEnabledPoints(t *testing.T) {
	settings := config.DefaultPaymentSettings()
	settings.ThirdPartyEnabled = true
	settings.Epay = config.EpaySettings{Enabled: true, Version: "v1", APIURL: "https://pay.example.com/", PID: "1000", Key: "secret"}
	settings.Methods = append(settings.Methods,
		config.PaymentMethodSetting{Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true},
		config.PaymentMethodSetting{Code: "points-disabled", Label: "禁用积分", Provider: "points", Enabled: false},
	)
	methods := EnabledPointMethods(&settings)
	if len(methods) != 1 || methods[0].Code != "points" {
		t.Fatalf("unexpected point methods: %#v", methods)
	}
	enabled := EnabledMethods(&settings, []string{"alipay"})
	if len(enabled) != 1 || enabled[0].Code != "alipay" {
		t.Fatalf("unexpected enabled methods: %#v", enabled)
	}
	settings.ThirdPartyEnabled = false
	enabled = EnabledMethods(&settings, []string{"alipay"})
	if len(enabled) != 0 {
		t.Fatalf("third party disabled should hide epay methods: %#v", enabled)
	}
}

func TestWaitPayPointsSuccess(t *testing.T) {
	db := newServiceTestDatabase(t)
	if _, err := db.AddUserPoints("union-pay", 200); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	service := NewService(db)
	result, err := service.WaitPay(WaitPayRequest{PluginID: "plugin-pay", UnionID: "union-pay", Platform: "test", UserID: "user", GroupID: "group", Subject: "测试消费", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30, PointsUnit: "积分", Metadata: map[string]interface{}{"from": "test"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, Listen: func(timeout int) string {
		if timeout != 30 {
			t.Fatalf("unexpected timeout %d", timeout)
		}
		return "1"
	}})
	if err != nil {
		t.Fatalf("WaitPay returned error: %v", err)
	}
	if result.Status != "paid" || result.PointsBalance != 100 || result.PointsAmount != 100 || result.OrderNo == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 1 || !strings.Contains(replies[0], "当前消费 1.00 RMB（100 积分）") || !strings.Contains(replies[0], "1. 积分支付（剩余积分：200）") {
		t.Fatalf("unexpected prompt: %#v", replies)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "paid" || order.Provider != "points" || order.Method != "points" {
		t.Fatalf("unexpected order: %#v", order)
	}
	events, err := db.ListPaymentEvents(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != "created" || events[1].EventType != "paid" {
		t.Fatalf("unexpected events: %#v", events)
	}
	transactions, total, err := db.ListPointTransactions(config.PointTransactionQuery{UnionID: "union-pay", Source: "payment", SourceID: result.OrderNo})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(transactions) != 1 || transactions[0].Delta != -100 || transactions[0].BalanceAfter != 100 {
		t.Fatalf("unexpected transactions total=%d items=%#v", total, transactions)
	}
}

func TestWaitPaySendsMethodButtons(t *testing.T) {
	db := newServiceTestDatabase(t)
	if _, err := db.AddUserPoints("union-buttons", 200); err != nil {
		t.Fatal(err)
	}
	prompts := []string{}
	var buttonRows [][]types.ButtonOption
	result, err := NewService(db).WaitPay(WaitPayRequest{PluginID: "plugin-pay", UnionID: "union-buttons", Platform: "test", UserID: "user-buttons", Subject: "按钮支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30, PointsUnit: "积分", Methods: []string{"points"}}, Interaction{Reply: func(text string) error {
		t.Fatalf("Reply should not be called when ReplyButtons is available: %s", text)
		return nil
	}, ReplyButtons: func(text string, buttons [][]types.ButtonOption) error {
		prompts = append(prompts, text)
		buttonRows = buttons
		return nil
	}, Listen: func(timeout int) string { return "1" }})
	if err != nil {
		t.Fatalf("WaitPay returned error: %v", err)
	}
	if result.Status != "paid" || result.PointsBalance != 100 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "请选择支付方式") {
		t.Fatalf("unexpected prompts: %#v", prompts)
	}
	if len(buttonRows) != 2 {
		t.Fatalf("unexpected buttons: %#v", buttonRows)
	}
	if buttonRows[0][0].Value != "1" || buttonRows[0][0].UserID != "user-buttons" || !strings.Contains(buttonRows[0][0].Text, "积分支付") {
		t.Fatalf("unexpected payment button: %#v", buttonRows[0][0])
	}
	if buttonRows[1][0] != (types.ButtonOption{Text: "取消", Value: "q", UserID: "user-buttons"}) {
		t.Fatalf("unexpected cancel button: %#v", buttonRows[1][0])
	}
}

func TestWaitPayCancelBeforeSelectingMethod(t *testing.T) {
	db := newServiceTestDatabase(t)
	replies := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-cancel-before", Subject: "取消支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, Listen: func(timeout int) string { return "q" }})
	if err != nil {
		t.Fatalf("cancel should not return error: %v", err)
	}
	if result.Status != "cancelled" || result.Message != "支付已取消" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 1 || !strings.Contains(replies[0], "发送 q 可取消") {
		t.Fatalf("unexpected replies: %#v", replies)
	}
	_, total, err := db.ListPaymentOrders(config.PaymentOrderQuery{UnionID: "union-cancel-before"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("cancel before selecting method should not create order, got %d", total)
	}
}

func TestWaitPayInsufficientPoints(t *testing.T) {
	db := newServiceTestDatabase(t)
	if _, err := db.AddUserPoints("union-low", 50); err != nil {
		t.Fatal(err)
	}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-low", Subject: "余额不足", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30}, Interaction{Listen: func(timeout int) string { return "1" }})
	if err == nil {
		t.Fatal("expected insufficient points to fail")
	}
	if result.Status != "failed" || result.PointsBalance != 50 || result.OrderNo == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "failed" {
		t.Fatalf("expected failed order, got %#v", order)
	}
	transactions, total, err := db.ListPointTransactions(config.PointTransactionQuery{UnionID: "union-low", Source: "payment"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(transactions) != 0 {
		t.Fatalf("insufficient points should not write negative transaction: %#v", transactions)
	}
}

func TestWaitPayAlipayBillOffsetsDuplicateAmount(t *testing.T) {
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Methods = []config.PaymentMethodSetting{{Code: "alipay_transfer", Label: "支付宝转账", Provider: "alipay_bill", Enabled: true}}
	settings.AlipayBill = testAlipayBillSettings("https://openapi.alipay.com/gateway.do")
	settings.AlipayBill.ReceiptQRURL = "https://qr.alipay.com/fkx11224piym1km7a20uka5"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProviderPaymentOrder(config.ProviderPaymentOrderInput{UnionID: "union-existing-alipay", Subject: "已有支付", AmountCents: 100, PointsAmount: 100, Provider: "alipay_bill", Method: "alipay_transfer", ExpiredAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-new-alipay", Subject: "支付宝金额偏移", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 1, Methods: []string{"alipay_transfer"}}, Interaction{Reply: func(text string) error {
		return nil
	}, SendImage: func(imageURL string) error {
		return nil
	}, Listen: func(timeout int) string { return "alipay_transfer" }})
	if err != nil {
		t.Fatalf("WaitPay alipay bill should expire without error: %v", err)
	}
	if result.AmountCents != 101 || result.PointsAmount != 100 || result.Status != "expired" {
		t.Fatalf("unexpected result: %#v", result)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.AmountCents != 101 || order.PointsAmount != 100 || !strings.Contains(order.Metadata, `"match_mode":"amount_unique"`) || !strings.Contains(order.Metadata, `"amount_offset_cents":1`) {
		t.Fatalf("unexpected offset order: %#v", order)
	}
}

func TestWaitPayEpayCreatesOrderAndExpires(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mapi.php" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Get("out_trade_no") == "" || r.PostForm.Get("type") != "alipay" || r.PostForm.Get("notify_url") == "" || r.PostForm.Get("name") != "后台伪造标题" {
			t.Fatalf("unexpected form: %#v", r.PostForm)
		}
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"TEPAY","payurl":"https://pay.example.com/pay","qrcode":"QR"}`))
	}))
	defer providerServer.Close()
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Epay.APIURL = providerServer.URL + "/"
	settings.Epay.ReturnURL = "https://app.example.com/api/open/payments/return/epay"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	settings.EpaySubmitSubject = "后台伪造标题"
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	images := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-epay", Subject: "第三方支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 1, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, SendImage: func(imageURL string) error {
		images = append(images, imageURL)
		return nil
	}, Listen: func(timeout int) string { return "alipay" }})
	if err != nil {
		t.Fatalf("WaitPay epay should expire without error: %v", err)
	}
	if result.Status != "expired" || result.OrderNo == "" || result.ProviderOrderNo != "TEPAY" || result.PayURL == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 2 || !strings.Contains(replies[0], "支付宝") || !strings.Contains(replies[1], "支付链接") || strings.Contains(replies[1], "二维码内容") || strings.Contains(replies[1], "QR") {
		t.Fatalf("unexpected replies: %#v", replies)
	}
	expectedImage := PaymentQRCodeURL(settings.QRCodeBaseURL, result.OrderNo, "QR")
	if len(images) != 1 || images[0] != expectedImage || !strings.HasSuffix(images[0], ".png") {
		t.Fatalf("unexpected qrcode images: %#v", images)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "expired" || order.ProviderOrderNo != "TEPAY" || order.PayURL == "" || order.Subject != "第三方支付" {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestWaitPayEpaySendsRichMessageWithQRCode(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"TEPAY_RICH","payurl":"https://pay.example.com/pay-rich","qrcode":"QR-RICH"}`))
	}))
	defer providerServer.Close()
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Epay.APIURL = providerServer.URL + "/"
	settings.Epay.ReturnURL = "https://app.example.com/api/open/payments/return/epay"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	images := []string{}
	richMessages := []types.RichMessage{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-epay-rich", Subject: "富消息支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 1, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, SendImage: func(imageURL string) error {
		images = append(images, imageURL)
		return nil
	}, SendRich: func(message types.RichMessage) error {
		richMessages = append(richMessages, message)
		return nil
	}, Listen: func(timeout int) string { return "alipay" }})
	if err != nil {
		t.Fatalf("WaitPay epay should expire without error: %v", err)
	}
	if result.Status != "expired" || result.OrderNo == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 1 || !strings.Contains(replies[0], "支付宝") {
		t.Fatalf("payment method prompt should remain separate: %#v", replies)
	}
	if len(images) != 0 {
		t.Fatalf("rich message should avoid separate image send: %#v", images)
	}
	if len(richMessages) != 1 || len(richMessages[0].Parts) != 2 {
		t.Fatalf("unexpected rich messages: %#v", richMessages)
	}
	if richMessages[0].Parts[0].Type != "text" || !strings.Contains(richMessages[0].Parts[0].Text, "支付链接") {
		t.Fatalf("unexpected rich text part: %#v", richMessages[0].Parts[0])
	}
	expectedImage := PaymentQRCodeURL(settings.QRCodeBaseURL, result.OrderNo, "QR-RICH")
	if richMessages[0].Parts[1].Type != "image" || richMessages[0].Parts[1].URL != expectedImage {
		t.Fatalf("unexpected rich image part: %#v", richMessages[0].Parts[1])
	}
}

func TestWaitPayEpayHidesPayURLAndSendsQRCodeImage(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"TEPAY_HIDDEN","payurl":"https://pay.example.com/pay-hidden","qrcode":"QR-HIDDEN"}`))
	}))
	defer providerServer.Close()
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.HidePayURL = true
	settings.Epay.APIURL = providerServer.URL + "/"
	settings.Epay.ReturnURL = "https://app.example.com/api/open/payments/return/epay"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	images := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-epay-hidden", Subject: "隐藏链接支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 1, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, SendImage: func(imageURL string) error {
		images = append(images, imageURL)
		return nil
	}, Listen: func(timeout int) string { return "alipay" }})
	if err != nil {
		t.Fatalf("WaitPay epay should expire without error: %v", err)
	}
	if result.Status != "expired" || result.OrderNo == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 2 || strings.Contains(replies[1], "支付链接") || strings.Contains(replies[1], "二维码内容") || strings.Contains(replies[1], "QR-HIDDEN") || strings.Contains(replies[1], "https://pay.example.com/pay-hidden") {
		t.Fatalf("hidden pay url should not expose link or qrcode text: %#v", replies)
	}
	expectedImage := PaymentQRCodeURL(settings.QRCodeBaseURL, result.OrderNo, "QR-HIDDEN")
	if len(images) != 1 || images[0] != expectedImage {
		t.Fatalf("expected qrcode image: %#v", images)
	}
}

func TestWaitPayEpayHiddenPayURLFailsWhenImageSendFails(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"TEPAY_IMAGE_FAIL","payurl":"https://pay.example.com/pay-hidden","qrcode":"QR-HIDDEN"}`))
	}))
	defer providerServer.Close()
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.HidePayURL = true
	settings.Epay.APIURL = providerServer.URL + "/"
	settings.Epay.ReturnURL = "https://app.example.com/api/open/payments/return/epay"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-epay-image-fail", Subject: "图片失败支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, SendImage: func(imageURL string) error {
		return errors.New("send image failed")
	}, Listen: func(timeout int) string { return "alipay" }})
	if err == nil {
		t.Fatal("expected image send failure")
	}
	if result.Status != "failed" || !strings.Contains(result.Message, "二维码图片发送失败") {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 3 || strings.Contains(replies[1], "支付链接") || strings.Contains(replies[1], "QR-HIDDEN") || !strings.Contains(replies[2], "二维码图片发送失败") {
		t.Fatalf("unexpected replies: %#v", replies)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "failed" {
		t.Fatalf("expected failed order, got %#v", order)
	}
}

func TestWaitPayEpayAutoQueriesPaidOrder(t *testing.T) {
	queryCount := 0
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mapi.php":
			_, _ = w.Write([]byte(`{"code":1,"trade_no":"TEPAY_AUTO","payurl":"https://pay.example.com/pay-auto","qrcode":"QR-AUTO"}`))
		case "/api.php":
			queryCount++
			_, _ = w.Write([]byte(`{"status":1,"trade_no":"TEPAY_AUTO","type":"alipay","money":"1.00"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer providerServer.Close()
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Epay.APIURL = providerServer.URL + "/"
	settings.Epay.ReturnURL = "https://app.example.com/api/open/payments/return/epay"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	settings.EpayQueryIntervalSeconds = 1
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-epay-auto", Subject: "自动查询支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 5, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		return nil
	}, SendImage: func(imageURL string) error {
		return nil
	}, Listen: func(timeout int) string {
		return "alipay"
	}})
	if err != nil {
		t.Fatalf("auto query should not return error: %v", err)
	}
	if result.Status != "paid" || result.OrderNo == "" || result.ProviderOrderNo != "TEPAY_AUTO" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if queryCount == 0 {
		t.Fatal("expected auto query to call provider")
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "paid" {
		t.Fatalf("expected paid order, got %#v", order)
	}
}

func TestWaitPayEpayCancelsPendingOrder(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"TEPAY_CANCEL","payurl":"https://pay.example.com/pay-cancel","qrcode":"QR-CANCEL"}`))
	}))
	defer providerServer.Close()
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Epay.APIURL = providerServer.URL + "/"
	settings.Epay.ReturnURL = "https://app.example.com/api/open/payments/return/epay"
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-epay-cancel", Subject: "取消第三方支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, SendImage: func(imageURL string) error {
		return nil
	}, Listen: func(timeout int) string {
		return "alipay"
	}, ListenUntil: func(timeout int, done <-chan struct{}) string {
		return "q"
	}})
	if err != nil {
		t.Fatalf("cancel should not return error: %v", err)
	}
	if result.Status != "cancelled" || result.OrderNo == "" || result.Message != "支付已取消" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 2 || strings.Contains(strings.Join(replies, "\n"), "支付已取消") {
		t.Fatalf("unexpected replies: %#v", replies)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "cancelled" {
		t.Fatalf("expected cancelled order, got %#v", order)
	}
}

func TestWaitPayRejectsWhenUserHasPendingOrder(t *testing.T) {
	db := newServiceTestDatabase(t)
	if _, err := db.CreateProviderPaymentOrder(config.ProviderPaymentOrderInput{UnionID: "union-repeat", Subject: "已有待支付", AmountCents: 100, PointsAmount: 100, Provider: "epay", Method: "alipay", ExpiredAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-repeat", Subject: "重复支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, Listen: func(timeout int) string { return "alipay" }})
	if err == nil {
		t.Fatal("expected repeat pending order to fail")
	}
	if result.Status != "failed" || !strings.Contains(result.Message, "已有待支付订单") {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 0 {
		t.Fatalf("should reject before prompting method: %#v", replies)
	}
}

func TestWaitPayEpayRejectsWhenPendingLimitReached(t *testing.T) {
	db := newServiceTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.MaxPendingPayments = 1
	if err := db.SavePaymentSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProviderPaymentOrder(config.ProviderPaymentOrderInput{UnionID: "union-existing", Subject: "已有待支付", AmountCents: 100, PointsAmount: 100, Provider: "epay", Method: "alipay", ExpiredAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	replies := []string{}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-new", Subject: "新支付", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 30, Methods: []string{"alipay"}}, Interaction{Reply: func(text string) error {
		replies = append(replies, text)
		return nil
	}, Listen: func(timeout int) string { return "alipay" }})
	if err == nil {
		t.Fatal("expected pending limit to reject payment")
	}
	if result.Status != "failed" || !strings.Contains(result.Message, "待支付") {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(replies) != 1 || !strings.Contains(replies[0], "支付宝") {
		t.Fatalf("expected only method prompt before rejection: %#v", replies)
	}
	orders, total, err := db.ListPaymentOrders(config.PaymentOrderQuery{UnionID: "union-new"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(orders) != 0 {
		t.Fatalf("rejected payment should not create order: %#v", orders)
	}
}

func TestWaitHubResolve(t *testing.T) {
	hub := NewWaitHub()
	ch, cancel := hub.Register("PWAIT")
	defer cancel()
	if !hub.Resolve("PWAIT", PaymentResult{Status: "paid", OrderNo: "PWAIT"}) {
		t.Fatal("expected resolve to wake waiter")
	}
	select {
	case result := <-ch:
		if result.Status != "paid" || result.OrderNo != "PWAIT" {
			t.Fatalf("unexpected result: %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("wait hub did not resolve")
	}
	if hub.Resolve("PWAIT", PaymentResult{}) {
		t.Fatal("resolved waiter should be removed")
	}
}

func TestWaitPayTimeoutDoesNotCreateOrder(t *testing.T) {
	db := newServiceTestDatabase(t)
	if _, err := db.AddUserPoints("union-timeout", 200); err != nil {
		t.Fatal(err)
	}
	result, err := NewService(db).WaitPay(WaitPayRequest{UnionID: "union-timeout", Subject: "超时", AmountRaw: json.RawMessage(`"1.00"`), Timeout: 1}, Interaction{Listen: func(timeout int) string { return "" }})
	if err != nil {
		t.Fatalf("timeout should not return error: %v", err)
	}
	if result.Status != "expired" || result.OrderNo != "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	orders, total, err := db.ListPaymentOrders(config.PaymentOrderQuery{UnionID: "union-timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(orders) != 0 {
		t.Fatalf("timeout should not create order: %#v", orders)
	}
}

func TestWaitPayMissingUnionIDDoesNotCreateOrder(t *testing.T) {
	db := newServiceTestDatabase(t)
	if _, err := NewService(db).WaitPay(WaitPayRequest{Subject: "无用户", AmountRaw: json.RawMessage(`"1.00"`)}, Interaction{}); err == nil {
		t.Fatal("expected missing union_id to fail")
	}
	_, total, err := db.ListPaymentOrders(config.PaymentOrderQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("missing union_id should not create order, got %d", total)
	}
}

func newServiceTestDatabase(t *testing.T) *config.Database {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	settings := config.DefaultPaymentSettings()
	settings.PointsPerRMB = 100
	settings.ThirdPartyEnabled = true
	settings.Methods = append(settings.Methods, config.PaymentMethodSetting{Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true})
	settings.Epay = config.EpaySettings{Enabled: true, Version: "v1", APIURL: "https://pay.example.com/", PID: "1000", Key: "secret", ReturnURL: "https://app.example.com/api/open/payments/return/epay"}
	if err := db.SavePaymentSettings(&settings); err != nil {
		t.Fatalf("SavePaymentSettings returned error: %v", err)
	}
	return db
}
