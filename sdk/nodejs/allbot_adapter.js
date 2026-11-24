'use strict';

const readline = require('node:readline');
const util = require('node:util');

function runAdapter(adapter) {
  for (const method of ['log', 'info', 'warn']) {
    console[method] = (...args) => process.stderr.write(`${util.format(...args)}\n`);
  }
  const input = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
  const send = (value) => process.stdout.write(`${JSON.stringify(value)}\n`);
  const handlers = {
    send_text: 'sendText', send_image: 'sendImage', send_audio: 'sendAudio',
    send_video: 'sendVideo', send_file: 'sendFile', send_markdown: 'sendMarkdown',
    send_rich: 'sendRich', send_buttons: 'sendButtons',
    send_text_sequence: 'sendTextWithSequence', get_user_info: 'getUserInfo',
    get_group_info: 'getGroupInfo', at_user: 'atUser', delete_message: 'deleteMessage', mute: 'mute',
  };

  input.on('line', async (line) => {
    let request;
    try {
      request = JSON.parse(line);
      const data = request.data || {};
      if (request.action === 'start') {
        if (String(data.protocol_version) !== '1') throw new Error('Unsupported protocol version');
        await adapter.start(data.config || {}, (message) => send({ event: 'message', message }), { callbackUrl: data.callback_url || '' });
        send({ id: request.id, ok: true, data: {} });
        return;
      }
      if (request.action === 'stop') {
        await adapter.stop?.();
        send({ id: request.id, ok: true, data: {} });
        input.close();
        return;
      }
      if (request.action === 'http_callback') {
        const result = typeof adapter.handleHttpCallback === 'function'
          ? await adapter.handleHttpCallback(data)
          : { status: 404, headers: { 'Content-Type': 'text/plain' }, body: 'Not found' };
        send({ id: request.id, ok: true, data: result || { status: 200, body: '' } });
        return;
      }
      const method = handlers[request.action];
      if (!method || typeof adapter[method] !== 'function') throw new Error(`Unsupported action: ${request.action}`);
      const result = await adapter[method](data);
      send({ id: request.id, ok: true, data: result === undefined ? {} : result });
    } catch (error) {
      if (request?.id) send({ id: request.id, ok: false, error: error?.message || String(error) });
      else console.error(error);
    }
  });
}

module.exports = { runAdapter };
