package builtin

import (
	"context"
	"strings"
	"time"

	"github.com/allbot/allbot/core/updater"
	"github.com/allbot/allbot/core/version"
)

func replyVersion(ctx *Context) error {
	return ctx.SendText(versionInfo(ctx))
}

func versionInfo(ctx *Context) string {
	current := strings.TrimSpace(version.Version)
	if current == "" {
		current = "unknown"
	}
	lines := []string{
		version.DisplayVersion(),
		"",
		"版本信息：",
		"当前版本：" + current,
	}
	client := ctx.ReleaseClient
	if client == nil {
		client = updater.NewGitHubClient()
	}
	requestCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	release, err := client.LatestRelease(requestCtx)
	if err != nil {
		lines = append(lines, "最新版本：获取失败", "", "失败原因："+err.Error())
		return strings.Join(lines, "\n")
	}
	latest := strings.TrimSpace(release.Version)
	if latest == "" {
		latest = "未知"
	}
	lines = append(lines, "最新版本："+latest)
	compare, compareErr := updater.CompareVersion(current, latest)
	body := strings.TrimSpace(release.Body)
	if body != "" {
		lines = append(lines, "", "更新内容：", body)
	}
	if compareErr != nil {
		lines = append(lines, "", "版本比较失败："+compareErr.Error())
		return strings.Join(lines, "\n")
	}
	if compare < 0 {
		lines = append(lines, "", "发送「更新」可升级到最新版本。")
	} else {
		lines = append(lines, "", "当前已是最新版本。")
	}
	return strings.Join(lines, "\n")
}
