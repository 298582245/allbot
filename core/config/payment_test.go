package config

import (
	"database/sql"
	"testing"
	"time"
)

func TestDefaultPaymentSettings(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatalf("GetPaymentSettings returned error: %v", err)
	}
	if settings.PointsPerRMB != 100 {
		t.Fatalf("expected default points_per_rmb 100, got %d", settings.PointsPerRMB)
	}
	if settings.CurrencyUnit != "RMB" {
		t.Fatalf("expected default currency_unit RMB, got %s", settings.CurrencyUnit)
	}
	if settings.EpayQueryIntervalSeconds != 5 {
		t.Fatalf("expected default epay query interval 5, got %d", settings.EpayQueryIntervalSeconds)
	}
	if settings.HidePayURL {
		t.Fatal("expected default hide_pay_url false")
	}
	if len(settings.Methods) != 2 || settings.Methods[0].Code != "points" || !settings.Methods[0].Enabled || settings.Methods[1].Code != "alipay_transfer" || settings.Methods[1].Provider != "alipay_bill" || settings.Methods[1].Enabled {
		t.Fatalf("unexpected default methods: %#v", settings.Methods)
	}
	if settings.AlipayBill.GatewayURL != "https://openapi.alipay.com/gateway.do" || settings.AlipayBill.MatchMode != "amount_unique" || settings.AlipayBill.QueryMinutesBack != 30 || settings.AlipayBill.CheckIntervalSeconds != 15 || settings.AlipayBill.OrderTimeoutSeconds != 300 || settings.AlipayBill.BillPageSize != 100 {
		t.Fatalf("unexpected default alipay bill settings: %#v", settings.AlipayBill)
	}
}

func TestNormalizePaymentSettingsAppendsMissingDefaultMethods(t *testing.T) {
	settings := DefaultPaymentSettings()
	settings.Methods = []PaymentMethodSetting{{Code: "points", Label: "积分支付", Provider: "points", Enabled: true}}
	normalized := NormalizePaymentSettings(&settings)
	if len(normalized.Methods) < 2 || normalized.Methods[1].Code != "alipay_transfer" || normalized.Methods[1].Provider != "alipay_bill" {
		t.Fatalf("expected alipay bill default method to be appended: %#v", normalized.Methods)
	}
}

func TestSaveAndGetPaymentSettings(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings := DefaultPaymentSettings()
	settings.PointsPerRMB = 80
	settings.CurrencyUnit = "元"
	settings.MaxPendingPayments = 3
	settings.EpayQueryIntervalSeconds = 2
	settings.ThirdPartyEnabled = true
	settings.HidePayURL = true
	settings.QRCodeBaseURL = "https://qr.example.com/base"
	settings.EpaySubmitSubject = "后台伪造标题"
	settings.Methods = append(settings.Methods, PaymentMethodSetting{Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true})
	settings.Epay = EpaySettings{Enabled: true, Version: "v2", APIURL: "https://pay.example.com/", PID: "1000", PlatformPublicKey: "platform", MerchantPrivateKey: "merchant"}

	if err := db.SavePaymentSettings(&settings); err != nil {
		t.Fatalf("SavePaymentSettings returned error: %v", err)
	}
	saved, err := db.GetPaymentSettings()
	if err != nil {
		t.Fatalf("GetPaymentSettings returned error: %v", err)
	}
	if saved.PointsPerRMB != 80 || saved.CurrencyUnit != "元" || saved.MaxPendingPayments != 3 || saved.EpayQueryIntervalSeconds != 2 || !saved.ThirdPartyEnabled || !saved.HidePayURL || saved.QRCodeBaseURL != "https://qr.example.com/base" || saved.EpaySubmitSubject != "后台伪造标题" || saved.Epay.SignType != "RSA" || saved.Epay.Version != "v2" {
		t.Fatalf("unexpected saved settings: %#v", saved)
	}
	if len(saved.Methods) != 3 || saved.Methods[2].Code != "alipay" {
		t.Fatalf("unexpected methods: %#v", saved.Methods)
	}
}

func TestSavePaymentSettingsRejectsInvalidPointsPerRMB(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings := DefaultPaymentSettings()
	settings.PointsPerRMB = 0
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected invalid points_per_rmb to fail")
	}
}

func TestSavePaymentSettingsRejectsInvalidMaxPendingPayments(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings := DefaultPaymentSettings()
	settings.MaxPendingPayments = 0
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected invalid max_pending_payments to fail")
	}
}

func TestSavePaymentSettingsRejectsInvalidEpayQueryInterval(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings := DefaultPaymentSettings()
	settings.EpayQueryIntervalSeconds = 301
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected invalid epay query interval to fail")
	}
}

func TestSavePaymentSettingsRejectsInvalidQRCodeBaseURL(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings := DefaultPaymentSettings()
	settings.QRCodeBaseURL = "not-url"
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected invalid qrcode_base_url to fail")
	}
	settings.QRCodeBaseURL = "/open-apis"
	if err := db.SavePaymentSettings(&settings); err != nil {
		t.Fatalf("relative qrcode_base_url should be allowed: %v", err)
	}
}

func TestSavePaymentSettingsRejectsInvalidEpayConfig(t *testing.T) {
	db := newPaymentTestDatabase(t)
	settings := DefaultPaymentSettings()
	settings.ThirdPartyEnabled = true
	settings.Methods = append(settings.Methods, PaymentMethodSetting{Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true})
	settings.Epay = EpaySettings{Enabled: true, Version: "v1", APIURL: "not-url", PID: "1000", Key: "secret"}
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected invalid epay apiurl to fail")
	}
	settings.Epay.APIURL = "https://pay.example.com/"
	settings.Epay.Key = ""
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected missing v1 key to fail")
	}
	settings.Epay.Version = "v2"
	settings.Epay.Key = "secret"
	settings.Epay.PlatformPublicKey = ""
	settings.Epay.MerchantPrivateKey = "merchant"
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected missing v2 platform key to fail")
	}
	settings.Epay.PlatformPublicKey = "platform"
	settings.Epay.MerchantPrivateKey = ""
	if err := db.SavePaymentSettings(&settings); err == nil {
		t.Fatal("expected missing v2 merchant key to fail")
	}
}

func TestCalculatePointsAmountRoundsUp(t *testing.T) {
	cases := []struct {
		amountCents  int64
		pointsPerRMB int64
		expected     int64
	}{
		{100, 100, 100},
		{990, 100, 990},
		{1, 100, 1},
		{101, 99, 100},
	}
	for _, item := range cases {
		actual, err := CalculatePointsAmount(item.amountCents, item.pointsPerRMB)
		if err != nil {
			t.Fatalf("CalculatePointsAmount returned error: %v", err)
		}
		if actual != item.expected {
			t.Fatalf("CalculatePointsAmount(%d, %d) = %d, expected %d", item.amountCents, item.pointsPerRMB, actual, item.expected)
		}
	}
	if _, err := CalculatePointsAmount(0, 100); err == nil {
		t.Fatal("expected zero amount to fail")
	}
}

func TestCreatePaymentOrder(t *testing.T) {
	db := newPaymentTestDatabase(t)
	order, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_CREATE"))
	if err != nil {
		t.Fatalf("CreatePaymentOrder returned error: %v", err)
	}
	if order.ID == 0 || order.OrderNo != "PTEST_CREATE" || order.Status != "pending" {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestCreatePaymentOrderRejectsDuplicateOrderNo(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_DUP")); err != nil {
		t.Fatalf("CreatePaymentOrder returned error: %v", err)
	}
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_DUP")); err == nil {
		t.Fatal("expected duplicate order_no to fail")
	}
}

func TestCreatePaymentOrderRejectsInvalidStatus(t *testing.T) {
	db := newPaymentTestDatabase(t)
	order := newPaymentTestOrder("PTEST_CREATE_BAD_STATUS")
	order.Status = "unknown"
	if _, err := db.CreatePaymentOrder(order); err == nil {
		t.Fatal("expected invalid initial status to fail")
	}
}

func TestCountPendingPaymentOrdersIgnoresExpired(t *testing.T) {
	db := newPaymentTestDatabase(t)
	active := newPaymentTestOrder("PTEST_PENDING_ACTIVE")
	active.ExpiredAt = time.Now().Add(time.Hour)
	expired := newPaymentTestOrder("PTEST_PENDING_EXPIRED")
	expired.ExpiredAt = time.Now().Add(-time.Hour)
	paid := newPaymentTestOrder("PTEST_PENDING_PAID")
	paid.Status = "paid"
	if _, err := db.CreatePaymentOrder(active); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreatePaymentOrder(expired); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreatePaymentOrder(paid); err != nil {
		t.Fatal(err)
	}
	count, err := db.CountPendingPaymentOrders()
	if err != nil {
		t.Fatalf("CountPendingPaymentOrders returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active pending order, got %d", count)
	}
}

func TestCountPendingPaymentOrdersByUnionID(t *testing.T) {
	db := newPaymentTestDatabase(t)
	active := newPaymentTestOrder("PTEST_USER_PENDING_ACTIVE")
	active.UnionID = "union-count"
	expired := newPaymentTestOrder("PTEST_USER_PENDING_EXPIRED")
	expired.UnionID = "union-count"
	expired.ExpiredAt = time.Now().Add(-time.Hour)
	other := newPaymentTestOrder("PTEST_USER_PENDING_OTHER")
	other.UnionID = "union-other"
	for _, order := range []*PaymentOrder{active, expired, other} {
		if _, err := db.CreatePaymentOrder(order); err != nil {
			t.Fatal(err)
		}
	}
	count, err := db.CountPendingPaymentOrdersByUnionID("union-count")
	if err != nil {
		t.Fatalf("CountPendingPaymentOrdersByUnionID returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active pending order, got %d", count)
	}
}

func TestListPaymentOrdersFilters(t *testing.T) {
	db := newPaymentTestDatabase(t)
	first := newPaymentTestOrder("PTEST_LIST_1")
	first.PluginID = "plugin-a"
	first.UnionID = "union-a"
	second := newPaymentTestOrder("PTEST_LIST_2")
	second.PluginID = "plugin-b"
	second.UnionID = "union-b"
	second.Method = "wxpay"
	if _, err := db.CreatePaymentOrder(first); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreatePaymentOrder(second); err != nil {
		t.Fatal(err)
	}
	orders, total, err := db.ListPaymentOrders(PaymentOrderQuery{UnionID: "union-b"})
	if err != nil {
		t.Fatalf("ListPaymentOrders returned error: %v", err)
	}
	if total != 1 || len(orders) != 1 || orders[0].OrderNo != "PTEST_LIST_2" {
		t.Fatalf("unexpected filtered orders total=%d items=%#v", total, orders)
	}
}

func TestUpdatePaymentOrderStatusWritesEvent(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_STATUS")); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdatePaymentOrderStatus("PTEST_STATUS", "failed", "支付失败", map[string]string{"reason": "test"}); err != nil {
		t.Fatalf("UpdatePaymentOrderStatus returned error: %v", err)
	}
	order, err := db.GetPaymentOrder("PTEST_STATUS")
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "failed" {
		t.Fatalf("expected failed status, got %s", order.Status)
	}
	events, err := db.ListPaymentEvents("PTEST_STATUS")
	if err != nil {
		t.Fatalf("ListPaymentEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "status_failed" || events[0].Payload == "{}" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if err := db.UpdatePaymentOrderStatus("PTEST_STATUS", "failed", "重复失败", nil); err != nil {
		t.Fatalf("repeat UpdatePaymentOrderStatus returned error: %v", err)
	}
	events, err = db.ListPaymentEvents("PTEST_STATUS")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("repeat same status should be idempotent, got events: %#v", events)
	}
}

func TestUpdatePaymentOrderStatusRejectsInvalidStatus(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_BAD_STATUS")); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdatePaymentOrderStatus("PTEST_BAD_STATUS", "", "", nil); err == nil {
		t.Fatal("expected empty status to fail")
	}
	if err := db.UpdatePaymentOrderStatus("PTEST_BAD_STATUS", "unknown", "", nil); err == nil {
		t.Fatal("expected unknown status to fail")
	}
}

func TestUpdatePaymentOrderProviderInfoMissingOrder(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if err := db.UpdatePaymentOrderProviderInfo("missing", "trade-no", "", "", "{}"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
	events, err := db.ListPaymentEvents("missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("missing order should not create events: %#v", events)
	}
}

func TestMarkPaymentOrderPaidIsIdempotent(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_PAID")); err != nil {
		t.Fatal(err)
	}
	paidAt := time.Now()
	if err := db.MarkPaymentOrderPaid("PTEST_PAID", "trade-1", paidAt, "raw-1"); err != nil {
		t.Fatalf("MarkPaymentOrderPaid returned error: %v", err)
	}
	if err := db.MarkPaymentOrderPaid("PTEST_PAID", "trade-2", paidAt.Add(time.Hour), "raw-2"); err != nil {
		t.Fatalf("repeat MarkPaymentOrderPaid returned error: %v", err)
	}
	order, err := db.GetPaymentOrder("PTEST_PAID")
	if err != nil {
		t.Fatal(err)
	}
	if order.ProviderOrderNo != "trade-1" || order.NotifyRaw != "raw-1" {
		t.Fatalf("repeat MarkPaymentOrderPaid should not overwrite paid order: %#v", order)
	}
	events, err := db.ListPaymentEvents("PTEST_PAID")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "paid" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestCreateProviderPaymentOrderConfirmAndExpire(t *testing.T) {
	db := newPaymentTestDatabase(t)
	order, err := db.CreateProviderPaymentOrder(ProviderPaymentOrderInput{PluginID: "plugin-provider", UnionID: "union-provider", Platform: "test", UserID: "user", Subject: "第三方支付", AmountCents: 123, PointsAmount: 123, Provider: "epay", Method: "alipay", Metadata: map[string]interface{}{"k": "v"}, Remark: "备注", ExpiredAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatalf("CreateProviderPaymentOrder returned error: %v", err)
	}
	if order.Status != "pending" || order.Provider != "epay" || order.Method != "alipay" || order.Metadata == "{}" {
		t.Fatalf("unexpected provider order: %#v", order)
	}
	events, err := db.ListPaymentEvents(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "created" {
		t.Fatalf("unexpected create events: %#v", events)
	}
	confirmed, alreadyPaid, err := db.ConfirmProviderPayment(ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: "alipay", AmountCents: 123, ProviderOrderNo: "T123", Raw: "raw-paid", PaidAt: time.Now()})
	if err != nil {
		t.Fatalf("ConfirmProviderPayment returned error: %v", err)
	}
	if alreadyPaid || confirmed.Status != "paid" || confirmed.ProviderOrderNo != "T123" || confirmed.PaidAt == nil {
		t.Fatalf("unexpected confirmed order: already=%v %#v", alreadyPaid, confirmed)
	}
	confirmed, alreadyPaid, err = db.ConfirmProviderPayment(ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: "alipay", AmountCents: 123, ProviderOrderNo: "T123", Raw: "raw-repeat", PaidAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("repeat ConfirmProviderPayment returned error: %v", err)
	}
	if !alreadyPaid || confirmed.NotifyRaw != "raw-paid" {
		t.Fatalf("repeat confirmation should be idempotent: already=%v %#v", alreadyPaid, confirmed)
	}
	events, err = db.ListPaymentEvents(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].EventType != "paid" {
		t.Fatalf("unexpected paid events: %#v", events)
	}
	if err := db.ExpirePaymentOrder(order.OrderNo, "超时"); err != nil {
		t.Fatalf("Expire paid order should be noop: %v", err)
	}
	confirmed, err = db.GetPaymentOrder(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != "paid" {
		t.Fatalf("paid order should not expire: %#v", confirmed)
	}
}

func TestCreditPaymentPointsCreditsPaidOrderOnce(t *testing.T) {
	db := newPaymentTestDatabase(t)
	order, err := db.CreateProviderPaymentOrder(ProviderPaymentOrderInput{UnionID: "union-recharge", Subject: "积分充值", AmountCents: 100, PointsAmount: 100, Provider: "epay", Method: "alipay", ExpiredAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.ConfirmProviderPayment(ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: "alipay", AmountCents: 100, ProviderOrderNo: "T-RECHARGE", Raw: "raw-paid", PaidAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	balance, err := db.CreditPaymentPoints(order.OrderNo, "充值积分")
	if err != nil {
		t.Fatalf("CreditPaymentPoints returned error: %v", err)
	}
	if balance != 100 {
		t.Fatalf("balance = %d, expected 100", balance)
	}
	balance, err = db.CreditPaymentPoints(order.OrderNo, "充值积分")
	if err != nil {
		t.Fatalf("repeat CreditPaymentPoints returned error: %v", err)
	}
	if balance != 100 {
		t.Fatalf("repeat balance = %d, expected 100", balance)
	}
	transactions, total, err := db.ListPointTransactions(PointTransactionQuery{UnionID: "union-recharge", Source: "recharge", SourceID: order.OrderNo})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(transactions) != 1 || transactions[0].Delta != 100 || transactions[0].BalanceAfter != 100 {
		t.Fatalf("unexpected transactions total=%d items=%#v", total, transactions)
	}
}

func TestConfirmProviderPaymentRejectsMismatchAndClosedStatus(t *testing.T) {
	db := newPaymentTestDatabase(t)
	order, err := db.CreateProviderPaymentOrder(ProviderPaymentOrderInput{UnionID: "union-mismatch", Subject: "第三方支付", AmountCents: 100, PointsAmount: 100, Provider: "epay", Method: "alipay", ExpiredAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.ConfirmProviderPayment(ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: "alipay", AmountCents: 101, ProviderOrderNo: "T-BAD", Raw: "bad"}); err == nil {
		t.Fatal("expected amount mismatch to fail")
	}
	events, err := db.ListPaymentEvents(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].EventType != "provider_rejected" {
		t.Fatalf("expected reject event: %#v", events)
	}
	if err := db.ExpirePaymentOrder(order.OrderNo, "超时"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.ConfirmProviderPayment(ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: "alipay", AmountCents: 100, ProviderOrderNo: "T-LATE", Raw: "late"}); err == nil {
		t.Fatal("expected expired order confirmation to fail")
	}
	order, err = db.GetPaymentOrder(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "expired" {
		t.Fatalf("unexpected order status: %#v", order)
	}
}

func TestConfirmProviderPaymentRejectsExpiredPendingOrder(t *testing.T) {
	db := newPaymentTestDatabase(t)
	order, err := db.CreateProviderPaymentOrder(ProviderPaymentOrderInput{UnionID: "union-expired-pending", Subject: "过期待支付", AmountCents: 100, PointsAmount: 100, Provider: "epay", Method: "alipay", ExpiredAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.ConfirmProviderPayment(ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: "alipay", AmountCents: 100, ProviderOrderNo: "T-EXPIRED", Raw: "expired"}); err == nil {
		t.Fatal("expected expired pending order confirmation to fail")
	}
	order, err = db.GetPaymentOrder(order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "pending" {
		t.Fatalf("expired pending reject should not change status: %#v", order)
	}
}

func TestListPaymentEvents(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder("PTEST_EVENTS")); err != nil {
		t.Fatal(err)
	}
	if err := db.AppendPaymentEvent("PTEST_EVENTS", "created", "订单创建", nil); err != nil {
		t.Fatalf("AppendPaymentEvent returned error: %v", err)
	}
	events, err := db.ListPaymentEvents("PTEST_EVENTS")
	if err != nil {
		t.Fatalf("ListPaymentEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].Message != "订单创建" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestDeletePaymentOrderRemovesOrderAndEventsKeepsPointTransactions(t *testing.T) {
	db := newPaymentTestDatabase(t)
	orderNo := "PTEST_DELETE"
	if _, err := db.CreatePaymentOrder(newPaymentTestOrder(orderNo)); err != nil {
		t.Fatal(err)
	}
	if err := db.AppendPaymentEvent(orderNo, "created", "订单创建", nil); err != nil {
		t.Fatalf("AppendPaymentEvent returned error: %v", err)
	}
	if _, err := db.RecordPointTransaction(nil, &PointTransaction{UnionID: "union-test", Delta: -100, BalanceAfter: 0, Source: "payment", SourceID: orderNo, Description: "测试流水"}); err != nil {
		t.Fatalf("RecordPointTransaction returned error: %v", err)
	}
	if err := db.DeletePaymentOrder(" " + orderNo + " "); err != nil {
		t.Fatalf("DeletePaymentOrder returned error: %v", err)
	}
	if _, err := db.GetPaymentOrder(orderNo); err != sql.ErrNoRows {
		t.Fatalf("expected deleted order to return sql.ErrNoRows, got %v", err)
	}
	events, err := db.ListPaymentEvents(orderNo)
	if err != nil {
		t.Fatalf("ListPaymentEvents returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected payment events deleted, got %#v", events)
	}
	transactions, total, err := db.ListPointTransactions(PointTransactionQuery{UnionID: "union-test", Source: "payment", SourceID: orderNo})
	if err != nil {
		t.Fatalf("ListPointTransactions returned error: %v", err)
	}
	if total != 1 || len(transactions) != 1 || transactions[0].SourceID != orderNo {
		t.Fatalf("expected point transaction kept, total=%d items=%#v", total, transactions)
	}
}

func TestDeletePaymentOrderMissing(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if err := db.DeletePaymentOrder("missing"); err != sql.ErrNoRows {
		t.Fatalf("expected missing order to return sql.ErrNoRows, got %v", err)
	}
	if err := db.DeletePaymentOrder("   "); err != sql.ErrNoRows {
		t.Fatalf("expected empty order no to return sql.ErrNoRows, got %v", err)
	}
}

func TestRecordAndListPointTransactions(t *testing.T) {
	db := newPaymentTestDatabase(t)
	tx, err := db.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.RecordPointTransaction(tx, &PointTransaction{UnionID: "union-points", Delta: 50, BalanceAfter: 150, Source: "payment", SourceID: "PTEST_POINTS", Description: "测试流水"})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("RecordPointTransaction returned error: %v", err)
	}
	if item.ID == 0 {
		_ = tx.Rollback()
		t.Fatal("expected transaction id")
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	items, total, err := db.ListPointTransactions(PointTransactionQuery{UnionID: "union-points", Source: "payment"})
	if err != nil {
		t.Fatalf("ListPointTransactions returned error: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].SourceID != "PTEST_POINTS" {
		t.Fatalf("unexpected point transactions total=%d items=%#v", total, items)
	}
}

func TestSettlePointsPaymentSuccess(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.AddUserPoints("union-settle", 300); err != nil {
		t.Fatal(err)
	}
	result, err := db.SettlePointsPayment(PointsPaymentSettlement{PluginID: "plugin-test", UnionID: "union-settle", Platform: "test", UserID: "user", Subject: "事务支付", AmountCents: 200, PointsAmount: 200, Provider: "points", Method: "points", Metadata: map[string]interface{}{"k": "v"}, ExpiredAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatalf("SettlePointsPayment returned error: %v", err)
	}
	if result.Status != "paid" || result.PointsBalance != 100 || result.OrderNo == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "paid" || order.ProviderOrderNo == "" || order.PaidAt == nil {
		t.Fatalf("unexpected order: %#v", order)
	}
	events, err := db.ListPaymentEvents(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != "created" || events[1].EventType != "paid" {
		t.Fatalf("unexpected events: %#v", events)
	}
	transactions, total, err := db.ListPointTransactions(PointTransactionQuery{UnionID: "union-settle", Source: "payment", SourceID: result.OrderNo})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(transactions) != 1 || transactions[0].Delta != -200 || transactions[0].BalanceAfter != 100 {
		t.Fatalf("unexpected transactions total=%d items=%#v", total, transactions)
	}
}

func TestSettlePointsPaymentInsufficientPoints(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.AddUserPoints("union-low-settle", 50); err != nil {
		t.Fatal(err)
	}
	result, err := db.SettlePointsPayment(PointsPaymentSettlement{UnionID: "union-low-settle", Subject: "余额不足", AmountCents: 200, PointsAmount: 200})
	if err == nil {
		t.Fatal("expected insufficient points to fail")
	}
	if result == nil || result.Status != "failed" || result.PointsBalance != 50 || result.OrderNo == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	order, err := db.GetPaymentOrder(result.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "failed" {
		t.Fatalf("unexpected order: %#v", order)
	}
	transactions, total, err := db.ListPointTransactions(PointTransactionQuery{UnionID: "union-low-settle", Source: "payment"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(transactions) != 0 {
		t.Fatalf("insufficient points should not write transaction: %#v", transactions)
	}
}

func TestGetPaymentOrderMissing(t *testing.T) {
	db := newPaymentTestDatabase(t)
	if _, err := db.GetPaymentOrder("missing"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func newPaymentTestDatabase(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newPaymentTestOrder(orderNo string) *PaymentOrder {
	return &PaymentOrder{
		OrderNo:      orderNo,
		PluginID:     "plugin-test",
		UnionID:      "union-test",
		Platform:     "test",
		UserID:       "user-test",
		GroupID:      "group-test",
		Subject:      "测试支付",
		AmountCents:  100,
		PointsAmount: 100,
		Provider:     "epay",
		Method:       "alipay",
		Status:       "pending",
		Metadata:     "{}",
		ExpiredAt:    time.Now().Add(time.Hour),
	}
}
