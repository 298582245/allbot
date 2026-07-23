const crypto = require('crypto');
const { createAccountQLPlugin, builtinPointsAuth } = require('../../sdk/nodejs/account_ql_plugin');

const ENV_NAME = 'QHCS';
const CONFIG = {
  baseUrl: 'https://farmgames.ioutu.cn',
  publicKeyUrl: '/api/web/open/encrypt/public-key',
  timeout: 20000
};
const DEFAULT_PUBLIC_KEY = 'MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA70sK419vy3MabW3lEGlk7Zh1u78OdnVlioVazp5Y46eBh+/TDqo/wZ9VrQ/4MmAtoP0vJ2vmwP5gqO3WPojb07WddXfF1eU+5M+Rj3s0eSRrvZvBcGZ3qK0dOgZJScK66IDQazt/c4xqhDcsItIyNRahUqB/IKc6E80GZJvMvFtZVSCseAXC0mAJXhi1AdUOlP+3Pv0fiUVejTJp1j7LBNWJ7Z5/8mRcclQH0vmxsdYsaV3qZiJ2d/CfNoKcwmI2IWmeZy8NP5U8Hn0AsxPEwjdHoEqG/iy/SoA46TZL+RLtWqUSHXpaKR/VFN0rbl25SE91X8FTfLqyD8LfGMCwRQIDAQAB';

let cachedPublicKey = null;
let cachedPublicKeyTime = 0;

async function getPublicKey() {
  if (cachedPublicKey && (Date.now() - cachedPublicKeyTime < 3600000)) return cachedPublicKey;
  try {
    const resp = await fetch(`${CONFIG.baseUrl}${CONFIG.publicKeyUrl}`, {
      headers: { 'Content-Type': 'application/json' },
      signal: AbortSignal.timeout(CONFIG.timeout)
    });
    const data = await resp.json();
    if (data && data.code === 0 && data.data && data.data.publicKey) {
      cachedPublicKey = data.data.publicKey;
      cachedPublicKeyTime = Date.now();
      return cachedPublicKey;
    }
  } catch (e) { /* fallback */ }
  return DEFAULT_PUBLIC_KEY;
}

function cleanPem(pem) {
  return pem.replace(/-----[A-Z ]+-----/g, '').replace(/[\r\n\s]/g, '');
}

function encryptRSAOAEP(data, publicKeyPem) {
  const key = crypto.createPublicKey({ key: publicKeyPem, format: 'pem', type: 'spki' });
  return crypto.publicEncrypt({ key, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' }, data);
}

function encryptAESGCM(plaintext, key, iv) {
  const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
  const encrypted = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const tag = cipher.getAuthTag();
  return Buffer.concat([encrypted, tag]);
}

async function encryptBody(data, publicKeyPem) {
  const aesKey = crypto.randomBytes(32);
  const iv = crypto.randomBytes(12);
  const plaintext = Buffer.from(JSON.stringify(data), 'utf8');
  const ciphertext = encryptAESGCM(plaintext, aesKey, iv);
  const encryptedKey = encryptRSAOAEP(aesKey, publicKeyPem);
  return {
    data: ciphertext.toString('base64'),
    key: encryptedKey.toString('base64'),
    iv: iv.toString('base64')
  };
}

function parseAccount(raw) {
  const parts = raw.split('+++').map(s => s.trim());
  if (parts.length < 4) throw new Error('格式错误');
  return { remark: parts[0], ua: parts[1], wid: parts[2], openId: parts[3] };
}

async function validateAccount(raw) {
  let account;
  try {
    account = parseAccount(raw);
  } catch (e) {
    return { valid: false, message: '格式错误，请使用：备注+++User-Agent+++wid+++openId' };
  }
  try {
    const publicKey = await getPublicKey();
    const encrypted = await encryptBody({ wid: account.wid, openId: account.openId }, publicKey);
    const resp = await fetch(`${CONFIG.baseUrl}/api/web/open/tomato/login?wid=${account.wid}&openId=${account.openId}`, {
      method: 'POST',
      headers: {
        'User-Agent': account.ua,
        'Content-Type': 'application/json',
        'X-Request-Encrypted': 'true',
        'Origin': 'https://farmgames.ioutu.cn',
        'Referer': 'https://farmgames.ioutu.cn/',
        'Accept-Language': 'zh-CN,zh-Hans;q=0.9'
      },
      body: JSON.stringify(encrypted),
      signal: AbortSignal.timeout(CONFIG.timeout)
    });
    const res = await resp.json();
    if (res.code !== 200) return { valid: false, message: res.msg || '登录失败' };
    return {
      valid: true,
      message: `昵称：${res.data.nickName}，能量：${res.data.energyBalance}，番茄：${res.data.tomatoBalance}`,
      data: res.data
    };
  } catch (e) {
    const errMsg = e.cause ? e.cause.message : (e.message || '网络错误');
    return { valid: false, message: errMsg };
  }
}

async function queryAccount(account) {
  try {
    const parts = String(account.env_value || '').split('+++');
    if (parts.length < 4) return `账号未知：格式错误`;
    const acct = { ua: parts[1].trim(), wid: parts[2].trim(), openId: parts[3].trim() };
    const publicKey = await getPublicKey();
    const encrypted = await encryptBody({ wid: acct.wid, openId: acct.openId }, publicKey);
    const resp = await fetch(`${CONFIG.baseUrl}/api/web/open/tomato/login?wid=${acct.wid}&openId=${acct.openId}`, {
      method: 'POST',
      headers: {
        'User-Agent': acct.ua,
        'Content-Type': 'application/json',
        'X-Request-Encrypted': 'true',
        'Origin': 'https://farmgames.ioutu.cn',
        'Referer': 'https://farmgames.ioutu.cn/',
        'Accept-Language': 'zh-CN,zh-Hans;q=0.9'
      },
      body: JSON.stringify(encrypted),
      signal: AbortSignal.timeout(CONFIG.timeout)
    });
    const res = await resp.json();
    if (res.code !== 200) return `登录失败：${res.msg || '未知错误'}`;
    return `昵称：${res.data.nickName}，能量：${res.data.energyBalance}，番茄：${res.data.tomatoBalance}`;
  } catch (e) {
    const errMsg = e.cause ? e.cause.message : (e.message || '网络错误');
    return `查询失败：${errMsg}`;
  }
}

function parseInput(raw, ctx) {
  const parts = raw.split('+++').map(s => s.trim());
  if (parts.length < 4) throw new Error('格式错误，请使用：备注+++User-Agent+++wid+++openId');
  const remark = parts[0];
  const ua = parts[1];
  const wid = parts[2];
  const openId = parts[3];
  if (!remark || /^(备注|remark|test|测试)$/i.test(remark)) throw new Error('备注无效，请填写有意义的名称');
  if (!ua || ua.length < 20 || /^(User-Agent|useragent|ua)$/i.test(ua)) throw new Error('User-Agent 无效，请填写真实的浏览器 UA');
  if (!wid || wid.length < 6 || /^(wid|id|test|123)$/i.test(wid)) throw new Error('wid 无效，请填写真实的数字 wid');
  if (!openId || openId.length < 10 || /^(openId|openid|test)$/i.test(openId)) throw new Error('openId 无效，请填写真实的 openId');
  return {
    uniqueKey: wid,
    envValue: raw,
    displayName: remark + '(' + wid.slice(0, 4) + '****' + wid.slice(-4) + ')',
    remark: remark
  };
}

async function query(account, ctx, index) {
  const parts = String(account.env_value || '').split('+++');
  const remark = parts[0] || '未知';
  const wid = parts[2] || '';
  const widMask = wid.length > 6 ? (wid.slice(0, 4) + '****' + wid.slice(-4)) : wid;
  const info = await queryAccount(account);
  return `账号${index + 1}：${remark}（${widMask}）\n当前资源：${info}`;
}

async function checkCk(account, ctx) {
  const result = await validateAccount(String(account.env_value || ''));
  return { valid: result.valid, reason: result.message };
}

async function afterRun(ctx, accounts, result, helpers) {
  if (result.status === 'success') {
    await ctx.reply(`茄皇脚本执行完成，共处理 ${accounts.length} 个账号`);
  } else {
    await ctx.reply(`茄皇脚本执行异常：${result.error || '未知错误'}`);
  }
}

async function login(ctx, helpers) {
  await ctx.reply('请发送账号信息，格式：备注+++User-Agent+++wid+++openId\n回复 q 退出：');
  const raw = String(await ctx.listen(120)).trim();
  if (!raw || raw.toLowerCase() === 'q') return ctx.reply('已取消登录');
  try {
    parseInput(raw, ctx);
  } catch (e) {
    return ctx.reply(e.message);
  }
  await ctx.reply('正在验证账号有效性...');
  const result = await validateAccount(raw);
  if (!result.valid) {
    return ctx.reply(`账号无效：${result.message}`);
  }
  const input = parseInput(raw, ctx);
  input.metadata = { nickName: result.data.nickName, energy: result.data.energyBalance, tomato: result.data.tomatoBalance };
  input.displayName = input.remark + '（' + (result.data.nickName || '未知') + '）';
  const saved = await helpers.saveAccount(input);
  return ctx.reply(`${saved.existing ? '覆盖更新' : '添加'}成功：${saved.account.account_name}\n验证通过：${result.message}\n发送【${helpers.prefix}授权】授权后即可运行。`);
}

createAccountQLPlugin({
  prefix: '茄皇',
  tableName: 'qh_accounts',
  envName: ENV_NAME,
  login,
  loginPrompt: '请发送账号信息，格式：备注+++User-Agent+++wid+++openId\n回复 q 退出：',
  account: {
    parseInput,
    query,
    checkCk
  },
  auth: { provider: builtinPointsAuth({ priceConfig: 'auth_price_per_month' }) },
  ql: {
    runtime: 'nodejs',
    runtimeConfig: 'script_runtime',
    script: 'scripts/task.js',
    scriptConfig: 'task_script',
    timeoutConfig: 'run_wait_timeout',
    waitScheduled: true,
    afterRun,
    env: (ctx, accounts) => ({ [ENV_NAME]: accounts.map((item) => item.env_value).join('\n') })
  },
  schedules: {
    run: { taskKey: 'qh-default-run', name: '茄皇自动运行', cronConfig: 'cron', cron: '0 6 * * *', content: '茄皇一键运行' },
    ckCheck: { taskKey: 'qh-ck-check', name: '茄皇 CK 检测', cronConfig: 'ck_check_cron', cron: '25 9 * * *', content: '茄皇CK检测' }
  }
});