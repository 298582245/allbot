package payment

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
)

type AlipayBillMonitor struct {
	database *config.Database
	mu       sync.Mutex
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func NewAlipayBillMonitor(database *config.Database) *AlipayBillMonitor {
	return &AlipayBillMonitor{database: database, stopCh: make(chan struct{}), doneCh: make(chan struct{})}
}

func (m *AlipayBillMonitor) Start() {
	if m == nil || m.database == nil {
		return
	}
	go m.loop()
}

func (m *AlipayBillMonitor) Stop() {
	if m == nil {
		return
	}
	select {
	case <-m.doneCh:
		return
	default:
	}
	close(m.stopCh)
	<-m.doneCh
}

func (m *AlipayBillMonitor) loop() {
	defer close(m.doneCh)
	interval := 15 * time.Second
	for {
		settings, err := m.database.GetPaymentSettings()
		if err == nil {
			interval = time.Duration(config.NormalizePaymentSettings(settings).AlipayBill.CheckIntervalSeconds) * time.Second
		}
		if err := m.RunOnce(context.Background()); err != nil {
			log.Printf("[PAYMENT] Alipay bill monitor failed: %v", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-m.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (m *AlipayBillMonitor) RunOnce(ctx context.Context) error {
	if m == nil || m.database == nil {
		return fmt.Errorf("数据库不可用")
	}
	if !m.mu.TryLock() {
		return nil
	}
	defer m.mu.Unlock()
	settings, err := m.database.GetPaymentSettings()
	if err != nil {
		return err
	}
	settingsValue := config.NormalizePaymentSettings(settings)
	if !settingsValue.ThirdPartyEnabled || !settingsValue.AlipayBill.Enabled {
		return nil
	}
	provider, err := NewAlipayBillProvider(settingsValue.AlipayBill, nil)
	if err != nil {
		_ = m.database.SetPaymentProviderState(providerAlipayBill, "last_error", err.Error())
		return err
	}
	orders, err := m.database.ListPendingProviderPaymentOrders(providerAlipayBill, 200)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		return nil
	}
	now := time.Now()
	activeOrders := make([]*config.PaymentOrder, 0, len(orders))
	for _, order := range orders {
		if order.ExpiredAt.Before(now) {
			_ = m.database.ExpirePaymentOrder(order.OrderNo, "支付超时")
			continue
		}
		activeOrders = append(activeOrders, order)
	}
	if len(activeOrders) == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	start := now.Add(-time.Duration(settingsValue.AlipayBill.QueryMinutesBack) * time.Minute)
	items, raw, err := provider.QueryBills(start, now)
	if err != nil {
		_ = m.database.SetPaymentProviderState(providerAlipayBill, "last_error", err.Error())
		return err
	}
	_ = m.database.SetPaymentProviderState(providerAlipayBill, "last_query_at", now.Format(time.RFC3339))
	_ = m.database.SetPaymentProviderState(providerAlipayBill, "last_error", "")
	for _, item := range items {
		record := &config.AlipayBillRecord{ProviderOrderNo: item.ProviderOrderNo, AccountLogID: item.AccountLogID, AmountCents: item.AmountCents, Direction: item.Direction, Remark: item.Remark, Summary: item.Summary, OppositeAccount: item.OppositeAccount, PaidAt: item.PaidAt, Raw: stringDefault(item.Raw, raw)}
		if err := m.database.SaveAlipayBillRecord(record); err != nil {
			_ = m.database.SetPaymentProviderState(providerAlipayBill, "last_error", err.Error())
			continue
		}
	}
	for _, item := range items {
		if !alipayBillItemIsIncome(item) {
			continue
		}
		order := alipayBillUniqueAmountOrder(item, activeOrders)
		if order == nil {
			continue
		}
		saved, err := m.database.FindAlipayBillRecordByProviderOrderNo(item.ProviderOrderNo)
		if err == nil && saved.MatchedAt != nil {
			continue
		}
		marked, err := m.database.MarkAlipayBillRecordMatched(item.ProviderOrderNo, order.OrderNo)
		if err != nil || !marked {
			continue
		}
		confirmed, _, err := m.database.ConfirmProviderPayment(config.ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: order.Method, AmountCents: item.AmountCents, ProviderOrderNo: item.ProviderOrderNo, Raw: item.Raw, PaidAt: item.PaidAt})
		if err != nil {
			_ = m.database.AppendPaymentEvent(order.OrderNo, "provider_rejected", err.Error(), item.Raw)
			continue
		}
		DefaultWaitHub.Resolve(order.OrderNo, PaymentResult{Status: "paid", OrderNo: confirmed.OrderNo, Provider: confirmed.Provider, Method: confirmed.Method, Subject: confirmed.Subject, AmountCents: confirmed.AmountCents, PointsAmount: confirmed.PointsAmount, PayURL: confirmed.PayURL, QRCode: confirmed.QRCode, ProviderOrderNo: confirmed.ProviderOrderNo, Message: "支付成功"})
	}
	return nil
}

func alipayBillUniqueAmountOrder(item alipayBillItem, orders []*config.PaymentOrder) *config.PaymentOrder {
	var matched *config.PaymentOrder
	for _, order := range orders {
		if order == nil || order.AmountCents != item.AmountCents || !alipayBillPaidAtInOrderWindow(item.PaidAt, order) {
			continue
		}
		if matched != nil {
			return nil
		}
		matched = order
	}
	return matched
}

func alipayBillPaidAtInOrderWindow(paidAt time.Time, order *config.PaymentOrder) bool {
	if order == nil || paidAt.IsZero() {
		return false
	}
	return !paidAt.Before(order.CreatedAt.Add(-time.Minute)) && !paidAt.After(order.ExpiredAt)
}

func alipayBillMethodCode(method config.PaymentMethodSetting) string {
	code := strings.TrimSpace(method.Code)
	if code == "" {
		return alipayBillMethodTransfer
	}
	return code
}
