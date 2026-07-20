'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const { action } = require('./PushMessage')

function createResponse() {
  return {
    statusCode: null,
    payload: null,
    status(code) {
      this.statusCode = code
      return this
    },
    json(payload) {
      this.payload = payload
      return this
    }
  }
}

async function invoke({ body = {}, query = {}, pushError, richError } = {}) {
  const calls = []
  const richCalls = []
  const ctx = {
    async push(payload) {
      calls.push(payload)
      if (pushError) throw pushError
      return { success: true }
    },
    async sendRichMessage(payload) {
      richCalls.push(payload)
      if (richError) throw richError
      return { success: true }
    }
  }
  const res = createResponse()
  await action(ctx, { method: 'POST', query, headers: {}, body }, res)
  assert.equal(res.statusCode, 200)
  assert.ok(Object.prototype.hasOwnProperty.call(res.payload, 'retcode'))
  return { calls, richCalls, res }
}

async function expectFailure(options, messagePattern) {
  const { calls, richCalls, res } = await invoke(options)
  assert.equal(res.payload.status, 'failed')
  assert.equal(res.payload.retcode, 100)
  assert.equal(res.payload.data, null)
  assert.match(res.payload.errmsg, messagePattern)
  assert.equal(calls.length, options.pushError ? 1 : 0)
  assert.equal(richCalls.length, options.richError ? 1 : 0)
}

test('不传 platform 时由 adapter_id 自动识别平台', async () => {
  const { calls, res } = await invoke({
    body: { message: 'QQ 官方群消息' },
    query: { adapter_id: '3', group_id: 'group_openid-abc_123', access_token: 'ignored' }
  })

  assert.deepEqual(calls, [{
    adapterId: '3',
    groupId: 'group_openid-abc_123',
    content: 'QQ 官方群消息'
  }])
  assert.deepEqual(res.payload, {
    status: 'ok',
    retcode: 0,
    errmsg: '',
    data: { adapterId: '3', groupId: 'group_openid-abc_123' }
  })
})

test('支持显式平台和非数字用户 ID', async () => {
  const { calls, res } = await invoke({
    body: { message: '私聊消息' },
    query: { platform: 'qq_office', adapter_id: '3', user_id: 'user_openid-xyz' }
  })

  assert.deepEqual(calls, [{
    adapterId: '3',
    userId: 'user_openid-xyz',
    content: '私聊消息',
    platform: 'qq_office'
  }])
  assert.deepEqual(res.payload, {
    status: 'ok',
    retcode: 0,
    errmsg: '',
    data: { platform: 'qq_office', adapterId: '3', userId: 'user_openid-xyz' }
  })
})

test('含 HTTP 链接时发送 Markdown 富消息并保留原文回退', async () => {
  const message = [
    '',
    '限时 *福利* [精选]',
    '商品名称：咖啡豆_特价',
    '地址：https://example.com/detail_(1)?from=a&tag=b',
    '备注：原价 #99，访问 https://other.example/a_(2)',
    '普通 (文本) <标签> ~结束~'
  ].join('\n')
  const { calls, richCalls, res } = await invoke({
    body: { message },
    query: { platform: 'telegram', adapter_id: '5', group_id: '-1001234567890' }
  })

  assert.deepEqual(calls, [])
  assert.deepEqual(richCalls, [{
    platform: 'telegram',
    adapterId: '5',
    groupId: '-1001234567890',
    parts: [{
      type: 'markdown',
      markdown: [
        '### 限时 \\*福利\\* \\[精选\\]',
        '**商品名称：** 咖啡豆\\_特价',
        '**地址：** [点击打开](https://example.com/detail_%281%29?from=a&tag=b)',
        '**备注：** 原价 \\#99，访问 [点击打开](https://other.example/a_%282%29)',
        '普通 \\(文本\\) \\<标签\\> \\~结束\\~'
      ].join('\n\n')
    }],
    prefer: 'markdown',
    fallbackText: message
  }])
  assert.deepEqual(res.payload, {
    status: 'ok',
    retcode: 0,
    errmsg: '',
    data: { platform: 'telegram', adapterId: '5', groupId: '-1001234567890' }
  })
})

test('标题跳过字段行并支持大小写混合的 HTTPS 地址标签', async () => {
  const message = [
    '任务状态：成功',
    '执行结果 [完成]',
    '下载地址: HTTPS://example.com/file_(final).zip。'
  ].join('\n')
  const { calls, richCalls } = await invoke({
    body: { message },
    query: { adapter_id: '3', user_id: 'user-openid' }
  })

  assert.deepEqual(calls, [])
  assert.equal(richCalls.length, 1)
  assert.deepEqual(richCalls[0].parts, [{
    type: 'markdown',
    markdown: [
      '**任务状态：** 成功',
      '### 执行结果 \\[完成\\]',
      '**下载地址:** [点击打开](HTTPS://example.com/file_%28final%29.zip)。'
    ].join('\n\n')
  }])
  assert.equal(richCalls[0].fallbackText, message)
  assert.equal(richCalls[0].prefer, 'markdown')
})

test('地址链接排除西文句末标点和未配对右括号', async () => {
  const message = '更新通知\n地址：https://example.com/a_(1)),.'
  const { richCalls } = await invoke({
    body: { message },
    query: { adapter_id: '3', user_id: 'user-openid' }
  })

  assert.equal(richCalls[0].parts[0].markdown, [
    '### 更新通知',
    '**地址：** [点击打开](https://example.com/a_%281%29)\\),\\.'
  ].join('\n\n'))
})

test('多条线报的标题、内容、时间和地址均独立成段', async () => {
  const message = [
    '线报-赚客吧 线报',
    '标题：仅剩10年前的大毛尸体',
    '内容：',
    '时间：2026-07-18 23:04',
    '地址：http://new.ixbk.net/zuankeba/6655007.html',
    '标题：800高价收支付宝探店品',
    '内容：高价收购上海团购券',
    '时间：2026-07-18 23:04',
    '地址：http://new.ixbk.net/zuankeba/6655004.html'
  ].join('\n')
  const { richCalls } = await invoke({
    body: { message },
    query: { adapter_id: '3', group_id: 'group-openid' }
  })

  assert.equal(richCalls.length, 1)
  assert.deepEqual(richCalls[0].parts[0].markdown.split('\n\n'), [
    '### 线报\\-赚客吧 线报',
    '**标题：** 仅剩10年前的大毛尸体',
    '**内容：**',
    '**时间：** 2026\\-07\\-18 23:04',
    '**地址：** [点击打开](http://new.ixbk.net/zuankeba/6655007.html)',
    '**标题：** 800高价收支付宝探店品',
    '**内容：** 高价收购上海团购券',
    '**时间：** 2026\\-07\\-18 23:04',
    '**地址：** [点击打开](http://new.ixbk.net/zuankeba/6655004.html)'
  ])
  assert.equal(richCalls[0].fallbackText, message)
})

test('普通字段中的已转义链接转换为手机 QQ 兼容的短链接文本', async () => {
  const message = [
    '大潮',
    '用户： 19002004136 微信领取链接：https://m\\.aihoge\\.com/lottery/rotor/drawRedPacket?CHECK\\_CODE=AX6a5c28ef73ca2bo9FB',
    '用户： 18174700780 微信领取链接：https://m\\.aihoge\\.com/lottery/rotor/drawRedPacket?CHECK\\_CODE=AX6a598660be2cepPRCe',
    '时间只不过是人类定义的，时间不会往回走'
  ].join('\n')
  const { richCalls } = await invoke({
    body: { message },
    query: { adapter_id: '3', group_id: 'group-openid' }
  })

  assert.equal(richCalls.length, 1)
  assert.deepEqual(richCalls[0].parts[0].markdown.split('\n\n'), [
    '### 大潮',
    '**用户：** 19002004136 微信领取链接：[点击打开](https://m.aihoge.com/lottery/rotor/drawRedPacket?CHECK_CODE=AX6a5c28ef73ca2bo9FB)',
    '**用户：** 18174700780 微信领取链接：[点击打开](https://m.aihoge.com/lottery/rotor/drawRedPacket?CHECK_CODE=AX6a598660be2cepPRCe)',
    '时间只不过是人类定义的，时间不会往回走'
  ])
  assert.equal(richCalls[0].fallbackText, message)
})

test('保留 Telegram 和钉钉等平台的原始目标 ID', async () => {
  const telegram = await invoke({
    body: { message: 'Telegram 消息' },
    query: { platform: 'telegram', adapter_id: '5', group_id: '-1001234567890' }
  })
  assert.equal(telegram.calls[0].groupId, '-1001234567890')
  assert.equal(telegram.res.payload.retcode, 0)

  const dingtalk = await invoke({
    body: { message: '钉钉消息' },
    query: { platform: 'dingtalk', adapter_id: '6', user_id: 'staff-user_abc' }
  })
  assert.equal(dingtalk.calls[0].userId, 'staff-user_abc')
  assert.equal(dingtalk.res.payload.retcode, 0)
})

test('拒绝缺失或非法消息正文', async () => {
  await expectFailure({
    body: {},
    query: { adapter_id: '1', group_id: 'group-a' }
  }, /message/)
  await expectFailure({
    body: { message: '   ' },
    query: { adapter_id: '1', group_id: 'group-a' }
  }, /message/)
  await expectFailure({
    body: { message: 123 },
    query: { adapter_id: '1', group_id: 'group-a' }
  }, /message/)
})

test('拒绝缺失、空白或重复的单值参数', async () => {
  await expectFailure({
    body: { message: 'test' },
    query: { group_id: 'group-a' }
  }, /adapter_id/)
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: '   ', group_id: 'group-a' }
  }, /adapter_id/)
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: ['1', '2'], group_id: 'group-a' }
  }, /adapter_id.*重复/)
  await expectFailure({
    body: { message: 'test' },
    query: { platform: ['qq', 'qq_office'], adapter_id: '1', group_id: 'group-a' }
  }, /platform.*重复/)
  await expectFailure({
    body: { message: 'test' },
    query: { platform: '   ', adapter_id: '1', group_id: 'group-a' }
  }, /platform/)
})

test('群组和用户目标必须且只能提供一个', async () => {
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: '1' }
  }, /必须且只能提供一个/)
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: '1', group_id: 'group-a', user_id: 'user-a' }
  }, /必须且只能提供一个/)
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: '1', group_id: '   ' }
  }, /group_id/)
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: '1', user_id: ['user-a', 'user-b'] }
  }, /user_id.*重复/)
})

test('发送失败仍返回 HTTP 200 和顶层 retcode 100', async () => {
  await expectFailure({
    body: { message: 'test' },
    query: { adapter_id: '3', group_id: 'group-openid' },
    pushError: new Error('消息发送失败')
  }, /消息发送失败/)
  await expectFailure({
    body: { message: '地址：https://example.com' },
    query: { adapter_id: '3', group_id: 'group-openid' },
    richError: new Error('富消息发送失败')
  }, /富消息发送失败/)
})
