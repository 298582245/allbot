/**
 * AllBot Node.js SDK - Direct Mode (stdin/stdout)
 *
 * 支持流式通信协议：
 * - 插件通过 stdout 发送 JSON 行指令
 * - reply: 立即发送消息
 * - listen: 等待用户输入，从 stdin 读取响应
 * - done: 执行结束
 */

const readline = require('readline');

class Context {
    constructor(data, rl) {
        this.plugin_id = data.plugin_id || '';
        this.platform = data.platform || '';
        this.user_id = data.user_id || '';
        this.group_id = data.group_id || '';
        this.content = data.content || '';
        this.message_id = data.message_id || '';
        this.metadata = data.metadata || {};
        this._rl = rl;
    }

    async reply(text) {
        const action = { action: 'reply', text };
        process.stdout.write(JSON.stringify(action) + '\n');
        return true;
    }

    async send_image(image_url) {
        return false;
    }

    async listen(timeout = 60) {
        // 发送 listen 指令
        const action = { action: 'listen', timeout };
        process.stdout.write(JSON.stringify(action) + '\n');

        // 等待 stdin 返回用户回复
        return new Promise((resolve) => {
            const timer = setTimeout(() => {
                resolve('');
            }, (timeout + 5) * 1000);

            this._rl.once('line', (line) => {
                clearTimeout(timer);
                try {
                    const response = JSON.parse(line);
                    if (response.action === 'listen_response') {
                        resolve(response.content || '');
                    } else {
                        resolve('');
                    }
                } catch (e) {
                    resolve('');
                }
            });
        });
    }
}

function runDirect(handler) {
    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
        terminal: false
    });

    let firstLine = true;

    rl.on('line', async (line) => {
        if (!firstLine) return;
        firstLine = false;

        try {
            const messageData = JSON.parse(line);
            const ctx = new Context(messageData, rl);

            await handler(ctx);

            // 发送完成信号
            const done = { action: 'done', success: true };
            process.stdout.write(JSON.stringify(done) + '\n');
            process.exit(0);

        } catch (error) {
            const done = { action: 'done', success: false, error: error.message };
            process.stdout.write(JSON.stringify(done) + '\n');
            process.exit(1);
        }
    });
}

module.exports = { Context, runDirect };
