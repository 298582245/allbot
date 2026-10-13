# AllBot 在线演示

AllBot 的在线演示使用 GitHub Pages 发布 `web-ui` 的静态构建产物。Demo 构建启用浏览器内模拟 API，不连接 Go 后端、平台适配器、数据库或真实插件运行时。页面操作只修改当前页面内存，刷新后恢复种子数据。

## 第一次配置

1. 将仓库推送到 GitHub。
2. 在仓库的 **Settings → Pages** 中将 **Source** 设置为 **GitHub Actions**。
3. 打开 **Actions → Publish AllBot Demo**，点击 **Run workflow**。
4. 等待 `build` 和 `deploy` 两个任务完成，在 workflow 的 environment 或 Pages 设置中打开站点地址。

这个 workflow 只有 `workflow_dispatch` 触发，不会因为提交代码自动发布。测试某个版本时，先把代码推送到目标分支，再手动运行 workflow。

## 本地预览

在仓库根目录执行：

```powershell
cd web-ui
$env:VITE_ALLBOT_DEMO = 'true'
$env:VITE_BASE_PATH = '/'
npx vite build --mode demo
npm run preview -- --host 127.0.0.1
```

GitHub Pages 构建由 workflow 自动使用 `/${仓库名}/` 作为资源和路由前缀；本地预览使用 `/`。

## 演示边界

Demo 中的插件、适配器、定时任务、脚本任务、日志、用户、支付和开放接口均为模拟数据。WebChat 也使用本地模拟消息，不会发送到 QQ、Telegram 或其他平台。不要在演示页面填写真实密码、Token、Cookie 或平台配置。
