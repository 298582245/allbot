/**
 * 脚本：wqwl_new_正淘系列合集.js
 * 作者：wqwlkj 裙：960690899
 * 描述：微信小程序正淘系列合集，抓包Authorization,openId,appid格式：Authorization#openId#appid#备注1
 * 环境变量：wqwl_zthj，多个换行或新建多个变量（不能混合使用）
 * 环境变量描述：wqwl_ztdl_daili，提取代理链接要返回txt格式的
 * cron: 15 0 9 * * *
 */

const weixinMini = {
    "wx13534641a6ba473e": "多提宝",
    "wx026eed201f18fbdb": "蜂淘好物",
    "wxb1051030cecd9b6a": "马蹄好物淘",
    "wx08b808d0bfbd1c1b": "惠量优品汇",
    "wx85f624cb4426ea37": "瑞选优品汇",
    "wxb0e6ef1ed22665c8": "衡养优品严选",
    "wxc86b6b6d33af73fa": "甄宝盒",
    "wx4a7009efeb51623e": "宝购坊",
    "wx81f6129d31e4f221": "智胜好物淘",
    "wx13a72e4f548d2868": "莲雾易淘",
    "wx49f60961692087d0": "正美淘好物",
    "wx567216ee6c60dd85": "柚柚好选",
    "wx477516c6fede9e99": "多提宝诚购",
    "wx0d57d3295558f67e": "多提宝优品",
    "wx4478b526c55b06dc": "多提宝城选",
    "wx3604f56ed58bbc2c": "多提宝云选",
    "wxddaca2ac42a46698": "鲸淘尚品",
    "wx25f49110bb9d6d6b": "卓著臻品购",
}

//环境变量
const ckName = 'wqwl_zthj';
//脚本名称
const scriptName = '微信小程序正淘系列合集';
//本地版本
const version = 1.0;
//是否需要文件存储
const isNeedFile = true;
//ck长度
const ckLength = 3;
//日志是否需要具体时间
const isNeedTimes = false;

//配置
const CONFIG = {
    proxy: '',//代理链接
    isProxy: true,//是否用代理
    bfs: 3,//并发数，默认3
    isNotify: true,//是否通知
    isDebug: false,//是否调试，非不要不用开
}

const proxy = CONFIG['proxy'] || process.env["wqwl_ztdl_daili"] || '';
const isProxy = CONFIG['isProxy'] || process.env["wqwl_useProxy"] || false;
const bfs = CONFIG['bfs'] || process.env["wqwl_bfs"] || 3;
const isNotify = CONFIG['isNotify'] || process.env["wqwl_isNotify"] || true;
const isDebug = CONFIG['isDebug'] || process.env["wqwl_isDebug"] || false;

/**
 * 其他全局环境变量说明
 * wqwl_daili：代理链接，需要返回单挑txt格式
 * wqwl_useProxy：是否用代理，默认使用（填了代理链接）
 * wqwl_bfs：并发数，默认4
 * wqwl_isNotify：是否进行通知
 * wqwl_isDebug：是否调试输出请求
 */


const axios = require('axios');
const fs = require('fs');
const crypto = require('crypto')

let wqwlkj;
// 先下载依赖文件
async function downloadRequire() {
    const filePath = 'wqwl_require.js';
    const url = 'https://raw.githubusercontent.com/298582245/wqwl_qinglong/refs/heads/main/wqwl_require.js';

    if (fs.existsSync(filePath)) {
        console.log('✅wqwl_require.js已存在，无需重新下载，如有报错请重新下载覆盖\n');
        wqwlkj = require('./wqwl_require');
        return true;
    } else {
        console.log('正在下载wqwl_require.js，请稍等...\n');
        console.log(`如果下载过慢，可以手动下载wqwl_require.js，并保存为wqwl_require.js，并重新运行脚本`);
        console.log('地址：' + url);
        try {
            const res = await axios.get(url);
            fs.writeFileSync(filePath, res.data);
            console.log('✅ 下载完成\n');
            wqwlkj = require('./wqwl_require');
            return true;
        } catch (e) {
            console.log('❌ 下载失败，请手动下载wqwl_require.js\n');
            console.log('地址：' + url);
            return false;
        }
    }
}


// 立即执行下载并等待完成
!(async function () {
    const downloadIsSuccess = await downloadRequire();
    if (!downloadIsSuccess) {
        console.log('❌ 依赖文件下载失败，脚本终止');
        process.exit(1);
    }
    if (!wqwlkj.WQWLBase || !wqwlkj.WQWLBaseTask) {
        console.log('❌ wqwl_require.js 未发现WQWLBase类、WQWLBaseTask类，请重新下载新版本');
        process.exit(1);
    }

    class Task extends wqwlkj.WQWLBaseTask {
        constructor(ck, index, base) {
            // 调用父类构造函数
            super(ck, index, base);
            this.baseUrl = 'https://www.yipintemian.com';
        }

        async init() {
            const ckData = this.ck.split('#')
            // console.log(ckData)
            if (ckData.length < ckLength) {
                this.sendMessage(`${this.index + 1} 环境变量有误，请检查环境变量是否正确`, true);
                return false;
            }
            else if (ckData.length === ckLength) {
                this.remark = `${ckData[0].slice(0, 8)}-${this.index}`;
            }
            else {
                this.remark = ckData[ckLength];
            }

            this.auth = ckData[0]
            this.openId = ckData[1]
            this.appId = ckData[2]
            if (weixinMini[this.appId]) {
                this.remark = `${weixinMini[this.appId]}-${this.remark}`
            } else {
                this.remark = `未知小程序-${this.remark}`
            }

            if (this.base.proxyUrl && this.base.isProxy) {
                this.proxy = await wqwlkj.getProxy(this.index, this.proxyConfig);
                this.sendMessage(`✅ 使用代理：${this.proxy}`);
            }
            else {
                this.proxy = ''
                this.sendMessage(`⚠️ 不使用代理`)
            }
            let ua;
            if (isNeedFile) {
                if (!this.base.fileData[this.remark])
                    this.base.fileData[this.remark] = {}

                if (!this.base.fileData[this.remark]['ua']) {
                    this.base.fileData[this.remark]['ua'] = this.base.wqwlkj.generateRandomUA()
                }
                ua = this.base.fileData[this.remark]['ua']
                this.sendMessage(`🎲 使用ua：${ua.slice(0, 50)}`)
            }

            this.headers = {
                "Host": "www.yipintemian.com",
                "User-Agent": ua,
                "xweb_xhr": "1",
                'appId': this.appId,
                'Authorization': this.auth,
                'openId': this.openId,
                "Content-Type": "application/json",
                "Sec-Fetch-Site": "cross-site",
                "Sec-Fetch-Mode": "cors",
                "Sec-Fetch-Dest": "empty",
                "Referer": `https://servicewechat.com/${this.appId}/85/page-frame.html`,
                "Accept-Language": "zh-CN,zh;q=0.9",
                "Accept-Encoding": "gzip, deflate, br"
            };

            return true
        }

        async sign() {
            const methodName = '签到'
            const data = JSON.stringify({})
            const enData = this.encryptData(data)
            const timestamp = Date.now()
            const nonce = this.randomString()
            const sign = this.generateSign(enData, timestamp, nonce)

            const method = async () => {
                const options = {
                    url: `${this.baseUrl}/mbuy/intf/userCoin/sign`,
                    method: "POST",
                    headers: this.headers,
                    data: JSON.stringify({
                        data: enData,
                        nonce: nonce,
                        sign: sign,
                        timestamp: timestamp
                    }
                    )
                }
                const res = await this.request(options, 0)
                if (res?.code === 0) {
                    this.sendMessage(`✅ [${methodName}] 成功,获得金币${res?.data?.rewardCoin}`)
                    this.statisticSetSuccessWithValue(methodName, res?.data?.rewardCoin, '金币')
                    return res?.data?.rewardCoin
                }
                else {
                    this.statisticSetFailure(methodName);
                    this.sendMessage(`接口返回：${res?.message || res?.errmsg || "未知错误信息"}`)
                }
            }
            return await this.safeExecute(method, methodName)
        }
        async afterWatchAds(times) {
            const methodName = '看广告领金币'
            const data = JSON.stringify({ watchTimes: times })
            const enData = this.encryptData(data)
            const timestamp = Date.now()
            const nonce = this.randomString()
            const sign = this.generateSign(enData, timestamp, nonce)

            const method = async () => {
                const options = {
                    url: `${this.baseUrl}/mbuy/intf/userCoin/afterWatchAds`,
                    method: "POST",
                    headers: this.headers,
                    data: JSON.stringify({
                        data: enData,
                        nonce: nonce,
                        sign: sign,
                        timestamp: timestamp
                    }
                    )
                }
                const res = await this.request(options, 0)
                if (res?.code === 0) {
                    this.sendMessage(`✅ [${methodName}] 成功,获得金币${res?.data?.rewardCoin}`)
                    this.statisticSetSuccessWithValue(methodName, res?.data?.rewardCoin, '金币')
                    return res?.data?.rewardCoin
                }
                else {
                    this.statisticSetFailure(methodName);
                    this.sendMessage(`❌ 接口返回：${res?.message || res?.errmsg || "未知错误信息"}`)
                }
            }
            return await this.safeExecute(method, methodName)
        }
        async receiveRecord() {
            const methodName = '金币接收'
            const method = async () => {
                const options = {
                    url: `${this.baseUrl}/mbuy/intf/userCoin/receiveRecord?changeType=1`,
                    method: "GET",
                    headers: this.headers
                }
                const res = await this.request(options, 0)
                if (res?.code === 0) {
                }
                else {
                    this.sendMessage(`接口返回：${res?.message || res?.errmsg || "未知错误信息"}`)
                }
            }
            return await this.safeExecute(method, methodName)
        }

        async userInfo() {
            const methodName = '用户信息'
            const method = async () => {
                const options = {
                    url: `${this.baseUrl}/mbuy/intf/userCash/info`,
                    method: "GET",
                    headers: this.headers
                }
                const res = await this.request(options, 0)

                if (res?.code === 0) {
                    const coin = res?.data.coin
                    const coinCash = (coin / 100).toFixed(2)
                    const cash = res?.data?.cash
                    const cashMoney = (cash / 100).toFixed(2)
                    const isPush = coin + cashMoney >= 50
                    this.sendMessage(`✅ [${methodName}] 成功,${isPush === true ? '可以提现了，' : ''}当前金币：${coin}(=${coinCash}r)，当前金额：${cashMoney}r`, isPush)
                }
                else {
                    this.statisticSetFailure(methodName);
                    this.sendMessage(`接口返回：${res?.message || res?.errmsg || "未知错误信息"}`)
                }
            }
            return await this.safeExecute(method, methodName)
        }


        async doTask() {
            let times = 0
            let isSuccess = true
            while (isSuccess) {
                this.sendMessage(`开始第${times + 1}次看广告`)
                const random = this.base.wqwlkj.getRandom(10, 35)
                this.sendMessage(`🕒 随机暂停${random}s`)
                await this.base.wqwlkj.sleep(random)
                isSuccess = await this.afterWatchAds(random)
                const random2 = this.base.wqwlkj.getRandom(1, 3)
                this.sendMessage(`🕒 随机暂停${random2}s`)
                await this.receiveRecord()
                if (isSuccess)
                    times++
            }
            this.sendMessage(`运行完成，共成功${times}次`)
        }

        encryptData(data) {
            const key = 'asdgkdd634464579';
            const iv = 'dadjadaddf125876';

            // 使用 aes-128-cbc 保持与原解密函数一致，输出 base64 格式
            return this.base.wqwlkj.aesEncrypt(data, key, iv, 'aes-128-cbc', 'utf8', 'utf8', 'base64');
        }
        generateSign(data, timestamp, nonce) {
            const signStr = data + timestamp + nonce + 'adweeoqi56789413';
            return crypto.createHash('sha256').update(signStr).digest('hex');
        }
        randomString(length = 8) {
            const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
            let result = '';
            for (let i = 0; i < length; i++) {
                result += chars.charAt(Math.floor(Math.random() * chars.length));
            }
            return result;
        }
        async main() {
            const init = await this.init()
            if (!init) return;
            await this.sign()
            let random2 = this.base.wqwlkj.getRandom(1, 3)
            this.sendMessage(`🕒 随机暂停${random2}s`)
            await this.doTask()
            random2 = this.base.wqwlkj.getRandom(1, 3)
            this.sendMessage(`🕒 随机暂停${random2}s`)
            await this.userInfo()
        }

    }

    if (wqwlkj.WQWLBase && wqwlkj.WQWLBaseTask) {
        const base = new wqwlkj.WQWLBase(wqwlkj, ckName, scriptName, version, isNeedFile, proxy, isProxy, bfs, isNotify, isDebug, isNeedTimes);
        await base.runTasks(Task);
    }
    else {
        // 如果 wqwl_require.js 没有导出 WQWLBase，可能需要手动处理
        console.log('❌ wqwl_require.js 未发现WQWLBase类、WQWLBaseTask类，请重新下载新版本');
        console.log('地址：' + url);
    }
})();