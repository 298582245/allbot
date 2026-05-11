/**
 * AllBot Node.js SDK - Direct Mode (stdin/stdout)
 *
 * 无端口直接执行模式，支持并发
 */

class Context {
    constructor(data) {
        this.plugin_id = data.plugin_id || '';
        this.platform = data.platform || '';
        this.user_id = data.user_id || '';
        this.group_id = data.group_id || '';
        this.content = data.content || '';
        this.message_id = data.message_id || '';
        this.metadata = data.metadata || {};
        this._replies = [];
    }

    async reply(text) {
        this._replies.push(text);
        return true;
    }

    async send_image(image_url) {
        // TODO: 实现图片发送
        return false;
    }

    async listen(timeout = 60) {
        // TODO: 实现连续对话
        return '';
    }
}

function runDirect(handler) {
    let inputData = '';

    // 读取 stdin
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => {
        inputData += chunk;
    });

    process.stdin.on('end', async () => {
        try {
            // 解析消息 JSON
            const messageData = JSON.parse(inputData);

            // 创建上下文
            const ctx = new Context(messageData);

            // 执行处理器
            await handler(ctx);

            // 输出结果到 stdout
            const result = {
                success: true,
                error: '',
                replies: ctx._replies
            };
            console.log(JSON.stringify(result));

        } catch (error) {
            // 输出错误
            const result = {
                success: false,
                error: error.message,
                replies: []
            };
            console.log(JSON.stringify(result));
            process.exit(1);
        }
    });
}

module.exports = { Context, runDirect };
