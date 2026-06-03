//[title: wqwl-粉象生活]
//[author: wqwlkj2985]
//[language: nodejs]
//[class: 工具类]
//[service: qq298582245] 售后联系方式
//[disable: false] 禁用开关，true表示禁用，false表示可用
//[admin: false] 是否为管理员指令
//[rule: ^(粉象)(登录|管理|查询|一键运行|全部提现|提现|授权|签到)$] 匹配规则1
//[cron: 38 0,22 * * *] cron定时，支持5位域和6位域
//[priority: 298582245] 优先级，数字越大表示优先级越高
//[platform: qq] 适用的平台
//[open_source: false]是否开源
//[icon: http://175.24.81.131:8044/admin/images/gallery/1771682713032996077.jpg]图标链接地址，请使用48像素的正方形图标，支持http和https
//[version: 1.0.3]版本号
//[public: true] 是否发布？值为true或false，不设置则上传aut云时会自动设置为true，false时上传后不显示在市场中，但是搜索能搜索到，方便开发者测试
//[price: 12.88] 上架价格
//[param: {"required":true,"key":"wqwl_points.fxsh","bool":false,"placeholder":"授权所需积分，默认为0","name":"授权所需积分","desc":"授权所需积分，默认为0，单位积分/月，仅适配了linzixuan的卡密系统"}]
//[param: {"required":false,"key":"wqwl_config.fxsh_isRandomUser","bool":true,"placeholder":"是否账号乱序，默认false","name":"是否账号乱序","desc":"是否账号乱序，默认false，据我观察每个账号固定时间点中奖概率高，但是中奖账号一般也是那一批账号,随机的话别的账号会高一点，但是其实也差不多，都是看脸中奖"}]
//[param: {"required":false,"key":"wqwl_config.fxsh_signDate","placeholder":"临时签到期数","name":"临时签到期数","desc":"临时签到期数，不知道以后出是否接口不变，现在先预留"}]
//[param: {"required":false,"key":"wqwl_config.fxsh_signEndDate","placeholder":"临时签到结束时间","name":"临时签到结束时间","desc":"临时签到结束时间，按照20260504的格式填，不然可能报错"}]

//[description: v1.0.3，新增临时签到，这一期只有七天，用户指令：粉象签到，管理员的已经放在粉象一键运行里面了<br>几个指令：粉象登录、粉象管理、粉象查询、粉象(全部)提现、粉象一键运行（管理员）、粉象授权（管理员）<br>每天需要运行指令粉象一键运行两次，一次开奖前（21:35），一次开奖后，可以使用autman计划任务自处理定时] 使用方法尽量写具体
const crypto = require("crypto")
const https = require('https');
const axios = require("axios")
let middlewareCache = null
function getMiddleware() {
    if (!middlewareCache) middlewareCache = require('./middleware')
    return middlewareCache
}
function push(...args) {
    return getMiddleware().push(...args)
}
const $ = new Env("粉象生活App");

https.globalAgent.options.rejectUnauthorized = false;
let userIdx = 0;
let strSplitor = "#"; //多变量分隔符

class Task {
    constructor(str) {
        this.index = ++userIdx;
        this.did = str.split(strSplitor)[0];
        this.finger = str.split(strSplitor)[1]; //单账号多变量分隔符
        this.token = str.split(strSplitor)[2]; //单账号多变量分隔符
        this.oaid = str.split(strSplitor)[3]; //单账号多变量分隔符
        this.name = str.split(strSplitor)[4];
        this.ckStatus = true;
        this.taskList = []

    }
    async main() {
        await this.user_info();
        if (!this.ckStatus) {
            return;
        }
        await this.sign_reward()
        //await this.sign()
        //await this.coin_receive()
        await this.play_video()
        //await this.open_box()
        /*
        let fruiterId = await this.query_fruiter();
        if (!fruiterId) {
            let fruiter_list = await this.query_fruiter_list();
            if (fruiter_list?.length) {
                $.log(`✅账号[${this.index}]  自动选择第一个水果，水果名为 ${fruiter_list[0]?.description}`);
                fruiterId = await this.plat_fruiter(fruiter_list[0]?.id);
                if (!fruiterId) {
                    return;
                }

            } else {
                console.log(`❌账号[${this.index}]  未获取到可种植水果列表！`);
            }
        }
        await this.stealWater();
        const fruiter_login_info = await this.fruiter_sign_detail();
        const todaySign = fruiter_login_info?.find(item => item?.status == 1);
        if (todaySign?.id) {
            await this.loginAwardReceive(todaySign?.id);
        }
        await $.wait(Math.random() * 2000 + 1000);
        let fruite_tasks = await this.query_fruite_tasks();
        for (let index = 0; index < fruite_tasks?.length; index++) {
            const fruite_task = fruite_tasks[index];
            await this.finish_fruiter_task(fruite_task?.type, fruite_task?.taskInfoId);
            await $.wait(Math.random() * 2000 + 1000);
        }
        while (true) {
            const waterRes = await this.water_fruiter(fruiterId);
            if (waterRes?.canAcquireWaterId) {
                await this.acquireWater_fruiter(fruiterId, waterRes?.canAcquireWaterId);
            }
            if (!waterRes) {
                break;
            }
            await $.wait(Math.random() * 2000 + 1000);
        }*/
        await this.activity_withdraw_check()
        await this.special_finish()
        await this.task_list()
        //console.log(this.taskList)
        for (let i of this.taskList) {
            await $.wait(3000)
            await this.task_finish(i.id, i.title)
        }
        /*
        const gameTasks = await this.game_task();
        for (let gameTask of gameTasks) {
            await $.wait(Number(gameTask?.item?.taskDuration) * 1000 + Math.random() * 1000);
            await this.game_finish(gameTask)
        }*/
    }

    async user_info() {
        let result = await this.taskRequest("get", `https://api.fenxianglife.com/njia/users/info`)
        //console.log(result);
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  欢迎用户，Id ${result.data.userInfo.id} 昵称 ${result.data.userInfo.nickname} 手机号 ${result.data.userInfo.mobile}🎉`)
            this.ckStatus = true;
        } else {
            console.log(`❌账号[${this.index}]  用户查询: 失败`);
            this.ckStatus = false;
            //console.log(result);
        }
    }
    async task_finish(id, title) {
        let result = await this.taskRequest("post", `https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/lotteryCode/task/finish`, JSON.stringify({
            "taskId": id
        }))
        console.log(result);
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  任务[${id}][${title}]完成🎉`)
        } else {
            console.log(`❌账号[${this.index}]  任务[${id}][${title}]失败`);
            //console.log(result);
        }
    }

    async activity_withdraw_check() {
        let result = await this.taskRequest("get", `https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/withdraw/index`, '')
        // console.log(result);
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  今天开奖金额 ${result.data?.totalRewardAmount / 100}元 ${result.data?.amountReceiveStatus == 1 ? '未领取' : '已领取'}🎉`)

            await this.activity_receive_all();

        } else {
            console.log(`❌账号[${this.index}]  查询今日开奖金额: 失败`);
            //console.log(result);
        }
    }

    async activity_receive_all() {
        let result = await this.taskRequest("post", `https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/periodical/open/result/receiveAll`, JSON.stringify({}))
        console.log(result);
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  提现开奖金额成功🎉`)
        } else {
            console.log(`❌账号[${this.index}]  提现开奖金额: 失败`);
            //console.log(result);
        }
    }

    async special_finish() {
        let result = await this.taskRequest("post", `https://api.fenxianglife.com/njia/game/task/special/finish`, JSON.stringify({
        }))
        console.log(result);
        if (result.errcode == 0) {
            $.log(`✅账号[${this.index}]  欢迎用户: ${result.errcode}🎉`)
            this.ckStatus = true;
        } else {
            console.log(`❌账号[${this.index}]  用户查询: 失败`);
            this.ckStatus = false;
            //console.log(result);
        }
    }

    async game_finish(gameTask) {
        let result = await this.taskRequest("post", `https://api.fenxianglife.com/njia/game/task/finish`,
            JSON.stringify({ "taskId": gameTask?.item?.id, "gameType": gameTask?.activityType }))
        // console.log(result);
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  完成领现金兑商品活动 任务[${gameTask?.name}][${gameTask?.item?.id}][${gameTask?.item?.taskChanceUse}/${gameTask?.item?.taskChance}]: ${result.data?.toastText || `获得${result.data?.awardCount || 0}金币`}🎉`)
            this.ckStatus = true;
            if (result.data?.awardCount) {
                if (gameTask?.item?.taskChanceUse < gameTask?.item?.taskChance) {
                    $.log(`✅账号[${this.index}]  延迟${gameTask?.item?.taskDuration}秒执行下一个任务`);
                    await $.wait(Number(gameTask?.item?.taskDuration) * 1000 + Math.random() * 1000);
                    return this.game_finish(gameTask);
                }
            }
        } else {
            console.log(`❌账号[${this.index}]  完成领现金兑商品活动 任务[${gameTask?.name}][${gameTask?.item?.id}] 失败`);
            this.ckStatus = false;
            console.log(result);
        }
    }

    async open_box() {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/elephant/activity/limitedTime/complete`,
            JSON.stringify({ "doublingKey": "doubling" }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  开宝箱成功，获得 ${result?.data?.reward || 0} 金币🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  开宝箱失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async invite(initiatorId = "515233097") {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/elephant/mammon/help`,
            JSON.stringify({
                "initiatorId": initiatorId
            }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  助力成功🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  助力失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async sign() {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/elephant/sign`,
            JSON.stringify({}))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  签到成功🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  签到失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async coin_receive() {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/elephant/coin/receive`,
            JSON.stringify({}))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  收取金币成功🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  收取金币失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async play_video() {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/game/task/finish`,
            JSON.stringify({
                "taskId": 1,
                "gameType": 2,
            }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  短视频观看成功🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  短视频观看失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async game_task() {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/game/task/list`,
            JSON.stringify({ "gameType": 2, "platform": "android", "version": "6.7.1" }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  获取游戏任务列表成功🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  获取游戏任务列表失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async sign_reward() {
        let result = await this.taskRequest("post", `https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/user/sign/reward`, JSON.stringify({
        }))
        //console.log(result);
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  签到成功🎉`)
        } else {
            console.log(`❌账号[${this.index}]  签到失败`);
            console.log(result);
        }
    }
    async task_list() {
        let result = await this.taskRequest("post", 'https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/home/data/V2', JSON.stringify({
            "plateform": "android",
            "version": "6.7.1"
        }));
        //console.log(result);
        if (result.code == 200) {
            for (let i of result.data.taskModule.taskResult) {
                if (i.taskStatus == 0) {
                    this.taskList.push(i)
                }
            }

        } else {
            console.log(`❌账号[${this.index}]  获取任务失败`);
            //console.log(result);
        }
    }
    async query_fruiter() {
        let result = await this.taskRequest("get", `https://api-1.fenxianglife.com/njia/orchard/user/fruiter/detail`, '')
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  查询果园信息成功：${result?.data?.fruiterDesc || '未种植'}🎉`);
            return result?.data?.id;
        } else {
            console.log(`❌账号[${this.index}]  查询果园信息失败`);
            console.log(result);
        }
    }
    async fruiter_sign_detail() {
        let result = await this.taskRequest("get", `https://api-1.fenxianglife.com/njia/orchard/loginAward/user/detail`, '')
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  获取果园签到信息成功🎉`);
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  获取果园签到信息失败`);
            console.log(result);
        }
    }
    async query_fruite_tasks() {
        let result = await this.taskRequest("get", `https://api-1.fenxianglife.com/njia/orchard/task/list`, '')
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  查询果园任务成功：${result?.data?.length || '0'}个任务 🎉`);
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  查询果园任务失败`);
            console.log(result);
        }
    }
    async query_fruiter_list() {
        let result = await this.taskRequest("get", `https://api-1.fenxianglife.com/njia/orchard/fruiter/list`, '')
        if (result.code == 200) {
            $.log(`✅账号[${this.index}]  当前可选择的种植水果个数为 ：${result?.data?.length || '0'}🎉`);
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  查询可选择的种植水果信息失败`);
            console.log(result);
        }
    }

    async water_fruiter(userFruiterId) {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/user/fruiter/water`,
            JSON.stringify({ "userFruiterId": userFruiterId }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  浇水[${userFruiterId}]成功  ${result?.data?.upgradeContext || ''} 🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  浇水[${userFruiterId}]失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async loginAwardReceive(day) {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/loginAward/receive`,
            JSON.stringify({ "day": day }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  领取签到奖励成功 🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  领取签到奖励失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async finish_fruiter_task(type, taskInfoId) {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/task/finish`,
            JSON.stringify({ "type": type, "taskInfoId": taskInfoId }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  完成任务[${taskInfoId}]成功，获得水滴 ${result?.data?.upgradeContext || ''} 🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  完成任务[${taskInfoId}]失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async finish_push() {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/task/push/finish`,
            JSON.stringify({}))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  领取开启推送通知奖励成功，获得 ${result?.data?.awardCount || 0}个水滴 🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  领取开启推送通知奖励失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async acquireWater_fruiter(userFruiterId, canAcquireWaterId) {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/user/fruiter/acquireWater`,
            JSON.stringify({ "canAcquireWaterId": canAcquireWaterId, "userFruiterId": userFruiterId }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  领取浇水奖励成功 🎉`)
            return result?.data;
        } else {
            console.log(`❌账号[${this.index}]  领取浇水奖励失败`);
            console.log(result);
            //console.log(result);
        }
    }

    async plat_fruiter(fruiterId) {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/user/fruiter/plant`,
            JSON.stringify({ "fruiterId": fruiterId }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  种植水果[${fruiterId}]成功，水果树ID为 ${result?.data?.userFruiterId || ''} 🎉`)
            return result?.data?.userFruiterId;
        } else {
            console.log(`❌账号[${this.index}]  种植水果[${fruiterId}]失败`);
            console.log(result);
            //console.log(result);
        }
    }
    async stealWater(friendId = -1) {
        let result = await this.taskRequest("post", `https://api-1.fenxianglife.com/njia/orchard/friend/stealWater`,
            JSON.stringify({ "friendId": friendId }))
        if (result.success == true) {
            $.log(`✅账号[${this.index}]  从朋友[${friendId}]  ${result?.data || ''} 🎉`)
            return result?.data?.userFruiterId;
        } else {
            console.log(`❌账号[${this.index}]  从朋友[${friendId}]偷取水滴失败`);
            console.log(result);
            //console.log(result);
        }
    }


    //信息查询
    async query() {
        let result = '=====账号信息=====\n';
        try {

            let raw1 = await this.taskRequest("post", `https://api.fenxianglife.com/njia/order/withdraw/v4/create`,
                JSON.stringify({ "orderType": 5 })
            )

            let raw2 = await this.taskRequest("get", `https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/withdraw/index`, '')

            let raw3 = await this.taskRequest("post", `https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/home/data/V2`,
                JSON.stringify({
                    "plateform": "android",
                    "version": "6.7.1"
                })
            )
            const dataStr = raw2?.data?.dateStr || ''

            const isToday = this.getDateRange().includes(dataStr) ? true : false
            //result += `🎉今日开奖：${isToday ? '是' : '否'}${this.getDate()},${raw3?.data?.openLotteryModule?.now?.rewardCodes?.[0]?.dateStr}`
            result += `🤪 用户ID(备注)：${this.name}\n`
            if (raw1.code == 200) {
                result += `🧧活动奖励余额：${raw1?.data?.maxWithdrawAmount / 100}元(最低0.1起提)\n`
            } else {
                result += `🧧活动奖励余额：查询失败，原因：${raw1?.message || '未知原因'}\n`
            }

            if (raw2.code == 200) {

                if (isToday) {
                    result += `🎰本期开奖期数：${dataStr}期\n`
                    result += `🎉本期开奖金额：${raw2?.data?.totalRewardAmount / 100}元 (${result.data?.amountReceiveStatus == 1 ? '未领取' : '已领取'})\n`
                    if (raw2?.data?.freeOrderCount > 0 && raw2?.data?.freeItem?.itemTitle) {
                        result += `🎁本期免单商品：【${raw2?.data?.freeItem?.itemTitle}】\n`
                    } else {
                        result += '🎁本期免单商品：无\n'
                    }
                }
                else {
                    result += '🎉本期开奖金额：0元(未开奖)\n'
                }
            } else {
                result += `🎉本期开奖金额：查询失败，原因：${raw2?.message || '未知原因'}\n`
            }

            if (raw3.code == 200) {
                if (raw3?.data?.openLotteryModule?.now?.rewardCodes.length <= 0 || raw3?.data?.openLotteryModule?.now?.rewardCodes[0]?.dateStr === undefined)
                    result += `🏆现有奖码个数：${raw3?.data?.openLotteryModule?.now?.rewardCodes.length}个 \n`
                else
                    result += `🏆现有奖码个数：${raw3?.data?.openLotteryModule?.now?.rewardCodes.length}个 (${raw3?.data?.openLotteryModule?.now?.rewardCodes[0]?.dateStr}期)\n`
            } else {
                result += `🏆现有奖码个数：查询失败，原因：${raw3?.message || '未知原因'}\n`
            }

            //🏆
        } catch (e) {
            console.error("请求失败:", e);
            result = `❌用户【${this.name}】信息查询失败，原因：${e.message}\n`
        }
        return result
    }

    //短信登录
    async smsCode(phone) {
        const result = await this.taskRequest('post', 'https://api.fenxianglife.com/njia/util/sms/code',
            JSON.stringify({
                "validateType": 1,
                "mobileArea": "86",
                "mobile": phone,
                "type": 1
            })
        )
        return result
    }
    //登录验证
    async login(phone, code) {
        const result = await this.taskRequest('post', 'https://api.fenxianglife.com/njia/login/mobile',
            JSON.stringify({
                "mobileArea": "86",
                "smsCode": this.MD5(code),
                "mobile": phone
            })
        )
        return result
    }

    //提现
    async withDraw(userName) {
        let result = {
            isSuccess: false,
            msg: '',
            money: 0
        }

        //https://api.fenxianglife.com/njia/order/withdraw/v4/create
        const res1 = await this.taskRequest('post', 'https://api.fenxianglife.com/njia/order/withdraw/v4/create',
            JSON.stringify({
                "orderType": 5
            })
        )

        if (res1?.code != 200 || res1?.data?.alipayAccountInfo?.userName == null || res1?.data?.alipayAccountInfo?.identityCard == null || res1?.data?.alipayAccountInfo?.alipayAccount == null) {
            result.msg = `❌${userName}提现创建失败，请确保绑定了zfb,接口返回原因：${res1?.message}`;
            result.money = 0;
            result.isSuccess = false;
            return result;
        }

        const totalWithdrawAmount = res1?.data?.totalWithdrawAmount;

        const maxWithdrawAmount = res1?.data?.maxWithdrawAmount || 0;

        if (maxWithdrawAmount < 10) {
            result.money = 0;
            result.isSuccess = false;
            result.msg = `❌${userName}提现创建失败，还不够0.1提啥呢，当前可提现余额：${maxWithdrawAmount / 100}元`;
            return result;
        }

        const res2 = await this.taskRequest('post', 'https://api.fenxianglife.com/njia/order/withdraw/submit',
            JSON.stringify({
                "orderType": 5,
                "withdrawAmount": maxWithdrawAmount
            })
        )

        if (res2?.code != 200) {
            result.money = 0;
            result.isSuccess = false;
            result.msg = `❌${userName}提现提交失败，接口返回原因：${res2?.message || '未知原因'}`;
            return result;
        }
        //将\n换成,
        const subTitle = res2?.data?.subTitle.replace(/\n/g, ",");
        result.msg = `🎉${userName}提现提交成功，估计到账${maxWithdrawAmount / 100}元,${subTitle}`;
        result.isSuccess = true;
        result.money = maxWithdrawAmount / 100;
        return result;
    }

    //临时活动签到
    async clockSign(activityId) {
        const res = await this.taskRequest('post', 'https://api.fenxianglife.com/njia/takeaway/clock/in/sign',
            JSON.stringify({
                "activityId": activityId
            })
        )
        return res
    }

    tokenStr(phone) {
        return `${this.did}#${this.finger}#${this.token}#${this.oaid}#${phone.slice(0, 3)}****${phone.slice(-4)}`
    }
    async taskRequest(method, url, body = "") {
        function convertObjectToQueryString(obj) {
            let queryString = "";
            if (obj) {
                const keys = Object.keys(obj).sort();
                keys.forEach(key => {
                    const value = obj[key];
                    if (value !== null && typeof value !== 'object') {
                        queryString += `&${key}=${value}`;
                    }
                });
            }
            return queryString.slice(1);
        }

        const g = {
            traceid: this.MD5((new Date).getTime().toString() + Math.random().toString()),
            noncestr: Math.random().toString().slice(2, 10),
            timestamp: Date.now(),
            platform: "android",
            did: this.did,
            version: "6.7.1",
            finger: this.finger,
            token: this.token,
            oaid: this.oaid,
        }
        const c = "粉象好牛逼nb3b16f5a02479a0e34df78d14aefe76"
        let s = method === "get" ? void 0 : JSON.parse(body)
        let e = void 0 === s ? {} : s
        g.sign = this.MD5(c + convertObjectToQueryString(e) + convertObjectToQueryString(g))
        let headers = {
            'User-Agent': 'Mozilla/5.0 (Linux; Android 10; MI 8 Lite Build/QKQ1.190910.002; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/80.0.3987.99 Mobile Safari/537.36 AgentWeb/5.0.0  UCBrowser/11.6.4.950',
            'Accept': 'application/json, text/plain, */*',
            'Accept-Encoding': 'gzip, deflate',
            'Content-Type': 'application/json',
            'origin': 'https://m.fenxianglife.com',
            'sec-fetch-dest': 'empty',
            'x-requested-with': 'com.n_add.android',
            'sec-fetch-site': 'same-site',
            'sec-fetch-mode': 'cors',
            'referer': 'https://m.fenxianglife.com',
            'accept-language': 'zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7',
            "Content-Type": "application/json"
        }
        Object.assign(headers, g)
        //console.log(headers)
        const requestOptions = {
            url: url,
            method: method,
            headers: headers,
            data: body  // axios 中使用 data 而不是 body
        }

        try {
            const response = await axios(requestOptions);
            return response.data;
        } catch (error) {
            return { code: -1, message: "接口请求失败", error: error.message };
        }
    }
    MD5(e) {
        return crypto.createHash("md5").update(e).digest("hex")
    }

    //期号
    getDate() {
        const now = new Date();
        // 补零函数
        const padZero = (num) => String(num).padStart(2, '0');

        const year = now.getFullYear().toString().slice(2); // 取后两位：2025 → '25'
        const month = padZero(now.getMonth() + 1); // getMonth() 是从 0 开始的，所以要 +1
        const day = padZero(now.getDate());

        return `${year}${month}${day}`;
    }
    // 获取今天和昨天的期号数组 [今天, 昨天]
    getDateRange() {
        const padZero = (num) => String(num).padStart(2, '0');

        // 获取指定日期的期号
        const getDateStr = (date) => {
            const year = date.getFullYear().toString().slice(2);
            const month = padZero(date.getMonth() + 1);
            const day = padZero(date.getDate());
            return `${year}${month}${day}`;
        };

        const today = new Date();
        const yesterday = new Date();
        yesterday.setDate(yesterday.getDate() - 1);

        return [getDateStr(today), getDateStr(yesterday)];
    }
}

function Env(t, s) { return new (class { constructor(t, s) { this.name = t; this.logs = []; this.logSeparator = "\n"; this.startTime = new Date().getTime(); Object.assign(this, s); this.log("", `🔔${this.name},开始!`) } isNode() { return "undefined" != typeof module && !!module.exports } isQuanX() { return "undefined" != typeof $task } isSurge() { return "undefined" != typeof $httpClient && "undefined" == typeof $loon } isLoon() { return "undefined" != typeof $loon } initRequestEnv(t) { try { require.resolve("got") && ((this.requset = require("got")), (this.requestModule = "got")) } catch (e) { } try { require.resolve("axios") && ((this.requset = require("axios")), (this.requestModule = "axios")) } catch (e) { } this.cktough = this.cktough ? this.cktough : require("tough-cookie"); this.ckjar = this.ckjar ? this.ckjar : new this.cktough.CookieJar(); if (t) { t.headers = t.headers ? t.headers : {}; if (typeof t.headers.Cookie === "undefined" && typeof t.cookieJar === "undefined") { t.cookieJar = this.ckjar } } } queryStr(options) { return Object.entries(options).map(([key, value]) => `${key}=${typeof value === "object" ? JSON.stringify(value) : value}`).join("&") } getURLParams(url) { const params = {}; const queryString = url.split("?")[1]; if (queryString) { const paramPairs = queryString.split("&"); paramPairs.forEach((pair) => { const [key, value] = pair.split("="); params[key] = value }) } return params } isJSONString(str) { try { return JSON.parse(str) && typeof JSON.parse(str) === "object" } catch (e) { return false } } isJson(obj) { var isjson = typeof obj == "object" && Object.prototype.toString.call(obj).toLowerCase() == "[object object]" && !obj.length; return isjson } async sendMsg(message) { if (!message) return; if (this.isNode()) { await notify.sendNotify(this.name, message) } else { this.msg(this.name, "", message) } } async httpRequest(options) { let t = { ...options }; t.headers = t.headers || {}; if (t.params) { t.url += "?" + this.queryStr(t.params) } t.method = t.method.toLowerCase(); if (t.method === "get") { delete t.headers["Content-Type"]; delete t.headers["Content-Length"]; delete t.headers["content-type"]; delete t.headers["content-length"]; delete t.body } else if (t.method === "post") { let ContentType; if (!t.body) { t.body = "" } else if (typeof t.body === "string") { ContentType = this.isJSONString(t.body) ? "application/json" : "application/x-www-form-urlencoded" } else if (this.isJson(t.body)) { t.body = JSON.stringify(t.body); ContentType = "application/json" } if (!t.headers["Content-Type"] && !t.headers["content-type"]) { t.headers["Content-Type"] = ContentType } } if (this.isNode()) { this.initRequestEnv(t); if (this.requestModule === "axios" && t.method === "post") { t.data = t.body; delete t.body } let httpResult; if (this.requestModule === "got") { httpResult = await this.requset(t); if (this.isJSONString(httpResult.body)) { httpResult.body = JSON.parse(httpResult.body) } } else if (this.requestModule === "axios") { httpResult = await this.requset(t); httpResult.body = httpResult.data } return httpResult } if (this.isQuanX()) { t.method = t.method.toUpperCase(); return new Promise((resolve, reject) => { $task.fetch(t).then((response) => { if (this.isJSONString(response.body)) { response.body = JSON.parse(response.body) } resolve(response) }) }) } } randomNumber(length) { const characters = "0123456789"; return Array.from({ length }, () => characters[Math.floor(Math.random() * characters.length)]).join("") } randomString(length) { const characters = "abcdefghijklmnopqrstuvwxyz0123456789"; return Array.from({ length }, () => characters[Math.floor(Math.random() * characters.length)]).join("") } timeStamp() { return new Date().getTime() } uuid() { return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, function (c) { var r = (Math.random() * 16) | 0, v = c == "x" ? r : (r & 0x3) | 0x8; return v.toString(16) }) } time(t) { let s = { "M+": new Date().getMonth() + 1, "d+": new Date().getDate(), "H+": new Date().getHours(), "m+": new Date().getMinutes(), "s+": new Date().getSeconds(), "q+": Math.floor((new Date().getMonth() + 3) / 3), S: new Date().getMilliseconds(), }; /(y+)/.test(t) && (t = t.replace(RegExp.$1, (new Date().getFullYear() + "").substr(4 - RegExp.$1.length))); for (let e in s) new RegExp("(" + e + ")").test(t) && (t = t.replace(RegExp.$1, 1 == RegExp.$1.length ? s[e] : ("00" + s[e]).substr(("" + s[e]).length))); return t } msg(s = t, e = "", i = "", o) { const h = (t) => !t || (!this.isLoon() && this.isSurge()) ? t : "string" == typeof t ? this.isLoon() ? t : this.isQuanX() ? { "open-url": t } : void 0 : "object" == typeof t && (t["open-url"] || t["media-url"]) ? this.isLoon() ? t["open-url"] : this.isQuanX() ? t : void 0 : void 0; this.isMute || (this.isSurge() || this.isLoon() ? $notification.post(s, e, i, h(o)) : this.isQuanX() && $notify(s, e, i, h(o))); let logs = ["", "==============📣系统通知📣=============="]; logs.push(t); e ? logs.push(e) : ""; i ? logs.push(i) : ""; console.log(logs.join("\n")); this.logs = this.logs.concat(logs) } log(...t) { t.length > 0 && (this.logs = [...this.logs, ...t]), console.log(t.join(this.logSeparator)) } logErr(t, s) { const e = !this.isSurge() && !this.isQuanX() && !this.isLoon(); e ? this.log("", `❗️${this.name},错误!`, t.stack) : this.log("", `❗️${this.name},错误!`, t) } wait(t) { return new Promise((s) => setTimeout(s, t)) } done(t = {}) { const s = new Date().getTime(), e = (s - this.startTime) / 1e3; this.log("", `🔔${this.name},结束!🕛 ${e}秒`); this.log(); if (this.isNode()) { process.exit(1) } if (this.isQuanX()) { $done(t) } } })(t, s) }

!(async function () {
    if (process.env.ALLBOT_SCRIPT_RUN) {
        const raw = process.env.FXSH_COOKIE || process.env.wqwl_fxsh || process.env.fxsh || '';
        const accounts = raw.split(/\r?\n/).map(item => item.trim()).filter(Boolean);
        if (accounts.length === 0) {
            console.log('❌未检测到粉象账号环境变量 FXSH_COOKIE');
            return;
        }
        console.log(`🚀AllBot 粉象生活任务开始，共 ${accounts.length} 个账号`);
        let success = 0;
        for (const ck of accounts) {
            try {
                const task = new Task(ck);
                await task.main();
                success++;
            } catch (error) {
                console.log(`❌账号运行失败：${error.message}`);
            }
        }
        console.log(`✅AllBot 粉象生活任务结束，成功运行 ${success}/${accounts.length} 个账号`);
        return;
    }

})()
