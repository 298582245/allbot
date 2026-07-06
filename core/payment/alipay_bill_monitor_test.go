package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
)

func TestAlipayBillMonitorConfirmsMatchingPendingOrder(t *testing.T) {
	db, server := newAlipayBillMonitorTestDatabase(t, `{"alipay_data_bill_accountlog_query_response":{"code":"10000","detail_list":[{"trade_no":"ABILL_MONITOR","trans_amount":"1.00","balance_type":"收入","memo":"ORDER_PLACEHOLDER","summary":"转账","trans_dt":"TIME_PLACEHOLDER"}]}}`)
	order := createAlipayBillMonitorOrder(t, db, 100, time.Now().Add(time.Minute))
	server.response = strings.ReplaceAll(server.response, "ORDER_PLACEHOLDER", order.OrderNo)
	server.response = strings.ReplaceAll(server.response, "TIME_PLACEHOLDER", time.Now().Format("2006-01-02 15:04:05"))

	if err := NewAlipayBillMonitor(db).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	updated, err := db.GetPaymentOrder(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "paid" || updated.ProviderOrderNo != "ABILL_MONITOR" {
		t.Fatalf("unexpected order: %#v", updated)
	}
	record, err := db.FindAlipayBillRecordByProviderOrderNo("ABILL_MONITOR")
	if err != nil || record.MatchedAt == nil || record.OrderNo != order.OrderNo {
		t.Fatalf("unexpected bill record: %#v err=%v", record, err)
	}
}

func TestAlipayBillMonitorRejectsMismatchAndAmbiguousAmount(t *testing.T) {
	db, server := newAlipayBillMonitorTestDatabase(t, `{"alipay_data_bill_accountlog_query_response":{"code":"10000","detail_list":[{"trade_no":"ABILL_BAD_AMOUNT","trans_amount":"2.00","balance_type":"收入","memo":"ORDER_PLACEHOLDER","summary":"转账","trans_dt":"TIME_PLACEHOLDER"},{"trade_no":"ABILL_AMBIGUOUS","trans_amount":"1.00","balance_type":"收入","memo":"OTHER","summary":"转账","trans_dt":"TIME_PLACEHOLDER"}]}}`)
	first := createAlipayBillMonitorOrder(t, db, 100, time.Now().Add(time.Minute))
	second := createAlipayBillMonitorOrder(t, db, 100, time.Now().Add(time.Minute))
	server.response = strings.ReplaceAll(server.response, "ORDER_PLACEHOLDER", first.OrderNo)
	server.response = strings.ReplaceAll(server.response, "TIME_PLACEHOLDER", time.Now().Format("2006-01-02 15:04:05"))

	if err := NewAlipayBillMonitor(db).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	for _, order := range []*config.PaymentOrder{first, second} {
		updated, err := db.GetPaymentOrder(order.OrderNo)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != "pending" {
			t.Fatalf("mismatch or ambiguous amount should not confirm: %#v", updated)
		}
	}
}

func TestAlipayBillMonitorExpiresOldOrder(t *testing.T) {
	db, _ := newAlipayBillMonitorTestDatabase(t, `{"alipay_data_bill_accountlog_query_response":{"code":"10000","detail_list":[]}}`)
	order := createAlipayBillMonitorOrder(t, db, 100, time.Now().Add(-time.Minute))
	if err := NewAlipayBillMonitor(db).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	updated, err := db.GetPaymentOrder(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "expired" {
		t.Fatalf("expected expired order: %#v", updated)
	}
}

type alipayBillMonitorServer struct{ response string }

func newAlipayBillMonitorTestDatabase(t *testing.T, response string) (*config.Database, *alipayBillMonitorServer) {
	t.Helper()
	state := &alipayBillMonitorServer{response: response}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(state.response))
	}))
	t.Cleanup(server.Close)
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	settings := config.DefaultPaymentSettings()
	settings.ThirdPartyEnabled = true
	settings.AlipayBill = testAlipayBillSettings(server.URL)
	settings.Methods = []config.PaymentMethodSetting{{Code: "alipay_transfer", Label: "支付宝转账", Provider: "alipay_bill", Enabled: true}}
	if err := db.SavePaymentSettings(&settings); err != nil {
		t.Fatalf("SavePaymentSettings returned error: %v", err)
	}
	return db, state
}

func createAlipayBillMonitorOrder(t *testing.T, db *config.Database, amountCents int64, expiredAt time.Time) *config.PaymentOrder {
	t.Helper()
	order, err := db.CreateProviderPaymentOrder(config.ProviderPaymentOrderInput{UnionID: "union-alipay-monitor", Subject: "支付宝账单支付", AmountCents: amountCents, PointsAmount: amountCents, Provider: "alipay_bill", Method: "alipay_transfer", Metadata: map[string]interface{}{"match_mode": "amount_unique"}, ExpiredAt: expiredAt})
	if err != nil {
		t.Fatalf("CreateProviderPaymentOrder returned error: %v", err)
	}
	return order
}
