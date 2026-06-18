package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	paymentPointsPerRMBKey = "payment.points_per_rmb"
	paymentConfigKey       = "payment.config"
)

var validPaymentOrderStatuses = map[string]bool{
	"created":   true,
	"pending":   true,
	"paid":      true,
	"failed":    true,
	"expired":   true,
	"cancelled": true,
}

type PaymentSettings struct {
	PointsPerRMB             int64                  `json:"points_per_rmb"`
	CurrencyUnit             string                 `json:"currency_unit"`
	ThirdPartyEnabled        bool                   `json:"third_party_enabled"`
	HidePayURL               bool                   `json:"hide_pay_url"`
	QRCodeBaseURL            string                 `json:"qrcode_base_url,omitempty"`
	EpaySubmitSubject        string                 `json:"epay_submit_subject,omitempty"`
	MaxPendingPayments       int                    `json:"max_pending_payments"`
	EpayQueryIntervalSeconds int                    `json:"epay_query_interval_seconds"`
	Methods                  []PaymentMethodSetting `json:"methods"`
	Epay                     EpaySettings           `json:"epay"`
}

type PaymentMethodSetting struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Enabled  bool   `json:"enabled"`
}

type EpaySettings struct {
	Enabled            bool   `json:"enabled"`
	Version            string `json:"version"`
	APIURL             string `json:"apiurl"`
	PID                string `json:"pid"`
	Key                string `json:"key,omitempty"`
	SignType           string `json:"sign_type"`
	PlatformPublicKey  string `json:"platform_public_key,omitempty"`
	MerchantPrivateKey string `json:"merchant_private_key,omitempty"`
	ReturnURL          string `json:"return_url,omitempty"`
	HasKey             bool   `json:"has_key"`
	HasPlatformKey     bool   `json:"has_platform_public_key"`
	HasMerchantKey     bool   `json:"has_merchant_private_key"`
}

type PaymentOrder struct {
	ID              int64      `json:"id"`
	OrderNo         string     `json:"order_no"`
	PluginID        string     `json:"plugin_id"`
	UnionID         string     `json:"union_id"`
	Platform        string     `json:"platform"`
	AdapterID       string     `json:"adapter_id"`
	UserID          string     `json:"user_id"`
	GroupID         string     `json:"group_id"`
	Subject         string     `json:"subject"`
	AmountCents     int64      `json:"amount_cents"`
	PointsAmount    int64      `json:"points_amount"`
	Provider        string     `json:"provider"`
	Method          string     `json:"method"`
	Status          string     `json:"status"`
	ProviderOrderNo string     `json:"provider_order_no"`
	PayURL          string     `json:"pay_url"`
	QRCode          string     `json:"qrcode"`
	NotifyRaw       string     `json:"notify_raw"`
	Metadata        string     `json:"metadata"`
	ExpiredAt       time.Time  `json:"expired_at"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PaymentOrderQuery struct {
	OrderNo  string
	UnionID  string
	PluginID string
	Provider string
	Method   string
	Status   string
	Limit    int
	Offset   int
}

type PaymentStatsSummary struct {
	TotalOrders          int64 `json:"total_orders"`
	PendingOrders        int64 `json:"pending_orders"`
	PaidOrders           int64 `json:"paid_orders"`
	FailedOrders         int64 `json:"failed_orders"`
	ExpiredOrders        int64 `json:"expired_orders"`
	CancelledOrders      int64 `json:"cancelled_orders"`
	PaidAmountCents      int64 `json:"paid_amount_cents"`
	TodayPaidAmountCents int64 `json:"today_paid_amount_cents"`
}

type PaymentEvent struct {
	ID        int64     `json:"id"`
	OrderNo   string    `json:"order_no"`
	EventType string    `json:"event_type"`
	Message   string    `json:"message"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

type PointTransaction struct {
	ID           int64     `json:"id"`
	UnionID      string    `json:"union_id"`
	Delta        int64     `json:"delta"`
	BalanceAfter int64     `json:"balance_after"`
	Source       string    `json:"source"`
	SourceID     string    `json:"source_id"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type PointTransactionQuery struct {
	UnionID  string
	Source   string
	SourceID string
	Limit    int
	Offset   int
}

type PointsPaymentSettlement struct {
	PluginID     string
	UnionID      string
	Platform     string
	AdapterID    string
	UserID       string
	GroupID      string
	Subject      string
	AmountCents  int64
	PointsAmount int64
	Provider     string
	Method       string
	Metadata     map[string]interface{}
	ExpiredAt    time.Time
}

type PointsPaymentSettlementResult struct {
	OrderNo       string
	Status        string
	PointsBalance int64
	Message       string
}

type ProviderPaymentOrderInput struct {
	PluginID     string
	UnionID      string
	Platform     string
	AdapterID    string
	UserID       string
	GroupID      string
	Subject      string
	AmountCents  int64
	PointsAmount int64
	Provider     string
	Method       string
	Metadata     map[string]interface{}
	Remark       string
	ExpiredAt    time.Time
}

type ProviderPaymentConfirmation struct {
	OrderNo         string
	Provider        string
	Method          string
	AmountCents     int64
	ProviderOrderNo string
	Raw             string
	PaidAt          time.Time
}

func DefaultPaymentSettings() PaymentSettings {
	return PaymentSettings{
		PointsPerRMB:             100,
		CurrencyUnit:             "RMB",
		ThirdPartyEnabled:        false,
		MaxPendingPayments:       10,
		EpayQueryIntervalSeconds: 5,
		Methods: []PaymentMethodSetting{
			{Code: "points", Label: "积分支付", Provider: "points", Enabled: true},
		},
		Epay: EpaySettings{Enabled: false, Version: "v1", SignType: "MD5"},
	}
}

func NormalizePaymentSettings(settings *PaymentSettings) PaymentSettings {
	result := DefaultPaymentSettings()
	if settings == nil {
		return result
	}
	result = *settings
	if result.PointsPerRMB <= 0 {
		result.PointsPerRMB = 100
	}
	result.CurrencyUnit = strings.TrimSpace(result.CurrencyUnit)
	if result.CurrencyUnit == "" {
		result.CurrencyUnit = "RMB"
	}
	if result.MaxPendingPayments <= 0 {
		result.MaxPendingPayments = 10
	}
	if result.EpayQueryIntervalSeconds <= 0 {
		result.EpayQueryIntervalSeconds = 5
	}
	result.QRCodeBaseURL = strings.TrimSpace(result.QRCodeBaseURL)
	result.EpaySubmitSubject = strings.TrimSpace(result.EpaySubmitSubject)
	if len(result.Methods) == 0 {
		result.Methods = DefaultPaymentSettings().Methods
	}
	for i := range result.Methods {
		result.Methods[i].Code = strings.TrimSpace(result.Methods[i].Code)
		result.Methods[i].Label = strings.TrimSpace(result.Methods[i].Label)
		result.Methods[i].Provider = strings.TrimSpace(result.Methods[i].Provider)
		if result.Methods[i].Label == "" {
			result.Methods[i].Label = result.Methods[i].Code
		}
	}
	result.Epay.Version = strings.ToLower(strings.TrimSpace(result.Epay.Version))
	if result.Epay.Version == "" {
		result.Epay.Version = "v1"
	}
	result.Epay.APIURL = strings.TrimSpace(result.Epay.APIURL)
	result.Epay.PID = strings.TrimSpace(result.Epay.PID)
	result.Epay.ReturnURL = strings.TrimSpace(result.Epay.ReturnURL)
	if result.Epay.Version == "v2" {
		result.Epay.SignType = "RSA"
	} else {
		result.Epay.SignType = "MD5"
	}
	return result
}

func ValidatePaymentSettings(settings *PaymentSettings) error {
	if settings == nil {
		return fmt.Errorf("支付配置不能为空")
	}
	if settings.PointsPerRMB <= 0 {
		return fmt.Errorf("积分兑换比例必须大于 0")
	}
	if len([]rune(strings.TrimSpace(settings.CurrencyUnit))) > 16 {
		return fmt.Errorf("支付金额单位不能超过 16 个字符")
	}
	if settings.MaxPendingPayments <= 0 {
		return fmt.Errorf("同时支付个数必须大于 0")
	}
	if settings.EpayQueryIntervalSeconds <= 0 || settings.EpayQueryIntervalSeconds > 300 {
		return fmt.Errorf("易支付自动查询间隔必须在 1 到 300 秒之间")
	}
	if len([]rune(strings.TrimSpace(settings.EpaySubmitSubject))) > 128 {
		return fmt.Errorf("易支付提交标题不能超过 128 个字符")
	}
	if err := validatePaymentQRCodeBaseURL(settings.QRCodeBaseURL); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, method := range settings.Methods {
		code := strings.TrimSpace(method.Code)
		provider := strings.TrimSpace(method.Provider)
		if code == "" || provider == "" {
			return fmt.Errorf("支付方式 code 和 provider 不能为空")
		}
		if seen[code] {
			return fmt.Errorf("支付方式 code 不能重复: %s", code)
		}
		seen[code] = true
	}
	version := strings.ToLower(strings.TrimSpace(settings.Epay.Version))
	if version != "" && version != "v1" && version != "v2" {
		return fmt.Errorf("易支付版本只能是 v1 或 v2")
	}
	if settings.Epay.Enabled || settings.ThirdPartyEnabled || paymentMethodsUseProvider(settings.Methods, "epay") {
		if err := validateEpaySettings(settings.Epay); err != nil {
			return err
		}
	}
	return nil
}

func validatePaymentQRCodeBaseURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "/") {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("二维码图片基础地址必须是完整 URL 或以 / 开头的路径")
	}
	return nil
}

func paymentMethodsUseProvider(methods []PaymentMethodSetting, provider string) bool {
	for _, method := range methods {
		if method.Enabled && strings.EqualFold(strings.TrimSpace(method.Provider), provider) {
			return true
		}
	}
	return false
}

func validateEpaySettings(settings EpaySettings) error {
	apiURL, err := url.ParseRequestURI(strings.TrimSpace(settings.APIURL))
	if err != nil || apiURL.Scheme == "" || apiURL.Host == "" {
		return fmt.Errorf("易支付接口地址必须是完整 URL")
	}
	if strings.TrimSpace(settings.PID) == "" {
		return fmt.Errorf("易支付商户 ID 不能为空")
	}
	version := strings.ToLower(strings.TrimSpace(settings.Version))
	if version == "v2" {
		if strings.TrimSpace(settings.PlatformPublicKey) == "" || strings.TrimSpace(settings.MerchantPrivateKey) == "" {
			return fmt.Errorf("易支付 V2 平台公钥和商户私钥不能为空")
		}
		return nil
	}
	if strings.TrimSpace(settings.Key) == "" {
		return fmt.Errorf("易支付 V1 商户密钥不能为空")
	}
	return nil
}

func validatePaymentOrderStatus(status string) error {
	if !validPaymentOrderStatuses[strings.TrimSpace(status)] {
		return fmt.Errorf("支付订单状态无效: %s", status)
	}
	return nil
}

func (d *Database) GetPaymentSettings() (*PaymentSettings, error) {
	settings, err := d.readPaymentConfigOnly()
	if err != nil {
		return nil, err
	}
	pointsPerRMB, err := d.GetPointsPerRMB()
	if err != nil {
		return nil, err
	}
	settings.PointsPerRMB = pointsPerRMB
	settings = NormalizePaymentSettings(&settings)
	return &settings, nil
}

func (d *Database) SavePaymentSettings(settings *PaymentSettings) error {
	if settings != nil && settings.PointsPerRMB <= 0 {
		return fmt.Errorf("积分兑换比例必须大于 0")
	}
	if settings != nil && settings.MaxPendingPayments <= 0 {
		return fmt.Errorf("同时支付个数必须大于 0")
	}
	current, _ := d.readPaymentConfigOnly()
	if settings != nil {
		if settings.Epay.Key == "" && settings.Epay.HasKey {
			settings.Epay.Key = current.Epay.Key
		}
		if settings.Epay.PlatformPublicKey == "" && settings.Epay.HasPlatformKey {
			settings.Epay.PlatformPublicKey = current.Epay.PlatformPublicKey
		}
		if settings.Epay.MerchantPrivateKey == "" && settings.Epay.HasMerchantKey {
			settings.Epay.MerchantPrivateKey = current.Epay.MerchantPrivateKey
		}
	}
	normalized := NormalizePaymentSettings(settings)
	if err := ValidatePaymentSettings(&normalized); err != nil {
		return err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	if err = d.SetSetting(paymentPointsPerRMBKey, strconv.FormatInt(normalized.PointsPerRMB, 10), "积分兑换比例"); err != nil {
		return err
	}
	return d.SetSetting(paymentConfigKey, string(data), "支付配置")
}

func (d *Database) GetPointsPerRMB() (int64, error) {
	value, err := d.GetSetting(paymentPointsPerRMBKey)
	if err == sql.ErrNoRows {
		return 100, nil
	}
	if err != nil {
		return 0, err
	}
	pointsPerRMB, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || pointsPerRMB <= 0 {
		return 0, fmt.Errorf("积分兑换比例无效")
	}
	return pointsPerRMB, nil
}

func (d *Database) SavePointsPerRMB(pointsPerRMB int64) error {
	if pointsPerRMB <= 0 {
		return fmt.Errorf("积分兑换比例必须大于 0")
	}
	settings, err := d.readPaymentConfigOnly()
	if err != nil {
		return err
	}
	settings.PointsPerRMB = pointsPerRMB
	return d.SavePaymentSettings(&settings)
}

func CalculatePointsAmount(amountCents int64, pointsPerRMB int64) (int64, error) {
	if amountCents <= 0 || pointsPerRMB <= 0 {
		return 0, fmt.Errorf("金额和积分兑换比例必须大于 0")
	}
	maxInt64 := int64(^uint64(0) >> 1)
	if amountCents > maxInt64/pointsPerRMB {
		return 0, fmt.Errorf("积分金额溢出")
	}
	return (amountCents*pointsPerRMB + 99) / 100, nil
}

func (d *Database) SettlePointsPayment(input PointsPaymentSettlement) (*PointsPaymentSettlementResult, error) {
	input.UnionID = strings.TrimSpace(input.UnionID)
	input.Subject = strings.TrimSpace(input.Subject)
	input.Provider = stringDefault(input.Provider, "points")
	input.Method = stringDefault(input.Method, "points")
	if input.UnionID == "" || input.Subject == "" {
		return nil, fmt.Errorf("支付用户和标题不能为空")
	}
	if input.AmountCents <= 0 || input.PointsAmount <= 0 {
		return nil, fmt.Errorf("订单金额和积分金额必须大于 0")
	}
	metadata, err := marshalPayload(input.Metadata)
	if err != nil {
		return nil, err
	}
	if input.ExpiredAt.IsZero() {
		input.ExpiredAt = time.Now().Add(15 * time.Minute)
	}
	orderNo, err := GeneratePaymentOrderNo()
	if err != nil {
		return nil, err
	}
	d.pointsMu.Lock()
	defer d.pointsMu.Unlock()
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, input.UnionID); err != nil {
		return nil, err
	}
	var balance int64
	if err = tx.QueryRow(`SELECT points FROM user_points WHERE union_id = ?`, input.UnionID).Scan(&balance); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`
		INSERT INTO payment_orders (order_no, plugin_id, union_id, platform, adapter_id, user_id, group_id, subject, amount_cents, points_amount, provider, method, status, metadata, expired_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, orderNo, input.PluginID, input.UnionID, input.Platform, input.AdapterID, input.UserID, input.GroupID, input.Subject, input.AmountCents, input.PointsAmount, input.Provider, input.Method, metadata, input.ExpiredAt); err != nil {
		return nil, err
	}
	if err = appendPaymentEventTx(tx, orderNo, "created", "订单创建", metadata); err != nil {
		return nil, err
	}
	if balance < input.PointsAmount {
		message := fmt.Sprintf("积分不足，当前 %d，需要 %d", balance, input.PointsAmount)
		if _, err = tx.Exec(`UPDATE payment_orders SET status = 'failed', updated_at = CURRENT_TIMESTAMP WHERE order_no = ?`, orderNo); err != nil {
			return nil, err
		}
		if err = appendPaymentEventTx(tx, orderNo, "status_failed", message, metadata); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return &PointsPaymentSettlementResult{OrderNo: orderNo, Status: "failed", PointsBalance: balance, Message: message}, fmt.Errorf("%s", message)
	}
	remaining := balance - input.PointsAmount
	if _, err = tx.Exec(`UPDATE user_points SET points = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, remaining, input.UnionID); err != nil {
		return nil, err
	}
	if _, err = d.RecordPointTransaction(tx, &PointTransaction{UnionID: input.UnionID, Delta: -input.PointsAmount, BalanceAfter: remaining, Source: "payment", SourceID: orderNo, Description: input.Subject}); err != nil {
		return nil, err
	}
	paidAt := time.Now()
	providerOrderNo := "points:" + orderNo
	if _, err = tx.Exec(`UPDATE payment_orders SET status = 'paid', provider_order_no = ?, paid_at = ?, updated_at = CURRENT_TIMESTAMP WHERE order_no = ?`, providerOrderNo, paidAt, orderNo); err != nil {
		return nil, err
	}
	if err = appendPaymentEventTx(tx, orderNo, "paid", "积分支付成功", metadata); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &PointsPaymentSettlementResult{OrderNo: orderNo, Status: "paid", PointsBalance: remaining, Message: "支付成功"}, nil
}

func (d *Database) CreditPaymentPoints(orderNo string, description string) (int64, error) {
	orderNo = strings.TrimSpace(orderNo)
	description = strings.TrimSpace(description)
	if orderNo == "" {
		return 0, sql.ErrNoRows
	}
	d.pointsMu.Lock()
	defer d.pointsMu.Unlock()
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	order, err := scanPaymentOrder(tx.QueryRow(paymentOrderSelectSQL()+` WHERE order_no = ?`, orderNo))
	if err != nil {
		return 0, err
	}
	if order.Status != "paid" {
		return 0, fmt.Errorf("订单状态 %s 不能入账", order.Status)
	}
	if order.UnionID == "" || order.PointsAmount <= 0 {
		return 0, fmt.Errorf("订单缺少用户或积分金额")
	}
	var existing int64
	if err = tx.QueryRow(`SELECT COUNT(1) FROM point_transactions WHERE source = 'recharge' AND source_id = ?`, orderNo).Scan(&existing); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, order.UnionID); err != nil {
		return 0, err
	}
	var current int64
	if err = tx.QueryRow(`SELECT points FROM user_points WHERE union_id = ?`, order.UnionID).Scan(&current); err != nil {
		return 0, err
	}
	if existing > 0 {
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return current, nil
	}
	remaining := current + order.PointsAmount
	if _, err = tx.Exec(`UPDATE user_points SET points = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, remaining, order.UnionID); err != nil {
		return 0, err
	}
	if description == "" {
		description = order.Subject
	}
	if _, err = d.RecordPointTransaction(tx, &PointTransaction{UnionID: order.UnionID, Delta: order.PointsAmount, BalanceAfter: remaining, Source: "recharge", SourceID: orderNo, Description: description}); err != nil {
		return 0, err
	}
	payload, err := marshalPayload(map[string]interface{}{"points": order.PointsAmount, "balance": remaining})
	if err != nil {
		return 0, err
	}
	if err = appendPaymentEventTx(tx, orderNo, "points_credited", "充值积分已入账", payload); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return remaining, nil
}

func (d *Database) CreateProviderPaymentOrder(input ProviderPaymentOrderInput) (*PaymentOrder, error) {
	input.UnionID = strings.TrimSpace(input.UnionID)
	input.Subject = strings.TrimSpace(input.Subject)
	input.Provider = stringDefault(input.Provider, "epay")
	input.Method = strings.TrimSpace(input.Method)
	if input.UnionID == "" || input.Subject == "" || input.Method == "" {
		return nil, fmt.Errorf("支付用户、标题和方式不能为空")
	}
	if input.AmountCents <= 0 || input.PointsAmount <= 0 {
		return nil, fmt.Errorf("订单金额和积分金额必须大于 0")
	}
	if input.ExpiredAt.IsZero() {
		input.ExpiredAt = time.Now().Add(15 * time.Minute)
	}
	metadata, err := providerPaymentMetadata(input.Metadata, input.Remark)
	if err != nil {
		return nil, err
	}
	orderNo, err := GeneratePaymentOrderNo()
	if err != nil {
		return nil, err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`
		INSERT INTO payment_orders (order_no, plugin_id, union_id, platform, adapter_id, user_id, group_id, subject, amount_cents, points_amount, provider, method, status, metadata, expired_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, orderNo, input.PluginID, input.UnionID, input.Platform, input.AdapterID, input.UserID, input.GroupID, input.Subject, input.AmountCents, input.PointsAmount, input.Provider, input.Method, metadata, input.ExpiredAt); err != nil {
		return nil, err
	}
	if err = appendPaymentEventTx(tx, orderNo, "created", "订单创建", metadata); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetPaymentOrder(orderNo)
}

func (d *Database) ExpirePaymentOrder(orderNo, message string) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return sql.ErrNoRows
	}
	payloadText, err := marshalPayload(map[string]string{"message": message})
	if err != nil {
		return err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE payment_orders SET status = 'expired', updated_at = CURRENT_TIMESTAMP WHERE order_no = ? AND status = 'pending'`, orderNo)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int
		if scanErr := tx.QueryRow(`SELECT 1 FROM payment_orders WHERE order_no = ?`, orderNo).Scan(&exists); scanErr != nil {
			return scanErr
		}
		return tx.Commit()
	}
	if strings.TrimSpace(message) == "" {
		message = "订单已超时"
	}
	if err = appendPaymentEventTx(tx, orderNo, "status_expired", message, payloadText); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) ConfirmProviderPayment(input ProviderPaymentConfirmation) (*PaymentOrder, bool, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.Provider = strings.TrimSpace(input.Provider)
	input.Method = strings.TrimSpace(input.Method)
	input.ProviderOrderNo = strings.TrimSpace(input.ProviderOrderNo)
	if input.OrderNo == "" {
		return nil, false, sql.ErrNoRows
	}
	if input.AmountCents <= 0 {
		return nil, false, fmt.Errorf("支付金额必须大于 0")
	}
	if input.PaidAt.IsZero() {
		input.PaidAt = time.Now()
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	order, err := scanPaymentOrder(tx.QueryRow(paymentOrderSelectSQL()+` WHERE order_no = ?`, input.OrderNo))
	if err != nil {
		return nil, false, err
	}
	if err = validateProviderPaymentOrderTx(tx, order, input); err != nil {
		if eventErr := appendProviderPaymentRejectEvent(tx, input.OrderNo, err.Error(), input); eventErr != nil {
			return nil, false, eventErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return nil, false, commitErr
		}
		return nil, false, err
	}
	if order.Status == "paid" {
		if err = tx.Commit(); err != nil {
			return nil, false, err
		}
		return order, true, nil
	}
	if order.Status != "pending" {
		err = fmt.Errorf("订单状态 %s 不允许确认支付", order.Status)
		if eventErr := appendProviderPaymentRejectEvent(tx, input.OrderNo, err.Error(), input); eventErr != nil {
			return nil, false, eventErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return nil, false, commitErr
		}
		return nil, false, err
	}
	result, err := tx.Exec(`
		UPDATE payment_orders
		SET status = 'paid', provider_order_no = ?, notify_raw = ?, paid_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE order_no = ? AND status = 'pending'
	`, input.ProviderOrderNo, input.Raw, input.PaidAt, input.OrderNo)
	if err != nil {
		return nil, false, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, false, fmt.Errorf("订单状态已变更")
	}
	if err = appendPaymentEventTx(tx, input.OrderNo, "paid", "第三方支付成功", input.Raw); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	updated, err := d.GetPaymentOrder(input.OrderNo)
	return updated, false, err
}

func (d *Database) CreatePaymentOrder(order *PaymentOrder) (*PaymentOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("订单不能为空")
	}
	if strings.TrimSpace(order.OrderNo) == "" {
		orderNo, err := GeneratePaymentOrderNo()
		if err != nil {
			return nil, err
		}
		order.OrderNo = orderNo
	}
	order.OrderNo = strings.TrimSpace(order.OrderNo)
	order.UnionID = strings.TrimSpace(order.UnionID)
	order.Subject = strings.TrimSpace(order.Subject)
	order.Provider = strings.TrimSpace(order.Provider)
	order.Method = strings.TrimSpace(order.Method)
	order.Status = strings.TrimSpace(order.Status)
	if order.UnionID == "" || order.Subject == "" || order.Provider == "" || order.Method == "" || order.Status == "" {
		return nil, fmt.Errorf("订单用户、标题、渠道、方式和状态不能为空")
	}
	if err := validatePaymentOrderStatus(order.Status); err != nil {
		return nil, err
	}
	if order.AmountCents <= 0 || order.PointsAmount <= 0 {
		return nil, fmt.Errorf("订单金额和积分金额必须大于 0")
	}
	if order.Metadata == "" {
		order.Metadata = "{}"
	}
	if order.ExpiredAt.IsZero() {
		order.ExpiredAt = time.Now().Add(15 * time.Minute)
	}
	_, err := d.db.Exec(`
		INSERT INTO payment_orders (order_no, plugin_id, union_id, platform, adapter_id, user_id, group_id, subject, amount_cents, points_amount, provider, method, status, provider_order_no, pay_url, qrcode, notify_raw, metadata, expired_at, paid_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, order.OrderNo, order.PluginID, order.UnionID, order.Platform, order.AdapterID, order.UserID, order.GroupID, order.Subject, order.AmountCents, order.PointsAmount, order.Provider, order.Method, order.Status, order.ProviderOrderNo, order.PayURL, order.QRCode, order.NotifyRaw, order.Metadata, order.ExpiredAt, order.PaidAt)
	if err != nil {
		return nil, err
	}
	return d.GetPaymentOrder(order.OrderNo)
}

func GeneratePaymentOrderNo() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("P%s%06d", time.Now().Format("20060102150405"), value.Int64()), nil
}

func (d *Database) GetPaymentOrder(orderNo string) (*PaymentOrder, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, sql.ErrNoRows
	}
	return scanPaymentOrder(d.db.QueryRow(paymentOrderSelectSQL()+` WHERE order_no = ?`, orderNo))
}

func (d *Database) DeletePaymentOrder(orderNo string) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return sql.ErrNoRows
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRow(`SELECT 1 FROM payment_orders WHERE order_no = ?`, orderNo).Scan(&exists); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM payment_events WHERE order_no = ?`, orderNo); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM payment_orders WHERE order_no = ?`, orderNo)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (d *Database) CountPendingPaymentOrders() (int64, error) {
	var total int64
	err := d.db.QueryRow(`SELECT COUNT(*) FROM payment_orders WHERE status = 'pending' AND expired_at > ?`, time.Now()).Scan(&total)
	return total, err
}

func (d *Database) CountPendingPaymentOrdersByUnionID(unionID string) (int64, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return 0, nil
	}
	var total int64
	err := d.db.QueryRow(`SELECT COUNT(*) FROM payment_orders WHERE union_id = ? AND status = 'pending' AND expired_at > ?`, unionID, time.Now()).Scan(&total)
	return total, err
}

func (d *Database) GetPaymentStatsSummary() (PaymentStatsSummary, error) {
	var summary PaymentStatsSummary
	today := time.Now().Format("2006-01-02")
	err := d.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'pending' AND expired_at > ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'expired' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' AND paid_at IS NOT NULL AND substr(paid_at, 1, 10) = ? THEN amount_cents ELSE 0 END), 0)
		FROM payment_orders
	`, time.Now(), today).Scan(&summary.TotalOrders, &summary.PendingOrders, &summary.PaidOrders, &summary.FailedOrders, &summary.ExpiredOrders, &summary.CancelledOrders, &summary.PaidAmountCents, &summary.TodayPaidAmountCents)
	return summary, err
}

func (d *Database) ListPaymentOrders(query PaymentOrderQuery) ([]*PaymentOrder, int64, error) {
	where, args := paymentOrderWhere(query)
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM payment_orders`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := normalizeLimitOffset(query.Limit, query.Offset)
	rows, err := d.db.Query(paymentOrderSelectSQL()+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*PaymentOrder{}
	for rows.Next() {
		item, err := scanPaymentOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (d *Database) UpdatePaymentOrderStatus(orderNo, status string, message string, payload interface{}) error {
	orderNo = strings.TrimSpace(orderNo)
	status = strings.TrimSpace(status)
	if err := validatePaymentOrderStatus(status); err != nil {
		return err
	}
	payloadText, err := marshalPayload(payload)
	if err != nil {
		return err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE payment_orders SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE order_no = ? AND status <> ?`, status, orderNo, status)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int
		if scanErr := tx.QueryRow(`SELECT 1 FROM payment_orders WHERE order_no = ?`, orderNo).Scan(&exists); scanErr != nil {
			return scanErr
		}
		return tx.Commit()
	}
	if err = appendPaymentEventTx(tx, orderNo, "status_"+status, message, payloadText); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) UpdatePaymentOrderProviderInfo(orderNo, providerOrderNo, payURL, qrCode, raw string) error {
	orderNo = strings.TrimSpace(orderNo)
	payloadText, err := marshalPayload(map[string]string{"provider_order_no": providerOrderNo})
	if err != nil {
		return err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE payment_orders SET provider_order_no = ?, pay_url = ?, qrcode = ?, notify_raw = ?, updated_at = CURRENT_TIMESTAMP WHERE order_no = ?`, providerOrderNo, payURL, qrCode, raw, orderNo)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	if err = appendPaymentEventTx(tx, orderNo, "provider_updated", "支付渠道信息已更新", payloadText); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) MarkPaymentOrderPaid(orderNo, providerOrderNo string, paidAt time.Time, raw string) error {
	orderNo = strings.TrimSpace(orderNo)
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE payment_orders SET status = 'paid', provider_order_no = ?, notify_raw = ?, paid_at = ?, updated_at = CURRENT_TIMESTAMP WHERE order_no = ? AND status <> 'paid'`, providerOrderNo, raw, paidAt, orderNo)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int
		if scanErr := tx.QueryRow(`SELECT 1 FROM payment_orders WHERE order_no = ?`, orderNo).Scan(&exists); scanErr != nil {
			return scanErr
		}
		return tx.Commit()
	}
	if err = appendPaymentEventTx(tx, orderNo, "paid", "订单支付成功", raw); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) AppendPaymentEvent(orderNo, eventType, message string, payload interface{}) error {
	payloadText, err := marshalPayload(payload)
	if err != nil {
		return err
	}
	return appendPaymentEventTx(d.db, orderNo, eventType, message, payloadText)
}

func (d *Database) ListPaymentEvents(orderNo string) ([]*PaymentEvent, error) {
	rows, err := d.db.Query(`SELECT id, order_no, event_type, message, payload, created_at FROM payment_events WHERE order_no = ? ORDER BY id`, strings.TrimSpace(orderNo))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*PaymentEvent{}
	for rows.Next() {
		var item PaymentEvent
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.EventType, &item.Message, &item.Payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	return items, rows.Err()
}

func (d *Database) RecordPointTransaction(tx *sql.Tx, item *PointTransaction) (*PointTransaction, error) {
	if item == nil {
		return nil, fmt.Errorf("积分流水不能为空")
	}
	item.UnionID = strings.TrimSpace(item.UnionID)
	item.Source = strings.TrimSpace(item.Source)
	if item.UnionID == "" || item.Source == "" {
		return nil, fmt.Errorf("积分流水 union_id 和 source 不能为空")
	}
	exec := interface {
		Exec(string, ...interface{}) (sql.Result, error)
	}(d.db)
	if tx != nil {
		exec = tx
	}
	result, err := exec.Exec(`INSERT INTO point_transactions (union_id, delta, balance_after, source, source_id, description, created_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, item.UnionID, item.Delta, item.BalanceAfter, item.Source, item.SourceID, item.Description)
	if err != nil {
		return nil, err
	}
	item.ID, _ = result.LastInsertId()
	return item, nil
}

func (d *Database) ListPointTransactions(query PointTransactionQuery) ([]*PointTransaction, int64, error) {
	where, args := pointTransactionWhere(query)
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM point_transactions`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := normalizeLimitOffset(query.Limit, query.Offset)
	rows, err := d.db.Query(`SELECT id, union_id, delta, balance_after, source, source_id, description, created_at FROM point_transactions`+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*PointTransaction{}
	for rows.Next() {
		var item PointTransaction
		if err := rows.Scan(&item.ID, &item.UnionID, &item.Delta, &item.BalanceAfter, &item.Source, &item.SourceID, &item.Description, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func defaultPaymentConfigJSON() string {
	data, _ := json.Marshal(DefaultPaymentSettings())
	return string(data)
}

func (d *Database) readPaymentConfigOnly() (PaymentSettings, error) {
	value, err := d.GetSetting(paymentConfigKey)
	if err == sql.ErrNoRows || strings.TrimSpace(value) == "" {
		return DefaultPaymentSettings(), nil
	}
	if err != nil {
		return PaymentSettings{}, err
	}
	var settings PaymentSettings
	if err = json.Unmarshal([]byte(value), &settings); err != nil {
		return PaymentSettings{}, err
	}
	return NormalizePaymentSettings(&settings), nil
}

func paymentOrderSelectSQL() string {
	return `SELECT id, order_no, plugin_id, union_id, platform, adapter_id, user_id, group_id, subject, amount_cents, points_amount, provider, method, status, provider_order_no, pay_url, qrcode, notify_raw, metadata, expired_at, paid_at, created_at, updated_at FROM payment_orders`
}

func scanPaymentOrder(row interface{ Scan(...interface{}) error }) (*PaymentOrder, error) {
	var item PaymentOrder
	var paidAt sql.NullTime
	if err := row.Scan(&item.ID, &item.OrderNo, &item.PluginID, &item.UnionID, &item.Platform, &item.AdapterID, &item.UserID, &item.GroupID, &item.Subject, &item.AmountCents, &item.PointsAmount, &item.Provider, &item.Method, &item.Status, &item.ProviderOrderNo, &item.PayURL, &item.QRCode, &item.NotifyRaw, &item.Metadata, &item.ExpiredAt, &paidAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	if paidAt.Valid {
		item.PaidAt = &paidAt.Time
	}
	return &item, nil
}

func paymentOrderWhere(query PaymentOrderQuery) (string, []interface{}) {
	clauses := []string{}
	args := []interface{}{}
	add := func(column, value string) {
		if strings.TrimSpace(value) != "" {
			clauses = append(clauses, column+" = ?")
			args = append(args, strings.TrimSpace(value))
		}
	}
	add("order_no", query.OrderNo)
	add("union_id", query.UnionID)
	add("plugin_id", query.PluginID)
	add("provider", query.Provider)
	add("method", query.Method)
	add("status", query.Status)
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func pointTransactionWhere(query PointTransactionQuery) (string, []interface{}) {
	clauses := []string{}
	args := []interface{}{}
	for _, item := range []struct{ column, value string }{{"union_id", query.UnionID}, {"source", query.Source}, {"source_id", query.SourceID}} {
		if strings.TrimSpace(item.value) != "" {
			clauses = append(clauses, item.column+" = ?")
			args = append(args, strings.TrimSpace(item.value))
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func normalizeLimitOffset(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func providerPaymentMetadata(metadata map[string]interface{}, remark string) (string, error) {
	payload := map[string]interface{}{}
	for key, value := range metadata {
		payload[key] = value
	}
	if strings.TrimSpace(remark) != "" {
		payload["remark"] = strings.TrimSpace(remark)
	}
	return marshalPayload(payload)
}

func validateProviderPaymentOrderTx(tx *sql.Tx, order *PaymentOrder, input ProviderPaymentConfirmation) error {
	if order == nil {
		return sql.ErrNoRows
	}
	if order.Status == "pending" && !order.ExpiredAt.IsZero() && time.Now().After(order.ExpiredAt) {
		return fmt.Errorf("订单已过期")
	}
	if input.Provider != "" && !strings.EqualFold(order.Provider, input.Provider) {
		return fmt.Errorf("订单支付渠道不匹配")
	}
	if input.Method != "" && !strings.EqualFold(order.Method, input.Method) {
		return fmt.Errorf("订单支付方式不匹配")
	}
	if order.AmountCents != input.AmountCents {
		return fmt.Errorf("订单金额不一致")
	}
	if order.Status == "paid" {
		if strings.TrimSpace(order.ProviderOrderNo) != "" && input.ProviderOrderNo != "" && order.ProviderOrderNo != input.ProviderOrderNo {
			return fmt.Errorf("第三方订单号不一致")
		}
		return nil
	}
	if input.ProviderOrderNo != "" {
		var existing string
		err := tx.QueryRow(`SELECT order_no FROM payment_orders WHERE provider = ? AND provider_order_no = ? AND order_no <> ? LIMIT 1`, order.Provider, input.ProviderOrderNo, order.OrderNo).Scan(&existing)
		if err == nil && existing != "" {
			return fmt.Errorf("第三方订单号已关联其他订单")
		}
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}
	return nil
}

func appendProviderPaymentRejectEvent(tx *sql.Tx, orderNo, message string, input ProviderPaymentConfirmation) error {
	payload, err := marshalPayload(map[string]interface{}{
		"provider":          input.Provider,
		"method":            input.Method,
		"amount_cents":      input.AmountCents,
		"provider_order_no": input.ProviderOrderNo,
		"raw":               input.Raw,
	})
	if err != nil {
		return err
	}
	return appendPaymentEventTx(tx, orderNo, "provider_rejected", message, payload)
}

func marshalPayload(payload interface{}) (string, error) {
	if payload == nil {
		return "{}", nil
	}
	if value, ok := payload.(string); ok {
		if strings.TrimSpace(value) == "" {
			return "{}", nil
		}
		return value, nil
	}
	data, err := json.Marshal(payload)
	return string(data), err
}

func appendPaymentEventTx(exec interface {
	Exec(string, ...interface{}) (sql.Result, error)
}, orderNo, eventType, message, payload string) error {
	orderNo = strings.TrimSpace(orderNo)
	eventType = strings.TrimSpace(eventType)
	if orderNo == "" || eventType == "" {
		return fmt.Errorf("订单号和事件类型不能为空")
	}
	if strings.TrimSpace(payload) == "" {
		payload = "{}"
	}
	_, err := exec.Exec(`INSERT INTO payment_events (order_no, event_type, message, payload, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, orderNo, eventType, message, payload)
	return err
}
