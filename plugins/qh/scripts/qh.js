const axios = require('axios');
const crypto = require('crypto');
const CONFIG = {
    baseUrl: 'https://farmgames.ioutu.cn',
    publicKeyUrl: '/api/web/open/encrypt/public-key',
    timeout: 20000,
    accountDelay: 180000,
    energyDelay: 240000,
    taskCompleteDelay: 10000,
    loginDelay: 5000
};

const DEFAULT_PUBLIC_KEY = 'MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA70sK419vy3MabW3lEGlk7Zh1u78OdnVlioVazp5Y46eBh+/TDqo/wZ9VrQ/4MmAtoP0vJ2vmwP5gqO3WPojb07WddXfF1eU+5M+Rj3s0eSRrvZvBcGZ3qK0dOgZJScK66IDQazt/c4xqhDcsItIyNRahUqB/IKc6E80GZJvMvFtZVSCseAXC0mAJXhi1AdUOlP+3Pv0fiUVejTJp1j7LBNWJ7Z5/8mRcclQH0vmxsdYsaV3qZiJ2d/CfNoKcwmI2IWmeZy8NP5U8Hn0AsxPEwjdHoEqG/iy/SoA46TZL+RLtWqUSHXpaKR/VFN0rbl25SE91X8FTfLqyD8LfGMCwRQIDAQAB';

function maskWid(wid) {
    if (wid.length <= 8) return wid;
    return `${wid.substring(0, 4)}****${wid.substring(wid.length - 4)}`;
}
function maskStr(s) {
    if (!s || s.length <= 6) return s;
    return `${s.substring(0, 3)}****${s.substring(s.length - 3)}`;
}

let cachedPublicKey = null;
let cachedPublicKeyTime = 0;

async function getPublicKey() {
    if (cachedPublicKey && (Date.now() - cachedPublicKeyTime < 3600000)) {
        return cachedPublicKey;
    }
    try {
        const resp = await axios.get(`${CONFIG.baseUrl}${CONFIG.publicKeyUrl}`, {
            headers: { 'Content-Type': 'application/json' },
            timeout: CONFIG.timeout
        });
        if (resp.data && resp.data.code === 0 && resp.data.data && resp.data.data.publicKey) {
            cachedPublicKey = resp.data.data.publicKey;
            cachedPublicKeyTime = Date.now();
            return cachedPublicKey;
        }
    } catch (e) { /* fallback */ }
    return DEFAULT_PUBLIC_KEY;
}

function base64ToBytes(b64) {
    return new Uint8Array(Buffer.from(b64, 'base64'));
}

function bytesToBase64(bytes) {
    return Buffer.from(bytes).toString('base64');
}

function cleanPemKey(pem) {
    return pem
        .replace('-----BEGIN PUBLIC KEY-----', '')
        .replace('-----END PUBLIC KEY-----', '')
        .replace(/[\r\n\s]/g, '');
}

async function encryptBody(data, publicKeyPem) {
    const cleanKey = cleanPemKey(publicKeyPem);
    const keyBytes = base64ToBytes(cleanKey);

    const rsaKey = await crypto.subtle.importKey(
        'spki', keyBytes,
        { name: 'RSA-OAEP', hash: 'SHA-256' },
        false, ['encrypt']
    );

    const aesKey = await crypto.subtle.generateKey(
        { name: 'AES-GCM', length: 256 }, true, ['encrypt']
    );

    const iv = crypto.getRandomValues(new Uint8Array(12));
    const plaintext = new TextEncoder().encode(JSON.stringify(data));

    const ciphertext = await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv }, aesKey, plaintext
    );

    const rawAesKey = await crypto.subtle.exportKey('raw', aesKey);
    const encryptedKey = await crypto.subtle.encrypt(
        { name: 'RSA-OAEP' }, rsaKey, rawAesKey
    );

    return {
        data: bytesToBase64(new Uint8Array(ciphertext)),
        key: bytesToBase64(new Uint8Array(encryptedKey)),
        iv: bytesToBase64(iv)
    };
}

let encryptPublicKey = null;
async function ensureEncryptKey() {
    if (!encryptPublicKey) {
        encryptPublicKey = await getPublicKey();
    }
}

async function encryptedPost(url, bodyData, account, extraHeaders = {}) {
    await ensureEncryptKey();
    const encrypted = await encryptBody(bodyData, encryptPublicKey);
    const headers = getHeaders(account);
    Object.assign(headers, extraHeaders || {});
    headers['Content-Type'] = 'application/json';
    headers['X-Request-Encrypted'] = 'true';

    const response = await axios.post(url, encrypted, {
        headers: headers,
        timeout: CONFIG.timeout
    });
    return response.data;
}

function parseQhcsEnv() {
    const qhcsEnv = process.env.QHCS || '';
    if (!qhcsEnv) {
        console.log('info: 未检测到环境变量QHCS，请按配置说明设置！');
        console.log('info: 格式：备注+++User-Agent+++wid+++openId');
        console.log('info: 例如：宏+++Mozilla/5.0...+++10621338522+++oBk224rhkv3jiREBlckjTFTLr3est');
        return null;
    }

    const accountLines = qhcsEnv.replace(/\r\n/g, '\n').split('\n');
    const accounts = [];

    for (let idx = 0; idx < accountLines.length; idx++) {
        const line = accountLines[idx].trim();
        if (!line) {
            console.log(`info: 检测到第${idx + 1}个无效项（空内容），已跳过`);
            continue;
        }

        const parts = line.split('+++').map(item => item.trim());
        if (parts.length < 4) {
            console.log(`info: 第${idx + 1}个账号格式错误（需要备注+++ua+++wid+++openId），已跳过`);
            continue;
        }

        accounts.push({
            index: idx + 1,
            remark: parts[0],
            ua: parts[1],
            wid: parts[2],
            openId: parts[3],
            token: '',
            tomatoUserId: null,
            userData: {}
        });
    }

    if (accounts.length === 0) {
        console.log('info: 没有可用账号（所有项格式错误或为空），脚本终止');
        return null;
    }

    return accounts;
}

function getHeaders(account) {
    const headers = {
        'User-Agent': account.ua,
        'Origin': 'https://farmgames.ioutu.cn',
        'Referer': 'https://farmgames.ioutu.cn/',
        'Accept-Language': 'zh-CN,zh-Hans;q=0.9'
    };
    if (account.token) {
        headers['Authorization'] = account.token;
    }
    return headers;
}

async function loginAccount(account) {
    const loginUrl = `${CONFIG.baseUrl}/api/web/open/tomato/login?wid=${account.wid}&openId=${account.openId}`;

    try {
        console.log(`info: 账号${account.index}（备注：${account.remark}）：发起登录请求`);
        console.log(`info: 账号${account.index}：wid脱敏：${maskWid(account.wid)}，openId脱敏：${maskStr(account.openId)}`);

        await ensureEncryptKey();
        const encrypted = await encryptBody({
            wid: account.wid,
            openId: account.openId
        }, encryptPublicKey);

        const response = await axios.post(loginUrl, encrypted, {
            headers: {
                'User-Agent': account.ua,
                'Content-Type': 'application/json',
                'X-Request-Encrypted': 'true',
                'Origin': 'https://farmgames.ioutu.cn',
                'Referer': 'https://farmgames.ioutu.cn/',
                'Accept-Language': 'zh-CN,zh-Hans;q=0.9'
            },
            timeout: CONFIG.timeout
        });

        const res = response.data;
        if (res.code !== 200) {
            console.log(`info: 账号${account.index}（备注：${account.remark}）：登录失败，原因：${res.msg || '未知错误'}`);
            return false;
        }

        account.token = res.data.token;
        account.tomatoUserId = res.data.tomatoUserId;
        account.userData = res.data;

        console.log(`success: 账号${account.index}（备注：${account.remark}）：登录成功！`);
        console.log(`info: 账号${account.index}：昵称：${account.userData.nickName}，能量：${account.userData.energyBalance}，番茄：${account.userData.tomatoBalance}`);
        console.log(`info: 账号${account.index}：阶段：${account.userData.stageName || '未知'}`);

        console.log(`success: 账号${account.index}（备注：${account.remark}）登录成功`);

        await new Promise(resolve => setTimeout(resolve, CONFIG.loginDelay));
        return true;

    } catch (error) {
        const errMsg = error.response
            ? `[${error.response.status}] ${JSON.stringify(error.response.data)}`
            : error.message;
        console.log(`info: 账号${account.index}（备注：${account.remark}）：登录异常，原因：${errMsg}`);
        return false;
    }
}

async function getHome(account) {
    const url = `${CONFIG.baseUrl}/api/web/member/tomato/home`;
    try {
        const response = await axios.get(url, {
            headers: getHeaders(account),
            timeout: CONFIG.timeout
        });
        if (response.data.code === 200 && response.data.data) {
            account.userData = response.data.data;
            return account.userData;
        }
    } catch (e) { /* ignore */ }
    return account.userData;
}

async function getTasks(account) {
    const url = `${CONFIG.baseUrl}/api/web/member/tomato/tasks`;
    try {
        const response = await axios.get(url, {
            headers: getHeaders(account),
            timeout: CONFIG.timeout
        });

        const res = response.data;
        if (res.code !== 200) {
            console.log(`info: 账号${account.index}（备注：${account.remark}）：获取任务列表失败，原因：${res.msg || '未知错误'}`);
            return [];
        }

        const tasks = res.data || [];
        console.log('info: ' + '='.repeat(40));
        console.log(`info: 账号${account.index}（备注：${account.remark}） - 所有任务状态：`);
        const unfinished = [];
        for (const task of tasks) {
            const done = task.rewardClaimed === '1';
            const statusText = done ? '已领取' : '未完成';
            console.log(`info: 任务${task.taskId}：${task.taskName}（${task.taskCode}）| 奖励：${task.rewardEnergy}能量 | ${statusText}`);
            if (!done) {
                unfinished.push(task);
            }
        }
        console.log('info: ' + '='.repeat(40));
        return unfinished;

    } catch (error) {
        const errMsg = error.response ? `${error.response.status} - ${JSON.stringify(error.response.data)}` : error.message;
        console.log(`info: 账号${account.index}（备注：${account.remark}）：获取任务异常，原因：${errMsg}`);
        return [];
    }
}

async function completeTask(account, task) {
    const typeMap = { 'SIGN': 'SIGN', 'BROWSE': 'BROWSE', 'SHARE': 'SHARE', 'FRIEND_STEAL_ENERGY': 'FRIEND_STEAL_ENERGY' };
    const reqType = typeMap[task.taskType] || task.taskType;
    if (!reqType) {
        console.log(`info: 账号${account.index}：未知任务类型 ${task.taskType}，跳过`);
        return false;
    }

    try {
        console.log(`info: 账号${account.index}（备注：${account.remark}）：开始执行【${task.taskName}】任务（类型：${reqType}，编码：${task.taskCode}）`);

        const res = await encryptedPost(
            `${CONFIG.baseUrl}/api/web/member/tomato/tasks/complete`,
            { taskType: reqType, taskCode: task.taskCode },
            account
        );

        if (res.code !== 200) {
            console.log(`info: 账号${account.index}（备注：${account.remark}）：【${task.taskName}】失败，原因：${res.msg || '未知错误'}`);
            return false;
        }

        console.log(`success: 账号${account.index}（备注：${account.remark}）：【${task.taskName}】任务完成！`);
        return true;

    } catch (error) {
        const errMsg = error.response ? `${error.response.status} - ${JSON.stringify(error.response.data)}` : error.message;
        console.log(`info: 账号${account.index}（备注：${account.remark}）：【${task.taskName}】异常，原因：${errMsg}`);
        return false;
    }
}

async function handleShareTask(account, task) {
    try {
        console.log(`info: 账号${account.index}（备注：${account.remark}）：开始处理【分享好友】任务`);

        const res = await encryptedPost(
            `${CONFIG.baseUrl}/api/web/member/tomato/miniprogram/qrcode/create`,
            {},
            account
        );

        if (res.code === 200 && res.data && res.data.qrcodeUrl) {
            console.log(`info: 账号${account.index}：分享二维码已生成`);
        }

        await doPageVisit(account, '/packages/wm-cloud-qiehuang/share/index');

        await new Promise(resolve => setTimeout(resolve, 3000));

        return await completeTask(account, task);

    } catch (error) {
        const errMsg = error.response ? `${error.response.status} - ${JSON.stringify(error.response.data)}` : error.message;
        console.log(`info: 账号${account.index}（备注：${account.remark}）：分享任务异常，原因：${errMsg}`);
        return false;
    }
}

async function handleFriendEnergyTask(account, task) {
    const friendCount = task.availableFriendCount || 0;
    console.log(`info: 账号${account.index}（备注：${account.remark}）：好友能量任务，可收取好友数：${friendCount}`);

    if (friendCount > 0) {
        await doPageVisit(account, '/packages/wm-cloud-qiehuang/friend-energy/index');
        await new Promise(resolve => setTimeout(resolve, 3000));
        return await completeTask(account, task);
    }

    console.log(`info: 账号${account.index}（备注：${account.remark}）：暂无好友可收取能量，跳过`);
    return false;
}

async function doPageVisit(account, path) {
    try {
        console.log(`info: 账号${account.index}：上报页面访问：${path}`);

        const res = await encryptedPost(
            `${CONFIG.baseUrl}/api/web/member/tomato/page-visit`,
            { page: path },
            account
        );

        return res.code === 200;
    } catch (e) { return false; }
}

async function useEnergy(account, retryCount = 0) {
    const maxRetry = 3;
    try {
        console.log('info: ' + '='.repeat(40));
        console.log(`info: 账号${account.index}（备注：${account.remark}）：开始执行能量使用操作`);

        const res = await encryptedPost(
            `${CONFIG.baseUrl}/api/web/member/tomato/energy/use`,
            {},
            account
        );

        if (res.code !== 200) {
            if (res.msg && res.msg.includes('能量值不足')) {
                console.log(`info: 账号${account.index}（备注：${account.remark}）：能量使用失败，原因：${res.msg}`);
                return false;
            }
            console.log(`info: 账号${account.index}（备注：${account.remark}）：能量使用失败，原因：${res.msg || '未知错误'}`);
            return false;
        }

        await getHome(account);
        const energy = account.userData.energyBalance;
        const tomato = account.userData.tomatoBalance;
        console.log(`success: 账号${account.index}（备注：${account.remark}）：能量使用成功！`);
        console.log(`info: 账号${account.index}：剩余资源：能量${energy}，番茄${tomato}`);
        console.log('info: ' + '='.repeat(40));

        return true;

    } catch (error) {
        const errMsg = error.response ? `${error.response.status} - ${JSON.stringify(error.response.data)}` : error.message;
        if (error.response && error.response.status === 429 && retryCount < maxRetry) {
            const retryDelay = (retryCount + 1) * 60000;
            console.log(`info: 账号${account.index}（备注：${account.remark}）：能量使用触发限流，${retryDelay / 1000}秒后重试（第${retryCount + 1}/${maxRetry}次）`);
            await new Promise(resolve => setTimeout(resolve, retryDelay));
            return useEnergy(account, retryCount + 1);
        }
        console.log(`info: 账号${account.index}（备注：${account.remark}）：能量使用异常，原因：${errMsg}`);
        return false;
    }
}

function getUserStatus(account) {
    if (!account.userData) {
        console.log(`info: 账号${account.index}（备注：${account.remark}）：未获取到用户数据，返回默认资源值0`);
        return { energy: 0, tomato: 0 };
    }
    return {
        energy: account.userData.energyBalance || 0,
        tomato: account.userData.tomatoBalance || 0
    };
}

async function autoMultiAccount() {
    console.log('info: 【茄皇五期多账号自动化脚本】已启动');
    const accounts = parseQhcsEnv();
    if (!accounts) return;

    for (let i = 0; i < accounts.length; i++) {
        const account = accounts[i];
        const totalAccounts = accounts.length;

        console.log(`\ninfo: ` + '='.repeat(50));
        console.log(`info: 正在处理账号 ${account.index}/${totalAccounts}（备注：${account.remark}）`);
        console.log('info: ' + '='.repeat(50));

        const loginSuccess = await loginAccount(account);
        if (!loginSuccess) {
            console.log(`info: 账号${account.index}（备注：${account.remark}）：登录失败，跳过后续所有操作`);
            continue;
        }

        await getHome(account);

        const unfinishedTasks = await getTasks(account);
        if (unfinishedTasks.length > 0) {
            for (const task of unfinishedTasks) {
                if (task.taskCode === 'SIGN_IN') {
                    await doPageVisit(account, '/pages/tomato/index');
                    await completeTask(account, task);
                } else if (task.taskType === 'BROWSE') {
                    if (task.browseTarget) {
                        await doPageVisit(account, task.browseTarget);
                        await new Promise(resolve => setTimeout(resolve, 3000));
                    }
                    await completeTask(account, task);
                } else if (task.taskType === 'SHARE') {
                    await handleShareTask(account, task);
                } else if (task.taskType === 'FRIEND_STEAL_ENERGY') {
                    await handleFriendEnergyTask(account, task);
                } else {
                    console.log(`info: 账号${account.index}（备注：${account.remark}）：发现未知任务类型（${task.taskType}），尝试完成任务`);
                    await completeTask(account, task);
                }
            }

            await new Promise(resolve => setTimeout(resolve, CONFIG.taskCompleteDelay));

            const status = getUserStatus(account);
            console.log(`\ninfo: 账号${account.index}（备注：${account.remark}）：所有可处理任务已完成`);
            console.log(`info: 账号${account.index}：任务后资源：能量${status.energy}，番茄${status.tomato}`);
            console.log(`success: 账号${account.index}（备注：${account.remark}）所有任务处理完成`);
        } else {
            console.log(`\ninfo: 账号${account.index}（备注：${account.remark}）：无未完成任务或无法获取任务列表`);
            const status = getUserStatus(account);
            console.log(`info: 账号${account.index}：当前资源：能量${status.energy}，番茄${status.tomato}`);
        }

        console.log(`\ninfo: 账号${account.index}（备注：${account.remark}）：进入循环使用能量逻辑（触发条件：能量≥20）`);
        while (true) {
            await getHome(account);
            const status = getUserStatus(account);
            if (status.energy >= 20) {
                console.log(`\ninfo: 账号${account.index}（备注：${account.remark}）：能量满足（能量${status.energy}），执行能量使用`);
                const success = await useEnergy(account);
                if (!success) {
                    console.log(`info: 账号${account.index}（备注：${account.remark}）：能量使用失败，退出循环`);
                    break;
                }
                await new Promise(resolve => setTimeout(resolve, CONFIG.energyDelay));
            } else {
                console.log(`\ninfo: 账号${account.index}（备注：${account.remark}）：能量不足（能量${status.energy}），停止能量使用`);
                break;
            }
        }

        console.log(`\ninfo: 账号${account.index}/${totalAccounts}（备注：${account.remark}）：所有操作处理完毕`);
        if (i < totalAccounts - 1) {
            console.log(`info: 账号间延迟${CONFIG.accountDelay / 1000}秒，准备处理下一个账号...\n`);
            await new Promise(resolve => setTimeout(resolve, CONFIG.accountDelay));
        }
    }

    console.log(`\ninfo: ` + '='.repeat(50));
    console.log('info: 所有账号已全部处理完成！脚本执行结束');
    console.log('info: ' + '='.repeat(50));
}

autoMultiAccount().catch(err => {
    console.log(`info: 脚本执行异常：${err.message}`);
    process.exit(1);
});
