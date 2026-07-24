package payment

import (
	"bytes"
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
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

type EpayProvider struct {
	settings config.EpaySettings
	client   *http.Client
}

func (p *EpayProvider) CreateOrder(req ProviderCreateRequest) (*ProviderOrder, error) {
	if strings.EqualFold(p.settings.Version, "v2") {
		return p.createOrderV2(req)
	}
	return p.createOrderV1(req)
}

func (p *EpayProvider) VerifyNotify(r *http.Request) (*ProviderNotifyResult, error) {
	if strings.EqualFold(p.settings.Version, "v2") {
		return p.verifyNotifyV2(r)
	}
	return p.verifyNotifyV1(r)
}

func (p *EpayProvider) QueryOrder(orderNo, providerOrderNo string) (*ProviderQueryResult, error) {
	if strings.EqualFold(p.settings.Version, "v2") {
		return p.queryOrderV2(orderNo, providerOrderNo)
	}
	return p.queryOrderV1(orderNo, providerOrderNo)
}

func (p *EpayProvider) createOrderV1(req ProviderCreateRequest) (*ProviderOrder, error) {
	params := map[string]string{
		"pid":          p.settings.PID,
		"type":         req.Method,
		"notify_url":   req.NotifyURL,
		"return_url":   req.ReturnURL,
		"out_trade_no": req.OrderNo,
		"name":         req.Subject,
		"money":        formatCents(req.AmountCents),
	}
	signed := epayMD5SignedParams(params, p.settings.Key)
	body, err := p.postForm(joinURL(p.settings.APIURL, "mapi.php"), signed)
	if err != nil {
		return nil, err
	}
	fields, err := decodeJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("易支付 V1 下单响应无效: %w", err)
	}
	if !epayV1ResponseOK(fields) {
		return nil, fmt.Errorf("易支付 V1 下单失败: %s", firstMapString(fields, "msg", "message"))
	}
	order := &ProviderOrder{
		OrderNo:         req.OrderNo,
		ProviderOrderNo: firstMapString(fields, "trade_no", "provider_order_no"),
		PayURL:          firstMapString(fields, "payurl", "pay_url", "url"),
		QRCode:          firstMapString(fields, "qrcode", "qr_code"),
		Raw:             string(body),
	}
	if order.PayURL == "" {
		order.PayURL = buildURLWithQuery(joinURL(p.settings.APIURL, "submit.php"), signed)
	}
	return order, nil
}

func (p *EpayProvider) verifyNotifyV1(r *http.Request) (*ProviderNotifyResult, error) {
	values, raw, err := requestValues(r)
	if err != nil {
		return nil, err
	}
	params := valuesToStringMap(values)
	if strings.TrimSpace(params["pid"]) != strings.TrimSpace(p.settings.PID) {
		return nil, fmt.Errorf("易支付 V1 商户 ID 不匹配")
	}
	if epayMD5Sign(params, p.settings.Key) != params["sign"] {
		return nil, fmt.Errorf("易支付 V1 回调签名无效")
	}
	amount, err := centsFromMoney(params["money"])
	if err != nil {
		return nil, fmt.Errorf("易支付 V1 回调金额无效")
	}
	status := "pending"
	if strings.EqualFold(params["trade_status"], "TRADE_SUCCESS") || params["status"] == "1" {
		status = "paid"
	}
	return &ProviderNotifyResult{OrderNo: params["out_trade_no"], ProviderOrderNo: params["trade_no"], Method: params["type"], AmountCents: amount, Status: status, Raw: raw, PaidAt: time.Now()}, nil
}

func (p *EpayProvider) queryOrderV1(orderNo, providerOrderNo string) (*ProviderQueryResult, error) {
	values := url.Values{}
	values.Set("act", "order")
	values.Set("pid", p.settings.PID)
	values.Set("key", p.settings.Key)
	if strings.TrimSpace(providerOrderNo) != "" {
		values.Set("trade_no", strings.TrimSpace(providerOrderNo))
	} else {
		values.Set("out_trade_no", strings.TrimSpace(orderNo))
	}
	body, err := p.get(buildURLWithQuery(joinURL(p.settings.APIURL, "api.php"), values))
	if err != nil {
		return nil, err
	}
	fields, err := decodeJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("易支付 V1 查询响应无效: %w", err)
	}
	amount := int64(0)
	if money := firstMapString(fields, "money", "amount"); money != "" {
		amount, err = centsFromMoney(money)
		if err != nil {
			return nil, err
		}
	}
	status := "pending"
	if firstMapString(fields, "status") == "1" || strings.EqualFold(firstMapString(fields, "trade_status"), "TRADE_SUCCESS") {
		status = "paid"
	}
	return &ProviderQueryResult{OrderNo: stringDefault(firstMapString(fields, "out_trade_no"), orderNo), ProviderOrderNo: stringDefault(firstMapString(fields, "trade_no"), providerOrderNo), Method: firstMapString(fields, "type"), AmountCents: amount, Status: status, Raw: string(body), PaidAt: time.Now()}, nil
}

func (p *EpayProvider) createOrderV2(req ProviderCreateRequest) (*ProviderOrder, error) {
	params := map[string]string{
		"pid":          p.settings.PID,
		"type":         req.Method,
		"notify_url":   req.NotifyURL,
		"return_url":   req.ReturnURL,
		"out_trade_no": req.OrderNo,
		"name":         req.Subject,
		"money":        formatCents(req.AmountCents),
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
	}
	signed, err := p.epayRSASignedParams(params)
	if err != nil {
		return nil, err
	}
	body, err := p.postForm(joinURL(p.settings.APIURL, "api/pay/create"), signed)
	if err != nil {
		return nil, err
	}
	fields, err := decodeJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("易支付 V2 下单响应无效: %w", err)
	}
	if firstMapString(fields, "code") != "0" {
		return nil, fmt.Errorf("易支付 V2 下单失败: %s", firstMapString(fields, "msg", "message"))
	}
	if err = p.verifyRSAFields(fields); err != nil {
		return nil, err
	}
	flat := flattenResponseData(fields)
	order := &ProviderOrder{
		OrderNo:         req.OrderNo,
		ProviderOrderNo: firstMapString(flat, "trade_no", "provider_order_no"),
		PayURL:          firstMapString(flat, "payurl", "pay_url", "url"),
		QRCode:          firstMapString(flat, "qrcode", "qr_code"),
		Raw:             string(body),
	}
	return order, nil
}

func (p *EpayProvider) verifyNotifyV2(r *http.Request) (*ProviderNotifyResult, error) {
	values, raw, err := requestValues(r)
	if err != nil {
		return nil, err
	}
	params := valuesToStringMap(values)
	if strings.TrimSpace(params["pid"]) != strings.TrimSpace(p.settings.PID) {
		return nil, fmt.Errorf("易支付 V2 商户 ID 不匹配")
	}
	if err = p.verifyRSAFields(params); err != nil {
		return nil, err
	}
	amount, err := centsFromMoney(params["money"])
	if err != nil {
		return nil, fmt.Errorf("易支付 V2 回调金额无效")
	}
	status := "pending"
	if strings.EqualFold(params["trade_status"], "TRADE_SUCCESS") || params["status"] == "1" {
		status = "paid"
	}
	return &ProviderNotifyResult{OrderNo: params["out_trade_no"], ProviderOrderNo: params["trade_no"], Method: params["type"], AmountCents: amount, Status: status, Raw: raw, PaidAt: time.Now()}, nil
}

func (p *EpayProvider) queryOrderV2(orderNo, providerOrderNo string) (*ProviderQueryResult, error) {
	params := map[string]string{
		"pid":       p.settings.PID,
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	if strings.TrimSpace(providerOrderNo) != "" {
		params["trade_no"] = strings.TrimSpace(providerOrderNo)
	} else {
		params["out_trade_no"] = strings.TrimSpace(orderNo)
	}
	signed, err := p.epayRSASignedParams(params)
	if err != nil {
		return nil, err
	}
	body, err := p.postForm(joinURL(p.settings.APIURL, "api/pay/query"), signed)
	if err != nil {
		return nil, err
	}
	fields, err := decodeJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("易支付 V2 查询响应无效: %w", err)
	}
	if firstMapString(fields, "code") != "0" {
		return nil, fmt.Errorf("易支付 V2 查询失败: %s", firstMapString(fields, "msg", "message"))
	}
	if err = p.verifyRSAFields(fields); err != nil {
		return nil, err
	}
	flat := flattenResponseData(fields)
	amount := int64(0)
	if money := firstMapString(flat, "money", "amount"); money != "" {
		amount, err = centsFromMoney(money)
		if err != nil {
			return nil, err
		}
	}
	status := "pending"
	if firstMapString(flat, "status") == "1" || strings.EqualFold(firstMapString(flat, "trade_status"), "TRADE_SUCCESS") {
		status = "paid"
	}
	return &ProviderQueryResult{OrderNo: stringDefault(firstMapString(flat, "out_trade_no"), orderNo), ProviderOrderNo: stringDefault(firstMapString(flat, "trade_no"), providerOrderNo), Method: firstMapString(flat, "type"), AmountCents: amount, Status: status, Raw: string(body), PaidAt: time.Now()}, nil
}

func (p *EpayProvider) postForm(rawURL string, values url.Values) ([]byte, error) {
	request, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("易支付请求失败: HTTP %d", response.StatusCode)
	}
	return body, nil
}

func (p *EpayProvider) get(rawURL string) ([]byte, error) {
	response, err := p.client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("易支付请求失败: HTTP %d", response.StatusCode)
	}
	return body, nil
}

func epayMD5SignedParams(params map[string]string, key string) url.Values {
	values := stringMapValues(params)
	values.Set("sign", epayMD5Sign(params, key))
	values.Set("sign_type", "MD5")
	return values
}

func epayMD5Sign(params map[string]string, key string) string {
	content := epaySignContent(params, false)
	digest := md5.Sum([]byte(content + key))
	return hex.EncodeToString(digest[:])
}

func (p *EpayProvider) epayRSASignedParams(params map[string]string) (url.Values, error) {
	signature, err := p.rsaPrivateSign(epaySignContent(params, true))
	if err != nil {
		return nil, err
	}
	values := stringMapValues(params)
	values.Set("sign", signature)
	values.Set("sign_type", "RSA")
	return values, nil
}

func (p *EpayProvider) verifyRSAFields(fields map[string]string) error {
	if strings.TrimSpace(fields["sign"]) == "" {
		return fmt.Errorf("易支付 RSA 签名不能为空")
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(fields["timestamp"]), 10, 64)
	if err != nil || absInt64(time.Now().Unix()-timestamp) > 300 {
		return fmt.Errorf("易支付 RSA 时间戳无效")
	}
	ok, err := p.rsaPublicVerify(epaySignContent(fields, false), fields["sign"])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("易支付 RSA 签名无效")
	}
	return nil
}

func epaySignContent(params map[string]string, skipArray bool) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" || key == "sign" || key == "sign_type" || strings.TrimSpace(value) == "" {
			continue
		}
		if skipArray && (strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{")) {
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

func (p *EpayProvider) rsaPrivateSign(content string) (string, error) {
	privateKey, err := parseRSAPrivateKey(p.settings.MerchantPrivateKey)
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

func (p *EpayProvider) rsaPublicVerify(content, signatureText string) (bool, error) {
	publicKey, err := parseRSAPublicKey(p.settings.PlatformPublicKey)
	if err != nil {
		return false, err
	}
	signature, err := base64.StdEncoding.DecodeString(signatureText)
	if err != nil {
		return false, fmt.Errorf("易支付 RSA 签名 Base64 无效")
	}
	digest := sha256.Sum256([]byte(content))
	err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func parseRSAPrivateKey(text string) (*rsa.PrivateKey, error) {
	der, err := pemOrBase64DER(text, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("商户私钥不是 RSA 私钥")
	}
	pkcs1, pkcs1Err := x509.ParsePKCS1PrivateKey(der)
	if pkcs1Err != nil {
		return nil, fmt.Errorf("商户私钥解析失败")
	}
	return pkcs1, nil
}

func parseRSAPublicKey(text string) (*rsa.PublicKey, error) {
	der, err := pemOrBase64DER(text, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("平台公钥不是 RSA 公钥")
	}
	pkcs1, pkcs1Err := x509.ParsePKCS1PublicKey(der)
	if pkcs1Err != nil {
		return nil, fmt.Errorf("平台公钥解析失败")
	}
	return pkcs1, nil
}

func pemOrBase64DER(text, blockType string) ([]byte, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("RSA 密钥不能为空")
	}
	if strings.Contains(trimmed, "-----BEGIN") {
		block, _ := pem.Decode([]byte(trimmed))
		if block == nil {
			return nil, fmt.Errorf("RSA PEM 密钥无效")
		}
		return block.Bytes, nil
	}
	compact := strings.Join(strings.Fields(trimmed), "")
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("RSA %s Base64 无效", blockType)
	}
	return der, nil
}

func requestValues(r *http.Request) (url.Values, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("请求不能为空")
	}
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return nil, "", err
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		raw := string(body)
		values := url.Values{}
		for key, items := range r.URL.Query() {
			for _, item := range items {
				values.Add(key, item)
			}
		}
		if strings.TrimSpace(raw) != "" {
			parsed, parseErr := url.ParseQuery(raw)
			if parseErr != nil {
				return nil, raw, parseErr
			}
			for key, items := range parsed {
				for _, item := range items {
					values.Add(key, item)
				}
			}
		}
		if raw == "" {
			raw = values.Encode()
		}
		return values, raw, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, "", err
	}
	return r.Form, r.Form.Encode(), nil
}

func valuesToStringMap(values url.Values) map[string]string {
	result := map[string]string{}
	for key, items := range values {
		if len(items) > 0 {
			result[key] = items[0]
		}
	}
	return result
}

func stringMapValues(params map[string]string) url.Values {
	values := url.Values{}
	for key, value := range params {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	return values
}

func decodeJSONMap(body []byte) (map[string]string, error) {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(raw))
	for key, value := range raw {
		if bytes.Equal(value, []byte("null")) {
			result[key] = ""
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			result[key] = text
			continue
		}
		result[key] = string(value)
	}
	return result, nil
}

func interfaceMapToStringMap(raw map[string]interface{}) map[string]string {
	result := map[string]string{}
	for key, value := range raw {
		switch typed := value.(type) {
		case nil:
			result[key] = ""
		case string:
			result[key] = typed
		case json.Number:
			result[key] = typed.String()
		case float64:
			result[key] = strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			result[key] = strconv.FormatBool(typed)
		default:
			data, _ := json.Marshal(typed)
			result[key] = string(data)
		}
	}
	return result
}

func flattenResponseData(fields map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range fields {
		result[key] = value
	}
	dataText := strings.TrimSpace(fields["data"])
	if dataText == "" || !strings.HasPrefix(dataText, "{") {
		return result
	}
	decoder := json.NewDecoder(strings.NewReader(dataText))
	decoder.UseNumber()
	data := map[string]interface{}{}
	if err := decoder.Decode(&data); err != nil {
		return result
	}
	for key, value := range interfaceMapToStringMap(data) {
		result[key] = value
	}
	return result
}

func epayV1ResponseOK(fields map[string]string) bool {
	code := strings.TrimSpace(fields["code"])
	if code == "" {
		return true
	}
	return code == "1" || code == "0"
}

func firstMapString(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[key]); value != "" {
			return value
		}
	}
	return ""
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
