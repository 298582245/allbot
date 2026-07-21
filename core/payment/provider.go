package payment

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

type PaymentProvider interface {
	CreateOrder(ProviderCreateRequest) (*ProviderOrder, error)
	VerifyNotify(*http.Request) (*ProviderNotifyResult, error)
	QueryOrder(orderNo, providerOrderNo string) (*ProviderQueryResult, error)
}

type ProviderCreateRequest struct {
	OrderNo      string
	CashierToken string
	Subject      string
	AmountCents  int64
	Method       string
	NotifyURL    string
	ReturnURL    string
}

type ProviderOrder struct {
	OrderNo         string `json:"order_no"`
	ProviderOrderNo string `json:"provider_order_no"`
	PayURL          string `json:"pay_url"`
	QRCode          string `json:"qrcode"`
	Raw             string `json:"raw"`
}

type ProviderNotifyResult struct {
	OrderNo         string    `json:"order_no"`
	ProviderOrderNo string    `json:"provider_order_no"`
	Method          string    `json:"method"`
	AmountCents     int64     `json:"amount_cents"`
	Status          string    `json:"status"`
	Raw             string    `json:"raw"`
	PaidAt          time.Time `json:"paid_at"`
}

type ProviderQueryResult struct {
	OrderNo         string    `json:"order_no"`
	ProviderOrderNo string    `json:"provider_order_no"`
	Method          string    `json:"method"`
	AmountCents     int64     `json:"amount_cents"`
	Status          string    `json:"status"`
	Raw             string    `json:"raw"`
	PaidAt          time.Time `json:"paid_at"`
}

func NewEpayProvider(settings config.EpaySettings, client *http.Client) (*EpayProvider, error) {
	settings = config.NormalizePaymentSettings(&config.PaymentSettings{Epay: settings}).Epay
	if err := validateEpayRuntimeSettings(settings); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &EpayProvider{settings: settings, client: client}, nil
}

func validateEpayRuntimeSettings(settings config.EpaySettings) error {
	if strings.TrimSpace(settings.APIURL) == "" || strings.TrimSpace(settings.PID) == "" {
		return fmt.Errorf("易支付接口地址和商户 ID 不能为空")
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

func formatCents(amountCents int64) string {
	return fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
}

func centsFromMoney(value string) (int64, error) {
	return ParseRMBToCents([]byte(fmt.Sprintf("%q", strings.TrimSpace(value))))
}

func joinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	if path == "" {
		return base + "/"
	}
	return base + "/" + path
}

func buildURLWithQuery(rawURL string, values url.Values) string {
	if len(values) == 0 {
		return rawURL
	}
	separator := "?"
	if strings.Contains(rawURL, "?") {
		separator = "&"
	}
	return rawURL + separator + values.Encode()
}
