# KOOK 自定义适配器

此示例通过 KOOK Gateway WebSocket 接收频道和私聊消息，并通过 KOOK HTTP API 回复。它使用 Node.js 22 或更新版本的内置 `WebSocket`、`fetch`、`FormData`，不需要安装第三方 npm 包。

1. 在 KOOK 开发者平台创建应用和机器人，启用 WebSocket 连接并复制 Bot Token。
2. 在 AllBot 项目目录下，将本目录复制到 `./adapters/kook`。
3. 在 AllBot 的适配器管理中重载定义，再到“对接管理”创建 KOOK 实例并填写 Bot Token。
4. 将机器人邀请到自己的 KOOK 服务器，确认它有查看频道、发送消息和上传文件的权限。

适配器支持文本/KMarkdown、图片、音频、视频、文件、@用户，以及基础用户和服务器信息查询。媒体发送时会先上传到 KOOK，再发送平台资源链接。

Bot Token 是敏感凭据，只填入实例配置，不要写入 `adapter.json` 或提交到代码仓库。
