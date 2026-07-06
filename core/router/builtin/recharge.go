package builtin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/payment"
	"github.com/allbot/allbot/core/types"
)

func replyRechargePoints(ctx *Context) error {
	unit := pointsUnit(ctx)
	args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(ctx.Message.Content, "积分充值")))
	if ctx.IsAdmin() && len(args) == 2 {
		return ctx.SendText(rechargePointsByAdmin(ctx, args, unit))
	}
	if len(args) != 1 {
		return ctx.SendText(fmt.Sprintf("用法：积分充值 <金额>\n示例：积分充值 1\n示例：积分充值 9.90\n充值成功后按支付配置兑换为%s\n管理员给用户加%s：积分充值 <unionId或平台:userId> <数量>", unit, unit))
	}
	amountRaw := json.RawMessage(strconv.Quote(args[0]))
	amountCents, err := payment.ParseRMBToCents(amountRaw)
	if err != nil {
		return ctx.SendText("充值失败：" + err.Error())
	}
	account, err := ctx.Database.EnsureUserAccount(ctx.Message.Platform, ctx.Message.UserID)
	if err != nil {
		return ctx.SendText("充值失败：" + err.Error())
	}
	settings, err := ctx.Database.GetPaymentSettings()
	if err != nil {
		return ctx.SendText("读取支付配置失败：" + err.Error())
	}
	settingsValue := config.NormalizePaymentSettings(settings)
	methods := enabledRechargePaymentMethods(&settingsValue)
	if len(methods) == 0 {
		return ctx.SendText("充值失败：请先在支付配置中启用第三方支付方式")
	}
	pointsAmount, err := config.CalculatePointsAmount(amountCents, settingsValue.PointsPerRMB)
	if err != nil {
		return ctx.SendText("充值失败：" + err.Error())
	}
	promptTitle := rechargePaymentTitle(amountCents, pointsAmount, paymentCurrencyUnitName(settingsValue), unit)
	go runRechargePayment(ctx, account.UnionID, amountRaw, methods, unit, promptTitle)
	return nil
}

func rechargePointsByAdmin(ctx *Context, args []string, unit string) string {
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		return fmt.Sprintf("充值%s数量必须是大于 0 的整数", unit)
	}
	unionID, err := resolveRechargeTarget(ctx, args[0])
	if err != nil {
		return "充值失败：" + err.Error()
	}
	remaining, err := ctx.Database.AddUserPoints(unionID, amount)
	if err != nil {
		return "充值失败：" + err.Error()
	}
	return fmt.Sprintf("充值成功\nUnionID：%s\n本次充值：%d%s\n当前余额：%d%s", unionID, amount, unit, remaining, unit)
}

func runRechargePayment(ctx *Context, unionID string, amountRaw json.RawMessage, methods []config.PaymentMethodSetting, unit string, promptTitle string) {
	if ctx.Listen == nil {
		_ = ctx.SendText("连续对话功能未初始化")
		return
	}
	methodCodes := make([]string, 0, len(methods))
	for _, method := range methods {
		methodCodes = append(methodCodes, method.Code)
	}
	service := payment.NewService(ctx.Database)
	result, err := service.WaitPay(payment.WaitPayRequest{PluginID: "builtin:recharge_points", Platform: ctx.Message.Platform, AdapterID: ctx.adapterID(), UserID: ctx.Message.UserID, GroupID: ctx.Message.GroupID, UnionID: unionID, Subject: "积分充值", AmountRaw: amountRaw, Timeout: 300, PointsUnit: unit, Methods: methodCodes, Metadata: map[string]interface{}{"source": "builtin_recharge_points"}, PromptTitle: promptTitle}, payment.Interaction{Reply: func(text string) error {
		return ctx.SendText(text)
	}, ReplyButtons: func(text string, buttons [][]types.ButtonOption) error {
		if ctx.ReplyButtons == nil {
			return ctx.SendText(text)
		}
		return ctx.ReplyButtons(text, buttons)
	}, SendImage: func(imageURL string) error {
		if ctx.SendImage == nil {
			return nil
		}
		return ctx.SendImage(imageURL)
	}, SendRich: func(message types.RichMessage) error {
		if ctx.SendRich == nil {
			return fmt.Errorf("适配器不支持富消息")
		}
		return ctx.SendRich(message)
	}, Listen: func(timeout int) string {
		return ctx.ListenText(timeout)
	}, ListenUntil: func(timeout int, done <-chan struct{}) string {
		return ctx.ListenUntilDone(timeout, done)
	}})
	if err != nil {
		_ = ctx.SendText("充值失败：" + err.Error())
		return
	}
	if result.Status != "paid" {
		_ = ctx.SendText("充值未完成：" + result.Message)
		return
	}
	remaining, err := ctx.Database.CreditPaymentPoints(result.OrderNo, "充值积分")
	if err != nil {
		_ = ctx.SendText("支付成功，但积分入账失败：" + err.Error())
		return
	}
	_ = ctx.SendText(fmt.Sprintf("充值成功\n订单号：%s\n本次充值：%d%s\n当前余额：%d%s", result.OrderNo, result.PointsAmount, unit, remaining, unit))
}

func enabledRechargePaymentMethods(settings *config.PaymentSettings) []config.PaymentMethodSetting {
	all := payment.EnabledMethods(settings, nil)
	methods := make([]config.PaymentMethodSetting, 0, len(all))
	for _, method := range all {
		provider := strings.TrimSpace(method.Provider)
		if strings.EqualFold(provider, "epay") || strings.EqualFold(provider, "alipay_bill") {
			methods = append(methods, method)
		}
	}
	return methods
}

func rechargePaymentTitle(amountCents, pointsAmount int64, currencyUnit string, pointsUnit string) string {
	return fmt.Sprintf("当前充值 %s %s（到账 %d %s）", formatRechargeAmount(amountCents), currencyUnit, pointsAmount, pointsUnit)
}

func paymentCurrencyUnitName(settings config.PaymentSettings) string {
	unit := strings.TrimSpace(settings.CurrencyUnit)
	if unit == "" {
		return "RMB"
	}
	return unit
}

func formatRechargeAmount(amountCents int64) string {
	return fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
}

func resolveRechargeTarget(ctx *Context, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("充值目标不能为空")
	}
	if parts := strings.SplitN(target, ":", 2); len(parts) == 2 {
		platform := strings.TrimSpace(parts[0])
		userID := strings.TrimSpace(parts[1])
		if platform == "" || userID == "" {
			return "", fmt.Errorf("平台和用户 ID 不能为空")
		}
		account, err := ctx.Database.GetUserAccount(platform, userID)
		if err != nil {
			return "", fmt.Errorf("账号不存在，请确认用户已发送【注册】或已绑定过账号")
		}
		return account.UnionID, nil
	}
	exists, err := ctx.Database.UserUnionExists(target)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("账号不存在，请检查 UnionID 是否正确")
	}
	return target, nil
}
