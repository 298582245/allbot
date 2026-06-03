//[title: wqwl_qldock]
//[language: nodejs]
//[class: 工具类]
//[service: qq298582245] 售后联系方式
//[disable: false] 禁用开关，true表示禁用，false表示可用
//[admin: false] 是否为管理员指令
//[rule: ^(甬音)(登录|查询|管理|日志同步|管理|检测|管理查询|管理授权)$] 匹配规则1
//[cron: 38 0,22 * * *] cron定时，支持5位域和6位域
//[priority: 0] 优先级，数字越大表示优先级越高
//[platform: qq] 适用的平台
//[open_source: false]是否开源
//[icon: 图标url]图标链接地址，请使用48像素的正方形图标，支持http和https
//[version: 1.0.0]版本号
//[public: false] 是否发布？值为true或false，不设置则上传aut云时会自动设置为true，false时上传后不显示在市场中，但是搜索能搜索到，方便开发者测试
//[price: 0.01] 上架价格
//[description:] 使用方法尽量写具体
//[param: {"required":true,"key":"wqwl_qldock.qinglong","bool":false,"placeholder":"Host|cilentId|cilentSecret","name":"对接容器","desc":"各参数之间用中文符丨分割"}]

const middlleware = require('./middleware')
const axios = require('axios');
const crypto = require('crypto');
//const { console } = require('inspector');
const { push, notifyMasters } = require('./middleware')


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

//安全解析获取到的数据
function safeParse(rawData) {

    let allData = {};
    if (rawData && typeof rawData === 'object') {
        allData = rawData;
    } else if (rawData && typeof rawData === 'string') {
        try {
            allData = JSON.parse(rawData);
        } catch (e) {
            console.error("JSON 解析失败:", e);
            allData = {};
        }
    }
    return allData;

}

class AutManager {
    constructor(sender) {
        this.sender = sender;
    }

    //输入等待
    async waitInput(name, format = null) {
        await this.sender.reply(`${name}${format ? `\n请按照以下格式输入：\n ${format} \n 退出输入'q'!` : ''}`);
        let data = await this.sender.input(60000, 10000, true);
        if (data === 'q') {
            await this.sender.reply('✅已退出');
            return false;
        }
        if (data === '' || data === null) {
            await this.sender.reply('❌输入超时');
            return false;
        }
        return data;
    }
}

class UserManager extends AutManager {
    constructor(user, sender, activityName, QlEnv) {
        this.user = user;
        this.sender = sender;
        this.activityName = activityName;
        this.QlEnv = QlEnv;
        super(sender);
    }

    //获取其配置
    async init() {
        let config = await this.Sender.bucketGet(`wqwl_qldock_config`, this.activityName);
        config = safeParse(config);
        const { bucketName, envName, cost, ckSplit, ckLength, inputTips, queryType, queryCode } = config;
        if (!bucketName || !envName || !inputTips || !queryType) {
            await this.sender.reply(`❌ [${this.activityName}] 配置不完整，请联系管理员！`);
            return false;
        }
        this.bucketName = `wqwl_qldock_${bucketName}`
        this.envName = envName;
        this.cost = cost;
        this.ckSplit = ckSplit;
        this.ckLength = ckLength;
        this.inputTips = inputTips;
        this.queryType = queryType;
        this.queryCode = queryCode;
    }

    //添加账号
    async addUser() {
        try {
            const bucketName = `${this.bucketName}_user`;
            let allData = await this.Sender.bucketGet(bucketName, this.user);
            allData = safeParse(allData);
            const userData = await this.waitInput('请输入备注：(只用做🤖ck区分，如果备注不同，ck相同，请勿重复添加！)');
            if (!userData) {
                return;
            }
            if (this.isUserExist(allData, userData)) {
                const inputYN = await this.waitInput(`用户已经存在，是否进行覆盖？(y/n)`)
                if (inputYN.toLowerCase() === 'y') {

                } else {
                    await this.sender.reply('❌ 已取消添加！');
                    return;
                }
            }
            const userInput = await this.waitInput(`请输入[${this.activityName}]需要的ck`, this.inputTips);
            if (!userInput) {
                return;
            }
            try {
                userInput.split(this.ckSplit).forEach(ck => {
                    if (ck.length < this.ckLength) {
                        throw new Error(`ck格式错误！请按照${this.inputTips}重新输入！`);
                    }
                });
            }
            catch (error) {
                await this.sender.reply(`❌ 添加失败，${error.message}`);
                return;
            }
            allData[userData] = userInput;
            await this.Sender.bucketSet(bucketName, this.user, allData);
            await this.sender.reply(`=====添加提示=====
📱 账号: ${userData}
✅ 状态: 添加成功
------------------
发送"${this.activityName}管理"管理账号
发送"${this.activityName}查询"查询账号`);
        } catch (error) {
            console.error(error);
            await this.sender.reply('❌ 添加失败了，' + error.message);
            return;
        }
    }

    //管理账号
    async manageUser() {
        try {
            const bucketName = `${this.bucketName}_user`;
            let allData = await this.Sender.bucketGet(bucketName, this.user);
            allData = safeParse(allData);
            if (!allData) {
                await this.sender.reply(`❌ 未找到任何账号信息\n💡 输入 ${this.activityName}登录 添加账号`);
                return;
            }
            let authData = await this.Sender.bucketGet(`${this.bucketName}_auth`, this.user);
            authData = safeParse(authData);
            if (authData.length !== allData.length) {
                //根据key判断auth存不存在
                for (const key in allData) {
                    if (!authData[key]) {
                        authData[key] = 0
                    }
                }
            }
            let msg = `=====账号管理=====\n[0] 授权全部账号\n${this.splitLine()}`;
            let i = 0;
            for (const key in allData) {
                const auth = authData[key];
                const time = this.formatTime(auth);
                msg += `[${++i}] ${key}(${auth < Date.now() ? '❌ 未授权' : `✅ 过期时间 ${time}`}\n${this.splitLine()}`;
                `})`
            }
            await this.sender.reply(msg);
            let userInput = await this.waitInput('请输入数字选择账号\n回复"q"退出');
            if (!userInput)
                return;
            if (!this.isRightInput(userInput, {
                min: 0,
                max: i,
                type: 'number'
            })) {
                await this.sender.reply('❌ 输入有误，请重新输入！');
                return;
            }
            userInput = parseInt(userInput);
            if (userInput === 0) {
            } else {
                const userKey = Object.keys(allData)[userInput - 1];
                const userValue = allData[userKey];
                const msg = `请选择操作\n[1] 授权账号\n[2] 删除账号\n[3] 更新CK\n[4] 查看CK`;
                const choice = await this.waitInput(msg);
                switch (choice) {
                    case '1':
                        break;
                    case '2':
                        break;
                    case '3':
                        break;
                    case '4':
                        const ensure = await this.waitInput(`该信息涉及隐私信息，请确认是否查看？(y/n)`);
                        if (ensure.toLowerCase() === 'y') {
                            await this.sender.reply(`=====账号信息=====
🤪用户备注：${userKey}
📱账号C K ：${userValue}
🔓授权信息：${authData[userKey] <= Date.now() ? '✅' : '❌'} ${this.formatTime(authData[key])}`);
                        } else {
                            await this.sender.reply('✅已退出');
                            return false;
                        }
                        //${allData[Object.keys(allData)[userInput - 1]]}
                        break;
                    default:
                        await this.sender.reply('❌ 输入有误，请重新输入！');
                        break;
                }
            }
        } catch (error) {
            await this.sender.reply('❌ 管理加载失败了，' + error.message);
        }
    }

    //添加授权
    async addAuth(authData, authTime, index) {
        try {
            if (!this.isRightInput(authTime, { min: 1, type: 'number' }) || !Object.keys(authData)[index]) {
                await this.sender.reply('❌ 输入有误，请重新输入！');
                return;
            }
            const key = Object.keys(authData)[index];
            let isExpired = authData[key] <= Date.now();
            if (authData[key] <= Date.now()) {
                authData[key] = Date.now() + authTime * 60 * 1000;
            } else {
                authData[key] += authTime * 60 * 1000;
            }
            const money = this.cost * authTime;
            const mymoney = await this.getMoney();
            if (money > mymoney) {
                await this.sender.reply(`❌ 余额不足，请充值,当前金额为${mymoney}，还需要${money - mymoney}！`);
                return;
            }
            const adjust = await this.adjustMoney(-money);
            if (!adjust) {
                await this.sender.reply('❌ 修改授权失败了！,如多次尝试失败，请联系管理员');
                return;
            }

            await this.Sender.bucketSet(`${this.bucketName}_auth`, this.user, authData);
            //如果过期得重新添加数据到青龙

        } catch (error) {
            console.error(error);
            await this.sender.reply('❌ 添加授权失败了，' + error.message);
        }
    }

    //获取余额
    async getMoney() {
        try {
            const money = await this.Sender.bucketGet(`wqwl_qldock_money`, this.user);
            if (!money) {
                await this.Sender.bucketSet(`wqwl_qldock_money`, this.user, 0);
                return 0;
            }
            return parseFloat(money);
        } catch (error) {
            console.error(error);
            //  await this.sender.reply('❌ 获取余额失败了，' + error.message);
            return 0;
        }
    }

    //调整余额
    async adjustMoney(adjustment) {
        try {
            const money = await this.getMoney();
            if (!this.isRightInput(adjustment, { allowNegative: true, allowDecimal: true, min: -Infinity, type: 'number' })) {

                return false;
            }
            const newMoney = money + parseFloat(adjustment);
            if (newMoney < 0) {
                return false
            }
            await this.Sender.bucketSet(`wqwl_qldock_money`, this.user, newMoney);
            return true;
        }
        catch (error) {
            console.error(error);
            return false
        }
    }
    //检查ck是否存在
    isUserExist(allData, remark) {
        try {
            const obj = JSON.parse(allData);
            return remark in obj;  // 或者 obj.hasOwnProperty(key)
        } catch (error) {
            return false;  // 或者抛出错误
        }
    }
    //根据时间戳格式化时间
    formatTime(timestamp) {
        const date = new Date(timestamp);
        const year = date.getFullYear();
        const month = date.getMonth() + 1;
        const day = date.getDate();
        const hour = date.getHours();
        const minute = date.getMinutes();
        const second = date.getSeconds();
        return `${year}-${month}-${day} ${hour}:${minute}:${second}`;
    }

    //分割线
    splitLine(length = 20, split = '-') {
        return split.repeat(length) + '\n';
    }

    isRightInput(input, options = {}) {
        const {
            allowNegative = false,
            allowDecimal = false,
            min = -Infinity,
            max = Infinity
        } = options;

        // 基础检查
        if (input == null || input === '') return false;

        // 处理字符串
        if (typeof input === 'string') {
            input = input.trim();
            if (input === '') return false;
        }

        // 转换为数字
        const num = Number(input);
        if (isNaN(num) || !isFinite(num)) return false;

        // 验证范围
        if (num < min || num > max) return false;

        // 验证负数和整数
        if (!allowNegative && num < 0) return false;

        // 验证是否为整数（如果不允许小数）
        if (!allowDecimal && !Number.isInteger(num)) return false;

        return true;
    }
}

!(async function () {
    try {
        const settingData = await sender.bucketAll(`wqwl_qldock`)
        const { qinglong: qlData// 青龙配置

        } = settingData
        //await sender.reply(qinglongData)
        qinglong = qlData.split('丨')
        const ql = new QL_API(qinglong[0], qinglong[1], qinglong[2]);
        const QlToken = await ql.getToken();
        const QlEnv = new Env(QlToken, qinglong[0]);
        const QlLogs = new Logs(QlToken, qinglong[0]);
        const QlScripts = new Scripts(QlToken, qinglong[0]);
    }
    catch (e) {
        const message = `❌青龙配置错误，${e}`
        await sender.reply(message)
        console.log(message)
        return
    }
})()