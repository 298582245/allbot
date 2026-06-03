//[title: wqwl_自定义回答]
//[language: nodejs]
//[class: 工具类]
//[service: qq298582245] 售后联系方式
//[disable: false] 禁用开关，true表示禁用，false表示可用
//[admin: false] 是否为管理员指令
//[rule: ^(.*)$]匹配规则1
//[priority: -10000000000000000000000000086] 优先级，数字越大表示优先级越高
//[platform: qq] 适用的平台
//[open_source: false]是否开源
//[icon: 图标url]图标链接地址，请使用48像素的正方形图标，48x48，支持http和https
//[version: 1.0.0]版本号
//[public: false] 是否发布？true或false，不设置则上传aut云时会自动设置为true，false时上传后不显示在市场中，但是搜索能搜索到，方便开发者测试
//[price: 0.01] 上架价格
//[description: 自定义回答] 使用方法尽量写具体

const middlleware = require('./middleware')
const axios = require('axios')

const BUCKET_NAME = "wqwl_auto_responses"

const isDebug = false;

class Response {
    constructor() {
        this.senderID = '';
        this.sender = '';
        this.isAdmin = '';
        this.message = '';
        this.allReplies = '';
        this.userId = '';
        this.chatId = '';
        this.matchedAtNumbers = [];
    }

    async init() {
        this.senderID = await middlleware.getSenderID()
        this.sender = new middlleware.Sender(this.senderID)
        this.userId = await this.sender.getUserID()
        this.chatId = await this.sender.getChatID()
        this.isAdmin = await this.sender.isAdmin()
        this.message = await this.sender.getMessage()
        this.allReplies = await this.sender.bucketAll(`${BUCKET_NAME}`)
    }

    async send(msg) {
        if (isDebug)
            await this.sender.reply(this.message)
        await this.sender.reply(msg)
    }

    async main() {
        const isAnswer = this.getAnswer()
        if (isAnswer) {
            const answer = await this.answerDetail(isAnswer)
            if (answer !== null) { // 检查是否返回null
                await this.send(answer)
            }
            return
        }

        if (this.message.includes('回复')) {
            const reply = this.getReply()
            if (reply)
                await this.send(reply)
            return
        }
        if (this.message.includes('测试'))
            await this.send(this.message)


    }

    //匹配自定义回复
    getAnswer() {
        // 精确匹配
        if (this.allReplies[this.message]) {
            return this.allReplies[this.message];
        }

        // 处理[CQ:at]消息，提取纯文本和数字部分
        let processedMessage = this.message;
        let atNumbers = [];

        // 匹配[CQ:at,qq=数字]格式，提取数字
        const atRegex = /\[CQ:at,qq=(\d+)\]/g;
        let match;
        while ((match = atRegex.exec(this.message)) !== null) {
            atNumbers.push(match[1]);
        }

        // 将[CQ:at,qq=数字]替换为[at]用于匹配
        processedMessage = this.message.replace(/\[CQ:at,qq=\d+\]/g, '[at]');

        // 先尝试用处理后的消息进行精确匹配
        if (this.allReplies[processedMessage]) {
            // 如果匹配到包含[at]的规则，将提取的数字传递给后续处理
            this.matchedAtNumbers = atNumbers;
            return this.allReplies[processedMessage];
        }

        // 通配符匹配（使用处理后的消息）
        for (const [key, value] of Object.entries(this.allReplies)) {
            if (key.includes('*')) {
                const pattern = '^' + key.replace(/\*/g, '.*') + '$';
                if (new RegExp(pattern).test(processedMessage)) {
                    // 如果匹配到包含[at]的规则，将提取的数字传递给后续处理
                    if (key.includes('[at]') || processedMessage.includes('[at]')) {
                        this.matchedAtNumbers = atNumbers;
                    }
                    return value;
                }
            }
        }

        return null;
    }

    //获取回复内容
    async answerDetail(value) {
        const group = value.split(':')
        const type = group[0];
        const data = group.slice(1).join(':'); // 处理data中可能包含冒号的情况
        switch (type) {
            case 'text':
                return data
            case 'img':
                return `[CQ:image,file=${data}]`
            case 'api':
                const apiResult = await this.getAPIData(data)
                return apiResult !== null ? apiResult : null // 如果返回null，则传递null
            case 'js': // 新增JS代码类型
                return await this.executeJSCode(data)
            default:
                return value; // 未知类型返回原值
        }
    }
    //请求API获取内容
    async getAPIData(apiUrl) {
        try {
            // 检查[at]数量是否匹配
            const atPlaceholders = (apiUrl.match(/\[at\]/g) || []).length;
            const actualAtCount = this.matchedAtNumbers.length;

            // 如果URL中的[at]数量多于实际收到的@数量，则不触发
            if (atPlaceholders > actualAtCount) {
                console.log(`[at]数量不匹配: URL需要${atPlaceholders}个，实际收到${actualAtCount}个`);
                return null; // 返回null表示不触发
            }

            // 替换通配符参数
            let finalUrl = apiUrl;

            // 处理[at]数字替换
            if (apiUrl.includes('[at]') && this.matchedAtNumbers.length > 0) {
                // 逐个替换[at]，如果at数量不够，用最后一个at填充
                let atIndex = 0;
                finalUrl = finalUrl.replace(/\[at\]/g, () => {
                    // 如果还有对应的at数字，使用它；否则使用最后一个at数字
                    const atNumber = atIndex < actualAtCount ?
                        this.matchedAtNumbers[atIndex] :
                        this.matchedAtNumbers[actualAtCount - 1];
                    atIndex++;
                    return encodeURIComponent(atNumber);
                });
            }


            // 处理普通通配符*
            else if (apiUrl.includes('*')) {
                // 找到匹配的key来提取参数
                let processedMessage = this.message.replace(/\[CQ:at,qq=\d+\]/g, '[at]');
                const matchedKey = Object.keys(this.allReplies).find(key => {
                    if (key.includes('*')) {
                        const pattern = '^' + key.replace(/\*/g, '.*') + '$';
                        return new RegExp(pattern).test(processedMessage);
                    }
                    return false;
                });

                if (matchedKey) {
                    const pattern = '^' + matchedKey.replace(/\*/g, '(.*)') + '$';
                    const match = processedMessage.match(new RegExp(pattern));
                    if (match && match[1]) {
                        finalUrl = apiUrl.replace(/\*/g, encodeURIComponent(match[1]));
                    }
                }
            }

            // 新增：替换 userId 和 chatId 占位符
            finalUrl = finalUrl.replace(/\[userId\]/g, encodeURIComponent(this.userId || ''))
                .replace(/\[chatId\]/g, encodeURIComponent(this.chatId || ''));

            console.log('请求API:', finalUrl);

            // 其余API请求代码保持不变...
            const response = await axios.get(finalUrl, {
                timeout: 10000,
                responseType: 'arraybuffer'
            });

            if (response.status !== 200) {
                throw new Error(`HTTP ${response.status}`);
            }

            const contentType = response.headers['content-type'] || '';
            const data = response.data;

            console.log('响应类型:', contentType);

            if (contentType.includes('image/')) {
                return `[CQ:image,file=${finalUrl}]`;
            }

            let textData;
            if (Buffer.isBuffer(data)) {
                textData = data.toString('utf-8');
            } else {
                textData = data;
            }

            console.log('API响应内容:', textData.substring(0, 200));

            if (textData.match(/^https?:\/\/.*\.(jpg|jpeg|png|gif|bmp|webp)(\?.*)?$/i)) {
                return `[CQ:image,file=${textData.trim()}]`;
            }

            try {
                const jsonData = JSON.parse(textData);
                if (jsonData.imageUrl || jsonData.img || jsonData.url || jsonData.pic) {
                    const imageUrl = jsonData.imageUrl || jsonData.img || jsonData.url || jsonData.pic;
                    if (typeof imageUrl === 'string' && imageUrl.match(/^https?:\/\//)) {
                        return `[CQ:image,file=${imageUrl}]`;
                    }
                }
                return jsonData.text || jsonData.message || jsonData.msg || JSON.stringify(jsonData);
            } catch (jsonError) {
                return textData;
            }

        } catch (error) {
            console.error('API请求错误:', error.message);
            return 'API请求失败，请稍后重试';
        }
    }

    //获取菜单回复
    async getReply() {
        if (this.message === '回复列表') {
            const data = await this.sender.bucketAllKeys(`${BUCKET_NAME}`)
            await this.paginateData(data, '回复列表')
            return true
        }

        // 管理员功能
        if (this.isAdmin) {
            if (this.message === '添加回复') {
                await this.addReply()
                return true
            }

            if (this.message === '删除回复') {
                await this.deleteReply()
                return true
            }
        }

        return false
    }

    // 添加回复
    async addReply() {
        try {
            // 步骤1：获取key
            await this.send('请输入触发关键词（支持通配符*）：')
            const key = await this.sender.listen(30000)

            if (!key) {
                await this.send('输入超时，已取消添加回复')
                return
            }

            // 检查key是否已存在
            const existingData = await this.sender.bucketAll(`${BUCKET_NAME}`)
            if (existingData[key]) {
                await this.send(`关键词 "${key}" 已存在，请重新输入`)
                return
            }

            // 步骤2：选择回复类型
            await this.send('请选择回复类型：\n1. 文本回复\n2. 图片回复\n3. API回复\n4. JS代码回复\n请输入数字 1-4：')
            const typeInput = await this.sender.listen(30000)

            if (!typeInput) {
                await this.send('输入超时，已取消添加回复')
                return
            }

            let replyType, typeName
            switch (typeInput.trim()) {
                case '1':
                    replyType = 'text'
                    typeName = '文本回复'
                    break
                case '2':
                    replyType = 'img'
                    typeName = '图片回复'
                    break
                case '3':
                    replyType = 'api'
                    typeName = 'API回复'
                    break
                case '4': // 新增JS代码类型
                    replyType = 'js'
                    typeName = 'JS代码回复'
                    break
                default:
                    await this.send('输入无效，已取消添加回复')
                    return
            }

            // 步骤3：获取value值
            let prompt = ''
            if (replyType === 'text') {
                prompt = `请输入文本回复内容（${typeName}）：`
            } else if (replyType === 'img') {
                prompt = `请输入图片地址（${typeName}）：`
            } else if (replyType === 'api') {
                prompt = `请输入API地址（${typeName}，支持通配符*）：\n例如：https://api.example.com/search?keyword=*`
            }
            else if (replyType === 'js') {
                prompt = `请输入JS代码（${typeName}）：\n例如：return "当前时间：" + new Date().toLocaleString();\n注意：代码必须包含return语句返回结果`
            }

            await this.send(prompt)
            const valueInput = await this.sender.listen(30000)

            if (!valueInput) {
                await this.send('输入超时，已取消添加回复')
                return
            }

            // 步骤4：拼接value并存储
            const finalValue = `${replyType}:${valueInput}`
            await this.sender.bucketSet(`${BUCKET_NAME}`, key, finalValue)

            await this.send(`✅ 添加成功！\n关键词：${key}\n类型：${typeName}\n内容：${valueInput}`)

        } catch (error) {
            console.error('添加回复错误:', error)
            await this.send('添加回复过程中出现错误，请重试')
        }
    }

    // 删除回复
    async deleteReply() {
        try {
            // 获取所有回复列表供用户选择
            const allReplies = await this.sender.bucketAll(`${BUCKET_NAME}`)
            const keys = Object.keys(allReplies)

            if (keys.length === 0) {
                await this.send('暂无自定义回复可删除')
                return
            }

            // 显示回复列表供选择
            let message = '📋 请选择要删除的回复（输入序号）：\n'
            message += '─'.repeat(30) + '\n'

            keys.forEach((key, index) => {
                const value = allReplies[key]
                const [type, content] = value.split(':')
                const shortContent = content.length > 30 ? content.substring(0, 30) + '...' : content
                message += `${index + 1}. ${key} → [${type}] ${shortContent}\n`
            })

            message += '\n请输入要删除的回复序号（输入"取消"退出）：'

            await this.send(message)
            const userInput = await this.sender.listen(30000)

            if (!userInput) {
                await this.send('输入超时，已取消删除')
                return
            }

            if (userInput === '取消') {
                await this.send('已取消删除操作')
                return
            }

            const selectedIndex = parseInt(userInput) - 1

            if (isNaN(selectedIndex) || selectedIndex < 0 || selectedIndex >= keys.length) {
                await this.send('序号无效，请重新输入')
                return
            }

            const selectedKey = keys[selectedIndex]
            const selectedValue = allReplies[selectedKey]
            const [type, content] = selectedValue.split(':')

            // 确认删除
            await this.send(`确定要删除以下回复吗？\n关键词：${selectedKey}\n类型：${type}\n内容：${content}\n\n请输入"确认"来确认删除，输入其他内容取消：`)
            const confirmInput = await this.sender.listen(30000)

            if (confirmInput === '确认') {
                await this.sender.bucketDel(`${BUCKET_NAME}`, selectedKey)
                await this.send('✅ 删除成功！')
            } else {
                await this.send('已取消删除')
            }

        } catch (error) {
            console.error('删除回复错误:', error)
            await this.send('删除回复过程中出现错误，请重试')
        }
    }
    // 通用分页函数
    async paginateData(data, title = '列表', pageSize = 10) {
        let currentPage = 0
        const totalPages = Math.ceil(data.length / pageSize)

        // 显示第一页
        await this.sendPage(data, title, currentPage, totalPages, pageSize)

        // 如果只有一页，直接返回
        if (totalPages <= 1) return

        // 多页情况下的翻页循环
        while (true) {
            // 监听用户输入
            try {
                const userInput = await this.sender.listen(30000) // 30秒超时

                if (userInput === '下一页' && currentPage < totalPages - 1) {
                    currentPage++
                    await this.sendPage(data, title, currentPage, totalPages, pageSize)
                } else if (userInput === '上一页' && currentPage > 0) {
                    currentPage--
                    await this.sendPage(data, title, currentPage, totalPages, pageSize)
                } else if (userInput === '退出') {
                    await this.send('已退出列表查看')
                    break
                } else {
                    await this.send('已退出列表查看')
                    break
                }
            } catch (error) {
                await this.send('列表查看已超时')
                break
            }
        }
    }

    // 新增：发送单页内容的辅助函数
    async sendPage(data, title, currentPage, totalPages, pageSize) {
        const start = currentPage * pageSize
        const end = start + pageSize
        const pageData = data.slice(start, end)

        let message = `📋 ${title} (${currentPage + 1}/${totalPages})\n`
        message += '─'.repeat(15) + '\n'

        pageData.forEach((item, index) => {
            message += `${start + index + 1}. ${item}\n`
        })

        // 添加翻页提示
        if (totalPages > 1) {
            message += '\n📄 指令: '
            if (currentPage > 0) message += '"上一页" '
            if (currentPage < totalPages - 1) message += '"下一页" '
            message += '"退出"'
        } else {
            message += '\n✅ 已显示所有内容'
        }

        await this.send(message)
    }

    async executeJSCode(code) {
        let vm2;
        try {
            vm2 = require('vm2');
        } catch (error) {
            console.error('vm2模块未安装');
            return 'JS执行环境未安装';
        }

        try {
            const vm = new vm2.NodeVM({
                console: 'inherit',
                sandbox: this.createJSSandbox(),
                timeout: 8000,
                wrapper: 'commonjs',
                require: {
                    external: true,
                    builtin: ['http', 'https']
                }
            });

            // 自动包装为模块并执行
            const asyncModule = vm.run(`
                module.exports = async function() {
                    ${code}
                }
            `);

            // asyncModule 是一个异步函数
            const result = await asyncModule();

            return result;

        } catch (error) {
            console.error("JS执行错误:", error);
            return "JS执行错误: " + error.message;
        }
    }



    createJSSandbox() {
        const http = require('http');
        const https = require('https');
        const crypto = require('crypto')
        return {

            // 当前消息相关信息
            message: this.message,
            atNumbers: this.matchedAtNumbers,
            userId: this.userId,
            chatId: this.chatId,
            http: http,
            https: https,
            crypto: crypto,

            //http
            fetch: (...args) => fetch(...args),

            // 基础工具函数
            console: {
                log: (...args) => console.log('[JS Code]', ...args),
            },
            // 安全的数学和工具函数
            Math: Math,
            Date: Date,
            String: String,
            Number: Number,
            Boolean: Boolean,
            Array: Array,
            Object: Object,
            JSON: JSON,
            // URL编码函数
            encodeURIComponent: encodeURIComponent,
            decodeURIComponent: decodeURIComponent,
            // 限制版的定时器
            setTimeout: (fn, delay) => {
                if (delay > 3000) delay = 3000;
                return setTimeout(fn, delay);
            },
            setInterval: (fn, delay) => {
                if (delay > 3000) delay = 3000;
                return setInterval(fn, delay);
            },
            clearTimeout: clearTimeout,
            clearInterval: clearInterval,
        }
    }

}

!(async function () {
    const response = new Response()
    await response.init()
    await response.main()

})()

