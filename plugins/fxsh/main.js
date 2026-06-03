const crypto = require('crypto');
const axios = require('axios');
const { createAccountQLPlugin, builtinPointsAuth } = require('../../sdk/nodejs/account_ql_plugin');

const ENV_NAME = 'FXSH_COOKIE';

class FxshApi {
  constructor(ck, index = 1) {
    const parts = String(ck || '').split('#');
    this.index = index;
    this.did = parts[0] || '';
    this.finger = parts[1] || '';
    this.token = parts[2] || '';
    this.oaid = parts[3] || '';
    this.name = parts[4] || `账号${index}`;
  }

  md5(value) {
    return crypto.createHash('md5').update(String(value)).digest('hex');
  }

  async request(method, url, body = '') {
    const queryString = (obj = {}) => Object.keys(obj).sort().filter((key) => obj[key] !== null && typeof obj[key] !== 'object').map((key) => `${key}=${obj[key]}`).join('&');
    const headersBase = {
      traceid: this.md5(Date.now().toString() + Math.random().toString()),
      noncestr: Math.random().toString().slice(2, 10),
      timestamp: Date.now(),
      platform: 'android',
      did: this.did,
      version: '6.7.1',
      finger: this.finger,
      token: this.token,
      oaid: this.oaid
    };
    const payload = method === 'get' ? {} : JSON.parse(body || '{}');
    headersBase.sign = this.md5('粉象好牛逼nb3b16f5a02479a0e34df78d14aefe76' + queryString(payload) + queryString(headersBase));
    const headers = {
      'User-Agent': 'Mozilla/5.0 (Linux; Android 10; MI 8 Lite Build/QKQ1.190910.002; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/80.0.3987.99 Mobile Safari/537.36 AgentWeb/5.0.0 UCBrowser/11.6.4.950',
      Accept: 'application/json, text/plain, */*',
      'Content-Type': 'application/json',
      origin: 'https://m.fenxianglife.com',
      'x-requested-with': 'com.n_add.android',
      referer: 'https://m.fenxianglife.com',
      ...headersBase
    };
    try {
      const response = await axios({ url, method, headers, data: body });
      return response.data;
    } catch (error) {
      return { code: -1, message: '接口请求失败', error: error.message };
    }
  }

  userInfo() { return this.request('get', 'https://api.fenxianglife.com/njia/users/info'); }
  smsCode(phone) { return this.request('post', 'https://api.fenxianglife.com/njia/util/sms/code', JSON.stringify({ validateType: 1, mobileArea: '86', mobile: phone, type: 1 })); }
  login(phone, code) { return this.request('post', 'https://api.fenxianglife.com/njia/login/mobile', JSON.stringify({ mobileArea: '86', smsCode: this.md5(code), mobile: phone })); }
  tokenString(phone) { return `${this.did}#${this.finger}#${this.token}#${this.oaid}#${phone.slice(0, 3)}****${phone.slice(-4)}`; }

  async query() {
    const [user, withdraw, lottery, currentMoney] = await Promise.all([
      this.userInfo(),
      this.request('get', 'https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/withdraw/index'),
      this.request('post', 'https://fenxiang-lottery-api.fenxianglife.com/fenxiang-lottery/home/data/V2', JSON.stringify({ plateform: 'android', version: '6.7.1' })),
      this.request('post', 'https://api.fenxianglife.com/njia/order/withdraw/v4/create', JSON.stringify({ orderType: 5 }))
    ]);
    const lines = [`\n🤪 用户ID(备注)：${this.name}`];
    if (user.code === 200) lines.push(`👤 昵称：${user.data?.userInfo?.nickname || '-'}`);
    else lines.push(`👤 用户信息：查询失败，${user.message || '未知原因'}`);
    if (withdraw.code === 200) lines.push(`🧧活动奖励余额：${(currentMoney.data?.maxWithdrawAmount || 0) / 100}元(最低0.1起提)`);
    else lines.push(`🧧活动奖励余额：查询失败，${withdraw.message || '未知原因'}`);
    if (withdraw.code === 200) {
      lines.push(`🎰本期开奖期数：${withdraw?.data?.dateStr || ''}期`);
      lines.push(`🎉本期开奖金额：${withdraw?.data?.totalRewardAmount / 100}元 (${lottery.data?.amountReceiveStatus == 1 ? '未领取' : '已领取'})`);
      lines.push(withdraw?.data?.freeOrderCount > 0 && withdraw?.data?.freeItem?.itemTitle ? `🎁本期免单商品：【${withdraw.data.freeItem.itemTitle}】` : '🎁本期免单商品：无');
    }
    if (lottery.code === 200) lines.push(`🏆现有奖码个数：${lottery.data?.openLotteryModule?.now?.rewardCodes?.length || 0}个`);
    else lines.push(`🏆现有奖码个数：查询失败，${lottery.message || '未知原因'}`);
    return lines.join('\n');
  }

  async withdraw() {
    const create = await this.request('post', 'https://api.fenxianglife.com/njia/order/withdraw/v4/create', JSON.stringify({ orderType: 5 }));
    if (create?.code !== 200) return `❌${this.name}提现创建失败：${create?.message || '未知原因'}`;
    const amount = create?.data?.maxWithdrawAmount || 0;
    if (amount < 10) return `❌${this.name}当前可提现余额：${amount / 100}元，不足0.1元`;
    const submit = await this.request('post', 'https://api.fenxianglife.com/njia/order/withdraw/submit', JSON.stringify({ orderType: 5, withdrawAmount: amount }));
    if (submit?.code !== 200) return `❌${this.name}提现提交失败：${submit?.message || '未知原因'}`;
    const subTitle = String(submit?.data?.subTitle || '').replace(/\n/g, ',');
    return `🎉${this.name}提现提交成功，预计到账${amount / 100}元${subTitle ? `，${subTitle}` : ''}`;
  }
}

async function smsLogin(ctx, helpers) {
  await ctx.reply('请输入手机号，回复 q 退出：');
  const phone = String(await ctx.listen(60)).trim();
  if (!phone || phone.toLowerCase() === 'q') return ctx.reply('已退出登录');
  const did = randomDid();
  const api = new FxshApi(`${did}#${randomFinger()}##${randomOaid(did)}#${phone.slice(0, 3)}****${phone.slice(-4)}`);
  const sms = await api.smsCode(phone);
  if (sms?.code !== 200) return ctx.reply(`❌验证码发送失败：${sms?.message || '未知原因'}`);
  await ctx.reply('验证码已发送，请输入短信验证码：');
  const code = String(await ctx.listen(120)).trim();
  if (!code) return ctx.reply('已取消登录');
  const loginResult = await api.login(phone, code);
  if (loginResult?.code !== 200 || !loginResult?.data?.token) return ctx.reply(`❌登录失败：${loginResult?.message || '请检查验证码是否正确'}`);
  api.token = loginResult.data.token;
  const input = await buildFxshInput(api.tokenString(phone));
  const saved = await helpers.saveAccount(input);
  return ctx.reply(`✅${saved.existing ? '覆盖更新' : '添加'}成功：${saved.account.account_name}\n${saved.existingExpiresAt ? `已保留授权到期时间：${formatTime(saved.existingExpiresAt)}\n` : ''}发送【粉象授权】授权后即可运行。`);
}

async function login(ctx, helpers) {
  await ctx.reply('请选择登录方式：\n1. 短信登录\n2. CK登录（格式 did#finger#token#oaid#备注）\n回复 q 退出');
  const mode = String(await ctx.listen(60)).trim();
  if (mode.toLowerCase() === 'q') return ctx.reply('已退出');
  if (mode === '1') return smsLogin(ctx, helpers);
  await ctx.reply('请发送 CK：did#finger#token#oaid#备注');
  const ck = String(await ctx.listen(120)).trim();
  if (!ck || ck.toLowerCase() === 'q') return ctx.reply('已取消登录');
  const input = await buildFxshInput(ck);
  const saved = await helpers.saveAccount(input);
  return ctx.reply(`✅${saved.existing ? '覆盖更新' : '添加'}成功：${saved.account.account_name}\n${saved.existingExpiresAt ? `已保留授权到期时间：${formatTime(saved.existingExpiresAt)}\n` : ''}发送【粉象授权】授权后即可运行。`);
}

async function buildFxshInput(ck) {
  const parts = String(ck || '').split('#');
  if (parts.length < 4) throw new Error('CK格式错误，应为 did#finger#token#oaid#备注');
  const api = new FxshApi(ck);
  const info = await api.userInfo();
  if (info?.code !== 200) throw new Error(`账号校验失败：${info?.message || 'CK无效'}`);
  const displayName = info.data?.userInfo?.nickname || parts[4] || `粉象账号-${String(parts[0] || '').slice(0, 8)}`;
  const userKey = info.data?.userInfo?.id || info.data?.userInfo?.uid || parts[0] || displayName;
  return {
    envValue: ck,
    uniqueKey: `fxsh:${userKey}`,
    displayName,
    remark: parts[4] || displayName,
    metadata: { user_info: info, login_did: parts[0], fxsh_user_key: `fxsh:${userKey}` }
  };
}

async function withdraw(ctx, helpers) {
  const accounts = await helpers.listMine({ status: 'active' });
  if (!accounts.length) return ctx.reply('暂无账号，请发送【粉象登录】添加。');
  const results = [];
  for (let index = 0; index < accounts.length; index++) {
    const account = accounts[index];
    if (!helpers.isAuthorized(account)) results.push(`❌${helpers.accountName(account)}未授权或已过期，请先授权。`);
    else results.push(await new FxshApi(account.env_value, index + 1).withdraw());
  }
  return ctx.reply(results.join('\n'));
}

function randomDid() { return 'njia' + 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => { const random = Math.random() * 16 | 0; const value = char === 'x' ? random : (random & 0x3 | 0x8); return value.toString(16); }).slice(4); }
function randomOaid(did) { return crypto.createHash('md5').update(did).digest('hex').slice(8, 24); }
function randomFinger() { return crypto.createHash('md5').update(Date.now() + '').digest('hex'); }
function formatTime(value) { const date = new Date(value); return Number.isFinite(date.getTime()) ? date.toLocaleString('zh-CN', { hour12: false }) : String(value || '无'); }

createAccountQLPlugin({
  prefix: '粉象',
  tableName: 'fxsh_accounts',
  envName: ENV_NAME,
  login,
  routes: { 提现: withdraw },
  account: {
    async query(account, ctx, index) { return new FxshApi(account.env_value, index + 1).query(); },
    async checkCk(account) {
      const info = await new FxshApi(account.env_value).userInfo();
      if (info?.code === 200) return { valid: true };
      return { valid: false, reason: info?.message || 'CK 已失效' };
    }
  },
  auth: { provider: builtinPointsAuth({ priceConfig: 'auth_price_per_month' }) },
  ql: {
    runtime: 'nodejs',
    script: 'scripts/fxsh_task.js',
    scriptConfig: 'task_script',
    timeoutConfig: 'run_wait_timeout',
    env: (ctx, accounts) => ({ FXSH_COOKIE: accounts.map((item) => item.env_value).join('\n') })
  },
  schedules: {
    run: { taskKey: 'fxsh-default-run', name: '粉象生活自动运行', cronConfig: 'cron', cron: '38 0,22 * * *', content: '粉象一键运行' },
    expireCheck: { taskKey: 'fxsh-expiration-check', name: '粉象生活过期检测', cronConfig: 'expire_check_cron', cron: '15 9 * * *', content: '粉象过期检测' },
    ckCheck: { taskKey: 'fxsh-ck-check', name: '粉象生活 CK 检测', cronConfig: 'ck_check_cron', cron: '25 9 * * *', content: '粉象CK检测' }
  }
});
