const test = require('node:test');
const assert = require('node:assert/strict');
const { builtinPaymentAuth, AccountQLPlugin, normalizeSchedules } = require('./account_ql_plugin');

test('builtinPaymentAuth keeps point price config and payment options', () => {
  const provider = builtinPaymentAuth({
    priceConfig: 'auth_price_per_month',
    pointsPerRMBConfig: 'auth_payment_points_per_rmb',
    timeoutConfig: 'auth_payment_timeout',
    methodsConfig: 'auth_payment_methods',
    methods: ['alipay']
  });
  assert.equal(provider.type, 'builtin_payment');
  assert.equal(provider.priceConfig, 'auth_price_per_month');
  assert.equal(provider.pointsPerRMBConfig, 'auth_payment_points_per_rmb');
  assert.equal(provider.timeoutConfig, 'auth_payment_timeout');
  assert.equal(provider.methodsConfig, 'auth_payment_methods');
  assert.deepEqual(provider.methods, ['alipay']);
});

function makePlugin(ql = {}, account = {}) {
  return new AccountQLPlugin({
    prefix: '测试',
    tableName: 'test_accounts',
    envName: 'TEST_COOKIE',
    ql: { script: 'task.js', ...ql },
    account
  });
}

function makeCtx(options = {}) {
  const calls = { replies: [], runQLScripts: [], listens: [], scheduledTasks: [] };
  const listens = [...(options.listens || [])];
  return {
    content: '测试一键运行',
    unionId: 'union-1',
    platform: 'test',
    userId: 'user-1',
    meta: (key) => (key === 'fake' && options.fake ? 'true' : ''),
    config: (_key, defaultValue) => defaultValue,
    reply: async (text) => calls.replies.push(text),
    listen: async (timeout) => {
      calls.listens.push(timeout);
      if (!listens.length) throw new Error('unexpected listen');
      return listens.shift();
    },
    runQLScript: async (payload) => {
      calls.runQLScripts.push(payload);
      return options.result || { status: 'success', task_id: 'task-1' };
    },
    listPlatformAdmins: async () => [{ platform: 'test', adapter_id: 'adapter-1', user_id: 'admin-1' }],
    setScheduledTask: async (payload) => {
      calls.scheduledTasks.push(payload);
      return payload;
    },
    _request: async () => options.accounts || [],
    calls
  };
}

const accounts = [{ id: 1, account_name: '账号1', expires_at: '2999-01-01T00:00:00Z' }];

test('normalizeSchedules supports multiple run items', () => {
  const schedules = normalizeSchedules('云盘', {
    run: { taskKey: 'ydyp-exchange', content: '云盘一键抢兑' },
    runs: [{ taskKey: 'ydyp-default-run', cron: '13 8,15 * * *', content: '云盘一键运行' }]
  });
  assert.deepEqual(schedules.slice(0, 2).map((item) => item.taskKey), ['ydyp-exchange', 'ydyp-default-run']);
  assert.equal(schedules[0].content, '云盘一键抢兑');
  assert.equal(schedules[1].content, '云盘一键运行');
  assert.equal(schedules[1].cron, '13 8,15 * * *');
});

test('ensureSchedules uses task count as default maxCount', async () => {
  const plugin = new AccountQLPlugin({
    prefix: '测试',
    tableName: 'test_accounts',
    envName: 'TEST_COOKIE',
    ql: { script: 'task.js' },
    schedules: {
      run: { taskKey: 'task-1', content: '测试一键抢兑' },
      runs: [{ taskKey: 'task-2', content: '测试一键运行' }],
      expireCheck: { taskKey: 'task-3', content: '测试过期检测' },
      ckCheck: { taskKey: 'task-4', content: '测试CK检测' }
    }
  });
  const ctx = makeCtx();
  await plugin.ensureSchedules(ctx);

  assert.equal(ctx.calls.scheduledTasks.length, 4);
  assert.deepEqual(new Set(ctx.calls.scheduledTasks.map((item) => item.maxCount)), new Set([4]));
});

test('fake scheduled run submits script task by default and skips afterRun', async () => {
  let called = false;
  const plugin = makePlugin({ afterRun: async () => { called = true; } });
  const ctx = makeCtx({ fake: true, result: { status: 'running', task_id: 'task-1' } });
  await plugin.runAccounts(ctx, accounts, 'all_authorized', '测试一键运行');

  assert.equal(ctx.calls.runQLScripts[0].wait, false);
  assert.equal(called, false);
  assert.match(ctx.calls.replies.at(-1), /任务已提交/);
});

test('waitScheduled true makes scheduled run wait and calls afterRun with helpers context', async () => {
  let hookArgs;
  const plugin = makePlugin({
    waitScheduled: true,
    afterRun: async (ctx, hookAccounts, result, helpers) => {
      hookArgs = { ctx, hookAccounts, result, helpers };
    }
  });
  const ctx = makeCtx({ fake: true });
  await plugin.runAccounts(ctx, accounts, 'all_authorized', '测试一键运行');

  assert.equal(ctx.calls.runQLScripts[0].wait, true);
  assert.equal(hookArgs.ctx, ctx);
  assert.equal(hookArgs.hookAccounts, accounts);
  assert.equal(hookArgs.result.status, 'success');
  assert.equal(hookArgs.helpers.runMode, 'all_authorized');
  assert.equal(hookArgs.helpers.run_mode, 'all_authorized');
  assert.equal(hookArgs.helpers.title, '测试一键运行');
  assert.equal(hookArgs.helpers.isScheduled, true);
  assert.equal(hookArgs.helpers.is_scheduled, true);
});

test('waitScheduled false keeps scheduled run fire-and-forget and skips afterRun', async () => {
  let called = false;
  const plugin = makePlugin({ waitScheduled: false, afterRun: async () => { called = true; } });
  const ctx = makeCtx({ fake: true, result: { status: 'running', task_id: 'task-2' } });
  await plugin.runAccounts(ctx, accounts, 'all_authorized', '测试一键运行');

  assert.equal(ctx.calls.runQLScripts[0].wait, false);
  assert.equal(called, false);
  assert.match(ctx.calls.replies.at(-1), /任务已提交/);
});

test('after_run snake case alias is supported', async () => {
  let called = false;
  const plugin = makePlugin({ waitScheduled: true, after_run: async () => { called = true; } });
  const ctx = makeCtx({ fake: true });
  await plugin.runAccounts(ctx, accounts, 'all_authorized', '测试一键运行');
  assert.equal(called, true);
});

test('afterRun errors are swallowed and final reply still executes', async () => {
  const plugin = makePlugin({ waitScheduled: true, afterRun: async () => { throw new Error('hook failed'); } });
  const ctx = makeCtx({ fake: true });
  await plugin.runAccounts(ctx, accounts, 'all_authorized', '测试一键运行');
  assert.match(ctx.calls.replies.at(-1), /运行完成/);
});

test('all authorized run skips post run account query reply and shows summary', async () => {
  const plugin = makePlugin({}, { query: async () => '账号详情' });
  const ctx = makeCtx();
  await plugin.runAccounts(ctx, accounts, 'all_authorized', '测试一键运行');
  assert.equal(ctx.calls.replies.some((reply) => String(reply).includes('运行后账号信息')), false);
  assert.match(ctx.calls.replies.at(-1), /✅测试生活运行完成！共运行1个账号，耗时\d+\.\d{3}秒/);
});

test('current user run keeps post run account query reply', async () => {
  const plugin = makePlugin({}, { query: async () => '账号详情' });
  const ctx = makeCtx();
  await plugin.runAccounts(ctx, accounts, 'current_user', '测试运行');
  assert.match(ctx.calls.replies[1], /运行后账号信息/);
  assert.match(ctx.calls.replies.at(-1), /账号详情/);
});

test('afterRun is not called for non all_authorized run modes', async () => {
  let called = false;
  const plugin = makePlugin({ waitScheduled: true, afterRun: async () => { called = true; } });
  const ctx = makeCtx({ fake: true });
  await plugin.runAccounts(ctx, accounts, 'single_account', '测试账号运行');
  assert.equal(called, false);
});

test('single account query skips account selection', async () => {
  const plugin = makePlugin({}, { query: async (account) => `详情：${account.account_name}` });
  const ctx = makeCtx({ accounts });
  await plugin.queryMine(ctx, plugin.helpers(ctx));

  assert.equal(ctx.calls.listens.length, 0);
  assert.match(ctx.calls.replies.at(-1), /详情：账号1/);
});

test('query account selection paginates every 20 accounts', async () => {
  const manyAccounts = Array.from({ length: 21 }, (_, index) => ({ id: index + 1, account_name: `账号${index + 1}` }));
  const plugin = makePlugin({}, { query: async (account) => `详情：${account.account_name}` });
  const ctx = makeCtx({ accounts: manyAccounts, listens: ['n', '21'] });
  await plugin.queryMine(ctx, plugin.helpers(ctx));

  assert.match(ctx.calls.replies[0], /\[20\] 账号20/);
  assert.doesNotMatch(ctx.calls.replies[0], /\[21\] 账号21/);
  assert.match(ctx.calls.replies[0], /第 1\/2 页/);
  assert.match(ctx.calls.replies[1], /\[21\] 账号21/);
  assert.match(ctx.calls.replies.at(-1), /详情：账号21/);
});
