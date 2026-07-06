package payment

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

const (
	alipayBillMethodTransfer = "alipay_transfer"
	alipayBillMatchMode      = "amount_unique"
)

type AlipayBillProvider struct {
	settings config.AlipayBillSettings
	client   *http.Client
}

type alipayBillItem struct {
	ProviderOrderNo string    `json:"provider_order_no"`
	AccountLogID    string    `json:"account_log_id"`
	AmountCents     int64     `json:"amount_cents"`
	Direction       string    `json:"direction"`
	Remark          string    `json:"remark"`
	Summary         string    `json:"summary"`
	OppositeAccount string    `json:"opposite_account"`
	PaidAt          time.Time `json:"paid_at"`
	Raw             string    `json:"raw"`
}

func NewAlipayBillProvider(settings config.AlipayBillSettings, client *http.Client) (*AlipayBillProvider, error) {
	settings = config.NormalizePaymentSettings(&config.PaymentSettings{AlipayBill: settings}).AlipayBill
	if err := validateAlipayBillRuntimeSettings(settings); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &AlipayBillProvider{settings: settings, client: client}, nil
}

func validateAlipayBillRuntimeSettings(settings config.AlipayBillSettings) error {
	if strings.TrimSpace(settings.GatewayURL) == "" || strings.TrimSpace(settings.AppID) == "" {
		return fmt.Errorf("支付宝网关地址和 app_id 不能为空")
	}
	if strings.TrimSpace(settings.PrivateKey) == "" || strings.TrimSpace(settings.AlipayPublicKey) == "" {
		return fmt.Errorf("支付宝应用私钥和支付宝公钥不能为空")
	}
	if strings.TrimSpace(settings.TransferUserID) == "" && strings.TrimSpace(settings.ReceiptQRURL) == "" {
		return fmt.Errorf("支付宝收款 UID 或收款码地址至少填写一项")
	}
	return nil
}

func (p *AlipayBillProvider) CreateOrder(req ProviderCreateRequest) (*ProviderOrder, error) {
	req.OrderNo = strings.TrimSpace(req.OrderNo)
	if req.OrderNo == "" {
		return nil, fmt.Errorf("订单号不能为空")
	}
	if req.AmountCents <= 0 {
		return nil, fmt.Errorf("支付金额必须大于 0")
	}
	payURL, mode, err := p.buildPaymentURL(req)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"payment_url_mode":   mode,
		"receipt_qr_url":     strings.TrimSpace(p.settings.ReceiptQRURL),
		"cashier_base_url":   strings.TrimSpace(p.settings.CashierBaseURL),
		"transfer_user_id":   strings.TrimSpace(p.settings.TransferUserID),
		"transfer_user_name": strings.TrimSpace(p.settings.TransferUserName),
		"amount":             formatCents(req.AmountCents),
		"remark":             req.OrderNo,
		"match_mode":         alipayBillMatchMode,
		"generated_at":       time.Now().Format(time.RFC3339),
	})
	return &ProviderOrder{OrderNo: req.OrderNo, ProviderOrderNo: "", PayURL: payURL, QRCode: payURL, Raw: string(raw)}, nil
}

func (p *AlipayBillProvider) VerifyNotify(r *http.Request) (*ProviderNotifyResult, error) {
	return nil, fmt.Errorf("支付宝账单通道不支持异步回调")
}

func (p *AlipayBillProvider) QueryOrder(orderNo, providerOrderNo string) (*ProviderQueryResult, error) {
	return p.QueryOrderWithAmount(orderNo, providerOrderNo, 0)
}

func (p *AlipayBillProvider) QueryOrderWithAmount(orderNo, providerOrderNo string, amountCents int64) (*ProviderQueryResult, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, fmt.Errorf("订单号不能为空")
	}
	items, raw, err := p.QueryBills(time.Now().Add(-time.Duration(p.settings.QueryMinutesBack)*time.Minute), time.Now())
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !alipayBillItemIsIncome(item) || !alipayBillItemMatchesOrder(item, orderNo, amountCents) {
			continue
		}
		return &ProviderQueryResult{OrderNo: orderNo, ProviderOrderNo: item.ProviderOrderNo, Method: alipayBillMethodTransfer, AmountCents: item.AmountCents, Status: "paid", Raw: item.Raw, PaidAt: item.PaidAt}, nil
	}
	return &ProviderQueryResult{OrderNo: orderNo, ProviderOrderNo: providerOrderNo, Method: alipayBillMethodTransfer, Status: "pending", Raw: raw}, nil
}

func (p *AlipayBillProvider) QueryBills(start, end time.Time) ([]alipayBillItem, string, error) {
	bizContent, _ := json.Marshal(map[string]interface{}{
		"start_time": start.Format("2006-01-02 15:04:05"),
		"end_time":   end.Format("2006-01-02 15:04:05"),
		"page_no":    1,
		"page_size":  p.settings.BillPageSize,
	})
	params := map[string]string{
		"app_id":      p.settings.AppID,
		"method":      "alipay.data.bill.accountlog.query",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizContent),
	}
	if token := strings.TrimSpace(p.settings.AppAuthToken); token != "" {
		params["app_auth_token"] = token
	}
	values, err := p.alipayRSASignedParams(params)
	if err != nil {
		return nil, "", err
	}
	requestURL := buildURLWithQuery(p.settings.GatewayURL, values)
	response, err := p.client.Get(requestURL)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, string(body), fmt.Errorf("支付宝账单查询失败: HTTP %d", response.StatusCode)
	}
	items, err := parseAlipayBillItems(body)
	if err != nil {
		return nil, string(body), err
	}
	return items, string(body), nil
}

func (p *AlipayBillProvider) buildPaymentURL(req ProviderCreateRequest) (string, string, error) {
	remark := "订单:" + req.OrderNo
	if receiptURL := strings.TrimSpace(p.settings.ReceiptQRURL); receiptURL != "" {
		return buildAlipayReceiptQRURL(receiptURL, req.AmountCents, remark), "receipt_qr", nil
	}
	transferUserID := strings.TrimSpace(p.settings.TransferUserID)
	if transferUserID == "" {
		return "", "", fmt.Errorf("支付宝收款 UID 不能为空")
	}
	if cashierBaseURL := strings.TrimSpace(p.settings.CashierBaseURL); cashierBaseURL != "" {
		return buildAlipayCashierOpenURL(cashierBaseURL, req.OrderNo), "cashier", nil
	}
	return buildAlipayTransferURL(transferUserID, p.settings.TransferUserName, req.AmountCents, req.OrderNo), "transfer", nil
}

func buildAlipayReceiptQRURL(baseURL string, amountCents int64, remark string) string {
	baseURL = extractAlipayReceiptQRURL(strings.TrimSpace(baseURL))
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	values := parsed.Query()
	if strings.TrimSpace(values.Get("_s")) == "" {
		values.Set("_s", "web-other")
	}
	values.Set("a", formatCents(amountCents))
	values.Set("m", strings.TrimSpace(remark))
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func extractAlipayReceiptQRURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	if strings.EqualFold(parsed.Host, "render.alipay.com") || strings.EqualFold(parsed.Host, "ds.alipay.com") {
		query := parsed.Query()
		if qrcode := strings.TrimSpace(query.Get("qrcode")); qrcode != "" {
			return qrcode
		}
		if extracted := extractAlipayQrcodeParam(query.Get("scheme")); extracted != "" {
			return extracted
		}
	}
	if strings.EqualFold(parsed.Scheme, "alipays") {
		if qrcode := strings.TrimSpace(parsed.Query().Get("qrcode")); qrcode != "" {
			return qrcode
		}
	}
	return strings.TrimSpace(value)
}

func extractAlipayQrcodeParam(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil {
		return strings.TrimSpace(parsed.Query().Get("qrcode"))
	}
	marker := "qrcode="
	index := strings.Index(value, marker)
	if index < 0 {
		return ""
	}
	encoded := value[index+len(marker):]
	if end := strings.Index(encoded, "&"); end >= 0 {
		encoded = encoded[:end]
	}
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return strings.TrimSpace(encoded)
	}
	return strings.TrimSpace(decoded)
}

func buildAlipayCashierOpenURL(baseURL, orderNo string) string {
	cashierURL := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/open/payments/alipay-bill/cashier/" + url.PathEscape(strings.TrimSpace(orderNo))
	values := url.Values{}
	values.Set("appId", "20000067")
	values.Set("url", cashierURL)
	return "alipays://platformapi/startapp?" + values.Encode()
}

func buildAlipayTransferURL(userID, userName string, amountCents int64, remark string) string {
	values := url.Values{}
	values.Set("actionType", "toAccount")
	values.Set("userId", strings.TrimSpace(userID))
	values.Set("amount", formatCents(amountCents))
	values.Set("memo", strings.TrimSpace(remark))
	if strings.TrimSpace(userName) != "" {
		values.Set("userName", strings.TrimSpace(userName))
	}
	return "alipays://platformapi/startapp?appId=20000116&" + values.Encode()
}

func (p *AlipayBillProvider) alipayRSASignedParams(params map[string]string) (url.Values, error) {
	signature, err := p.alipayRSASign(alipaySignContent(params))
	if err != nil {
		return nil, err
	}
	values := stringMapValues(params)
	values.Set("sign", signature)
	return values, nil
}

func (p *AlipayBillProvider) alipayRSASign(content string) (string, error) {
	privateKey, err := parseRSAPrivateKey(p.settings.PrivateKey)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func alipaySignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" || key == "sign" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func parseAlipayBillItems(body []byte) ([]alipayBillItem, error) {
	var root map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("支付宝账单响应无效: %w", err)
	}
	response, ok := firstObject(root, "alipay_data_bill_accountlog_query_response", "alipay_bill_account_log_query_response", "response")
	if !ok {
		return nil, fmt.Errorf("支付宝账单响应缺少业务数据")
	}
	if code := strings.TrimSpace(valueString(response["code"])); code != "" && code != "10000" {
		return nil, fmt.Errorf("支付宝账单查询失败: %s", valueString(response["msg"]))
	}
	list := firstArray(response, "detail_list", "account_log_list", "bill_list", "items")
	items := make([]alipayBillItem, 0, len(list))
	for _, rawItem := range list {
		object, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		item := parseAlipayBillItem(object)
		if item.ProviderOrderNo == "" || item.AmountCents <= 0 || item.PaidAt.IsZero() {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func parseAlipayBillItem(object map[string]interface{}) alipayBillItem {
	raw, _ := json.Marshal(object)
	amount, _ := centsFromMoney(firstValueString(object, "trans_amount", "amount", "total_amount", "income"))
	paidAt := parseAlipayTime(firstValueString(object, "trans_dt", "trans_date", "create_time", "pay_time", "paid_at"))
	return alipayBillItem{
		ProviderOrderNo: firstValueString(object, "trade_no", "order_no", "trans_code_msg", "account_log_id", "bill_no"),
		AccountLogID:    firstValueString(object, "account_log_id", "trans_code_msg", "trade_no"),
		AmountCents:     amount,
		Direction:       firstValueString(object, "balance_type", "direction", "fund_flow", "type"),
		Remark:          firstValueString(object, "memo", "remark", "merchant_out_order_no"),
		Summary:         firstValueString(object, "summary", "title", "trans_memo", "goods_title"),
		OppositeAccount: firstValueString(object, "other_account", "opposite_account", "buyer_logon_id", "payer_account"),
		PaidAt:          paidAt,
		Raw:             string(raw),
	}
}

func alipayBillItemIsIncome(item alipayBillItem) bool {
	direction := strings.ToLower(strings.TrimSpace(item.Direction))
	return direction == "收入" || direction == "in" || direction == "income" || strings.Contains(direction, "收入")
}

func alipayBillItemMatchesOrder(item alipayBillItem, orderNo string, amountCents int64) bool {
	if strings.TrimSpace(orderNo) == "" || amountCents <= 0 {
		return false
	}
	return item.AmountCents == amountCents
}

func parseAlipayTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func firstObject(root map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	for _, key := range keys {
		if object, ok := root[key].(map[string]interface{}); ok {
			return object, true
		}
	}
	return nil, false
}

func firstArray(root map[string]interface{}, keys ...string) []interface{} {
	for _, key := range keys {
		switch value := root[key].(type) {
		case []interface{}:
			return value
		case map[string]interface{}:
			if nested := firstArray(value, "account_log_item", "item", "items", "list"); len(nested) > 0 {
				return nested
			}
		}
	}
	return nil
}

func firstValueString(object map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(valueString(object[key])); value != "" {
			return value
		}
	}
	return ""
}

func valueString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		data, _ := json.Marshal(typed)
		return string(data)
	}
}
