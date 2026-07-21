package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

const (
	defaultWaitTimeout   = 300
	providerPoints       = "points"
	providerAlipayBill   = "alipay_bill"
	methodPoints         = "points"
	methodAlipayTransfer = "alipay_transfer"
)

var amountPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?$`)

type WaitPayRequest struct {
	PluginID    string
	Platform    string
	AdapterID   string
	UserID      string
	GroupID     string
	UnionID     string
	Subject     string
	AmountRaw   json.RawMessage
	Timeout     int
	PointsUnit  string
	Methods     []string
	Metadata    map[string]interface{}
	Remark      string
	PromptTitle string
	NotifyURL   string
	ReturnURL   string
}

type Interaction struct {
	Reply        func(string) error
	ReplyButtons func(string, [][]types.ButtonOption) error
	SendImage    func(string) error
	SendRich     func(types.RichMessage) error
	Listen       func(timeout int) string
	ListenUntil  func(timeout int, done <-chan struct{}) string
}

type PaymentResult struct {
	Status          string `json:"status"`
	OrderNo         string `json:"order_no"`
	Provider        string `json:"provider"`
	Method          string `json:"method"`
	Subject         string `json:"subject"`
	AmountCents     int64  `json:"amount_cents"`
	PointsAmount    int64  `json:"points_amount"`
	PointsBalance   int64  `json:"points_balance"`
	PayURL          string `json:"pay_url"`
	QRCode          string `json:"qrcode"`
	ProviderOrderNo string `json:"provider_order_no"`
	Message         string `json:"message"`
}

type Service struct {
	database *config.Database
}

func NewService(database *config.Database) *Service {
	return &Service{database: database}
}

func ParseRMBToCents(raw json.RawMessage) (int64, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return 0, fmt.Errorf("支付金额不能为空")
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, fmt.Errorf("支付金额格式无效")
		}
		text = strings.TrimSpace(value)
	} else {
		var value interface{}
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, fmt.Errorf("支付金额格式无效")
		}
		switch value.(type) {
		case float64:
			text = strings.TrimSpace(text)
		default:
			return 0, fmt.Errorf("支付金额必须是数字或字符串")
		}
	}
	if text == "" || strings.ContainsAny(text, "eE+-") || !amountPattern.MatchString(text) {
		return 0, fmt.Errorf("支付金额格式无效")
	}
	parts := strings.Split(text, ".")
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("支付金额格式无效")
	}
	cents := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("支付金额格式无效")
		}
	}
	if yuan > (int64(^uint64(0)>>1)-cents)/100 {
		return 0, fmt.Errorf("支付金额过大")
	}
	amount := yuan*100 + cents
	if amount <= 0 {
		return 0, fmt.Errorf("支付金额必须大于 0")
	}
	return amount, nil
}

func EnabledPointMethods(settings *config.PaymentSettings) []config.PaymentMethodSetting {
	normalized := config.NormalizePaymentSettings(settings)
	methods := make([]config.PaymentMethodSetting, 0)
	for _, method := range normalized.Methods {
		if method.Enabled && strings.EqualFold(strings.TrimSpace(method.Provider), providerPoints) {
			methods = append(methods, method)
		}
	}
	return methods
}

func EnabledMethods(settings *config.PaymentSettings, requestedMethods []string) []config.PaymentMethodSetting {
	normalized := config.NormalizePaymentSettings(settings)
	requested := map[string]bool{}
	for _, code := range requestedMethods {
		code = strings.TrimSpace(code)
		if code != "" {
			requested[strings.ToLower(code)] = true
		}
	}
	methods := make([]config.PaymentMethodSetting, 0)
	for _, method := range normalized.Methods {
		if !method.Enabled || strings.TrimSpace(method.Code) == "" {
			continue
		}
		if len(requested) > 0 && !requested[strings.ToLower(strings.TrimSpace(method.Code))] {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(method.Provider))
		switch provider {
		case providerPoints:
			methods = append(methods, method)
		case "epay":
			if epayAvailable(normalized) {
				methods = append(methods, method)
			}
		case providerAlipayBill:
			if alipayBillAvailable(normalized) {
				methods = append(methods, method)
			}
		}
	}
	return methods
}

func epayAvailable(settings config.PaymentSettings) bool {
	if !settings.ThirdPartyEnabled || !settings.Epay.Enabled {
		return false
	}
	if strings.TrimSpace(settings.Epay.APIURL) == "" || strings.TrimSpace(settings.Epay.PID) == "" {
		return false
	}
	if strings.EqualFold(settings.Epay.Version, "v2") {
		return strings.TrimSpace(settings.Epay.PlatformPublicKey) != "" && strings.TrimSpace(settings.Epay.MerchantPrivateKey) != ""
	}
	return strings.TrimSpace(settings.Epay.Key) != ""
}

func alipayBillAvailable(settings config.PaymentSettings) bool {
	if !settings.ThirdPartyEnabled || !settings.AlipayBill.Enabled {
		return false
	}
	return strings.TrimSpace(settings.AlipayBill.GatewayURL) != "" && strings.TrimSpace(settings.AlipayBill.AppID) != "" && strings.TrimSpace(settings.AlipayBill.PrivateKey) != "" && strings.TrimSpace(settings.AlipayBill.AlipayPublicKey) != "" && (strings.TrimSpace(settings.AlipayBill.TransferUserID) != "" || strings.TrimSpace(settings.AlipayBill.ReceiptQRURL) != "")
}

func (s *Service) WaitPay(req WaitPayRequest, io Interaction) (PaymentResult, error) {
	if s == nil || s.database == nil {
		return PaymentResult{Status: "failed", Message: "数据库不可用"}, fmt.Errorf("数据库不可用")
	}
	req.UnionID = strings.TrimSpace(req.UnionID)
	req.Subject = strings.TrimSpace(req.Subject)
	if req.UnionID == "" {
		return PaymentResult{Status: "failed", Message: "用户 union_id 不能为空"}, fmt.Errorf("用户 union_id 不能为空")
	}
	if req.Subject == "" {
		return PaymentResult{Status: "failed", Message: "支付标题不能为空"}, fmt.Errorf("支付标题不能为空")
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultWaitTimeout
	}
	amountCents, err := ParseRMBToCents(req.AmountRaw)
	if err != nil {
		return PaymentResult{Status: "failed", Subject: req.Subject, Message: err.Error()}, err
	}
	settings, err := s.database.GetPaymentSettings()
	if err != nil {
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, Message: err.Error()}, err
	}
	settingsValue := config.NormalizePaymentSettings(settings)
	if amountCents > settingsValue.MaxPaymentAmountCents {
		message := fmt.Sprintf("单笔支付金额不能超过 %s %s", formatAmount(settingsValue.MaxPaymentAmountCents), paymentCurrencyUnit(settingsValue))
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, Message: message}, errors.New(message)
	}
	if err = s.ensureUserPendingPaymentCapacity(req.UnionID); err != nil {
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, Message: err.Error()}, err
	}
	methods := EnabledMethods(&settingsValue, req.Methods)
	if len(methods) == 0 {
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, Message: "没有可用的支付方式"}, fmt.Errorf("没有可用的支付方式")
	}
	pointsAmount, err := config.CalculatePointsAmount(amountCents, settingsValue.PointsPerRMB)
	if err != nil {
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, Message: err.Error()}, err
	}
	currencyUnit := paymentCurrencyUnit(settingsValue)
	pointsBalance, err := s.database.GetUserPoints(req.UnionID)
	if err != nil {
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	prompt := paymentPrompt(req.PromptTitle, amountCents, pointsAmount, currencyUnit, req.PointsUnit, methods, req.Timeout, pointsBalance)
	if io.ReplyButtons != nil {
		if err = io.ReplyButtons(prompt, paymentMethodButtons(methods, req.PointsUnit, pointsBalance, req.UserID)); err != nil {
			return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
		}
	} else if io.Reply != nil {
		if err = io.Reply(prompt); err != nil {
			return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
		}
	}
	choice := ""
	if io.Listen != nil {
		choice = strings.TrimSpace(io.Listen(req.Timeout))
	}
	if choice == "" {
		return PaymentResult{Status: "expired", Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: "支付超时"}, nil
	}
	if isCancelChoice(choice) {
		return PaymentResult{Status: "cancelled", Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: "支付已取消"}, nil
	}
	method, ok := selectedPaymentMethod(choice, methods)
	if !ok {
		return PaymentResult{Status: "failed", Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: "支付方式无效"}, fmt.Errorf("支付方式无效")
	}
	provider := strings.ToLower(strings.TrimSpace(method.Provider))
	if provider == providerPoints {
		return s.settlePoints(req, method, amountCents, pointsAmount)
	}
	if provider == "epay" {
		return s.waitEpay(req, settingsValue, method, amountCents, pointsAmount, io)
	}
	if provider == providerAlipayBill {
		return s.waitAlipayBill(req, settingsValue, method, amountCents, pointsAmount, io)
	}
	message := "不支持的支付渠道: " + method.Provider
	return PaymentResult{Status: "failed", Provider: method.Provider, Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: message}, fmt.Errorf("%s", message)
}

func (s *Service) settlePoints(req WaitPayRequest, method config.PaymentMethodSetting, amountCents, pointsAmount int64) (PaymentResult, error) {
	settled, err := s.database.SettlePointsPayment(config.PointsPaymentSettlement{
		PluginID:     req.PluginID,
		UnionID:      req.UnionID,
		Platform:     req.Platform,
		AdapterID:    req.AdapterID,
		UserID:       req.UserID,
		GroupID:      req.GroupID,
		Subject:      req.Subject,
		AmountCents:  amountCents,
		PointsAmount: pointsAmount,
		Provider:     providerPoints,
		Method:       stringDefault(method.Code, methodPoints),
		Metadata:     req.Metadata,
		ExpiredAt:    time.Now().Add(time.Duration(req.Timeout) * time.Second),
	})
	result := PaymentResult{Status: "failed", Provider: providerPoints, Method: stringDefault(method.Code, methodPoints), Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: "支付失败"}
	if settled != nil {
		result.OrderNo = settled.OrderNo
		result.Status = settled.Status
		result.PointsBalance = settled.PointsBalance
		result.Message = settled.Message
	}
	return result, err
}

func (s *Service) waitEpay(req WaitPayRequest, settings config.PaymentSettings, method config.PaymentMethodSetting, amountCents, pointsAmount int64, io Interaction) (result PaymentResult, err error) {
	if err := s.ensurePendingPaymentCapacity(settings, req.UnionID); err != nil {
		return PaymentResult{Status: "failed", Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	notifyURL := strings.TrimSpace(req.NotifyURL)
	returnURL := strings.TrimSpace(req.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(settings.Epay.ReturnURL)
	}
	if notifyURL == "" {
		notifyURL = deriveEpayNotifyURL(returnURL)
	}
	if notifyURL == "" || returnURL == "" {
		message := "请先配置易支付 Return URL，系统将据此推导 Notify URL"
		return PaymentResult{Status: "failed", Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: message}, fmt.Errorf("%s", message)
	}
	provider, err := NewEpayProvider(settings.Epay, nil)
	if err != nil {
		return PaymentResult{Status: "failed", Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	order, err := s.database.CreateProviderPaymentOrder(config.ProviderPaymentOrderInput{PluginID: req.PluginID, UnionID: req.UnionID, Platform: req.Platform, AdapterID: req.AdapterID, UserID: req.UserID, GroupID: req.GroupID, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Provider: "epay", Method: method.Code, Metadata: req.Metadata, Remark: req.Remark, ExpiredAt: time.Now().Add(time.Duration(req.Timeout) * time.Second)})
	if err != nil {
		return PaymentResult{Status: "failed", Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	ch, cancel := DefaultWaitHub.Register(order.OrderNo)
	defer cancel()
	providerOrder, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: order.OrderNo, Subject: epaySubmitSubject(settings, req.Subject), AmountCents: amountCents, Method: method.Code, NotifyURL: notifyURL, ReturnURL: returnURL})
	if err != nil {
		_ = s.database.UpdatePaymentOrderStatus(order.OrderNo, "failed", "第三方下单失败", map[string]string{"error": err.Error()})
		return PaymentResult{Status: "failed", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	if err = s.database.UpdatePaymentOrderProviderInfo(order.OrderNo, providerOrder.ProviderOrderNo, providerOrder.PayURL, providerOrder.QRCode, providerOrder.Raw); err != nil {
		return PaymentResult{Status: "failed", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	initial := PaymentResult{Status: "pending", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, ProviderOrderNo: providerOrder.ProviderOrderNo, Message: "等待支付"}
	baseURL := paymentQRCodePublicBaseURL(settings, returnURL)
	if err = sendEpayPaymentInfo(baseURL, initial, method, settings.HidePayURL, paymentCurrencyUnit(settings), io); err != nil {
		log.Printf("[PAYMENT] Send epay qrcode image failed: order=%s err=%v", order.OrderNo, err)
		if settings.HidePayURL {
			message := "二维码图片发送失败，请稍后重试"
			_ = s.database.UpdatePaymentOrderStatus(order.OrderNo, "failed", message, map[string]string{"error": err.Error()})
			if io.Reply != nil {
				_ = io.Reply(message)
			}
			return PaymentResult{Status: "failed", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, ProviderOrderNo: providerOrder.ProviderOrderNo, Message: message}, err
		}
	}
	done := make(chan struct{})
	defer close(done)
	cancelInput := listenForPaymentCancel(io, req.Timeout, done)
	queryResult := s.pollEpayOrder(provider, order.OrderNo, providerOrder.ProviderOrderNo, settings.EpayQueryIntervalSeconds, done)
	timeout := time.After(time.Duration(req.Timeout) * time.Second)
	for {
		select {
		case result, ok := <-ch:
			if !ok {
				return PaymentResult{Status: "expired", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: "支付超时"}, nil
			}
			return result, nil
		case query, ok := <-queryResult:
			if !ok {
				queryResult = nil
				continue
			}
			confirmed, confirmErr := s.confirmPolledEpayOrder(order.OrderNo, query)
			if confirmErr != nil {
				log.Printf("[PAYMENT] Auto query confirm failed: order=%s err=%v", order.OrderNo, confirmErr)
				continue
			}
			DefaultWaitHub.Resolve(order.OrderNo, PaymentResult{Status: "paid", OrderNo: confirmed.OrderNo, Provider: confirmed.Provider, Method: confirmed.Method, Subject: confirmed.Subject, AmountCents: confirmed.AmountCents, PointsAmount: confirmed.PointsAmount, PayURL: confirmed.PayURL, QRCode: confirmed.QRCode, ProviderOrderNo: confirmed.ProviderOrderNo, Message: "支付成功"})
		case choice, ok := <-cancelInput:
			if !ok {
				cancelInput = nil
				continue
			}
			if isCancelChoice(choice) {
				message := "支付已取消"
				_ = s.database.UpdatePaymentOrderStatus(order.OrderNo, "cancelled", message, nil)
				return PaymentResult{Status: "cancelled", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, ProviderOrderNo: providerOrder.ProviderOrderNo, Message: message}, nil
			}
			if strings.TrimSpace(choice) != "" {
				cancelInput = listenForPaymentCancel(io, req.Timeout, done)
			} else {
				cancelInput = nil
			}
		case <-timeout:
			_ = s.database.ExpirePaymentOrder(order.OrderNo, "支付超时")
			return PaymentResult{Status: "expired", OrderNo: order.OrderNo, Provider: "epay", Method: method.Code, Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, ProviderOrderNo: providerOrder.ProviderOrderNo, Message: "支付超时"}, nil
		}
	}
}

func (s *Service) waitAlipayBill(req WaitPayRequest, settings config.PaymentSettings, method config.PaymentMethodSetting, amountCents, pointsAmount int64, io Interaction) (result PaymentResult, err error) {
	if err := s.ensurePendingPaymentCapacity(settings, req.UnionID); err != nil {
		return PaymentResult{Status: "failed", Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	provider, err := NewAlipayBillProvider(settings.AlipayBill, nil)
	if err != nil {
		return PaymentResult{Status: "failed", Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	timeoutSeconds := alipayBillOrderTimeout(settings, req.Timeout)
	expiredAt := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	payableAmountCents, err := s.nextAlipayBillPayableAmount(amountCents)
	if err != nil {
		return PaymentResult{Status: "failed", Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	metadata := map[string]interface{}{}
	for key, value := range req.Metadata {
		metadata[key] = value
	}
	metadata["match_mode"] = alipayBillMatchMode
	metadata["requested_amount_cents"] = amountCents
	metadata["payable_amount_cents"] = payableAmountCents
	metadata["amount_offset_cents"] = payableAmountCents - amountCents
	order, err := s.database.CreateProviderPaymentOrder(config.ProviderPaymentOrderInput{PluginID: req.PluginID, UnionID: req.UnionID, Platform: req.Platform, AdapterID: req.AdapterID, UserID: req.UserID, GroupID: req.GroupID, Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Metadata: metadata, Remark: req.Remark, ExpiredAt: expiredAt})
	if err != nil {
		return PaymentResult{Status: "failed", Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: amountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	providerOrder, err := provider.CreateOrder(ProviderCreateRequest{OrderNo: order.OrderNo, CashierToken: order.CashierToken, Subject: req.Subject, AmountCents: payableAmountCents, Method: alipayBillMethodCode(method)})
	if err != nil {
		_ = s.database.UpdatePaymentOrderStatus(order.OrderNo, "failed", "支付宝账单下单失败", map[string]string{"error": err.Error()})
		return PaymentResult{Status: "failed", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	if err = s.database.UpdatePaymentOrderProviderInfo(order.OrderNo, providerOrder.ProviderOrderNo, providerOrder.PayURL, providerOrder.QRCode, providerOrder.Raw); err != nil {
		return PaymentResult{Status: "failed", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, Message: err.Error()}, err
	}
	ch, cancel := DefaultWaitHub.Register(order.OrderNo)
	defer cancel()
	initial := PaymentResult{Status: "pending", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, ProviderOrderNo: providerOrder.ProviderOrderNo, Message: "等待支付宝转账"}
	baseURL := paymentQRCodePublicBaseURL(settings, "")
	if err = sendAlipayBillPaymentInfo(baseURL, initial, method, settings.HidePayURL, paymentCurrencyUnit(settings), io); err != nil {
		log.Printf("[PAYMENT] Send alipay bill qrcode image failed: order=%s err=%v", order.OrderNo, err)
		if settings.HidePayURL {
			message := "二维码图片发送失败，请稍后重试"
			_ = s.database.UpdatePaymentOrderStatus(order.OrderNo, "failed", message, map[string]string{"error": err.Error()})
			if io.Reply != nil {
				_ = io.Reply(message)
			}
			return PaymentResult{Status: "failed", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, Message: message}, err
		}
	}
	done := make(chan struct{})
	defer close(done)
	cancelInput := listenForPaymentCancel(io, timeoutSeconds, done)
	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)
	for {
		select {
		case result, ok := <-ch:
			if !ok {
				return PaymentResult{Status: "expired", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, Message: "支付超时"}, nil
			}
			return result, nil
		case choice, ok := <-cancelInput:
			if !ok {
				cancelInput = nil
				continue
			}
			if isCancelChoice(choice) {
				message := "支付已取消"
				_ = s.database.UpdatePaymentOrderStatus(order.OrderNo, "cancelled", message, nil)
				return PaymentResult{Status: "cancelled", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, Message: message}, nil
			}
			if strings.TrimSpace(choice) != "" {
				cancelInput = listenForPaymentCancel(io, timeoutSeconds, done)
			} else {
				cancelInput = nil
			}
		case <-timeout:
			_ = s.database.ExpirePaymentOrder(order.OrderNo, "支付超时")
			return PaymentResult{Status: "expired", OrderNo: order.OrderNo, Provider: providerAlipayBill, Method: alipayBillMethodCode(method), Subject: req.Subject, AmountCents: payableAmountCents, PointsAmount: pointsAmount, PayURL: providerOrder.PayURL, QRCode: providerOrder.QRCode, Message: "支付超时"}, nil
		}
	}
}

func (s *Service) pollEpayOrder(provider PaymentProvider, orderNo, providerOrderNo string, intervalSeconds int, done <-chan struct{}) <-chan *ProviderQueryResult {
	ch := make(chan *ProviderQueryResult, 1)
	if provider == nil {
		close(ch)
		return ch
	}
	if intervalSeconds <= 0 {
		intervalSeconds = config.DefaultPaymentSettings().EpayQueryIntervalSeconds
	}
	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				query, err := provider.QueryOrder(orderNo, providerOrderNo)
				if err != nil {
					log.Printf("[PAYMENT] Auto query epay order failed: order=%s err=%v", orderNo, err)
					continue
				}
				if query != nil && query.Status == "paid" {
					select {
					case ch <- query:
					case <-done:
					}
					return
				}
			}
		}
	}()
	return ch
}

func (s *Service) confirmPolledEpayOrder(orderNo string, query *ProviderQueryResult) (*config.PaymentOrder, error) {
	if query == nil || query.Status != "paid" {
		return nil, fmt.Errorf("查询结果未支付")
	}
	if strings.TrimSpace(query.OrderNo) != "" && strings.TrimSpace(query.OrderNo) != strings.TrimSpace(orderNo) {
		return nil, fmt.Errorf("查询订单号不一致")
	}
	if query.AmountCents <= 0 || strings.TrimSpace(query.Method) == "" {
		return nil, fmt.Errorf("查询结果缺少金额或支付方式")
	}
	paidAt := query.PaidAt
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	confirmed, _, err := s.database.ConfirmProviderPayment(config.ProviderPaymentConfirmation{OrderNo: orderNo, Provider: "epay", Method: query.Method, AmountCents: query.AmountCents, ProviderOrderNo: query.ProviderOrderNo, Raw: query.Raw, PaidAt: paidAt})
	return confirmed, err
}

func listenForPaymentCancel(io Interaction, timeout int, done <-chan struct{}) <-chan string {
	ch := make(chan string, 1)
	if io.ListenUntil == nil && io.Listen == nil {
		close(ch)
		return ch
	}
	go func() {
		defer close(ch)
		choice := ""
		if io.ListenUntil != nil {
			choice = io.ListenUntil(timeout, done)
		} else {
			choice = io.Listen(timeout)
		}
		select {
		case ch <- strings.TrimSpace(choice):
		case <-done:
		}
	}()
	return ch
}

func (s *Service) ensureUserPendingPaymentCapacity(unionID string) error {
	userCount, err := s.database.CountPendingPaymentOrdersByUnionID(unionID)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return fmt.Errorf("你已有待支付订单，请先完成支付或发送 q 取消后再试")
	}
	return nil
}

func (s *Service) nextAlipayBillPayableAmount(amountCents int64) (int64, error) {
	orders, err := s.database.ListPendingProviderPaymentOrders(providerAlipayBill, 1000)
	if err != nil {
		return 0, err
	}
	used := map[int64]bool{}
	now := time.Now()
	for _, order := range orders {
		if order != nil && order.AmountCents > 0 && order.ExpiredAt.After(now) {
			used[order.AmountCents] = true
		}
	}
	payableAmountCents := amountCents
	for used[payableAmountCents] {
		payableAmountCents++
	}
	return payableAmountCents, nil
}

func (s *Service) ensurePendingPaymentCapacity(settings config.PaymentSettings, unionID string) error {
	if err := s.ensureUserPendingPaymentCapacity(unionID); err != nil {
		return err
	}
	limit := settings.MaxPendingPayments
	if limit <= 0 {
		limit = config.DefaultPaymentSettings().MaxPendingPayments
	}
	count, err := s.database.CountPendingPaymentOrders()
	if err != nil {
		return err
	}
	if count >= int64(limit) {
		return fmt.Errorf("当前待支付订单较多，请等待其他用户支付完成后再试")
	}
	return nil
}

func paymentPrompt(promptTitle string, amountCents, pointsAmount int64, currencyUnit, pointsUnit string, methods []config.PaymentMethodSetting, timeout int, pointsBalance int64) string {
	currencyUnit = strings.TrimSpace(currencyUnit)
	if currencyUnit == "" {
		currencyUnit = "RMB"
	}
	pointsUnit = strings.TrimSpace(pointsUnit)
	if pointsUnit == "" {
		pointsUnit = "积分"
	}
	promptTitle = strings.TrimSpace(promptTitle)
	if promptTitle == "" {
		promptTitle = fmt.Sprintf("当前消费 %s %s（%d %s）", formatAmount(amountCents), currencyUnit, pointsAmount, pointsUnit)
	}
	lines := []string{
		promptTitle,
		"请选择支付方式",
	}
	for i, method := range methods {
		label := strings.TrimSpace(method.Label)
		if label == "" {
			label = strings.TrimSpace(method.Code)
		}
		if strings.EqualFold(strings.TrimSpace(method.Provider), providerPoints) {
			label = fmt.Sprintf("%s（剩余%s：%d）", label, pointsUnit, pointsBalance)
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, label))
	}
	lines = append(lines, "", fmt.Sprintf("PS：发送对应数字进行选择，发送 q 可取消，订单将在 %d 秒后超时。", timeout))
	return strings.Join(lines, "\n")
}

func paymentMethodButtons(methods []config.PaymentMethodSetting, pointsUnit string, pointsBalance int64, userID string) [][]types.ButtonOption {
	pointsUnit = strings.TrimSpace(pointsUnit)
	if pointsUnit == "" {
		pointsUnit = "积分"
	}
	rows := make([][]types.ButtonOption, 0, len(methods)+1)
	for i, method := range methods {
		label := strings.TrimSpace(method.Label)
		if label == "" {
			label = strings.TrimSpace(method.Code)
		}
		if strings.EqualFold(strings.TrimSpace(method.Provider), providerPoints) {
			label = fmt.Sprintf("%s（剩余%s：%d）", label, pointsUnit, pointsBalance)
		}
		rows = append(rows, []types.ButtonOption{{Text: label, Value: fmt.Sprintf("%d", i+1), UserID: userID}})
	}
	rows = append(rows, []types.ButtonOption{{Text: "取消", Value: "q", UserID: userID}})
	return rows
}

func alipayBillOrderTimeout(settings config.PaymentSettings, fallback int) int {
	if settings.AlipayBill.OrderTimeoutSeconds > 0 {
		return settings.AlipayBill.OrderTimeoutSeconds
	}
	if fallback > 0 {
		return fallback
	}
	return defaultWaitTimeout
}

func epayPaymentMessage(result PaymentResult, method config.PaymentMethodSetting, hidePayURL bool, currencyUnit string) string {
	label := strings.TrimSpace(method.Label)
	if label == "" {
		label = method.Code
	}
	currencyUnit = strings.TrimSpace(currencyUnit)
	if currencyUnit == "" {
		currencyUnit = "RMB"
	}
	lines := []string{
		fmt.Sprintf("订单号：%s", result.OrderNo),
		fmt.Sprintf("支付方式：%s", label),
		fmt.Sprintf("支付金额：%s %s", formatAmount(result.AmountCents), currencyUnit),
	}
	if !hidePayURL && strings.TrimSpace(result.PayURL) != "" {
		lines = append(lines, "支付链接："+strings.TrimSpace(result.PayURL))
	}
	lines = append(lines, "请完成支付，系统会自动确认。")
	return strings.Join(lines, "\n")
}

func alipayBillPaymentMessage(result PaymentResult, method config.PaymentMethodSetting, hidePayURL bool, currencyUnit string) string {
	message := epayPaymentMessage(result, method, hidePayURL, currencyUnit)
	lines := []string{message, "请按上方支付金额精确付款，系统会按账单入账金额自动确认。"}
	return strings.Join(lines, "\n")
}

func sendAlipayBillPaymentInfo(baseURL string, result PaymentResult, method config.PaymentMethodSetting, hidePayURL bool, currencyUnit string, io Interaction) error {
	message := alipayBillPaymentMessage(result, method, hidePayURL, currencyUnit)
	imageURL, err := epayQRCodeImageURL(baseURL, result)
	if err != nil {
		if io.Reply != nil {
			_ = io.Reply(message)
		}
		return err
	}
	if io.SendRich != nil {
		if err := io.SendRich(types.RichMessage{Parts: []types.RichMessagePart{{Type: "text", Text: message + "\n"}, {Type: "image", URL: imageURL, Alt: "支付宝转账二维码"}}, FallbackText: message, Prefer: "auto"}); err == nil {
			return nil
		}
	}
	if io.Reply != nil {
		_ = io.Reply(message)
	}
	if io.SendImage == nil {
		return fmt.Errorf("适配器不支持发送图片")
	}
	return io.SendImage(imageURL)
}

func sendEpayPaymentInfo(baseURL string, result PaymentResult, method config.PaymentMethodSetting, hidePayURL bool, currencyUnit string, io Interaction) error {
	message := epayPaymentMessage(result, method, hidePayURL, currencyUnit)
	imageURL, err := epayQRCodeImageURL(baseURL, result)
	if err != nil {
		if io.Reply != nil {
			_ = io.Reply(message)
		}
		return err
	}
	if io.SendRich != nil {
		if err := io.SendRich(types.RichMessage{Parts: []types.RichMessagePart{{Type: "text", Text: message + "\n"}, {Type: "image", URL: imageURL, Alt: "支付二维码"}}, FallbackText: message, Prefer: "auto"}); err == nil {
			return nil
		}
	}
	if io.Reply != nil {
		_ = io.Reply(message)
	}
	if io.SendImage == nil {
		return fmt.Errorf("适配器不支持发送图片")
	}
	return io.SendImage(imageURL)
}

func epayQRCodeImageURL(baseURL string, result PaymentResult) (string, error) {
	content := strings.TrimSpace(result.QRCode)
	if content == "" {
		content = strings.TrimSpace(result.PayURL)
	}
	imageURL := PaymentQRCodeURL(baseURL, result.OrderNo, content)
	if imageURL == "" {
		return "", fmt.Errorf("二维码图片地址生成失败")
	}
	return imageURL, nil
}

func sendEpayQRCodeImage(baseURL string, result PaymentResult, io Interaction) error {
	imageURL, err := epayQRCodeImageURL(baseURL, result)
	if err != nil {
		return err
	}
	if io.SendImage == nil {
		return fmt.Errorf("适配器不支持发送图片")
	}
	return io.SendImage(imageURL)
}

func epaySubmitSubject(settings config.PaymentSettings, fallback string) string {
	if subject := strings.TrimSpace(settings.EpaySubmitSubject); subject != "" {
		return subject
	}
	return strings.TrimSpace(fallback)
}

func paymentCurrencyUnit(settings config.PaymentSettings) string {
	unit := strings.TrimSpace(settings.CurrencyUnit)
	if unit == "" {
		return "RMB"
	}
	return unit
}

func isCancelChoice(choice string) bool {
	choice = strings.ToLower(strings.TrimSpace(choice))
	return choice == "q" || choice == "quit" || choice == "exit" || choice == "取消"
}

func PaymentQRCodeContent(order *config.PaymentOrder) string {
	if order == nil {
		return ""
	}
	if content := strings.TrimSpace(order.QRCode); content != "" {
		return content
	}
	return strings.TrimSpace(order.PayURL)
}

func PaymentQRCodeToken(orderNo, content string) string {
	orderNo = strings.TrimSpace(orderNo)
	content = strings.TrimSpace(content)
	if orderNo == "" || content == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(orderNo + "\n" + content))
	return hex.EncodeToString(digest[:])[:32]
}

func PaymentQRCodeURL(baseURL, orderNo, content string) string {
	token := PaymentQRCodeToken(orderNo, content)
	if token == "" {
		return ""
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	if strings.Contains(baseURL, "{content}") {
		return strings.ReplaceAll(baseURL, "{content}", url.QueryEscape(strings.TrimSpace(content)))
	}
	if strings.HasSuffix(baseURL, "=") {
		return baseURL + url.QueryEscape(strings.TrimSpace(content))
	}
	path := "/api/open/payments/qrcode/" + strings.TrimSpace(orderNo) + "/" + token + ".png"
	if strings.HasPrefix(baseURL, "/") {
		return strings.TrimRight(baseURL, "/") + path
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func paymentQRCodePublicBaseURL(settings config.PaymentSettings, returnURL string) string {
	if baseURL := strings.TrimSpace(settings.QRCodeBaseURL); baseURL != "" {
		return baseURL
	}
	return strings.TrimSpace(returnURL)
}

func deriveEpayNotifyURL(returnURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(returnURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = "/api/open/payments/notify/epay"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func formatAmount(amountCents int64) string {
	return fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
}

func selectedPaymentMethod(choice string, methods []config.PaymentMethodSetting) (config.PaymentMethodSetting, bool) {
	choice = strings.TrimSpace(choice)
	for i, method := range methods {
		if choice == fmt.Sprintf("%d", i+1) || strings.EqualFold(choice, strings.TrimSpace(method.Code)) || choice == strings.TrimSpace(method.Label) || strings.EqualFold(choice, strings.TrimSpace(method.Provider)) {
			return method, true
		}
	}
	return config.PaymentMethodSetting{}, false
}

func isPointChoice(choice string, methods []config.PaymentMethodSetting) bool {
	method, ok := selectedPaymentMethod(choice, methods)
	return ok && strings.EqualFold(strings.TrimSpace(method.Provider), providerPoints)
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
