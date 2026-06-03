const axios = require('axios');
const { createAccountQLPlugin, builtinPointsAuth } = require('../../sdk/nodejs/account_ql_plugin');

const ENV_NAME = 'wqwl_xyyx';

function parseCredential(raw) {
  const value = String(raw || '').trim();
  if (!value || value.toLowerCase() === 'q') return null;
  if (/\s/.test(value)) throw new Error('账号内容不能包含空格或换行，请单个账号分次添加。');
  const parts = value.split('#');
  const token = String(parts.shift() || '').trim();
  const remark = parts.join('#').trim();
  if (!token) throw new Error('3rdsession 不能为空。');
  const envValue = `${token}${remark ? `#${remark}` : ''}`;
  return {
    token,
    remark,
    envValue,
    uniqueKey: remark ? `remark:${remark}` : `token:${token}`,
    displayName: remark || `星韵账号-${token.slice(0, 8)}`,
    metadata: { token_prefix: token.slice(0, 8) }
  };
}

async function fetchXyyxApi(token, payload) {
  const response = await axios({
    url: 'https://gzpengru.weimbo.com/api/index.php?ackey=GZYTAPPLET',
    method: 'POST',
    headers: {
      Host: 'gzpengru.weimbo.com',
      Connection: 'keep-alive',
      '3rdsession': token,
      'content-type': 'application/json',
      'User-Agent': 'Mozilla/5.0 (Linux; Android 13; M2012K11AC Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/120.0.0.0 Mobile Safari/537.36 MicroMessenger/8.0.45.2400(0x28002B3D) WeChat/arm64 Weixin NetType/WIFI Language/zh_CN ABI/arm64',
      Referer: 'https://servicewechat.com/wxc86c9aecdb67f876/9/page-frame.html'
    },
    data: payload,
    timeout: 15000
  });
  return response.data;
}

function tokenOf(account) {
  return String(account?.env_value || '').split('#')[0] || '';
}

createAccountQLPlugin({
  prefix: '星韵',
  tableName: 'xyyx_accounts',
  envName: ENV_NAME,
  loginPrompt: '请发送星韵优选账号，格式：3rdsession#备注\n备注会作为稳定账号名，后续 CK 过期后用同一备注登录会覆盖更新并保留授权。回复 q 退出：',
  account: {
    parseInput: parseCredential,
    async query(account) {
      const token = tokenOf(account);
      if (!token) return { 状态: 'CK为空' };
      const response = await fetchXyyxApi(token, { action: 'userInfoData' });
      if (!response || !response.Status) return { 状态: `CK失效：${response?.Message || 'Token失效'}` };
      return { 积分: response.Data?.u_money?.jifen ?? 0 };
    },
    async checkCk(account) {
      const token = tokenOf(account);
      if (!token) return { valid: false, reason: 'CK 为空' };
      const response = await fetchXyyxApi(token, { action: 'userInfoData' });
      if (response && response.Status) return { valid: true };
      return { valid: false, reason: response?.Message || 'Token失效' };
    }
  },
  auth: { provider: builtinPointsAuth({ priceConfig: 'auth_price_per_month' }) },
  ql: {
    runtime: 'nodejs',
    script: 'scripts/wqwl_new_星韵优选.js',
    scriptConfig: 'task_script',
    timeoutConfig: 'run_wait_timeout',
    env: (ctx) => ({
      wqwl_isNotify: String(ctx.config('wqwl_isNotify', 'true')),
      wqwl_isDebug: String(ctx.config('wqwl_isDebug', '2')),
      wqwl_bfs: String(ctx.config('wqwl_bfs', '3')),
      wqwl_useProxy: String(ctx.config('wqwl_useProxy', 'false')),
      wqwl_daili: String(ctx.config('wqwl_daili', ''))
    })
  },
  schedules: {
    run: { taskKey: 'xyyx-default-run', name: '星韵优选自动运行', cronConfig: 'cron', cron: '8 8 * * *', content: '星韵一键运行' },
    ckCheck: { taskKey: 'xyyx-ck-check', name: '星韵优选 CK 检测', cronConfig: 'ck_check_cron', cron: '25 9 * * *', content: '星韵CK检测' }
  }
});
