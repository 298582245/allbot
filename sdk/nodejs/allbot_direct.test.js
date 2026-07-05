const test = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { Context, PAY, WebRequest, WebResponse } = require('./allbot_direct');

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

test('sendMarkdown writes send_markdown protocol action', async () => {
    const { ctx } = makeContext();
    const actions = [];
    ctx._send = (action) => {
        actions.push(action);
        return true;
    };
    await ctx.sendMarkdown('**hi**');
    await ctx.send_markdown('## title');
    assert.deepEqual(actions, [
        { action: 'send_markdown', markdown: '**hi**' },
        { action: 'send_markdown', markdown: '## title' }
    ]);
});

test('sendRich writes send_rich protocol action and normalizes parts', async () => {
    const { ctx } = makeContext();
    const actions = [];
    ctx._send = (action) => {
        actions.push(action);
        return true;
    };
    await ctx.sendRich(['中文', { image: 'https://example.com/a.png', alt: '图' }, { markdown: '**价格**' }], { fallbackText: '中文 图', prefer: 'markdown' });
    await ctx.reply_rich([{ url: 'https://example.com/b.png' }]);
    assert.deepEqual(actions, [
        { action: 'send_rich', parts: [{ type: 'text', text: '中文' }, { type: 'image', url: 'https://example.com/a.png', alt: '图' }, { type: 'markdown', markdown: '**价格**' }], fallback_text: '中文 图', prefer: 'markdown' },
        { action: 'send_rich', parts: [{ type: 'image', url: 'https://example.com/b.png', alt: '' }], fallback_text: '', prefer: 'auto' }
    ]);
});

test('sendRichMessage sends request with target fields', async () => {
    const { ctx, calls } = makeContext({ platform: 'qq_office', adapter_id: '7', user_id: 'current' });
    await ctx.sendRichMessage({ platform: 'qq_office', userId: 'u1', groupId: 'g1', unionId: 'U1', parts: ['你好'], prefer: 'split' });
    assert.equal(calls[0].expectedAction, 'send_rich_message_response');
    assert.deepEqual(calls[0].action, {
        action: 'send_rich_message',
        platform: 'qq_office',
        adapter_id: '7',
        user_id: 'u1',
        group_id: 'g1',
        union_id: 'U1',
        parts: [{ type: 'text', text: '你好' }],
        fallback_text: '',
        prefer: 'split'
    });
});

test('sendButtons writes send_buttons protocol action', async () => {
    const { ctx } = makeContext();
    const actions = [];
    ctx._send = (action) => {
        actions.push(action);
        return true;
    };
    await ctx.sendButtons('请选择', [[{ text: 'A', value: '1', userId: 'u1' }]]);
    await ctx.send_buttons('继续', [[{ text: 'B', value: '2' }, { text: '', value: 'x' }]]);
    assert.deepEqual(actions, [
        { action: 'send_buttons', text: '请选择', buttons: [[{ text: 'A', value: '1', user_id: 'u1' }]] },
        { action: 'send_buttons', text: '继续', buttons: [[{ text: 'B', value: '2' }]] }
    ]);
});

test('sendMessage includes buttons when provided', async () => {
    const { ctx, calls } = makeContext({ platform: 'telegram', adapter_id: '1' });
    await ctx.sendMessage({ platform: 'telegram', userId: 'u1', text: 'hi', buttons: [[{ text: 'A', value: '1' }, { text: '', value: 'x' }]] });
    assert.deepEqual(calls[0].action.buttons, [[{ text: 'A', value: '1' }]]);
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

test('Context exposes pay helper and web router', () => {
    const { ctx } = makeContext();
    assert.ok(ctx.pay instanceof PAY);
    ctx.web.get('/orders', () => new WebResponse({ ok: true }));
    assert.equal(ctx.web.match('GET', 'orders').path, '/orders');
});

test('WebRequest parses json body and normalizes route data', async () => {
    const req = new WebRequest({ method: 'post', path: 'orders/', query: { page: ['1'] }, headers: { accept: ['json'] }, body: '{"id":1}' });
    assert.equal(req.method, 'POST');
    assert.equal(req.path, '/orders');
    assert.deepEqual(req.query, { page: '1' });
    assert.deepEqual(req.headers, { accept: 'json' });
    assert.deepEqual(await req.json(), { id: 1 });
    assert.deepEqual(req.jsonResponse({ ok: true }, 201).toAction(), {
        action: 'web_response',
        status: 201,
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
        json: { ok: true }
    });
});

test('runDirect dispatches web_api requests to registered web route', () => {
    const child = spawnSync(process.execPath, ['-e', `
        const { runDirect } = require('./allbot_direct');
        runDirect(async (ctx) => {
            ctx.web.post('/orders', async (req) => req.jsonResponse({ path: req.path, data: await req.json() }, 201));
        });
    `], {
        cwd: __dirname,
        input: JSON.stringify({ event_type: 'web_api', method: 'POST', path: '/orders', body: '{"sku":"A"}' }) + '\n',
        env: { ...process.env, ALLBOT_PLUGIN_ID: 'plugin-sdk-test' },
        encoding: 'utf8'
    });
    assert.equal(child.status, 0, child.stderr);
    assert.deepEqual(JSON.parse(child.stdout), {
        action: 'web_response',
        status: 201,
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
        json: { path: '/orders', data: { sku: 'A' } }
    });
});

test('Context exposes event from metadata event_name', () => {
    const { ctx } = makeContext({
        metadata: {
            message_type: 'event',
            event_name: 'GROUP_MEMBER_ADD',
            qq_office_timestamp: '123456',
            qq_office_group_openid: 'group-openid',
            qq_office_member_openid: 'member-openid'
        }
    });
    assert.equal(ctx.event.name, 'GROUP_MEMBER_ADD');
    assert.equal(ctx.event.eventName, 'GROUP_MEMBER_ADD');
    assert.equal(ctx.event.groupOpenid, 'group-openid');
    assert.equal(ctx.event.member_openid, 'member-openid');
    assert.equal(ctx.event.timestamp, '123456');
});

test('Context event is null for normal qq metadata', () => {
    const { ctx } = makeContext({ metadata: { qq_office_event_type: 'GROUP_MESSAGE_CREATE' } });
    assert.equal(ctx.event, null);
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

test('sendMessage same platform inherits context adapterId', async () => {
    const { ctx, calls } = makeContext({ platform: 'qq_office', adapter_id: '3', user_id: 'admin' });
    await ctx.sendMessage({ platform: 'qq_office', userId: 'target', text: 'hi' });
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.equal(calls[0].action.platform, 'qq_office');
    assert.equal(calls[0].action.adapter_id, '3');
    assert.equal(calls[0].action.user_id, 'target');
});

test('sendMessage cross platform does not inherit context adapterId', async () => {
    const { ctx, calls } = makeContext({ platform: 'qq_office', adapter_id: '3', user_id: 'admin' });
    await ctx.sendMessage({ platform: 'telegram', unionId: 'U_qq_123', text: 'hi' });
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.equal(calls[0].action.platform, 'telegram');
    assert.equal(calls[0].action.adapter_id, '');
    assert.equal(calls[0].action.union_id, 'U_qq_123');
});

test('sendMessage explicit adapterId still wins', async () => {
    const { ctx, calls } = makeContext({ platform: 'qq_office', adapter_id: '3' });
    await ctx.sendMessage({ platform: 'telegram', adapterId: '8', userId: 'u1', text: 'hi' });
    assert.equal(calls[0].expectedAction, 'send_message_response');
    assert.equal(calls[0].action.platform, 'telegram');
    assert.equal(calls[0].action.adapter_id, '8');
    assert.equal(calls[0].action.user_id, 'u1');
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
