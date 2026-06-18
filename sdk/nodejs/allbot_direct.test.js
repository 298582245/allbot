const test = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { Context, PAY } = require('./allbot_direct');

function makeContext(data = {}) {
    const ctx = new Context({ plugin_id: 'plugin-sdk', union_id: 'union-sdk', ...data }, { once() {} });
    const calls = [];
    ctx._request = async (action, expectedAction, timeoutSeconds) => {
        calls.push({ action, expectedAction, timeoutSeconds });
        return { status: 'paid', order_no: 'P1' };
    };
    return { ctx, calls };
}

function runNodeSdkSnippet(source, env = {}) {
    return spawnSync(process.execPath, ['-e', source], {
        cwd: __dirname,
        env: { ...process.env, ...env },
        encoding: 'utf8'
    });
}

test('plugin console.log goes to stderr and protocol stays on stdout', () => {
    const result = runNodeSdkSnippet(`
        const { Context } = require('./allbot_direct');
        console.log('debug %s', 'line');
        const ctx = new Context({ plugin_id: 'plugin-sdk-test' }, { once() {} });
        ctx._send({ action: 'reply', text: 'ok' });
    `, { ALLBOT_PLUGIN_ID: 'plugin-sdk-test' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, '{"action":"reply","text":"ok"}\n');
    assert.match(result.stderr, /\[PLUGIN_LOG\]\[INFO\] debug line/);
});

test('plugin console warn error debug include level prefixes', () => {
    const result = runNodeSdkSnippet(`
        require('./allbot_direct');
        console.warn('warn');
        console.error('error');
        console.debug('debug');
    `, { ALLBOT_PLUGIN_ID: 'plugin-sdk-test' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, '');
    assert.match(result.stderr, /\[PLUGIN_LOG\]\[WARN\] warn/);
    assert.match(result.stderr, /\[PLUGIN_LOG\]\[ERROR\] error/);
    assert.match(result.stderr, /\[PLUGIN_LOG\]\[DEBUG\] debug/);
});

test('non plugin environment keeps console stdout behavior', () => {
    const result = runNodeSdkSnippet(`
        require('./allbot_direct');
        console.log('normal');
    `, { ALLBOT_PLUGIN_ID: '' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, 'normal\n');
    assert.equal(result.stderr, '');
});

test('PAY(ctx).waitPay sends payment_wait request', async () => {
    const { ctx, calls } = makeContext();
    const result = await new PAY(ctx).waitPay('测试消费', '1.00', 60, { metadata: { k: 'v' } });
    assert.equal(result.order_no, 'P1');
    assert.equal(calls[0].expectedAction, 'payment_response');
    assert.equal(calls[0].timeoutSeconds, 70);
    assert.deepEqual(calls[0].action, {
        action: 'payment_wait',
        subject: '测试消费',
        amount: '1.00',
        timeout: 60,
        methods: [],
        metadata: { k: 'v' },
        remark: ''
    });
});

test('PAY() uses current context and wait_pay alias supports options timeout', async () => {
    const { calls } = makeContext();
    const result = await new PAY().wait_pay('别名消费', 2, { timeout: 45, metadata: { alias: true } });
    assert.equal(result.status, 'paid');
    assert.equal(calls[0].action.subject, '别名消费');
    assert.equal(calls[0].action.amount, '2');
    assert.equal(calls[0].action.timeout, 45);
    assert.deepEqual(calls[0].action.metadata, { alias: true });
    assert.equal(calls[0].timeoutSeconds, 55);
});

test('Context exposes pay helper', () => {
    const { ctx } = makeContext();
    assert.ok(ctx.pay instanceof PAY);
});

test('runScript sends runtimeProfile as runtime_profile', async () => {
    const { ctx, calls } = makeContext();
    await ctx.runScript({ runtime: 'nodejs', runtimeProfile: 'node18', script: 'task.js' });
    assert.equal(calls[0].expectedAction, 'script_response');
    assert.equal(calls[0].action.runtime_profile, 'node18');
});

test('runScript accepts snake_case runtime_profile', async () => {
    const { ctx, calls } = makeContext();
    await ctx.runScript({ runtime: 'nodejs', runtime_profile: 'node20', script: 'task.js' });
    assert.equal(calls[0].action.runtime_profile, 'node20');
});

test('push positional args sends send_message request', async () => {
    const { ctx, calls } = makeContext();
    await ctx.push('u1', 'g1', 'hello', 'telegram', '2');
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.deepEqual(calls[0].action, {
        action: 'send_message',
        platform: 'telegram',
        adapter_id: '2',
        user_id: 'u1',
        group_id: 'g1',
        union_id: '',
        text: 'hello'
    });
});

test('push object args accepts camelCase fields', async () => {
    const { ctx, calls } = makeContext();
    await ctx.push({ userId: 'u2', groupId: 'g2', content: 'hi', platform: 'telegram', adapterId: '3' });
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.deepEqual(calls[0].action, {
        action: 'send_message',
        platform: 'telegram',
        adapter_id: '3',
        user_id: 'u2',
        group_id: 'g2',
        union_id: '',
        text: 'hi'
    });
});

test('push omitted adapterId does not use context adapterId', async () => {
    const { ctx, calls } = makeContext({ platform: 'telegram', adapter_id: '9', user_id: 'current-user' });
    await ctx.push('u1', '', 'hello');
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.equal(calls[0].action.platform, 'telegram');
    assert.equal(calls[0].action.adapter_id, '');
    assert.equal(calls[0].action.user_id, 'u1');
    assert.equal(calls[0].action.group_id, '');
    assert.equal(calls[0].action.union_id, '');
});

test('push accepts explicit unionId', async () => {
    const { ctx, calls } = makeContext({ platform: 'telegram' });
    await ctx.push({ unionId: 'U_qq_123', content: 'hi' });
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.equal(calls[0].action.platform, 'telegram');
    assert.equal(calls[0].action.user_id, '');
    assert.equal(calls[0].action.union_id, 'U_qq_123');
});

test('push treats union-prefixed userId as unionId', async () => {
    const { ctx, calls } = makeContext({ platform: 'telegram' });
    await ctx.push('union:U_qq_123', '', 'hi');
    assert.equal(calls[0].action.user_id, '');
    assert.equal(calls[0].action.union_id, 'U_qq_123');
});
