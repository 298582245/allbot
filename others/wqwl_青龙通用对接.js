//[title: wqwl_青龙通用对接]
//[author: wqwlkj2985]
//[language: nodejs]
//[class: 工具类]
//[service: qq298582245] 售后联系方式
//[disable: false] 禁用开关，true表示禁用，false表示可用
//[admin: false] 是否为管理员指令
//[rule: ^(.*)(登录|查询|管理|日志同步|管理|检测|教程|管理查询|管理授权|日志测试)|项目(配置|列表|日志同步|检测)$] 匹配规则1
//[cron: 38 0,22 * * *] cron定时，支持5位域和6位域
//[priority: -100] 优先级，数字越大表示优先级越高
//[platform: qq] 适用的平台
//[open_source: false]是否开源
//[icon: 图标url]图标链接地址，请使用48像素的正方形图标，支持http和https
//[version: 1.0.0]版本号
//[public: false] 是否发布？值为true或false，不设置则上传aut云时会自动设置为true，false时上传后不显示在市场中，但是搜索能搜索到，方便开发者测试
//[price: 999] 上架价格
//[description:] 使用方法尽量写具体
//[param: {"required":true,"key":"wqwl_config.qinglong","bool":false,"placeholder":"Host|cilentId|cilentSecret","name":"对接容器","desc":"各参数之间用中文符丨分割"}]
//[param: {"spliter":true}]
//[param: {"required":true,"key":"wqwl_config.pay","bool":false,"placeholder":"易支付地址（仅支持彩虹mapi.php接口），例如:https://baidu.com","name":"易支付地址","desc":"末尾不要加/"}]
//[param: {"required":true,"key":"wqwl_config.pay_id","bool":false,"placeholder":"易支付ID","name":"易支付ID","desc":"易支付ID"}]
//[param: {"required":true,"key":"wqwl_config.pay_key","bool":false,"placeholder":"易支付密钥","name":"易支付密钥","desc":"易支付密钥"}]
//[param: {"required":true,"key":"wqwl_config.pay_type","bool":false,"placeholder":"易支付支付方式","name":"易支付支付方式","desc":"易支付支付方式，跟你系统有关，格式：系统字段:显示名称：比如alipay:支付宝,wxpay:微信支付"}]
//[param: {"spliter":true}]
//[param: {"required":true,"key":"wqwl_config.qr_url","bool":false,"placeholder":"二维码生成地址","name":"二维码生成地址","desc":"不填无法使用"}]
//[param: {"spliter":true}]
//[param: {"required":false,"key":"wqwl_config.apikey","bool":false,"placeholder":"远程配置APIkey","name":"远程配置APIkey","desc":"暂时没有用"}]
const middlleware = require('./middleware')
const axios = require('axios');
const crypto = require('crypto');
const { push } = require('./middleware')



class QL_API {
    constructor(url, id, secret) {
        this.url = url;
        this.id = id;
        this.secret = secret;
        this.token = "";
    }

    async getToken() {
        try {
            const res = await axios.get(`${this.url}/open/auth/token`, {
                params: {
                    client_id: this.id,
                    client_secret: this.secret
                }
            });
            console.log("获取 Token 成功:", res.data);
            this.token = res.data.data.token;
            return this.token;
        } catch (error) {
            console.error("获取 Token 失败:", error.response?.data || error.message);
            throw error;
        }
    }
}

class Tasks extends QL_API {
    constructor(token, url = null, id = null, secret = null) {
        super(url, id, secret);
        this.token = token;
        this.headers = {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.token}`,
            'Accept': 'application/json'
        };
    }

    async timedTask() {
        try {
            const res = await axios.get(`${this.url}/open/crons/views`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("获取定时任务视图失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async getTimedTask() {
        try {
            const res = await axios.get(`${this.url}/open/crons`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("获取所有定时任务失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async getTasksDetail(id) {
        try {
            const res = await axios.get(`${this.url}/open/crons/${id}`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`获取任务 ${id} 详情失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    async createTasks({
        command = null,
        schedule = null,
        name = null,
        labels = [],
        sub_id = null,
        extra_schedules = null,
        task_before = null,
        task_after = null
    }) {
        const taskData = {
            command,
            schedule,
            name,
            labels,
            sub_id,
            extra_schedules,
            task_before,
            task_after
        };

        try {
            const res = await axios.post(`${this.url}/open/crons`, taskData, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("创建任务失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async runTask(taskIds) {
        try {
            const res = await axios.put(`${this.url}/open/crons/run`, taskIds, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("运行任务失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async stopTask(taskIds) {
        try {
            const res = await axios.put(`${this.url}/open/crons/stop`, taskIds, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("停止任务失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async getTasksLogs(taskId) {
        try {
            const res = await axios.get(`${this.url}/open/crons/${taskId}/log`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`获取任务 ${taskId} 日志失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    async toppingTasks(taskIds) {
        try {
            const res = await axios.put(`${this.url}/open/crons/pin`, taskIds, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("置顶任务失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async unToppingTasks(taskIds) {
        try {
            const res = await axios.put(`${this.url}/open/crons/unpin`, taskIds, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("取消置顶任务失败:", error.response?.data || error.message);
            throw error;
        }
    }
}

class Env extends Tasks {
    constructor(token, url = null) {
        super(token, url);
        this.token = token;
    }

    async createVariable(name, value, remarks) {
        const data = [{
            name,
            value,
            remarks
        }];

        try {
            const res = await axios.post(`${this.url}/open/envs`, data, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("创建变量失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async getEnvList(queryKeyWord = null) {
        try {
            const res = await axios.get(`${this.url}/open/envs`, {
                headers: this.headers,
                params: { searchValue: queryKeyWord }
            });
            return res.data;
        } catch (error) {
            console.error("获取环境变量列表失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async getSingleEnv(envId) {
        try {
            const res = await axios.get(`${this.url}/open/envs/${envId}`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`获取环境变量 ${envId} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    async delVariable(envIds) {
        try {
            const res = await axios.delete(`${this.url}/open/envs`, {
                headers: this.headers,
                data: envIds
            });
            return res.data;
        } catch (error) {
            console.error("删除变量失败:", error.response?.data || error.message);
            throw error;
        }
    }
    async disableVariable(envIds) {
        //console.log(envIds, `${this.url}/open/envs/disable`, this.headers)
        try {
            const res = await axios.put(`${this.url}/open/envs/disable`, envIds, {
                headers: this.headers
            });
            return res.data;
        } catch (error) {
            console.error("禁用变量失败:", error.response?.data || error.message);
            throw error;
        }
    }
    async enableVariable(envIds) {
        //console.log(envIds, `${this.url}/open/envs/enable`, this.headers)
        try {
            const res = await axios.put(`${this.url}/open/envs/enable`, envIds, {
                headers: this.headers
            });
            return res.data;
        } catch (error) {
            console.error("启用变量失败:", error.response?.data || error.message);
            throw error;
        }
    }
    async updateVariable(envId, name, value, remarks) {
        const data = {
            id: envId,
            name,
            value,
            remarks
        };

        try {
            const res = await axios.put(`${this.url}/open/envs`, data, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`更新环境变量 ${envId} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }
}

class Logs extends Tasks {
    constructor(token, url = null) {
        super(token, url);
        this.token = token;
    }

    async logs() {
        try {
            const res = await axios.get(`${this.url}/open/logs`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("获取日志列表失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async detailLogs(logPath, fileName) {
        try {
            const res = await axios.get(`${this.url}/open/logs/detail/?path=${logPath}&file=${fileName}`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`获取日志详情 ${logPath}/${fileName} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    async FileLogs(logPath, key) {
        try {
            const res = await axios.get(`${this.url}/open/logs/${logPath}?path=${key}`, { headers: this.headers },);
            return res.data;
        } catch (error) {
            console.error(`获取日志文件 ${logPath} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }
}

class Scripts extends Tasks {
    constructor(token, url = null) {
        super(token, url);
        this.token = token;
    }

    /**
     * 获取脚本列表
     * @returns {Promise<Object>} 脚本列表数据
     */
    async list() {
        try {
            const res = await axios.get(`${this.url}/open/scripts`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("获取脚本列表失败:", error.response?.data || error.message);
            throw error;
        }
    }

    /**
     * 获取脚本详情
     * @param {string} path 脚本路径
     * @returns {Promise<Object>} 脚本详情数据
     */
    async detail(path) {
        try {
            const res = await axios.get(`${this.url}/open/scripts/detail?path=${encodeURIComponent(path)}`, {
                headers: this.headers
            });
            return res.data;
        } catch (error) {
            console.error(`获取脚本详情 ${path} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    /**
     * 保存脚本
     * @param {string} path 脚本路径
     * @param {string} content 脚本内容
     * @returns {Promise<Object>} 保存结果
     */
    async save(path, content) {
        try {
            const res = await axios.post(`${this.url}/open/scripts/save`, {
                path,
                content
            }, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`保存脚本 ${path} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    /**
     * 删除脚本
     * @param {string} path 脚本路径
     * @returns {Promise<Object>} 删除结果
     */
    async remove(path) {
        try {
            const res = await axios.delete(`${this.url}/open/scripts/delete?path=${encodeURIComponent(path)}`, {
                headers: this.headers
            });
            return res.data;
        } catch (error) {
            console.error(`删除脚本 ${path} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }

    /**
     * 执行脚本
     * @param {string} path 脚本路径
     * @param {Object} [params={}] 执行参数
     * @returns {Promise<Object>} 执行结果
     */
    async run(path, params = {}) {
        try {
            const res = await axios.post(`${this.url}/open/scripts/run`, {
                path,
                ...params
            }, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error(`执行脚本 ${path} 失败:`, error.response?.data || error.message);
            throw error;
        }
    }
}

class System extends QL_API {
    async basicInformation() {
        try {
            const res = await axios.get(`${this.url}/open/system`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("获取系统信息失败:", error.response?.data || error.message);
            throw error;
        }
    }

    async getSystemConfig() {
        try {
            const res = await axios.get(`${this.url}/open/config`, { headers: this.headers });
            return res.data;
        } catch (error) {
            console.error("获取系统配置失败:", error.response?.data || error.message);
            throw error;
        }
    }
}

class Pay {
    constructor(url, pid, token) {
        this.url = url;
        this.pid = pid;
        this.token = token;
        this.headers = {
            'Content-Type': 'application/x-www-form-urlencoded',
        }
    }

    async createPay(type, notify_url, name, money, ip, sign_type, path) {
        // 手动设置默认值
        type = type === undefined || type === '' ? 'alipay' : type;
        notify_url = notify_url === undefined || notify_url === '' ? this.url + '/notify_url.php' : notify_url;
        name = name === undefined || name === '' ? '积分充值' : name;
        money = money === undefined || money === '' ? 1.00 : money;
        ip = ip === undefined || ip === '' ? '127.0.0.1' : ip;
        sign_type = sign_type === undefined || sign_type === '' ? 'MD5' : sign_type;
        path = path === undefined || path === '' ? '/mapi.php' : path;
        let params = {
            pid: this.pid,
            type: type,
            notify_url: notify_url,
            name: name,
            money: money,
            clientip: ip,
            sign_type: sign_type,
            out_trade_no: this.randomOutTradeNo(),
        };
        const sign = this.signautre(params);

        params = this.sortsParams(params);
        params += `&sign=${sign}&sign_type=${sign_type}`
        // console.log(params);
        try {
            const config = {
                url: this.url + path,
                method: 'POST',
                headers: this.headers,
                data: params,
                timeout: 30000,
            }
            const response = await axios(config);
            return response.data;
        } catch (error) {
            console.error('创建订单抛出异常', error);
            return null;
        }

    }

    async queryPay(trade_no, path = '/api.php') {
        let params = {
            act: 'order',
            pid: this.pid,
            key: this.token,
            trade_no: trade_no,
        };

        params = this.sortsParams(params);
        // console.log(params);
        const url = `${this.url}${path}?${params}`;
        console.log(url);
        try {
            const config = {
                url: url,
                headers: this.headers,
                method: 'GET'
            }
            const response = await axios(config);
            return response.data;
        } catch (e) {
            console.log('查询订单抛出异常', e);
            return null;
        }
    }

    async checkOrderStatus(out_trade_no, interval = 5, timeout = 60) {
        return new Promise((resolve, reject) => {
            let elapsed = 0;
            const timer = setInterval(async () => {
                try {
                    const data = await this.queryPay(out_trade_no);

                    console.log(`⏳支付订单结果监控中`)
                    if (data && data.status === "1") {
                        clearInterval(timer);
                        resolve({ success: true, data: data });
                        return;
                    }

                    elapsed += interval;

                    if (elapsed >= timeout) {
                        clearInterval(timer);
                        resolve({ success: false, reason: '用户未支付', data: data });
                    }
                } catch (error) {
                    console.log('监控订单抛出异常', error);
                    clearInterval(timer);
                    reject(error);

                }
            }, interval * 1000);
        });
    }

    randomOutTradeNo(prefix = 'WQ') {
        const now = new Date();
        // 获取年月日
        const year = now.getFullYear();
        const month = String(now.getMonth() + 1).padStart(2, '0'); // 月份从0开始
        const day = String(now.getDate()).padStart(2, '0');
        // 获取小时和分钟
        const hours = String(now.getHours()).padStart(2, '0');
        const minutes = String(now.getMinutes()).padStart(2, '0');
        // 生成5位随机数字
        const randomNum = String(Math.floor(Math.random() * 90000000) + 100000000);
        // 拼接订单号
        return `${prefix}${year}${month}${day}${hours}${minutes}${randomNum}`;
    }

    signautre(params) {
        params = this.sortsParams(params);
        return crypto.createHash('md5').update(params + this.token).digest('hex').toLowerCase();
    }

    sortsParams(data) {
        // 过滤掉 sign、sign_type 和空值
        const filtered = {};
        for (const key in data) {
            if (
                data.hasOwnProperty(key) &&
                key !== 'sign' &&
                key !== 'sign_type' &&
                data[key] !== null &&
                data[key] !== undefined &&
                data[key] !== ''
            ) {
                filtered[key] = data[key];
            }
        }
        const sortedKeys = Object.keys(filtered).sort();
        const result = sortedKeys.map(key => `${key}=${filtered[key]}`).join('&');
        return result;
    }
}
// ==================== 常量 ====================
const CONFIG_TABLE = 'wqwl_config_projects'
const CONFIG_INDEX_KEY = '_index'
const USER_POINTS_TABLE = 'dd_sign_points'

// ==================== 辅助函数 ====================
function safeParseArray(raw) {
    if (!raw) return []
    if (Array.isArray(raw)) return raw
    if (typeof raw !== 'string') return []
    try {
        const parsed = JSON.parse(raw)
        return Array.isArray(parsed) ? parsed : []
    } catch (e) {
        return []
    }
}

function safeParseObject(raw) {
    if (!raw) return null
    if (typeof raw === 'object') return raw
    try {
        return JSON.parse(raw)
    } catch (e) {
        return null
    }
}

function formatDate(timestamp = Date.now()) {
    const date = new Date(timestamp.toString().length === 10 ? timestamp * 1000 : timestamp)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    return year + '-' + month + '-' + day
}

async function wait(s) {
    return new Promise(resolve => setTimeout(resolve, s * 1000))
}

function getAccountName(ckStr, ckSpliter, ckNameIndex) {
    const parts = ckStr.split(ckSpliter)
    if (ckNameIndex >= 0 && ckNameIndex < parts.length) {
        return parts[ckNameIndex]
    }
    return parts[0] || '未知'
}

function maskStr(str) {
    if (!str || str.length <= 4) return str || '***'
    return str.substring(0, 2) + '****' + str.substring(str.length - 2)
}

//构造青龙备注：userId-账号名
function buildRemarks(userId, accountName) {
    return `${userId}-${accountName}`
}

// 在青龙变量列表中查找匹配的envId
function findEnvId(envList, userId, accountName, envName) {
    if (!envList || !Array.isArray(envList)) return null
    const targetRemarks = buildRemarks(userId, accountName)
    return envList.find(item => {
        if (item.name !== envName) return false
        return item.remarks && item.remarks === targetRemarks
    })?.id
}

//初始化青龙（优先使用项目独立配置，没有则用全局配置）
async function initProjectQinglong(sender, project) {
    let qlConfig = null
    if (project && project.qinglong_config) {
        qlConfig = project.qinglong_config
    } else {
        qlConfig = await sender.bucketGet('wqwl_config', 'qinglong')
    }
    if (!qlConfig) throw new Error('青龙配置未设置')
    const qinglong = qlConfig.split('丨')
    const ql = new QL_API(qinglong[0], qinglong[1], qinglong[2])
    const qlToken = await ql.getToken()
    if (!qlToken) throw new Error('青龙配置错误')
    const qlEnv = new Env(qlToken, qinglong[0])
    const qlLogs = new Logs(qlToken, qinglong[0])
    return { qlEnv, qlLogs }
}

//计算居中
function getStringWidth(str) {
    // 将字符串转为数组，逐个字符判断
    return str.split('').reduce((width, char) => {
        // 如果字符的 Unicode 码点大于 255，认为是中文/全角符号，宽度算 2
        return width + (char.charCodeAt(0) > 255 ? 2 : 1);
    }, 0);
}

//内容居中
function centerText(text, targetLength) {
    const textWidth = getStringWidth(text); // 计算实际显示宽度（中文算2个字符）
    const paddingTotal = targetLength - textWidth;
    const leftPadding = Math.floor(paddingTotal / 2);
    const rightPadding = paddingTotal - leftPadding;

    return '-'.repeat(leftPadding) + text + '-'.repeat(rightPadding);
}

// ==================== 项目配置读写 ====================
// 获取所有前缀列表
async function getPrefixList(sender) {
    const raw = await sender.bucketGet(CONFIG_TABLE, CONFIG_INDEX_KEY)
    return safeParseArray(raw)
}

// 保存前缀列表
async function savePrefixList(sender, list) {
    return await sender.bucketSet(CONFIG_TABLE, CONFIG_INDEX_KEY, JSON.stringify(list))
}

// 获取单个项目配置
async function getProjectConfig(sender, prefix) {
    const raw = await sender.bucketGet(CONFIG_TABLE, prefix)
    return safeParseObject(raw)
}

// 保存单个项目配置
async function saveProjectConfig(sender, prefix, config) {
    return await sender.bucketSet(CONFIG_TABLE, prefix, JSON.stringify(config))
}

// 删除单个项目配置
async function deleteProjectConfig(sender, prefix) {
    return await sender.bucketSet(CONFIG_TABLE, prefix, '')
}

// ==================== 管理员配置入口 ====================
async function adminConfigProject(sender) {
    const isAdmin = await sender.isAdmin()
    if (!isAdmin) return

    let msg = '=====项目配置管理=====\n'
    msg += '[1] 添加项目\n'
    msg += '[2] 编辑项目\n'
    msg += '[3] 删除项目\n'
    msg += '[4] 查看所有项目\n'
    msg += '[5] 管理项目\n'
    msg += '------------------\n'
    msg += '回复数字选择操作\n回复"q"退出'
    await sender.reply(msg)

    let input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    switch (input) {
        case '1':
            await addProject(sender)
            break
        case '2':
            await editProject(sender)
            break
        case '3':
            await deleteProject(sender)
            break
        case '4':
            await viewProjects(sender)
            break
        case '5':
            await manageProjectStatus(sender)
            break
        default:
            sender.reply('❌输入错误')
    }
}

// ==================== 管理项目（启用/禁用） ====================
async function manageProjectStatus(sender) {
    const prefixList = await getPrefixList(sender)
    if (prefixList.length === 0) return sender.reply('❌暂无项目配置')

    // 加载所有项目并按状态排序（未启用的排前面）
    const projects = []
    for (const prefix of prefixList) {
        const p = await getProjectConfig(sender, prefix)
        if (p) projects.push(p)
    }
    projects.sort((a, b) => (a.status === 0 ? 0 : 1) - (b.status === 0 ? 0 : 1))

    let msg = '=====管理项目=====\n'
    for (let i = 0; i < projects.length; i++) {
        const statusText = projects[i].status === 0 ? '🔴关闭' : '🟢启用'
        msg += `[${i + 1}] ${statusText} ${projects[i].prefix}\n`
    }
    msg += '--------------------\n回复编号切换启用/禁用\n回复q退出'
    await sender.reply(msg)

    const input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    const idx = parseInt(input) - 1
    if (isNaN(idx) || idx < 0 || idx >= projects.length) return sender.reply('❌无效选择')

    const target = projects[idx]
    const newStatus = target.status === 0 ? 1 : 0
    target.status = newStatus
    await saveProjectConfig(sender, target.prefix, target)

    const statusLabel = newStatus === 1 ? '🟢已启用' : '🔴已关闭'
    sender.reply(`✅项目【${target.prefix}】${statusLabel}`)
}

// ==================== 添加项目 ====================
async function addProject(sender) {
    const project = {}

    // 1. 前缀
    sender.reply('请输入匹配前缀（如"伊利"，用户发送"伊利登录"等触发）：')
    let input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    // 检查是否已存在
    const existing = await getProjectConfig(sender, input)
    if (existing) return sender.reply('❌该前缀已存在')
    project.prefix = input

    // 2. 积分价格
    sender.reply('请输入积分价格（积分/月，0表示免费）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    project.price = parseInt(input) || 0

    // 3. BUCKET_NAME
    sender.reply('请输入BUCKET_NAME（数据桶前缀，如"wqwl_yy"）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    project.bucketName = input

    // 4. 容器变量名
    sender.reply('请输入容器的环境变量名称（如"Huaji_YP_YY"）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    project.envName = input

    // 4.5 独立青龙配置（可选）
    sender.reply('请输入该项目的独立青龙配置（格式：Host丨cilentId丨cilentSecret，留空则使用全局配置）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    project.qinglong_config = (input === '' || input === null) ? '' : input

    // 5. 日志关键词
    sender.reply('请输入日志同步关键词（匹配青龙日志任务名，如"DDYY"，多个用逗号分隔，留空跳过日志功能）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    project.logKeyword = (input === '' || input === null) ? '' : input

    // 6. 日志正则 + 7. 输出模板
    if (project.logKeyword) {
        sender.reply('请输入日志解析正则（捕获组用()标记）：\n例如：账号(\\d{3})\\*\\*\\*\\*(\\d{4}):\\s*现金([\\d.]+)元\\s*\\|\\s*成功\\[(\\d+)\\]次')
        input = await sender.listen(120000)
        if (input === 'q') return sender.reply('✅已退出')
        if (input === '' || input === null) return sender.reply('❌输入超时')
        project.logRegex = input

        sender.reply('请输入正则中【账号标识】是第几个捕获组（从1开始）：')
        input = await sender.listen(60000)
        if (input === 'q') return sender.reply('✅已退出')
        if (input === '' || input === null) return sender.reply('❌输入超时')
        project.logAccountIndex = parseInt(input) || 1

        // 多数据值配置
        project.logValues = []
        sender.reply('现在配置要提取的数据值（支持多个）\n请输入第1个数据的名称（如"今日金额"，输入q结束）：')
        let valIdx = 1
        while (true) {
            input = await sender.listen(60000)
            if (input === 'q' || input === '' || input === null) break

            const valItem = { name: input }

            sender.reply(`请输入【${input}】对应的捕获组序号（从1开始）：`)
            input = await sender.listen(60000)
            if (input === 'q' || input === '' || input === null) break
            valItem.index = parseInt(input) || 1

            sender.reply(`请输入【${valItem.name}】的单位（如：元、积分、金币）：`)
            input = await sender.listen(60000)
            if (input === 'q' || input === '' || input === null) break
            valItem.unit = input

            project.logValues.push(valItem)
            valIdx++
            sender.reply(`✅已添加【${valItem.name}】\n请输入第${valIdx}个数据的名称（输入q结束）：`)
        }

        if (project.logValues.length === 0) {
            return sender.reply('❌至少需要配置一个数据值')
        }
    } else {
        project.logRegex = ''
        project.logAccountIndex = 1
        project.logValues = []
    }

    // 分类
    sender.reply('请输入项目分类（如"生活服务"、"电商平台"，留空则为"未分类"）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    project.category = (input === '' || input === null) ? '未分类' : input

    // 教程
    sender.reply('请输入使用教程内容（展示给用户看，留空跳过）：')
    input = await sender.listen(120000)
    if (input === 'q') return sender.reply('✅已退出')
    project.tutorial = (input === '' || input === null) ? '' : input

    // 8. ck格式
    sender.reply('请输入登录ck的格式说明（展示给用户看）：\n例如：access-token#tenant-id#备注（可选）')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    project.ckFormat = input

    // 9. ck分隔符
    sender.reply('请输入ck分隔符（如"#"）：')
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    project.ckSpliter = input

    // 10. 账号名位置
    sender.reply(`请输入账号名在ck中的位置（从0开始，格式为"${project.ckFormat}"，分隔后第几位作为账号名）：`)
    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')
    project.ckNameIndex = parseInt(input) || 0

    // 默认启用
    project.status = 1

    // 保存项目配置
    const res = await saveProjectConfig(sender, project.prefix, project)
    if (!res) return sender.reply('❌保存项目配置失败')

    // 更新前缀索引
    const prefixList = await getPrefixList(sender)
    if (!prefixList.includes(project.prefix)) {
        prefixList.push(project.prefix)
        await savePrefixList(sender, prefixList)
    }

    let summary = '=====项目添加成功=====\n'
    summary += `📌 前缀：${project.prefix}\n`
    summary += `💰 价格：${project.price}积分/月\n`
    summary += `🟢 状态：启用\n`
    summary += `📂 分类：${project.category}\n`
    summary += `🗄️ 数据桶：${project.bucketName}\n`
    summary += `📦 变量名：${project.envName}\n`
    summary += `📋 ck格式：${project.ckFormat}\n`
    summary += `✂️ 分隔符：${project.ckSpliter}\n`
    summary += `👤 账号名位置：第${project.ckNameIndex}位\n`
    summary += `🖥️ 独立青龙：${project.qinglong_config || '未配置（使用全局）'}\n`
    if (project.logKeyword) {
        summary += `📝 日志关键词：${project.logKeyword}\n`
        summary += `🔍 日志正则：${project.logRegex}\n`
        summary += `🔢 账号捕获组：第${project.logAccountIndex}组\n`
        summary += `📊 数据值配置：\n`
        for (const v of project.logValues) {
            summary += `   - ${v.name}：第${v.index}组（${v.unit}）\n`
        }
    } else {
        summary += `📝 日志同步：未配置\n`
    }
    summary += '=================='
    sender.reply(summary)
}

// ==================== 编辑项目 ====================
async function editProject(sender) {
    const prefixList = await getPrefixList(sender)
    if (prefixList.length === 0) return sender.reply('❌暂无项目配置')

    let msg = '=====选择要编辑的项目=====\n'
    for (let i = 0; i < prefixList.length; i++) {
        msg += `[${i + 1}] ${prefixList[i]}\n`
    }
    msg += '回复数字选择，回复q退出'
    sender.reply(msg)

    let input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    const idx = parseInt(input) - 1
    if (isNaN(idx) || idx < 0 || idx >= prefixList.length) return sender.reply('❌无效选择')

    const prefix = prefixList[idx]
    const project = await getProjectConfig(sender, prefix)
    if (!project) return sender.reply('❌项目配置不存在')

    let editMsg = `=====编辑项目【${prefix}】=====\n`
    editMsg += `[1] 前缀：${project.prefix}\n`
    editMsg += `[2] 积分价格：${project.price}\n`
    editMsg += `[3] 状态：${project.status === 0 ? '🔴已关闭' : '🟢已启用'}\n`
    editMsg += `[4] 分类：${project.category || '未分类'}\n`
    editMsg += `[5] BUCKET_NAME：${project.bucketName}\n`
    editMsg += `[6] 变量名：${project.envName}\n`
    editMsg += `[7] 日志关键词：${project.logKeyword || '未配置'}\n`
    editMsg += `[8] 日志正则：${project.logRegex || '未配置'}\n`
    editMsg += `[9] 账号捕获组：${project.logAccountIndex || '未配置'}\n`
    editMsg += `[10] 数据值配置：${(project.logValues && project.logValues.length > 0) ? project.logValues.map(v => `${v.name}(第${v.index}组,${v.unit})`).join('、') : '未配置'}\n`
    editMsg += `[11] ck格式：${project.ckFormat}\n`
    editMsg += `[12] 分隔符：${project.ckSpliter}\n`
    editMsg += `[13] 账号名位置：${project.ckNameIndex}\n`
    editMsg += `[14] 教程：${project.tutorial ? '已配置' : '未配置'}\n`
    editMsg += `[15] 独立青龙配置：${project.qinglong_config || '未配置（使用全局）'}\n`
    editMsg += '------------------\n回复数字选择要修改的字段\n回复q退出'
    sender.reply(editMsg)

    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    // 数据值配置需要特殊交互
    if (input === '10') {
        project.logValues = []
        sender.reply('重新配置数据值\n请输入第1个数据的名称（如"今日金额"，输入q结束）：')
        let valIdx = 1
        while (true) {
            input = await sender.listen(60000)
            if (input === 'q' || input === '' || input === null) break
            const valItem = { name: input }
            sender.reply(`请输入【${input}】对应的捕获组序号（从1开始）：`)
            input = await sender.listen(60000)
            if (input === 'q' || input === '' || input === null) break
            valItem.index = parseInt(input) || 1
            sender.reply(`请输入【${valItem.name}】的单位（如：元、积分、金币）：`)
            input = await sender.listen(60000)
            if (input === 'q' || input === '' || input === null) break
            valItem.unit = input
            project.logValues.push(valItem)
            valIdx++
            sender.reply(`✅已添加【${valItem.name}】\n请输入第${valIdx}个数据的名称（输入q结束）：`)
        }
        await saveProjectConfig(sender, prefix, project)
        return sender.reply(`✅数据值配置已更新，共${project.logValues.length}项`)
    }

    const fieldMap = {
        '1': { key: 'prefix', name: '前缀' },
        '2': { key: 'price', name: '积分价格', isNum: true },
        '3': { key: 'status', name: '状态（1启用/0关闭）', isNum: true },
        '4': { key: 'category', name: '分类' },
        '5': { key: 'bucketName', name: 'BUCKET_NAME' },
        '6': { key: 'envName', name: '变量名' },
        '7': { key: 'logKeyword', name: '日志关键词' },
        '8': { key: 'logRegex', name: '日志正则' },
        '9': { key: 'logAccountIndex', name: '账号捕获组', isNum: true },
        '11': { key: 'ckFormat', name: 'ck格式' },
        '12': { key: 'ckSpliter', name: '分隔符' },
        '13': { key: 'ckNameIndex', name: '账号名位置', isNum: true },
        '14': { key: 'tutorial', name: '教程' },
        '15': { key: 'qinglong_config', name: '独立青龙配置（留空使用全局）' }
    }

    const field = fieldMap[input]
    if (!field) return sender.reply('❌无效选择')

    sender.reply(`请输入新的${field.name}（当前值：${project[field.key]}）：`)
    input = await sender.listen(120000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    const oldPrefix = project.prefix

    // 编辑前缀时检查重复
    if (field.key === 'prefix') {
        const existCheck = await getProjectConfig(sender, input)
        if (existCheck) return sender.reply('❌该前缀已被其他项目使用')
    }

    project[field.key] = field.isNum ? (parseInt(input) || 0) : input

    // 如果修改了前缀，需要迁移数据
    if (field.key === 'prefix' && oldPrefix !== project.prefix) {
        // 删除旧键
        await deleteProjectConfig(sender, oldPrefix)
        // 保存新键
        await saveProjectConfig(sender, project.prefix, project)
        // 更新索引
        const list = await getPrefixList(sender)
        const i = list.indexOf(oldPrefix)
        if (i !== -1) list[i] = project.prefix
        await savePrefixList(sender, list)
    } else {
        await saveProjectConfig(sender, oldPrefix, project)
    }

    sender.reply(`✅${field.name}已更新为：${project[field.key]}`)
}

// ==================== 删除项目 ====================
async function deleteProject(sender) {
    const prefixList = await getPrefixList(sender)
    if (prefixList.length === 0) return sender.reply('❌暂无项目配置')

    let msg = '=====选择要删除的项目=====\n'
    for (let i = 0; i < prefixList.length; i++) {
        msg += `[${i + 1}] ${prefixList[i]}\n`
    }
    msg += '回复数字选择，回复q退出'
    sender.reply(msg)

    let input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    const idx = parseInt(input) - 1
    if (isNaN(idx) || idx < 0 || idx >= prefixList.length) return sender.reply('❌无效选择')

    const prefix = prefixList[idx]
    sender.reply(`⚠️确定删除项目【${prefix}】吗？(y/n)`)
    input = await sender.listen(60000)
    if (input === '' || input === null) return sender.reply('❌输入超时')

    if (input.toLowerCase() === 'y') {
        await deleteProjectConfig(sender, prefix)
        prefixList.splice(idx, 1)
        await savePrefixList(sender, prefixList)
        sender.reply(`✅项目【${prefix}】已删除`)
    } else {
        sender.reply('❌已取消')
    }
}

// ==================== 查看所有项目 ====================
async function viewProjects(sender) {
    const prefixList = await getPrefixList(sender)
    if (prefixList.length === 0) return sender.reply('❌暂无项目配置')

    for (let i = 0; i < prefixList.length; i++) {
        const p = await getProjectConfig(sender, prefixList[i])
        if (!p) continue
        let msg = `=====项目[${i + 1}]=====\n`
        msg += `📌 前缀：${p.prefix}\n`
        msg += `💰 价格：${p.price}积分/月\n`
        msg += `${p.status === 0 ? '🔴 状态：已关闭' : '🟢 状态：已启用'}\n`
        msg += `📂 分类：${p.category || '未分类'}\n`
        msg += `🗄️ 数据桶：${p.bucketName}\n`
        msg += `📦 变量名：${p.envName}\n`
        msg += `📋 ck格式：${p.ckFormat}\n`
        msg += `✂️ 分隔符：${p.ckSpliter}\n`
        msg += `👤 账号名位置：第${p.ckNameIndex}位\n`
        msg += `🖥️ 独立青龙：${p.qinglong_config || '未配置（使用全局）'}\n`
        if (p.logKeyword) {
            msg += `📝 日志关键词：${p.logKeyword}\n`
            msg += `🔍 日志正则：${p.logRegex}\n`
            msg += `🔢 账号捕获组：第${p.logAccountIndex}组\n`
            if (p.logValues && p.logValues.length > 0) {
                msg += `📊 数据值配置：\n`
                for (const v of p.logValues) {
                    msg += `   - ${v.name}：第${v.index}组（${v.unit}）\n`
                }
            }
        } else {
            msg += `📝 日志同步：未配置\n`
        }
        msg += '=================='
        await sender.reply(msg)
    }
}
// ==================== 用户登录（不收积分） ====================
async function handleLogin(sender, user, project, qlEnv) {
    const { prefix, bucketName, ckFormat, ckSpliter, ckNameIndex, envName } = project

    sender.reply(`请按以下格式输入账号信息:\n${ckFormat}\n回复'q'退出`)
    let ckInput = await sender.listen(60000)
    if (ckInput === 'q') return sender.reply('✅已退出')
    if (ckInput === '' || ckInput === null) return sender.reply('❌输入超时')

    // 给备注字段加上用户ID前缀（前三后二脱敏）
    const ckParts = ckInput.split(ckSpliter)
    if (ckParts.length > ckNameIndex) {
        const userStr = String(user)
        const maskedUser = userStr.length > 5
            ? userStr.substring(0, 3) + '***' + userStr.substring(userStr.length - 2)
            : userStr
        const originalName = ckParts[ckNameIndex] || ''
        ckParts[ckNameIndex] = originalName ? `${maskedUser}-${originalName}` : maskedUser
        ckInput = ckParts.join(ckSpliter)
    }

    const rawData = await sender.bucketGet(`${bucketName}_users`, user)
    let userData = safeParseArray(rawData)

    const accountName = getAccountName(ckInput, ckSpliter, ckNameIndex)
    const remarks = buildRemarks(user, accountName)

    const existIdx = userData.findIndex(ck => getAccountName(ck, ckSpliter, ckNameIndex) === accountName)
    if (existIdx !== -1) {
        sender.reply(`账号【${maskStr(accountName)}】已存在，是否覆盖？(y/n)`)
        let confirm = await sender.listen(60000)
        if (confirm === '' || confirm === null) return sender.reply('❌输入超时')
        if (confirm.toLowerCase() !== 'y') return sender.reply('❌已取消')
        userData[existIdx] = ckInput
    } else {
        userData.push(ckInput)
    }

    const res1 = await sender.bucketSet(`${bucketName}_users`, user, JSON.stringify(userData))

    try {
        if (existIdx !== -1) {
            const envData = await qlEnv.getEnvList(envName)
            const envId = findEnvId(envData.data, user, accountName, envName)
            if (envId) {
                await qlEnv.updateVariable(envId, envName, ckInput, remarks)
            } else {
                await qlEnv.createVariable(envName, ckInput, remarks)
            }
        } else {
            await qlEnv.createVariable(envName, ckInput, remarks)
        }
    } catch (e) {
        console.error('同步青龙失败:', e)
        await sender.reply(`⚠️青龙同步失败：${e.message}，请联系管理员`)
    }

    if (res1) {
        sender.reply(`✅账号【${maskStr(accountName)}】添加成功！\n发送【${prefix}管理】进行授权后才能自动执行任务`)
    } else {
        sender.reply('❌保存失败，请重试')
    }
}

// ==================== 用户管理 ====================
async function handleManage(sender, user, project, qlEnv, pay, qrUrl) {
    const { prefix, bucketName, ckSpliter, ckNameIndex, envName } = project

    const rawData = await sender.bucketGet(`${bucketName}_users`, user)
    let userData = safeParseArray(rawData)
    if (userData.length === 0) {
        return sender.reply(`=====未绑定账号=====\n❌ 未找到任何账号信息\n💡 发送 ${prefix}登录 绑定账号\n==================`)
    }

    const authRaw = await sender.bucketGet(`${bucketName}_auth`, user)
    let authData = safeParseArray(authRaw)

    let msg = '=====账号列表=====\n'
    msg += '[0] 授权全部账号\n'
    for (let i = 0; i < userData.length; i++) {
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        const authTime = authData[i] || 0
        let status
        if (authTime === 0) status = '❌未授权'
        else if (authTime < Date.now()) status = '❌已过期'
        else status = `✅${formatDate(authTime)}`
        msg += `[${i + 1}] ${maskStr(name)}\n📋授权：${status}\n`
    }
    msg += '------------------\n回复数字选择账号\n回复q退出'
    await sender.reply(msg)

    let inputId = await sender.listen(60000)
    if (inputId === 'q') return sender.reply('✅已退出')
    if (inputId === '' || inputId === null) return sender.reply('❌输入超时')

    inputId = parseInt(inputId)
    if (isNaN(inputId) || inputId < 0 || inputId > userData.length) {
        return sender.reply('❌输入有误')
    }

    if (inputId === 0) {
        await authAll(sender, user, project, userData, authData, qlEnv, pay, qrUrl)
    } else {
        const index = inputId - 1
        const name = getAccountName(userData[index], ckSpliter, ckNameIndex)
        const currentAuth = authData[index] || 0
        let authStatus = currentAuth > Date.now() ? `✅${formatDate(currentAuth)}` : '❌未授权或已过期'

        let opMsg = '=====账号操作=====\n'
        opMsg += `👤 账号：${maskStr(name)}\n`
        opMsg += `📋 授权：${authStatus}\n`
        opMsg += '------------------\n'
        opMsg += '[1] 授权账号\n'
        opMsg += '[2] 删除账号\n'
        opMsg += '------------------\n回复数字选择\n回复q退出'
        await sender.reply(opMsg)

        let op = await sender.listen(60000)
        if (op === 'q') return sender.reply('✅已退出')
        if (op === '' || op === null) return sender.reply('❌输入超时')

        if (op === '1') {
            await authSingle(sender, user, project, userData, authData, index, qlEnv, pay, qrUrl)
        } else if (op === '2') {
            await deleteAccount(sender, user, project, userData, authData, index, qlEnv)
        } else {
            sender.reply('❌输入有误')
        }
    }
}

// 删除账号（删青龙变量 + 删bucket所有相关数据）
async function deleteAccount(sender, user, project, userData, authData, index, qlEnv) {
    const { bucketName, ckSpliter, ckNameIndex, envName } = project
    const name = getAccountName(userData[index], ckSpliter, ckNameIndex)

    sender.reply(`⚠️确定删除账号【${maskStr(name)}】吗？将同时删除青龙变量和所有相关数据(y/n)`)
    let confirm = await sender.listen(60000)
    if (confirm === '' || confirm === null) return sender.reply('❌输入超时')
    if (confirm.toLowerCase() !== 'y') return sender.reply('❌已取消删除')

    // 1. 删除青龙变量
    try {
        const envData = await qlEnv.getEnvList(envName)
        const envId = findEnvId(envData.data, user, name, envName)
        if (envId) {
            await qlEnv.delVariable([envId])
        }
    } catch (e) {
        console.error('删除青龙变量失败:', e)
    }

    // 2. 删除日志数据（键名为账号名）
    try {
        await sender.bucketSet(`${bucketName}_logs`, name, '')
    } catch (e) {
        console.error('删除日志数据失败:', e)
    }

    // 3. 从用户数组和授权数组中移除
    userData.splice(index, 1)
    authData.splice(index, 1)

    const res1 = await sender.bucketSet(`${bucketName}_users`, user, JSON.stringify(userData))
    const res2 = await sender.bucketSet(`${bucketName}_auth`, user, JSON.stringify(authData))

    // 4. 如果用户已经没有账号了，清空用户相关的所有bucket键
    if (userData.length === 0) {
        await sender.bucketSet(`${bucketName}_users`, user, '')
        await sender.bucketSet(`${bucketName}_auth`, user, '')
    }

    if (res1 && res2) {
        sender.reply(`✅账号【${maskStr(name)}】已删除，青龙变量已移除`)
    } else {
        sender.reply('❌删除失败，请联系管理员')
    }
}
// 授权单个账号（支持积分支付和易支付）
async function authSingle(sender, user, project, userData, authData, index, qlEnv, pay, qrUrl) {
    const { bucketName, price, ckSpliter, ckNameIndex, envName, prefix } = project
    const name = getAccountName(userData[index], ckSpliter, ckNameIndex)

    sender.reply('请输入授权月数：')
    let monthInput = await sender.listen(60000)
    if (monthInput === 'q') return sender.reply('✅已退出')
    if (monthInput === '' || monthInput === null) return sender.reply('❌输入超时')

    const months = parseInt(monthInput)
    if (isNaN(months) || months <= 0) return sender.reply('❌请输入正整数')

    // 积分价格
    const pointsPrice = price || 0
    const totalPoints = pointsPrice * months

    // 积分兑换比例：dd_sign_config的rate，格式"1:xxx"，默认1:100
    const rateRaw = await sender.bucketGet('dd_sign_config', 'rate')
    let rate = 100
    if (rateRaw) {
        const parts = rateRaw.split(':')
        if (parts.length === 2) rate = parseInt(parts[1]) || 100
    }

    // 换算金额：积分 / 比例，最低0.01
    let totalMoney = parseFloat((totalPoints / rate).toFixed(2))
    if (totalMoney > 0 && totalMoney < 0.01) totalMoney = 0.01

    // 解析支付方式配置：wqwl_config.pay_type 格式 "alipay:支付宝,wxpay:微信支付"
    const payTypeRaw = await sender.bucketGet('wqwl_config', 'pay_type')
    let payTypes = []
    if (payTypeRaw && pay && qrUrl) {
        payTypes = payTypeRaw.split(',').map(item => {
            const [type, label] = item.split(':')
            return { type: type.trim(), label: label ? label.trim() : type.trim() }
        }).filter(item => item.type)
    }

    const hasPoints = pointsPrice > 0
    const hasOnlinePay = payTypes.length > 0 && totalMoney > 0
    const isFree = pointsPrice === 0

    // 免费项目直接授权
    if (!isFree) {
        if (!hasPoints && !hasOnlinePay) {
            return sender.reply('❌支付方式配置异常，请联系管理员')
        }

        // 构建支付选择菜单
        let menuMsg = '请选择支付方式：\n'
        let optionIndex = 1
        const optionMap = {}

        if (hasPoints) {
            menuMsg += `[${optionIndex}] 积分支付（${totalPoints}积分，${pointsPrice}积分/月×${months}月）\n`
            optionMap[String(optionIndex)] = { method: 'points' }
            optionIndex++
        }

        if (hasOnlinePay) {
            for (const pt of payTypes) {
                menuMsg += `[${optionIndex}] ${pt.label}（${totalMoney}元）\n`
                optionMap[String(optionIndex)] = { method: 'online', type: pt.type, label: pt.label }
                optionIndex++
            }
        }

        menuMsg += '回复数字选择，回复q退出'
        sender.reply(menuMsg)

        let payInput = await sender.listen(60000)
        if (payInput === 'q') return sender.reply('✅已退出')
        if (payInput === '' || payInput === null) return sender.reply('❌输入超时')

        const selected = optionMap[payInput]
        if (!selected) return sender.reply('❌输入有误')

        // 积分支付
        if (selected.method === 'points') {
            const userPointsRaw = await sender.bucketGet(USER_POINTS_TABLE, user)
            const userPoints = userPointsRaw ? parseInt(userPointsRaw) : 0

            if (userPoints < totalPoints) {
                return sender.reply(`❌积分不足！需要${totalPoints}积分（${pointsPrice}积分/月×${months}月），当前积分：${userPoints}`)
            }

            sender.reply(`⚠️本次授权需要扣除${totalPoints}积分（${pointsPrice}积分/月×${months}月），当前积分：${userPoints}，确认吗？(y/n)`)
            let confirm = await sender.listen(60000)
            if (confirm === 'q') return sender.reply('✅已退出')
            if (confirm === '' || confirm === null) return sender.reply('❌输入超时')
            if (confirm.toLowerCase() !== 'y') return sender.reply('❌已取消')

            const newPoints = userPoints - totalPoints
            await sender.bucketSet(USER_POINTS_TABLE, user, newPoints.toString())
            sender.reply(`✅已扣除${totalPoints}积分，剩余积分：${newPoints}`)
        }

        // 在线支付
        if (selected.method === 'online') {
            const order = await pay.createPay(selected.type, '', `用户${user}${prefix}续费${months}个月`, totalMoney)
            if (!order || order.code !== 1) {
                return sender.reply('❌创建订单失败，请联系管理员')
            }

            const tradeNo = order.trade_no
            sender.reply(`生成订单成功，请使用${selected.label}支付\n订单号：${tradeNo}[CQ:image,file=${qrUrl}${order.qrcode}]`)
            const result = await pay.checkOrderStatus(tradeNo, 5, 300)

            if (!result.success) {
                return sender.reply('❌用户未支付或超时')
            }
            sender.reply('✅支付成功，正在更新授权...')
        }
    }

    // 更新授权时间
    while (authData.length <= index) authData.push(0)

    const currentAuth = authData[index] || 0
    if (currentAuth > Date.now()) {
        authData[index] = currentAuth + months * 30 * 24 * 60 * 60 * 1000
    } else {
        authData[index] = Date.now() + months * 30 * 24 * 60 * 60 * 1000
    }

    const res = await sender.bucketSet(`${bucketName}_auth`, user, JSON.stringify(authData))

    // 启用青龙变量
    try {
        const envData = await qlEnv.getEnvList(envName)
        const envId = findEnvId(envData.data, user, name, envName)
        if (envId) await qlEnv.enableVariable([envId])
    } catch (e) {
        console.error('启用青龙变量失败:', e)
    }

    if (res) {
        sender.reply(`=====授权成功=====\n👤 账号：${maskStr(name)}\n⏰ 时长：${months * 30}天\n📅 到期：${formatDate(authData[index])}\n==================`)
    } else {
        sender.reply('❌授权保存失败')
    }
}

// 授权全部账号
async function authAll(sender, user, project, userData, authData, qlEnv, pay, qrUrl) {
    const { bucketName, price, ckSpliter, ckNameIndex, envName, prefix } = project

    sender.reply('请输入授权月数：')
    let monthInput = await sender.listen(60000)
    if (monthInput === 'q') return sender.reply('✅已退出')
    if (monthInput === '' || monthInput === null) return sender.reply('❌输入超时')

    const months = parseInt(monthInput)
    if (isNaN(months) || months <= 0) return sender.reply('❌请输入正整数')

    const accountCount = userData.length
    const pointsPrice = price || 0
    const totalPoints = pointsPrice * months * accountCount

    // 积分兑换比例
    const rateRaw = await sender.bucketGet('dd_sign_config', 'rate')
    let rate = 100
    if (rateRaw) {
        const parts = rateRaw.split(':')
        if (parts.length === 2) rate = parseInt(parts[1]) || 100
    }

    let totalMoney = parseFloat((totalPoints / rate).toFixed(2))
    if (totalMoney > 0 && totalMoney < 0.01) totalMoney = 0.01

    const payTypeRaw = await sender.bucketGet('wqwl_config', 'pay_type')
    let payTypes = []
    if (payTypeRaw && pay && qrUrl) {
        payTypes = payTypeRaw.split(',').map(item => {
            const [type, label] = item.split(':')
            return { type: type.trim(), label: label ? label.trim() : type.trim() }
        }).filter(item => item.type)
    }

    const hasPoints = pointsPrice > 0
    const hasOnlinePay = payTypes.length > 0 && totalMoney > 0
    const isFree = pointsPrice === 0

    if (!isFree) {
        if (!hasPoints && !hasOnlinePay) {
            return sender.reply('❌支付方式配置异常，请联系管理员')
        }

        let menuMsg = '请选择支付方式：\n'
        let optionIndex = 1
        const optionMap = {}

        if (hasPoints) {
            menuMsg += `[${optionIndex}] 积分支付（${totalPoints}积分，${pointsPrice}积分/月×${months}月×${accountCount}个账号）\n`
            optionMap[String(optionIndex)] = { method: 'points' }
            optionIndex++
        }

        if (hasOnlinePay) {
            for (const pt of payTypes) {
                menuMsg += `[${optionIndex}] ${pt.label}（${totalMoney}元）\n`
                optionMap[String(optionIndex)] = { method: 'online', type: pt.type, label: pt.label }
                optionIndex++
            }
        }

        menuMsg += '回复数字选择，回复q退出'
        sender.reply(menuMsg)

        let payInput = await sender.listen(60000)
        if (payInput === 'q') return sender.reply('✅已退出')
        if (payInput === '' || payInput === null) return sender.reply('❌输入超时')

        const selected = optionMap[payInput]
        if (!selected) return sender.reply('❌输入有误')

        if (selected.method === 'points') {
            const userPointsRaw = await sender.bucketGet(USER_POINTS_TABLE, user)
            const userPoints = userPointsRaw ? parseInt(userPointsRaw) : 0

            if (userPoints < totalPoints) {
                return sender.reply(`❌积分不足！需要${totalPoints}积分（${pointsPrice}积分/月×${months}月×${accountCount}个账号），当前积分：${userPoints}`)
            }

            sender.reply(`⚠️本次授权需要扣除${totalPoints}积分（${pointsPrice}积分/月×${months}月×${accountCount}个账号），当前积分：${userPoints}，确认吗？(y/n)`)
            let confirm = await sender.listen(60000)
            if (confirm === 'q') return sender.reply('✅已退出')
            if (confirm === '' || confirm === null) return sender.reply('❌输入超时')
            if (confirm.toLowerCase() !== 'y') return sender.reply('❌已取消')

            const newPoints = userPoints - totalPoints
            await sender.bucketSet(USER_POINTS_TABLE, user, newPoints.toString())
            sender.reply(`✅已扣除${totalPoints}积分，剩余积分：${newPoints}`)
        }

        if (selected.method === 'online') {
            const order = await pay.createPay(selected.type, '', `用户${user}${prefix}全部续费${months}个月`, totalMoney)
            if (!order || order.code !== 1) {
                return sender.reply('❌创建订单失败，请联系管理员')
            }

            const tradeNo = order.trade_no
            sender.reply(`生成订单成功，请使用${selected.label}支付\n订单号：${tradeNo}[CQ:image,file=${qrUrl}${order.qrcode}]`)
            const result = await pay.checkOrderStatus(tradeNo, 5, 300)

            if (!result.success) {
                return sender.reply('❌用户未支付或超时')
            }
            sender.reply('✅支付成功，正在更新授权...')
        }
    }

    // 更新所有账号授权时间
    let success = 0
    for (let i = 0; i < userData.length; i++) {
        while (authData.length <= i) authData.push(0)
        const currentAuth = authData[i] || 0
        if (currentAuth > Date.now()) {
            authData[i] = currentAuth + months * 30 * 24 * 60 * 60 * 1000
        } else {
            authData[i] = Date.now() + months * 30 * 24 * 60 * 60 * 1000
        }
        success++

        // 启用青龙变量
        try {
            const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
            const envData = await qlEnv.getEnvList(envName)
            const envId = findEnvId(envData.data, user, name, envName)
            if (envId) await qlEnv.enableVariable([envId])
        } catch (e) {
            console.error('启用青龙变量失败:', e)
        }
    }

    const res = await sender.bucketSet(`${bucketName}_auth`, user, JSON.stringify(authData))
    if (res) {
        sender.reply(`=====授权成功=====\n✅ 成功：${success}个账号\n❌ 失败：${userData.length - success}个账号\n⏰ 时长：${months * 30}天\n==================`)
    } else {
        sender.reply('❌授权保存失败')
    }
}

// ==================== 日志同步（管理员） ====================
async function handleLogSync(sender, user, project, qlLogs) {
    const isAdmin = await sender.isAdmin()
    if (!isAdmin) return

    const { bucketName, logKeyword, logRegex, logAccountIndex, logValues, ckSpliter, ckNameIndex } = project
    const accountIdx = logAccountIndex || 1
    const valuesConfig = logValues || []
    if (!logKeyword || !logRegex) {
        return sender.reply('❌该项目未配置日志同步（缺少日志关键词或正则）')
    }
    if (valuesConfig.length === 0) {
        return sender.reply('❌该项目未配置数据值（logValues为空）')
    }

    await sender.reply('正在同步日志...')

    try {
        const today = formatDate()
        const logList = await qlLogs.logs()

        // 按关键词匹配日志任务（支持多个关键词，逗号分隔）
        const keywords = logKeyword.split(',').map(k => k.trim())
        const targetTasks = logList.data.filter(item =>
            item.title && keywords.some(kw => item.title.includes(kw))
        )

        if (targetTasks.length === 0) {
            return sender.reply(`❌未找到包含【${logKeyword}】的日志任务`)
        }

        // 收集今日日志文件
        const itemsToProcess = []
        for (const task of targetTasks) {
            const children = task.children || []
            const filtered = children.filter(child =>
                child.title && child.title.includes(today)
            )
            filtered.forEach(child => {
                itemsToProcess.push({
                    parent: task.title,
                    title: child.title
                })
            })
        }

        if (itemsToProcess.length === 0) {
            return sender.reply(`❌未找到今日（${today}）的相关日志记录`)
        }

        // 获取已有日志数据
        let logs = await sender.bucketAll(`${bucketName}_logs`)
        if (typeof logs === 'string') {
            try { logs = JSON.parse(logs) } catch (e) { logs = {} }
        }
        if (!logs || typeof logs !== 'object') logs = {}

        const regex = new RegExp(logRegex, 'g')
        // 用 {账号标识: {文件标识: [捕获组值数组]}} 汇总
        const dailyRaw = {}

        for (const { parent, title } of itemsToProcess) {
            try {
                const detail = await qlLogs.detailLogs(parent, title)
                const logContent = detail.data || ''

                // 从文件名提取标识：如 2026-02-21-08-05-00-768.log -> 08-05-00-768
                const fileId = title.replace(/\.log$/, '').replace(/^\d{4}-\d{2}-\d{2}-/, '')

                let match
                regex.lastIndex = 0
                // 记录同一文件内每个账号的匹配次数，用于生成 fileId-index 标识
                const fileMatchCount = {}
                while ((match = regex.exec(logContent)) !== null) {
                    const accountKey = match[accountIdx] || 'unknown'

                    // 提取多个数据值
                    const matchValues = valuesConfig.map(vc => ({
                        name: vc.name,
                        value: parseFloat(match[vc.index]) || 0,
                        unit: vc.unit
                    }))

                    if (!dailyRaw[accountKey]) dailyRaw[accountKey] = {}
                    if (!fileMatchCount[accountKey]) fileMatchCount[accountKey] = 0
                    fileMatchCount[accountKey]++

                    // 标识 = 文件时间部分-序号，如 08-05-00-768-1、08-05-00-768-2
                    const entryId = `${fileId}-${fileMatchCount[accountKey]}`
                    dailyRaw[accountKey][entryId] = matchValues
                }
            } catch (error) {
                console.error(`处理日志详情失败 [${parent} - ${title}]:`, error)
            }
        }

        // 将 dailyRaw 转换为存储格式并写入 bucket
        const savedAccounts = []

        for (const accountKey of Object.keys(dailyRaw)) {
            let accountData = {}
            if (logs[accountKey]) {
                try {
                    accountData = typeof logs[accountKey] === 'string'
                        ? JSON.parse(logs[accountKey])
                        : logs[accountKey]
                } catch (e) {
                    accountData = {}
                }
            }

            // 构建今日数据：数组，每个元素 {biaoshi, values: [{name, value, unit}]}
            const todayEntries = []
            for (const [entryId, matchValues] of Object.entries(dailyRaw[accountKey])) {
                todayEntries.push({ biaoshi: entryId, values: matchValues })
            }

            accountData[today] = todayEntries

            logs[accountKey] = JSON.stringify(accountData)
            try {
                await sender.bucketSet(`${bucketName}_logs`, accountKey, logs[accountKey])
                savedAccounts.push(accountKey)
            } catch (error) {
                console.error(`保存账号 ${accountKey} 日志数据失败:`, error)
            }
        }

        if (savedAccounts.length > 0) {
            await sender.reply(`✅ 今日日志同步完成，共更新 ${savedAccounts.length} 个账号：\n${savedAccounts.join('\n')}`)
        } else {
            await sender.reply('ℹ️ 今日日志已最新或无匹配数据')
        }
    } catch (error) {
        console.error('日志同步失败:', error)
        await sender.reply(`❌日志同步失败：${error.message}`)
    }
}

// ==================== 检测过期账号（管理员） ====================
async function handleCheck(sender, project, qlEnv) {
    const { bucketName, ckSpliter, ckNameIndex, envName, prefix } = project

    await sender.reply(`正在检测【${prefix}】...`)

    const allUser = await sender.bucketAll(`${bucketName}_users`)
    const allAuth = await sender.bucketAll(`${bucketName}_auth`)

    if (!allUser || typeof allUser !== 'object') {
        return sender.reply(`❌【${prefix}】暂无用户数据`)
    }

    const envData = await qlEnv.getEnvList(envName)
    let notifyCount = 0
    let deleteCount = 0
    let totalCount = 0

    for (const userId in allUser) {
        let userData
        try {
            userData = typeof allUser[userId] === 'string' ? JSON.parse(allUser[userId]) : allUser[userId]
        } catch (e) { continue }
        if (!Array.isArray(userData) || userData.length === 0) continue

        let authData
        try {
            authData = allAuth[userId] ? (typeof allAuth[userId] === 'string' ? JSON.parse(allAuth[userId]) : allAuth[userId]) : []
        } catch (e) { authData = [] }
        if (!Array.isArray(authData)) authData = []

        totalCount += userData.length
        let dataChanged = false

        // 倒序遍历，方便删除
        for (let i = userData.length - 1; i >= 0; i--) {
            const authTime = authData[i] || 0
            if (authTime > Date.now()) continue

            // 已过期
            const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
            const notifyKey = `${userId}_${name}`

            // 读取通知次数
            const notifyRaw = await sender.bucketGet(`${bucketName}_notify`, notifyKey)
            let notifyTimes = notifyRaw ? (parseInt(notifyRaw) || 0) : 0

            const expiredDays = Math.floor((Date.now() - authTime) / (24 * 60 * 60 * 1000))

            if (notifyTimes >= 7 || expiredDays >= 7) {
                // 超过7次通知或过期超过7天 → 删除账号数据和青龙变量
                try {
                    const envId = findEnvId(envData.data, userId, name, envName)
                    if (envId) await qlEnv.delVariable([envId])
                } catch (e) {
                    console.error(`删除青龙变量失败[${name}]:`, e)
                }

                // 删除日志数据
                try {
                    await sender.bucketSet(`${bucketName}_logs`, name, '')
                } catch (e) { }

                // 从数组中移除
                userData.splice(i, 1)
                authData.splice(i, 1)
                dataChanged = true

                // 清除通知计数
                await sender.bucketSet(`${bucketName}_notify`, notifyKey, '')

                // 通知用户账号已被删除
                let msg = `📱 账号: ${name}\n`
                msg += `📢 消息:\n`
                msg += `🗑️ 账号已被清理\n`
                msg += `❌ 过期${expiredDays}天，已通知${notifyTimes}次\n`
                msg += `💡 如需继续使用请重新登录并授权\n`
                msg += `==================`
                push('qq', '', userId, `=====${prefix}过期通知=====`, msg)

                deleteCount++
            } else {
                // 未超限 → 禁用青龙变量 + 通知 + 计数+1
                try {
                    const envId = findEnvId(envData.data, userId, name, envName)
                    if (envId) await qlEnv.disableVariable([envId])
                } catch (e) {
                    console.error(`禁用青龙变量失败[${name}]:`, e)
                }

                let msg = `📱 账号: ${name}\n`
                msg += `📢 消息:\n`
                msg += `⚠️ 授权已过期\n`
                msg += `❌ 过期时间：${formatDate(authTime)}\n`
                msg += `💡 请及时续费授权\n`
                msg += `==================`
                push('qq', '', userId, `=====${prefix}过期通知=====`, msg)

                notifyTimes++
                await sender.bucketSet(`${bucketName}_notify`, notifyKey, notifyTimes.toString())

                notifyCount++
            }
            await wait(2)
        }

        // 如果有删除操作，保存更新后的数据
        if (dataChanged) {
            if (userData.length === 0) {
                await sender.bucketSet(`${bucketName}_users`, userId, '')
                await sender.bucketSet(`${bucketName}_auth`, userId, '')
            } else {
                await sender.bucketSet(`${bucketName}_users`, userId, JSON.stringify(userData))
                await sender.bucketSet(`${bucketName}_auth`, userId, JSON.stringify(authData))
            }
        }
    }

    return { totalCount, notifyCount, deleteCount }
}

// ==================== 管理授权（管理员） ====================
async function handleAdminAuth(sender, user, project, qlEnv) {
    const isAdmin = await sender.isAdmin()
    if (!isAdmin) return

    const { bucketName, ckSpliter, ckNameIndex, envName, prefix } = project

    sender.reply('请输入要授权的用户ID：')
    let targetUser = await sender.listen(60000)
    if (targetUser === 'q') return sender.reply('✅已退出')
    if (targetUser === '' || targetUser === null) return sender.reply('❌输入超时')

    const rawData = await sender.bucketGet(`${bucketName}_users`, targetUser)
    const userData = safeParseArray(rawData)
    if (userData.length === 0) {
        return sender.reply(`❌用户【${targetUser}】未绑定任何账号`)
    }

    const authRaw = await sender.bucketGet(`${bucketName}_auth`, targetUser)
    let authData = safeParseArray(authRaw)

    // 显示该用户的账号列表
    let msg = `=====用户【${targetUser}】账号列表=====\n`
    msg += '[0] 授权全部账号\n'
    for (let i = 0; i < userData.length; i++) {
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        const authTime = authData[i] || 0
        let status
        if (authTime === 0) status = '❌未授权'
        else if (authTime < Date.now()) status = '❌已过期'
        else status = `✅${formatDate(authTime)}`
        msg += `[${i + 1}] ${name}\n📋授权：${status}\n`
    }
    msg += '------------------\n回复数字选择账号\n回复q退出'
    await sender.reply(msg)

    let inputId = await sender.listen(60000)
    if (inputId === 'q') return sender.reply('✅已退出')
    if (inputId === '' || inputId === null) return sender.reply('❌输入超时')

    inputId = parseInt(inputId)
    if (isNaN(inputId) || inputId < 0 || inputId > userData.length) {
        return sender.reply('❌输入有误')
    }

    sender.reply('请输入授权月数（正数增加，负数扣除）：')
    let monthInput = await sender.listen(60000)
    if (monthInput === 'q') return sender.reply('✅已退出')
    if (monthInput === '' || monthInput === null) return sender.reply('❌输入超时')

    const months = parseInt(monthInput)
    if (isNaN(months) || months === 0) return sender.reply('❌请输入非零整数')

    // 确定要授权的账号范围
    const startIdx = inputId === 0 ? 0 : inputId - 1
    const endIdx = inputId === 0 ? userData.length : inputId

    let success = 0
    for (let i = startIdx; i < endIdx; i++) {
        while (authData.length <= i) authData.push(0)

        const currentAuth = authData[i] || 0
        const delta = months * 30 * 24 * 60 * 60 * 1000

        if (months > 0) {
            // 增加授权
            if (currentAuth > Date.now()) {
                authData[i] = currentAuth + delta
            } else {
                authData[i] = Date.now() + delta
            }
        } else {
            // 扣除授权
            authData[i] = currentAuth + delta // delta 本身是负数
            if (authData[i] < 0) authData[i] = 0
        }

        // 如果授权过期，禁用青龙变量
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        try {
            const envData = await qlEnv.getEnvList(envName)
            const envId = findEnvId(envData.data, targetUser, name, envName)
            if (envId) {
                if (authData[i] > Date.now()) {
                    await qlEnv.enableVariable([envId])
                } else {
                    await qlEnv.disableVariable([envId])
                }
            }
        } catch (e) {
            console.error('操作青龙变量失败:', e)
        }
        success++
    }

    const res = await sender.bucketSet(`${bucketName}_auth`, targetUser, JSON.stringify(authData))
    if (res) {
        const action = months > 0 ? '增加' : '扣除'
        const absDays = Math.abs(months) * 30
        let resultMsg = `=====管理授权成功=====\n`
        resultMsg += `👤 用户：${targetUser}\n`
        resultMsg += `📋 操作：${action}${absDays}天\n`
        resultMsg += `✅ 成功：${success}个账号\n`
        for (let i = startIdx; i < endIdx; i++) {
            const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
            resultMsg += `${name} → ${authData[i] > Date.now() ? formatDate(authData[i]) : '已过期'}\n`
        }
        resultMsg += '=================='
        sender.reply(resultMsg)
    } else {
        sender.reply('❌授权保存失败')
    }
}

// ==================== 管理查询（管理员） ====================
async function handleAdminQuery(sender, user, project) {
    const isAdmin = await sender.isAdmin()
    if (!isAdmin) return

    const { bucketName, ckSpliter, ckNameIndex, logValues, prefix } = project
    const valuesConfig = logValues || []

    sender.reply('请输入要查询的用户ID：')
    let targetUser = await sender.listen(60000)
    if (targetUser === 'q') return sender.reply('✅已退出')
    if (targetUser === '' || targetUser === null) return sender.reply('❌输入超时')

    const rawData = await sender.bucketGet(`${bucketName}_users`, targetUser)
    const userData = safeParseArray(rawData)
    if (userData.length === 0) {
        return sender.reply(`❌用户【${targetUser}】未绑定任何账号`)
    }

    const authRaw = await sender.bucketGet(`${bucketName}_auth`, targetUser)
    const authData = safeParseArray(authRaw)

    // 构建账号选择菜单
    let menuMsg = `=====用户【${targetUser}】账号=====\n`
    menuMsg += '[0] 全部查询\n--------------------\n'
    for (let i = 0; i < userData.length; i++) {
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        menuMsg += `[${i + 1}] ${name}\n`
    }
    menuMsg += '--------------------\n选0查询全部，选在范围内只查询指定号'
    await sender.reply(menuMsg)

    const selectInput = await sender.listen(60000)
    if (selectInput === 'q') return sender.reply('✅已退出')
    if (selectInput === '' || selectInput === null) return sender.reply('❌输入超时')

    const selectNum = parseInt(selectInput)
    if (isNaN(selectNum) || selectNum < 0 || selectNum > userData.length) {
        return sender.reply('❌ 输入无效，请输入正确的编号')
    }

    await sender.reply('正在查询...')

    const today = formatDate()
    const startIdx = selectNum === 0 ? 0 : selectNum - 1
    const endIdx = selectNum === 0 ? userData.length : selectNum

    for (let i = startIdx; i < endIdx; i++) {
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        const authTime = authData[i] || 0

        const logRaw = await sender.bucketGet(`${bucketName}_logs`, name)
        let accountData = {}
        if (logRaw) {
            try {
                accountData = typeof logRaw === 'string' ? JSON.parse(logRaw) : logRaw
            } catch (e) {
                accountData = {}
            }
        }

        let msg = ''

        if (valuesConfig.length === 1) {
            const vc = valuesConfig[0]
            const unit = vc.unit
            const authStatus = authTime > Date.now() ? '✅已授权' : '❌未授权或已过期'

            msg += `=====账号详情[${i + 1}]=====\n`
            msg += `👤 用户：${targetUser}\n`
            msg += `📱账号：${name}\n`
            msg += `🔑授权状态：${authStatus}\n`
            msg += `⏰到期时间：${formatDate(authTime)}\n`
            msg += `====💰收益状况💰====\n`

            const todayEntries = accountData[today] || []
            const todayValue = todayEntries.reduce((sum, e) => {
                const vals = e.values || []
                const found = vals.find(v => v.name === vc.name)
                return sum + (found ? found.value : 0)
            }, 0)
            const todayCount = todayEntries.length

            const now = new Date()
            const thirtyDaysAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
            let monthValue = 0, monthCount = 0
            let totalValue = 0, totalCount = 0
            const historyEntries = []

            for (const [dateKey, entries] of Object.entries(accountData)) {
                if (!Array.isArray(entries)) continue
                const dayValue = entries.reduce((sum, e) => {
                    const vals = e.values || []
                    const found = vals.find(v => v.name === vc.name)
                    return sum + (found ? found.value : 0)
                }, 0)
                const dayCount = entries.length
                totalValue += dayValue
                totalCount += dayCount
                const dateObj = new Date(dateKey)
                if (dateObj >= thirtyDaysAgo) {
                    monthValue += dayValue
                    monthCount += dayCount
                }
                historyEntries.push({ date: dateKey, value: dayValue, count: dayCount })
            }

            if (todayCount > 0) {
                msg += `🌞今日收益：${parseFloat(todayValue.toFixed(2))}${unit}(共${todayCount}次)\n`
            } else {
                msg += `🌞今日收益：今日还没有任何收益\n`
            }
            msg += `📈本月收益：${parseFloat(monthValue.toFixed(2))}${unit}(共${monthCount}次)\n`
            msg += `📊总计收益：${parseFloat(totalValue.toFixed(2))}${unit}(共${totalCount}次)\n`

            msg += `==== 📖历史记录📖====\n`
            historyEntries.sort((a, b) => new Date(b.date) - new Date(a.date))
            const recentHistory = historyEntries.slice(0, 5)
            if (recentHistory.length > 0) {
                for (const h of recentHistory) {
                    msg += `📅${h.date}：${parseFloat(h.value.toFixed(2))}${unit}，成功${h.count}次\n`
                }
            } else {
                msg += '暂无历史记录\n'
            }
            msg += '=================='

        } else if (valuesConfig.length > 1) {
            msg += `=====账号信息=====\n`
            msg += `👤 用户：${targetUser}\n`
            msg += `🤪 用户账号: ${name}\n`

            const todayEntries = accountData[today] || []
            if (todayEntries.length > 0) {
                const latestEntry = todayEntries[todayEntries.length - 1]
                const latestValues = latestEntry.values || []
                for (const vc of valuesConfig) {
                    const found = latestValues.find(v => v.name === vc.name)
                    const val = found ? parseFloat(found.value.toFixed(2)) : 0
                    msg += `💰 ${vc.name}: ${val}${vc.unit}\n`
                }
            } else {
                const allDates = Object.keys(accountData).filter(k => Array.isArray(accountData[k])).sort().reverse()
                if (allDates.length > 0) {
                    const lastEntries = accountData[allDates[0]]
                    const lastEntry = lastEntries[lastEntries.length - 1]
                    const lastValues = lastEntry.values || []
                    msg += `(最近数据 ${allDates[0]})\n`
                    for (const vc of valuesConfig) {
                        const found = lastValues.find(v => v.name === vc.name)
                        const val = found ? parseFloat(found.value.toFixed(2)) : 0
                        msg += `💰 ${vc.name}: ${val}${vc.unit}\n`
                    }
                } else {
                    msg += `😓 暂无数据记录\n`
                }
            }
            msg += `☁️ 授权到期: ${formatDate(authTime)}\n`
            msg += '=================='

        } else {
            msg += `=====账号信息=====\n`
            msg += `👤 用户：${targetUser}\n`
            msg += `🤪 用户账号: ${name}\n`
            msg += `暂未配置数据值\n`
            msg += `☁️ 授权到期: ${formatDate(authTime)}\n`
            msg += '=================='
        }
        await sender.reply(msg)
        await wait(2)
    }
}

// ==================== 用户查询 ====================
async function handleQuery(sender, user, project) {
    const { bucketName, ckSpliter, ckNameIndex, logValues, prefix } = project
    const valuesConfig = logValues || []

    const rawData = await sender.bucketGet(`${bucketName}_users`, user)
    const userData = safeParseArray(rawData)
    if (userData.length === 0) {
        return sender.reply(`=====未绑定账号=====\n❌ 未找到任何账号信息\n💡 发送 ${prefix}登录 绑定账号\n==================`)
    }

    const authRaw = await sender.bucketGet(`${bucketName}_auth`, user)
    const authData = safeParseArray(authRaw)

    // 构建账号选择菜单
    let menuMsg = '请输入要查询的账号：\n[0] 全部查询\n--------------------\n'
    for (let i = 0; i < userData.length; i++) {
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        menuMsg += `[${i + 1}] ${name}\n`
    }
    menuMsg += '--------------------\n选0查询全部，选在范围内只查询指定号'
    await sender.reply(menuMsg)

    const selectInput = await sender.input()
    const selectNum = parseInt(selectInput)
    if (isNaN(selectNum) || selectNum < 0 || selectNum > userData.length) {
        return sender.reply('❌ 输入无效，请输入正确的编号')
    }

    await sender.reply('正在查询...')

    const today = formatDate()

    // 确定要查询的账号范围
    const startIdx = selectNum === 0 ? 0 : selectNum - 1
    const endIdx = selectNum === 0 ? userData.length : selectNum

    for (let i = startIdx; i < endIdx; i++) {
        const name = getAccountName(userData[i], ckSpliter, ckNameIndex)
        const authTime = authData[i] || 0

        // 读取该账号的日志数据
        const logRaw = await sender.bucketGet(`${bucketName}_logs`, name)
        let accountData = {}
        if (logRaw) {
            try {
                accountData = typeof logRaw === 'string' ? JSON.parse(logRaw) : logRaw
            } catch (e) {
                accountData = {}
            }
        }

        let msg = ''

        if (valuesConfig.length === 1) {
            // 单数据值模式：保持原来的今日/本月/总计+历史记录格式
            const vc = valuesConfig[0]
            const unit = vc.unit
            const authStatus = authTime > Date.now() ? '✅已授权' : '❌未授权或已过期'

            msg += `=====账号详情[${i + 1}]=====\n`
            msg += `📱账号：${name}\n`
            msg += `🔑授权状态：${authStatus}\n`
            msg += `⏰到期时间：${formatDate(authTime)}\n`
            msg += `====💰收益状况💰====\n`

            // 统计今日
            const todayEntries = accountData[today] || []
            const todayValue = todayEntries.reduce((sum, e) => {
                const vals = e.values || []
                const found = vals.find(v => v.name === vc.name)
                return sum + (found ? found.value : 0)
            }, 0)
            const todayCount = todayEntries.length

            // 统计近30天和总计
            const now = new Date()
            const thirtyDaysAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
            let monthValue = 0, monthCount = 0
            let totalValue = 0, totalCount = 0
            const historyEntries = []

            for (const [dateKey, entries] of Object.entries(accountData)) {
                if (!Array.isArray(entries)) continue
                const dayValue = entries.reduce((sum, e) => {
                    const vals = e.values || []
                    const found = vals.find(v => v.name === vc.name)
                    return sum + (found ? found.value : 0)
                }, 0)
                const dayCount = entries.length

                totalValue += dayValue
                totalCount += dayCount

                const dateObj = new Date(dateKey)
                if (dateObj >= thirtyDaysAgo) {
                    monthValue += dayValue
                    monthCount += dayCount
                }
                historyEntries.push({ date: dateKey, value: dayValue, count: dayCount })
            }

            if (todayCount > 0) {
                msg += `🌞今日收益：${parseFloat(todayValue.toFixed(2))}${unit}(共${todayCount}次)\n`
            } else {
                msg += `🌞今日收益：今日还没有任何收益\n`
            }
            msg += `📈本月收益：${parseFloat(monthValue.toFixed(2))}${unit}(共${monthCount}次)\n`
            msg += `📊总计收益：${parseFloat(totalValue.toFixed(2))}${unit}(共${totalCount}次)\n`

            msg += `==== 📖历史记录📖====\n`
            historyEntries.sort((a, b) => new Date(b.date) - new Date(a.date))
            const recentHistory = historyEntries.slice(0, 5)
            if (recentHistory.length > 0) {
                for (const h of recentHistory) {
                    msg += `📅${h.date}：${parseFloat(h.value.toFixed(2))}${unit}，成功${h.count}次\n`
                }
            } else {
                msg += '暂无历史记录\n'
            }
            msg += '=================='

        } else if (valuesConfig.length > 1) {
            // 多数据值模式：每个数据名称取最新一条的值显示
            msg += `=====账号信息=====\n`
            msg += `🤪 用户账号: ${name}\n`

            const todayEntries = accountData[today] || []
            if (todayEntries.length > 0) {
                const latestEntry = todayEntries[todayEntries.length - 1]
                const latestValues = latestEntry.values || []
                for (const vc of valuesConfig) {
                    const found = latestValues.find(v => v.name === vc.name)
                    const val = found ? parseFloat(found.value.toFixed(2)) : 0
                    msg += `💰 ${vc.name}: ${val}${vc.unit}\n`
                }
            } else {
                const allDates = Object.keys(accountData).filter(k => Array.isArray(accountData[k])).sort().reverse()
                if (allDates.length > 0) {
                    const lastEntries = accountData[allDates[0]]
                    const lastEntry = lastEntries[lastEntries.length - 1]
                    const lastValues = lastEntry.values || []
                    msg += `(最近数据 ${allDates[0]})\n`
                    for (const vc of valuesConfig) {
                        const found = lastValues.find(v => v.name === vc.name)
                        const val = found ? parseFloat(found.value.toFixed(2)) : 0
                        msg += `💰 ${vc.name}: ${val}${vc.unit}\n`
                    }
                } else {
                    msg += `😓 暂无数据记录\n`
                }
            }
            msg += `☁️ 授权到期: ${formatDate(authTime)}\n`
            msg += '=================='

        } else {
            msg += `=====账号信息=====\n`
            msg += `🤪 用户账号: ${name}\n`
            msg += `😓 暂未配置数据值\n`
            msg += `☁️ 授权到期: ${formatDate(authTime)}\n`
            msg += '=================='
        }
        await sender.reply(msg)
        await wait(2)
    }
}

// ==================== 日志获取测试（管理员） ====================
async function handleLogTest(sender, project) {
    const isAdmin = await sender.isAdmin()
    if (!isAdmin) return

    // 初始化青龙（使用项目独立配置或全局配置）
    let qlLogs
    try {
        const result = await initProjectQinglong(sender, project)
        qlLogs = result.qlLogs
    } catch (e) {
        return sender.reply(`❌青龙初始化失败：${e.message}`)
    }

    // 获取日志任务列表
    const logList = await qlLogs.logs()
    if (!logList || !logList.data || logList.data.length === 0) {
        return sender.reply('❌未找到任何日志任务')
    }

    // 显示日志任务
    let msg = '=====选择日志任务=====\n'
    for (let i = 0; i < logList.data.length; i++) {
        msg += `[${i + 1}] ${logList.data[i].title}\n`
    }
    msg += '------------------\n回复数字选择，回复q退出'
    await sender.reply(msg)

    let input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    const taskIdx = parseInt(input) - 1
    if (isNaN(taskIdx) || taskIdx < 0 || taskIdx >= logList.data.length) {
        return sender.reply('❌无效选择')
    }

    const task = logList.data[taskIdx]
    const children = task.children || []
    if (children.length === 0) {
        return sender.reply('❌该任务下没有日志文件')
    }

    // 显示日志文件（最近10个，倒序）
    const recentFiles = children.slice(-10).reverse()
    let fileMsg = `=====【${task.title}】日志文件=====\n`
    for (let i = 0; i < recentFiles.length; i++) {
        fileMsg += `[${i + 1}] ${recentFiles[i].title}\n`
    }
    fileMsg += '------------------\n回复数字选择，回复q退出'
    await sender.reply(fileMsg)

    input = await sender.listen(60000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    const fileIdx = parseInt(input) - 1
    if (isNaN(fileIdx) || fileIdx < 0 || fileIdx >= recentFiles.length) {
        return sender.reply('❌无效选择')
    }

    // 获取日志内容
    const detail = await qlLogs.detailLogs(task.title, recentFiles[fileIdx].title)
    const logContent = detail.data || ''
    if (!logContent) {
        return sender.reply('❌日志内容为空')
    }

    // 显示预览
    const preview = logContent.length > 500 ? logContent.substring(0, 500) + '\n...(内容过长已截断)' : logContent
    await sender.reply(`=====日志内容预览=====\n${preview}\n==================\n请输入正则表达式进行测试（回复q退出）：`)

    input = await sender.listen(120000)
    if (input === 'q') return sender.reply('✅已退出')
    if (input === '' || input === null) return sender.reply('❌输入超时')

    // 测试正则
    try {
        const regex = new RegExp(input, 'g')
        let match
        const matches = []
        while ((match = regex.exec(logContent)) !== null) {
            matches.push([...match])
            if (matches.length >= 20) break
        }

        if (matches.length === 0) {
            return sender.reply('❌没有匹配到任何结果，请检查正则表达式')
        }

        let resultMsg = `=====正则测试结果=====\n`
        resultMsg += `📝 正则：${input}\n`
        resultMsg += `🔢 匹配数：${matches.length}${matches.length >= 20 ? '（仅显示前20条）' : ''}\n`
        resultMsg += '------------------\n'

        for (let i = 0; i < matches.length; i++) {
            resultMsg += `【匹配${i + 1}】${matches[i][0]}\n`
            for (let g = 1; g < matches[i].length; g++) {
                resultMsg += `  捕获组${g}：${matches[i][g] || '(空)'}\n`
            }
        }
        resultMsg += '=================='
        await sender.reply(resultMsg)
    } catch (e) {
        sender.reply(`❌正则表达式错误：${e.message}`)
    }
}

// ==================== 主入口 ====================
!(async function () {
    const senderID = await middlleware.getSenderID()
    const sender = new middlleware.Sender(senderID)
    const user = await sender.getUserID()
    let message = await sender.getMessage()

    try {
        // 管理员专用：项目配置管理（发送"通用配置"触发）
        if (message === '项目配置') {
            await adminConfigProject(sender)
            return
        }

        // 所有人可用：项目列表
        if (message === '项目列表') {
            const allPrefixList = await getPrefixList(sender)
            if (allPrefixList.length === 0) return sender.reply('❌暂无可用项目')

            // 按分类分组，只显示启用的项目
            const categoryMap = {}
            for (const prefix of allPrefixList) {
                const p = await getProjectConfig(sender, prefix)
                if (!p || p.status === 0) continue
                const cat = p.category || '未分类'
                if (!categoryMap[cat]) categoryMap[cat] = []
                categoryMap[cat].push(p)
            }

            const categories = Object.keys(categoryMap)
            if (categories.length === 0) return sender.reply('❌暂无开启的项目')
            const totalCount = (data) => {
                return Object.values(data).reduce((total, category) => {
                    return total + (Array.isArray(category) ? category.length : 0);
                }, 0);
            };
            // sender.reply(JSON.stringify(categoryMap))
            let listMsg = `=====可用项目列表(${totalCount(categoryMap)})=====`
            const targetLength = getStringWidth(listMsg);
            listMsg += `\n`
            for (const cat of categories) {
                const name = `${cat}(${categoryMap[cat].length})`
                listMsg += `${centerText(name, targetLength)}\n`
                for (const p of categoryMap[cat]) {
                    const priceText = p.price === 0 ? '免费' : `${p.price}积分/月`
                    listMsg += `📌${p.prefix}-${priceText}\n`
                }
                listMsg += `-`.repeat(targetLength) + `\n`
            }
            listMsg += '==================\n'
            listMsg += '发送 前缀+登录 即可使用\n例如：伊利登录\n教程发 前缀+教程 获取教程'
            return sender.reply(listMsg)
        }

        // 项目日志同步：按顺序同步所有启用的项目
        if (message === '项目日志同步') {
            const isAdmin = await sender.isAdmin()
            if (!isAdmin) return
            const allPrefixList = await getPrefixList(sender)
            if (allPrefixList.length === 0) return sender.reply('❌暂无项目配置')

            let syncCount = 0
            for (const prefix of allPrefixList) {
                const p = await getProjectConfig(sender, prefix)
                if (!p || p.status === 0) continue
                if (!p.logKeyword) continue
                syncCount++
                await sender.reply(`正在同步【${p.prefix}】...`)
                try {
                    const { qlLogs } = await initProjectQinglong(sender, p)
                    await handleLogSync(sender, user, p, qlLogs)
                } catch (e) {
                    await sender.reply(`❌【${p.prefix}】同步失败：${e.message}`)
                }
            }
            if (syncCount === 0) {
                return sender.reply('❌没有需要同步的项目（无启用项目或未配置日志关键词）')
            }
            return sender.reply(`✅全部同步完成，共${syncCount}个项目`)
        }

        // 项目检测：按顺序检测所有启用的项目
        if (message === '项目检测') {
            const isAdmin = await sender.isAdmin()
            if (!isAdmin) return

            const allPrefixList = await getPrefixList(sender)
            if (allPrefixList.length === 0) return sender.reply('❌暂无项目配置')

            let totalNotify = 0
            let totalDelete = 0
            let totalCheck = 0
            let checkCount = 0

            for (const prefix of allPrefixList) {
                const p = await getProjectConfig(sender, prefix)
                if (!p || p.status === 0) continue
                checkCount++
                try {
                    const { qlEnv } = await initProjectQinglong(sender, p)
                    const result = await handleCheck(sender, p, qlEnv)
                    if (result) {
                        totalCheck += result.totalCount
                        totalNotify += result.notifyCount
                        totalDelete += result.deleteCount
                    }
                } catch (e) {
                    await sender.reply(`❌【${p.prefix}】检测失败：${e.message}`)
                }
            }
            if (checkCount === 0) {
                return sender.reply('❌没有需要检测的项目')
            }
            return sender.reply(`✅全部检测完成，共${checkCount}个项目\n📊 检测账号：${totalCheck}个\n📢 通知过期：${totalNotify}个\n🗑️ 清理账号：${totalDelete}个`)
        }

        // 读取前缀列表，匹配消息
        const prefixList = await getPrefixList(sender)
        let matchedPrefix = null
        let action = null

        for (const prefix of prefixList) {
            const actions = ['登录', '查询', '管理', '教程', '日志同步', '检测', '管理查询', '管理授权', '日志测试']
            for (const act of actions) {
                if (message === prefix + act) {
                    matchedPrefix = prefix
                    action = act
                    break
                }
            }
            if (matchedPrefix) break
        }

        if (!matchedPrefix) return

        // 获取项目配置
        const project = await getProjectConfig(sender, matchedPrefix)
        if (!project) {
            // return sender.reply(`❌项目【${matchedPrefix}】配置不存在`)
            return
        }

        // 检查项目是否启用
        if (project.status === 0) {
            return
            //   return sender.reply(`❌项目【${matchedPrefix}】已关闭，暂不可用`)
        }

        // 教程指令（不需要初始化青龙）
        if (action === '教程') {
            const p = matchedPrefix
            let tutorialMsg = `${p}教程\n`
            tutorialMsg += project.tutorial || '暂无教程内容'
            tutorialMsg += `\n==================\n`
            tutorialMsg += `🎯用户指令\n`
            tutorialMsg += `${p}登录 - 登录账号\n`
            tutorialMsg += `${p}管理 - 管理账号\n`
            tutorialMsg += `${p}查询 - 查询账号收益\n`
            tutorialMsg += `${p}教程 - 查看使用教程\n`
            tutorialMsg += `==================\n`
            tutorialMsg += `🎯管理员指令\n`
            tutorialMsg += `${p}管理授权 - 给用户账号授权\n`
            tutorialMsg += `${p}管理查询 - 查询用户账号信息\n`
            tutorialMsg += `${p}日志同步 - 同步青龙日志\n`
            tutorialMsg += `==================`
            return sender.reply(tutorialMsg)
        }

        // 初始化青龙（优先使用项目独立配置）
        const { qlEnv, qlLogs } = await initProjectQinglong(sender, project)

        // 初始化支付（可选，部分操作不需要）
        let pay = null
        let qrUrl = null
        try {
            const payUrl = await sender.bucketGet('wqwl_config', 'pay')
            const payId = await sender.bucketGet('wqwl_config', 'pay_id')
            const payKey = await sender.bucketGet('wqwl_config', 'pay_key')
            qrUrl = await sender.bucketGet('wqwl_config', 'qr_url')
            if (payUrl && payId && payKey) {
                pay = new Pay(payUrl, payId, payKey)
            }
        } catch (e) {
            console.error('易支付初始化失败:', e)
        }

        switch (action) {
            case '登录':
                await handleLogin(sender, user, project, qlEnv)
                break
            case '管理':
                await handleManage(sender, user, project, qlEnv, pay, qrUrl)
                break
            case '日志同步':
                await handleLogSync(sender, user, project, qlLogs)
                break
            case '查询':
                await handleQuery(sender, user, project)
                break
            case '管理授权':
                await handleAdminAuth(sender, user, project, qlEnv)
                break
            case '管理查询':
                await handleAdminQuery(sender, user, project)
                break
            case '检测':
                if (await sender.isAdmin()) {
                    const result = await handleCheck(sender, project, qlEnv)
                    if (result) {
                        await sender.reply(`✅【${project.prefix}】检测完成\n📊 检测账号：${result.totalCount}个\n📢 通知过期：${result.notifyCount}个\n🗑️ 清理账号：${result.deleteCount}个`)
                    }
                }
                break
            case '日志测试':
                await handleLogTest(sender, project)
                break
            default:
            //不用理
            //  sender.reply(`❌操作【${action}】暂未实现`)
        }

    } catch (e) {
        await sender.reply(`❌发生错误：${e.message}`)
        console.error(e)
    }
})()