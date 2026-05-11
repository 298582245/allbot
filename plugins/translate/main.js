/**
 * 翻译插件示例
 *
 * 展示如何使用 AllBot Node.js SDK 开发插件
 */

const path = require('path');

// 添加SDK路径
const sdkPath = path.join(__dirname, '../../sdk/nodejs');
process.env.NODE_PATH = sdkPath + (process.env.NODE_PATH ? ':' + process.env.NODE_PATH : '');
require('module').Module._initPaths();

// 检查运行模式
if (process.argv.includes('--mode=direct')) {
    // 直接执行模式（无端口）
    const { runDirect } = require(path.join(sdkPath, 'allbot_direct'));

    async function handle(ctx) {
        const content = ctx.content;

        // 翻译 你好
        if (content.startsWith('翻译 ')) {
            const text = content.substring(3).trim();

            if (!text) {
                await ctx.reply('请输入要翻译的文本');
                return;
            }

            // 模拟翻译（实际应用中应该调用翻译API）
            const translation = await translate(text);
            await ctx.reply(`翻译结果：${translation}`);
        } else {
            // 帮助信息
            await ctx.reply(
                '翻译插件使用方法：\n' +
                '翻译 <文本> - 翻译文本'
            );
        }
    }

    async function translate(text) {
        // 模拟翻译（实际应用中应该调用真实的翻译API）
        // 例如：使用 axios 调用 Google Translate API
        return `[翻译] ${text}`;
    }

    runDirect(handle);
} else {
    // HTTP服务器模式（兼容旧版）
    const { startPlugin } = require(path.join(sdkPath, 'allbot_sdk'));
    startPlugin();
}
