package web

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/payment"
	qrcode "github.com/skip2/go-qrcode"
)

func (s *Server) handlePaymentSettings(w http.ResponseWriter, r *http.Request) {
	db := s.adapterManager.GetDatabase()
	switch r.Method {
	case http.MethodGet:
		settings, err := db.GetPaymentSettings()
		if err != nil {
			s.jsonError(w, "获取支付设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, publicPaymentSettings(settings))
	case http.MethodPut:
		var settings config.PaymentSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := db.SavePaymentSettings(&settings); err != nil {
			s.jsonError(w, "保存支付设置失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		saved, err := db.GetPaymentSettings()
		if err != nil {
			s.jsonError(w, "读取支付设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, publicPaymentSettings(saved))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePaymentOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	orders, total, err := s.adapterManager.GetDatabase().ListPaymentOrders(config.PaymentOrderQuery{
		OrderNo:  strings.TrimSpace(query.Get("order_no")),
		UnionID:  strings.TrimSpace(query.Get("union_id")),
		PluginID: strings.TrimSpace(query.Get("plugin_id")),
		Provider: strings.TrimSpace(query.Get("provider")),
		Method:   strings.TrimSpace(query.Get("method")),
		Status:   strings.TrimSpace(query.Get("status")),
		Limit:    intQueryValue(query.Get("limit"), 20),
		Offset:   intQueryValue(query.Get("offset"), 0),
	})
	if err != nil {
		s.jsonError(w, "获取支付订单失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"items": orders, "total": total})
}

func (s *Server) handlePaymentOrderDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/payments/orders/")
	path = strings.Trim(path, "/")
	if strings.HasSuffix(path, "/query") {
		s.handlePaymentOrderQuery(w, r)
		return
	}
	orderNo := strings.TrimSpace(path)
	if orderNo == "" || strings.Contains(orderNo, "/") {
		s.jsonError(w, "订单号不能为空", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.writePaymentOrderDetail(w, orderNo)
	case http.MethodDelete:
		s.deletePaymentOrder(w, orderNo)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) writePaymentOrderDetail(w http.ResponseWriter, orderNo string) {
	db := s.adapterManager.GetDatabase()
	order, err := db.GetPaymentOrder(orderNo)
	if err == sql.ErrNoRows {
		s.jsonError(w, "订单不存在", http.StatusNotFound)
		return
	}
	if err != nil {
		s.jsonError(w, "获取支付订单失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	events, err := db.ListPaymentEvents(orderNo)
	if err != nil {
		s.jsonError(w, "获取支付事件失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"order": order, "events": events})
}

func (s *Server) deletePaymentOrder(w http.ResponseWriter, orderNo string) {
	err := s.adapterManager.GetDatabase().DeletePaymentOrder(orderNo)
	if err == sql.ErrNoRows {
		s.jsonError(w, "订单不存在", http.StatusNotFound)
		return
	}
	if err != nil {
		s.jsonError(w, "删除支付订单失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]string{"message": "支付订单已删除"})
}

func (s *Server) handlePaymentNotifyEpay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ok := s.confirmEpayRequest(r, true)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if ok {
		_, _ = w.Write([]byte("success"))
		return
	}
	_, _ = w.Write([]byte("fail"))
}

func (s *Server) handlePaymentReturnEpay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ok := s.confirmEpayRequest(r, false)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		_, _ = w.Write([]byte("<html><body>支付成功，请返回聊天窗口。</body></html>"))
		return
	}
	_, _ = w.Write([]byte("<html><body>支付结果待确认，请返回聊天窗口稍后查看。</body></html>"))
}

func (s *Server) handleOpenAPIQRCode(w http.ResponseWriter, r *http.Request) {
	content := strings.TrimSpace(r.URL.Query().Get("text"))
	if content == "" {
		content = strings.TrimSpace(r.URL.Query().Get("content"))
	}
	if content == "" {
		s.jsonError(w, "二维码内容不能为空", http.StatusBadRequest)
		return
	}
	if len(content) > 2048 {
		s.jsonError(w, "二维码内容不能超过 2048 字符", http.StatusBadRequest)
		return
	}
	s.writeQRCodePNG(w, content)
}

func (s *Server) handlePaymentQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/open/payments/qrcode/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".png") {
		s.jsonError(w, "二维码路径无效", http.StatusBadRequest)
		return
	}
	orderNo := strings.TrimSpace(parts[0])
	token := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".png")
	if orderNo == "" || token == "" {
		s.jsonError(w, "二维码路径无效", http.StatusBadRequest)
		return
	}
	order, err := s.adapterManager.GetDatabase().GetPaymentOrder(orderNo)
	if err == sql.ErrNoRows {
		s.jsonError(w, "订单不存在", http.StatusNotFound)
		return
	}
	if err != nil {
		s.jsonError(w, "获取支付订单失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	content := payment.PaymentQRCodeContent(order)
	if content == "" {
		s.jsonError(w, "订单缺少二维码内容", http.StatusBadRequest)
		return
	}
	if token != payment.PaymentQRCodeToken(order.OrderNo, content) {
		s.jsonError(w, "二维码 token 无效", http.StatusForbidden)
		return
	}
	s.writeQRCodePNG(w, content)
}

func (s *Server) writeQRCodePNG(w http.ResponseWriter, content string) {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		s.jsonError(w, "生成二维码失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func (s *Server) confirmEpayRequest(r *http.Request, writeOrder bool) bool {
	db := s.adapterManager.GetDatabase()
	settings, err := db.GetPaymentSettings()
	if err != nil {
		return false
	}
	provider, err := payment.NewEpayProvider(settings.Epay, nil)
	if err != nil {
		return false
	}
	result, err := provider.VerifyNotify(r)
	if err != nil || result == nil || result.Status != "paid" {
		if result != nil && strings.TrimSpace(result.OrderNo) != "" {
			_ = db.AppendPaymentEvent(result.OrderNo, "provider_rejected", errString(err), result.Raw)
		}
		return false
	}
	if !writeOrder {
		order, err := db.GetPaymentOrder(result.OrderNo)
		if err != nil || order == nil || !strings.EqualFold(order.Provider, "epay") || !strings.EqualFold(order.Method, result.Method) || order.AmountCents != result.AmountCents {
			return false
		}
		return true
	}
	order, alreadyPaid, err := db.ConfirmProviderPayment(config.ProviderPaymentConfirmation{OrderNo: result.OrderNo, Provider: "epay", Method: result.Method, AmountCents: result.AmountCents, ProviderOrderNo: result.ProviderOrderNo, Raw: result.Raw, PaidAt: result.PaidAt})
	if err != nil || order == nil {
		return false
	}
	message := "支付成功"
	if alreadyPaid {
		message = "支付已确认"
	}
	payment.DefaultWaitHub.Resolve(order.OrderNo, payment.PaymentResult{Status: "paid", OrderNo: order.OrderNo, Provider: order.Provider, Method: order.Method, Subject: order.Subject, AmountCents: order.AmountCents, PointsAmount: order.PointsAmount, PayURL: order.PayURL, QRCode: order.QRCode, ProviderOrderNo: order.ProviderOrderNo, Message: message})
	return true
}

func (s *Server) handlePaymentOrderQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	orderNo := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/payments/orders/"), "/query")
	orderNo = strings.Trim(orderNo, "/")
	if orderNo == "" || strings.Contains(orderNo, "/") {
		s.jsonError(w, "订单号不能为空", http.StatusBadRequest)
		return
	}
	db := s.adapterManager.GetDatabase()
	order, err := db.GetPaymentOrder(orderNo)
	if err == sql.ErrNoRows {
		s.jsonError(w, "订单不存在", http.StatusNotFound)
		return
	}
	if err != nil {
		s.jsonError(w, "获取支付订单失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !strings.EqualFold(order.Provider, "epay") {
		s.jsonError(w, "仅支持查询易支付订单", http.StatusBadRequest)
		return
	}
	settings, err := db.GetPaymentSettings()
	if err != nil {
		s.jsonError(w, "获取支付设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	provider, err := payment.NewEpayProvider(settings.Epay, nil)
	if err != nil {
		s.jsonError(w, "易支付配置无效: "+err.Error(), http.StatusBadRequest)
		return
	}
	queryResult, err := provider.QueryOrder(order.OrderNo, order.ProviderOrderNo)
	if err != nil {
		s.jsonError(w, "查询易支付订单失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if queryResult.Status == "paid" {
		amount := queryResult.AmountCents
		method := strings.TrimSpace(queryResult.Method)
		if amount <= 0 || method == "" {
			s.jsonError(w, "查询结果缺少金额或支付方式", http.StatusBadRequest)
			return
		}
		paidAt := queryResult.PaidAt
		if paidAt.IsZero() {
			paidAt = time.Now()
		}
		confirmed, _, confirmErr := db.ConfirmProviderPayment(config.ProviderPaymentConfirmation{OrderNo: order.OrderNo, Provider: "epay", Method: method, AmountCents: amount, ProviderOrderNo: queryResult.ProviderOrderNo, Raw: queryResult.Raw, PaidAt: paidAt})
		if confirmErr != nil {
			s.jsonError(w, "确认支付失败: "+confirmErr.Error(), http.StatusBadRequest)
			return
		}
		order = confirmed
		payment.DefaultWaitHub.Resolve(order.OrderNo, payment.PaymentResult{Status: "paid", OrderNo: order.OrderNo, Provider: order.Provider, Method: order.Method, Subject: order.Subject, AmountCents: order.AmountCents, PointsAmount: order.PointsAmount, PayURL: order.PayURL, QRCode: order.QRCode, ProviderOrderNo: order.ProviderOrderNo, Message: "支付成功"})
	}
	s.jsonResponse(w, map[string]interface{}{"order": order, "query": queryResult})
}

func publicPaymentSettings(settings *config.PaymentSettings) config.PaymentSettings {
	result := config.NormalizePaymentSettings(settings)
	result.Epay.HasKey = strings.TrimSpace(result.Epay.Key) != ""
	result.Epay.HasPlatformKey = strings.TrimSpace(result.Epay.PlatformPublicKey) != ""
	result.Epay.HasMerchantKey = strings.TrimSpace(result.Epay.MerchantPrivateKey) != ""
	result.Epay.Key = ""
	result.Epay.PlatformPublicKey = ""
	result.Epay.MerchantPrivateKey = ""
	return result
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func intQueryValue(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
