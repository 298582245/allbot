/**
 * 脚本：wqwl_new_星韵优选.js
 * 作者：wqwlkj 裙：960690899
 * 描述：微信小程序星韵优选，抓包gzpengru.weimbo.com请求头的3rdsession
 * 环境变量：wqwl_xyyx，多个换行或新建多个变量（不能混合使用）
 * 环境变量描述：
 * cron: 8 8 * * *
 */

//环境变量
const ckName = 'wqwl_xyyx';
//脚本名称
const scriptName = '微信小程序星韵优选';
//本地版本
const version = 1.0;
//是否需要文件存储
const isNeedFile = true;
//ck长度
const ckLength = 1;
//日志是否需要具体时间
const isNeedTimes = false;
//日志是否需要推送汇总
const isNeedDetailed = true;

const proxy = process.env["wqwl_daili"] || '';
const isProxy = process.env["wqwl_useProxy"] || false;
const bfs = process.env["wqwl_bfs"] || 3;
const isNotify = process.env["wqwl_isNotify"] || true;
const isDebug = process.env["wqwl_isDebug"] || 2;

const axios = require('axios');
const fs = require('fs');
const crypto = require('crypto');

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

// 创建基于种子的随机数生成器
class SeededRandom {
    constructor(seed) {
        this.seed = seed;
    }

    // 生成0-1之间的随机数
    random() {
        const hash = crypto.createHash('md5').update(this.seed.toString()).digest('hex');
        const num = parseInt(hash.substring(0, 8), 16);
        this.seed = (num + 1) % 0xFFFFFFFF;
        return num / 0xFFFFFFFF;
    }

    // 生成min到max之间的随机整数
    randint(min, max) {
        return Math.floor(this.random() * (max - min + 1)) + min;
    }

    // 生成min到max之间的随机浮点数
    randomFloat(min, max) {
        return this.random() * (max - min) + min;
    }

    // 从数组中随机选择一个元素
    choice(arr) {
        const index = this.randint(0, arr.length - 1);
        return arr[index];
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
            this.baseUrl = 'https://gzpengru.weimbo.com';

            // 任务状态
            this.isSignCompleted = false;
            this.isVideoCompleted = false;
            this.isAllDone = false;
            this.nextRunTime = 0;
        }

        async init() {
            const ckData = this.ck.split('#');
            if (ckData.length < ckLength) {
                this.sendMessage(`${this.index + 1} 环境变量有误，请检查环境变量是否正确`, true);
                return false;
            } else if (ckData.length === ckLength) {
                this.remark = `${ckData[0].slice(0, 8)}-${this.index}`;
            } else {
                this.remark = ckData[ckLength];
            }

            this.token = ckData[0];

            // 生成绑定UA
            this.ua = this.generateBoundUA(this.token);
            this.sendMessage(`🎲 使用ua：${this.ua.slice(0, 60)}...`);

            // 设置请求头
            this.headers = {
                'Host': 'gzpengru.weimbo.com',
                'Connection': 'keep-alive',
                '3rdsession': this.token,
                'content-type': 'application/json',
                'User-Agent': this.ua,
                'Referer': 'https://servicewechat.com/wxc86c9aecdb67f876/9/page-frame.html'
            };

            if (this.proxyConfig && this.isProxy) {
                this.proxy = await wqwlkj.getProxy(this.index, this.proxyConfig);
                this.sendMessage(`✅ 使用代理：${this.proxy}`);
            } else {
                this.proxy = '';
            }

            return true;
        }

        // 生成绑定UA（根据token生成固定UA）
        generateBoundUA(token) {
            // 使用token的hash值作为种子
            const seed = crypto.createHash('md5').update(token).digest('hex');
            const random = new SeededRandom(seed);

            const osType = random.choice(["Android", "iOS"]);

            if (osType === "Android") {
                const androidVer = random.choice(["10", "11", "12", "13", "14"]);
                const chromeVer = `${random.randint(86, 120)}.0.${random.randint(4000, 6000)}.${random.randint(100, 200)}`;
                const phoneModel = random.choice(["SM-G9810", "V2055A", "M2012K11AC", "PADT00", "KB2000", "MI 10"]);
                return `Mozilla/5.0 (Linux; Android ${androidVer}; ${phoneModel} Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/${chromeVer} MicroMessenger/8.0.45.2400(0x28002B3D) WeChat/arm64 Weixin NetType/WIFI Language/zh_CN ABI/arm64`;
            } else {
                const iosVer = random.choice(["15_0", "16_2", "17_1"]);
                return `Mozilla/5.0 (iPhone; CPU iPhone OS ${iosVer} like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.46(0x18002e2f) NetType/WIFI Language/zh_CN`;
            }
        }

        // 发送请求
        async postRequest(payload) {
            const options = {
                url: 'https://gzpengru.weimbo.com/api/index.php?ackey=GZYTAPPLET',
                headers: this.headers,
                method: 'POST',
                data: payload
            };

            await this.base.wqwlkj.sleep(await this.base.wqwlkj.getRandom(1, 3));

            try {
                const data = await this.base.wqwlkj.request(options, this.proxy);
                return data;
            } catch (e) {
                this.sendMessage(`请求异常: ${e.message}`);
                return null;
            }
        }

        // 获取用户信息
        async getUserInfo(isPush = false) {
            const methodName = '获取用户信息';
            this.sendMessage(`🔍 正在${methodName}...`);

            const payload = { "action": "userInfoData" };
            const data = await this.postRequest(payload);

            if (data && data.Status) {
                const userData = data.Data || {};
                const userName = userData.user?.name || "未知";
                const jifen = userData.u_money?.jifen || 0;
                this.sendMessage(`👤 用户: ${userName} | 当前积分: ${jifen}`, isPush);
                return true;
            } else {
                this.sendMessage(`❌ Token失效`, true);
                return false;
            }
        }

        // 检查任务进度
        async checkTaskProgress() {
            const payload = { "action": "getIntegralInfo", "type": "jifen" };
            const data = await this.postRequest(payload);

            let signStr = "未知";
            let videoStr = "0/3";

            if (data && data.Status) {
                const advArr = data.Data?.adv_arr || [];

                for (const task of advArr) {
                    const title = task.title || "";
                    const taskId = task.id;

                    if (taskId === 2) { // 打卡任务
                        const match = title.match(/\((\d+)\/(\d+)\)/);
                        if (match) {
                            const curr = parseInt(match[1]);
                            const total = parseInt(match[2]);
                            signStr = `${curr}/${total}`;
                            this.isSignCompleted = (curr >= total);
                        }
                    } else if (taskId === 3) { // 视频任务
                        const match = title.match(/\((\d+)\/(\d+)\)/);
                        if (match) {
                            const curr = parseInt(match[1]);
                            const total = parseInt(match[2]);
                            videoStr = `${curr}/${total}`;
                            this.isVideoCompleted = (curr >= total);
                        }
                    }
                }

                this.isAllDone = this.isSignCompleted && this.isVideoCompleted;

                if (this.isAllDone) {
                    this.sendMessage(`🎉 今日所有任务已完成 (打卡:${signStr} 视频:${videoStr})`, true);
                } else {
                    this.sendMessage(`📊 当前进度: 打卡[${signStr}] 视频[${videoStr}]`);
                }

                return true;
            }
            return false;
        }

        // 执行视频广告任务
        async executeVideoAdTask() {
            if (this.isVideoCompleted) {
                this.sendMessage("🎬 视频任务: 今日已全部完成，跳过");
                return;
            }

            for (let i = 0; i < 3; i++) {
                const payloadAd = { "action": "IntegralGiveReward" };
                const res = await this.postRequest(payloadAd);

                if (res && res.Status) {
                    const msg = res.Data || "";
                    this.sendMessage(`🎬 视频任务: ✅ ${msg}`);
                    const match = msg.match(/获得(\d+)积分/);
                    if (match && match[1]) {
                        this.statisticSetSuccessWithValue(`视频任务`, match[1], '积分')
                    }
                } else {
                    const msg = res?.Message || "未知错误";
                    if (msg.includes("上限") || msg.includes("完成")) {
                        this.sendMessage("🎬 视频任务: ❌ 今日已达上限");
                        this.isVideoCompleted = true;
                        break;
                    } else {
                        this.sendMessage(`⚠️ 视频任务失败: ${msg}`);
                    }
                }
                await this.base.wqwlkj.sleep(await this.base.wqwlkj.getRandom(30, 50))
            }
        }

        // 处理任务循环
        async processCycle() {
            if (!await this.checkTaskProgress()) {
                return 60;
            }

            if (this.isAllDone) {
                return -1;
            }

            if (!this.isVideoCompleted) {
                await this.executeVideoAdTask();
                await this.checkTaskProgress();
            }

            if (this.isAllDone) {
                return -1;
            }

            if (this.isSignCompleted) {
                if (this.isVideoCompleted) {
                    return -1;
                } else {
                    return 60;
                }
            }

            // 检查打卡状态
            const payloadStatus = { "action": "getIntegralInfo", "type": "sign" };
            const dataStatus = await this.postRequest(payloadStatus);

            let waitSeconds = 60;

            if (dataStatus && dataStatus.Status) {
                const statusData = dataStatus.Data || {};
                const signTime = statusData.sign_time || 0;
                const qiands = statusData.qiands || "未知";

                if (signTime > 0) {
                    this.sendMessage(`📍 打卡状态: ${qiands} | 冷却中: ${signTime}秒`);
                    waitSeconds = signTime + 5;
                } else {
                    this.sendMessage("📍 冷却归零，执行打卡...");
                    const payloadSign = { "action": "userQiandao" };
                    const dataSign = await this.postRequest(payloadSign);

                    if (dataSign && dataSign.Status) {
                        const res = dataSign.Data || {};
                        const addJf = res.add_jf || 0;
                        const newJf = res.user_jf || 0;
                        this.statisticSetSuccessWithValue(`打卡`, addJf, '积分')
                        this.sendMessage(`✅ 打卡成功! +${addJf}分 | 总分: ${newJf}`);
                        return 1;
                    } else {
                        const msg = dataSign?.Message || "无响应";
                        this.sendMessage(`❌ 打卡失败: ${msg}`);
                        waitSeconds = 60;
                    }
                }
            }

            return waitSeconds;
        }

        // 检查并运行任务
        async checkAndRun() {
            const now = Date.now() / 1000;

            if (now >= this.nextRunTime) {
                if (await this.getUserInfo()) {
                    const waitSeconds = await this.processCycle();

                    if (waitSeconds === -1) {
                        this.sendMessage("🏆 该账号今日任务全部完成，停止运行。");
                        return true;
                    }

                    this.nextRunTime = now + waitSeconds;
                    const nextTime = new Date(this.nextRunTime * 1000).toLocaleTimeString('zh-CN');
                    this.sendMessage(`本轮结束，下次运行: ${nextTime}`);
                } else {
                    this.nextRunTime = now + 3600;
                    this.sendMessage("账号Token异常，暂停1小时");
                }
            }
            return false;
        }

        async main() {
            const init = await this.init();
            if (!init) return;

            // 持续运行直到所有任务完成
            while (true) {
                const isFinished = await this.checkAndRun();
                if (isFinished) break;

                const sleepTime = Math.max(1, this.nextRunTime - Date.now() / 1000);
                if (sleepTime > 30) {
                    this.sendMessage(`--- 系统待机: 等待 ${Math.floor(sleepTime)} 秒 ---`);
                }

                await this.base.wqwlkj.sleep(Math.min(sleepTime, 30));
            }
            await this.getUserInfo(true)
        }
    }

    if (wqwlkj.WQWLBase && wqwlkj.WQWLBaseTask) {
        const base = new wqwlkj.WQWLBase(wqwlkj, ckName, scriptName, version, isNeedFile, proxy, isProxy, bfs, isNotify, isDebug, isNeedTimes, isNeedDetailed);
        await base.runTasks(Task);
    } else {
        console.log('❌ wqwl_require.js 未发现WQWLBase类、WQWLBaseTask类，请重新下载新版本');
    }
})();