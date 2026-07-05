package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/allbot/allbot/core/updater"
)

func replyUpdate(ctx *Context) error {
	if !ctx.IsAdmin() {
		return ctx.SendText("仅平台管理员可使用更新")
	}
	if ctx.UpdateHandler == nil {
		return ctx.SendText("更新功能未初始化")
	}
	state := ctx.UpdateHandler.CurrentState()
	if state.Status == updater.UpgradeStatusDownloading || state.Status == updater.UpgradeStatusRestarting {
		return ctx.SendText("更新已在执行：" + state.Message)
	}
	requestCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	state, err := ctx.UpdateHandler.StartUpgrade(requestCtx)
	if err != nil {
		return ctx.SendText(err.Error())
	}
	return ctx.SendText(fmt.Sprintf("已开始更新到 %s，资产：%s\n%s", state.Version, state.AssetName, state.Message))
}
