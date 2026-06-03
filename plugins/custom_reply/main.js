const path = require('path');
const axios = require('axios');

const sdkPath = path.join(__dirname, '../../sdk/nodejs');
const { runDirect } = require(path.join(sdkPath, 'allbot_direct'));

const TABLE_NAME = 'responses';
const IMAGE_URL_PATTERN = /^https?:\/\/.*\.(jpg|jpeg|png|gif|bmp|webp)(\?.*)?$/i;

class CustomReplyPlugin {
    constructor(ctx) {
        this.ctx = ctx;
        this.message = (ctx.content || '').trim();
        this.userId = ctx.userId;
        this.chatId = ctx.chatId();
        this.replies = [];
        this.matchedAtNumbers = [];
    }

    async init() {
        await this.ctx.db.createTable(TABLE_NAME, [
            { name: 'keyword', type: 'TEXT' },
            { name: 'reply_type', type: 'TEXT' },
            { name: 'content', type: 'TEXT' }
        ]);
        await this.ctx.db.setView(TABLE_NAME, {
            viewName: '自定义回答',
            groupName: '插件数据',
            description: '自定义回答插件保存的关键词和回复内容',
            columns: ['id', 'keyword', 'reply_type', 'content', 'created_at', 'updated_at']
        });
        const data = await this.ctx.db.query(TABLE_NAME, { orderBy: 'id', orderDir: 'ASC', size: 500 });
        this.replies = data.rows || [];
    }

    async main() {
        const matched = this.matchReply();
        if (matched) {
            await this.sendReply(matched);
            return;
        }

        if (this.message === '回复列表') {
            await this.showReplyList();
            return;
        }
        if (this.message === '添加回复') {
            await this.addReply();
            return;
        }
        if (this.message === '删除回复') {
            await this.deleteReply();
            return;
        }
        if (this.message.includes('测试')) {
            await this.ctx.reply(this.message);
        }
    }

    matchReply() {
        const exact = this.replies.find((item) => item.keyword === this.message);
        if (exact) return exact;

        const atNumbers = [];
        const atRegex = /\[CQ:at,qq=(\d+)\]/g;
        let match;
        while ((match = atRegex.exec(this.message)) !== null) atNumbers.push(match[1]);

        const processedMessage = this.message.replace(/\[CQ:at,qq=\d+\]/g, '[at]');
        const atExact = this.replies.find((item) => item.keyword === processedMessage);
        if (atExact) {
            this.matchedAtNumbers = atNumbers;
            return atExact;
        }

        for (const item of this.replies) {
            if (!item.keyword.includes('*')) continue;
            const pattern = '^' + escapeRegExp(item.keyword).replace(/\\\*/g, '.*') + '$';
            if (new RegExp(pattern).test(processedMessage)) {
                if (item.keyword.includes('[at]') || processedMessage.includes('[at]')) this.matchedAtNumbers = atNumbers;
                return item;
            }
        }
        return null;
    }

    async sendReply(reply) {
        const type = reply.reply_type || 'text';
        const content = reply.content || '';
        if (type === 'img') {
            await this.ctx.sendImage(content);
            return;
        }
        if (type === 'api') {
            const result = await this.fetchApiReply(content);
            if (result) await this.outputAutoReply(result);
            return;
        }
        if (type === 'js') {
            await this.ctx.reply(this.executeJSCode(content));
            return;
        }
        await this.ctx.reply(content);
    }

    async fetchApiReply(apiUrl) {
        try {
            let finalUrl = this.fillPlaceholders(apiUrl);
            if (!finalUrl) return '';

            const response = await axios.get(finalUrl, { timeout: 10000, responseType: 'arraybuffer' });
            const contentType = response.headers['content-type'] || '';
            if (contentType.includes('image/')) return { type: 'img', content: finalUrl };

            const textData = Buffer.isBuffer(response.data) ? response.data.toString('utf-8') : String(response.data || '');
            const text = textData.trim();
            if (IMAGE_URL_PATTERN.test(text)) return { type: 'img', content: text };

            try {
                const jsonData = JSON.parse(text);
                const imageUrl = jsonData.imageUrl || jsonData.img || jsonData.url || jsonData.pic;
                if (typeof imageUrl === 'string' && /^https?:\/\//.test(imageUrl)) return { type: 'img', content: imageUrl };
                return { type: 'text', content: jsonData.text || jsonData.message || jsonData.msg || JSON.stringify(jsonData) };
            } catch (error) {
                return { type: 'text', content: text };
            }
        } catch (error) {
            return { type: 'text', content: 'API请求失败，请稍后重试' };
        }
    }

    fillPlaceholders(apiUrl) {
        const needAtCount = (apiUrl.match(/\[at\]/g) || []).length;
        if (needAtCount > this.matchedAtNumbers.length) return '';

        let finalUrl = apiUrl;
        if (apiUrl.includes('[at]') && this.matchedAtNumbers.length > 0) {
            let atIndex = 0;
            finalUrl = finalUrl.replace(/\[at\]/g, () => encodeURIComponent(this.matchedAtNumbers[Math.min(atIndex++, this.matchedAtNumbers.length - 1)]));
        } else if (apiUrl.includes('*')) {
            const processedMessage = this.message.replace(/\[CQ:at,qq=\d+\]/g, '[at]');
            const matched = this.replies.find((item) => item.keyword.includes('*') && new RegExp('^' + escapeRegExp(item.keyword).replace(/\\\*/g, '(.*)') + '$').test(processedMessage));
            if (matched) {
                const pattern = '^' + escapeRegExp(matched.keyword).replace(/\\\*/g, '(.*)') + '$';
                const match = processedMessage.match(new RegExp(pattern));
                if (match && match[1]) finalUrl = apiUrl.replace(/\*/g, encodeURIComponent(match[1]));
            }
        }
        return finalUrl
            .replace(/\[userId\]/g, encodeURIComponent(this.userId || ''))
            .replace(/\[chatId\]/g, encodeURIComponent(this.chatId || ''));
    }

    async outputAutoReply(result) {
        if (typeof result === 'string') {
            await this.ctx.reply(result);
            return;
        }
        if (result.type === 'img') await this.ctx.sendImage(result.content);
        else await this.ctx.reply(result.content || '');
    }

    async showReplyList() {
        if (this.replies.length === 0) {
            await this.ctx.reply('暂无自定义回复');
            return;
        }
        const items = this.replies.map((item) => `${item.keyword} → [${item.reply_type}] ${shortText(item.content, 40)}`);
        await this.paginateData(items, '回复列表', 10);
    }

    async addReply() {
        if (!this.ctx.isAdmin()) {
            await this.ctx.reply('无权操作')
            return
        }
        await this.ctx.reply('请输入触发关键词（支持通配符*，例如：天气*）：');
        const keyword = (await this.ctx.listen(30)).trim();
        if (!keyword) return this.ctx.reply('输入超时，已取消添加回复');
        if (this.replies.some((item) => item.keyword === keyword)) return this.ctx.reply(`关键词 "${keyword}" 已存在，请重新输入`);

        await this.ctx.reply('请选择回复类型：\n1. 文本回复\n2. 图片回复\n3. API回复\n4. JS代码回复\n请输入数字 1-4：');
        const typeInput = (await this.ctx.listen(30)).trim();
        const typeMap = { 1: ['text', '文本回复'], 2: ['img', '图片回复'], 3: ['api', 'API回复'], 4: ['js', 'JS代码回复'] };
        const selected = typeMap[typeInput];
        if (!selected) return this.ctx.reply('输入无效，已取消添加回复');
        const [replyType, typeName] = selected;

        const promptMap = {
            text: `请输入文本回复内容（${typeName}）：`,
            img: `请输入图片地址（${typeName}）：`,
            api: `请输入API地址（${typeName}，支持通配符*、[userId]、[chatId]、[at]）：\n例如：https://api.example.com/search?keyword=*`,
            js: `请输入JS代码（${typeName}）：\n例如：return "当前时间：" + new Date().toLocaleString();`
        };
        await this.ctx.reply(promptMap[replyType]);
        const content = await this.ctx.listen(30);
        if (!content) return this.ctx.reply('输入超时，已取消添加回复');

        await this.ctx.db.insert(TABLE_NAME, { keyword, reply_type: replyType, content });
        await this.ctx.reply(`✅ 添加成功！\n关键词：${keyword}\n类型：${typeName}\n内容：${content}`);
    }

    async deleteReply() {
        if (this.replies.length === 0) return this.ctx.reply('暂无自定义回复可删除');

        let message = '📋 请选择要删除的回复（输入序号）：\n' + '─'.repeat(30) + '\n';
        this.replies.forEach((item, index) => {
            message += `${index + 1}. ${item.keyword} → [${item.reply_type}] ${shortText(item.content, 30)}\n`;
        });
        message += '\n请输入要删除的回复序号（输入"取消"退出）：';
        await this.ctx.reply(message);

        const input = (await this.ctx.listen(30)).trim();
        if (!input) return this.ctx.reply('输入超时，已取消删除');
        if (input === '取消') return this.ctx.reply('已取消删除操作');

        const index = Number.parseInt(input, 10) - 1;
        if (Number.isNaN(index) || index < 0 || index >= this.replies.length) return this.ctx.reply('序号无效，请重新输入');

        const selected = this.replies[index];
        await this.ctx.reply(`确定要删除以下回复吗？\n关键词：${selected.keyword}\n类型：${selected.reply_type}\n内容：${selected.content}\n\n请输入"确认"来确认删除，输入其他内容取消：`);
        const confirm = (await this.ctx.listen(30)).trim();
        if (confirm !== '确认') return this.ctx.reply('已取消删除');

        await this.ctx.db.delete(TABLE_NAME, selected.__rowid__ || selected.id);
        await this.ctx.reply('✅ 删除成功！');
    }

    async paginateData(data, title = '列表', pageSize = 10) {
        let currentPage = 0;
        const totalPages = Math.ceil(data.length / pageSize);
        await this.sendPage(data, title, currentPage, totalPages, pageSize);
        while (totalPages > 1) {
            const input = (await this.ctx.listen(30)).trim();
            if (input === '下一页' && currentPage < totalPages - 1) currentPage++;
            else if (input === '上一页' && currentPage > 0) currentPage--;
            else {
                await this.ctx.reply(input ? '已退出列表查看' : '列表查看已超时');
                break;
            }
            await this.sendPage(data, title, currentPage, totalPages, pageSize);
        }
    }

    async sendPage(data, title, currentPage, totalPages, pageSize) {
        const start = currentPage * pageSize;
        const pageData = data.slice(start, start + pageSize);
        let message = `📋 ${title} (${currentPage + 1}/${totalPages})\n` + '─'.repeat(15) + '\n';
        pageData.forEach((item, index) => {
            message += `${start + index + 1}. ${item}\n`;
        });
        if (totalPages > 1) {
            message += '\n📄 指令: ';
            if (currentPage > 0) message += '"上一页" ';
            if (currentPage < totalPages - 1) message += '"下一页" ';
            message += '"退出"';
        } else {
            message += '\n✅ 已显示所有内容';
        }
        await this.ctx.reply(message);
    }

    executeJSCode(code) {
        try {
            const fn = new Function('message', 'userId', 'chatId', 'atNumbers', 'encodeURIComponent', 'decodeURIComponent', 'JSON', 'Math', 'Date', code);
            const result = fn(this.message, this.userId, this.chatId, this.matchedAtNumbers, encodeURIComponent, decodeURIComponent, JSON, Math, Date);
            return result === undefined || result === null ? '' : String(result);
        } catch (error) {
            return 'JS执行错误: ' + error.message;
        }
    }
}

function escapeRegExp(text) {
    return String(text).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function shortText(text, maxLength) {
    const value = String(text || '');
    return value.length > maxLength ? value.slice(0, maxLength) + '...' : value;
}

async function handle(ctx) {
    const plugin = new CustomReplyPlugin(ctx);
    await plugin.init();
    await plugin.main();
}

runDirect(handle);
