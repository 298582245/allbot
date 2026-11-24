'use strict';

const path = require('node:path');
const { runAdapter } = require(path.resolve(__dirname, '../../../sdk/nodejs/allbot_adapter'));

const adapter = {
  async start(config, emit, context) {
    if (!config.token) throw new Error('token is required');
    this.emit = emit;
    console.error('Callback URL:', context.callbackUrl);
    // Connect to the platform and call this.emit(unifiedMessage) for incoming messages.
  },
  async stop() {},
  async sendText({ target, text }) { console.error(`send text to ${target}: ${text}`); },
  async sendImage() { throw new Error('Image sending is not implemented in this sample'); },
  async sendAudio() { throw new Error('Audio sending is not implemented in this sample'); },
  async sendVideo() { throw new Error('Video sending is not implemented in this sample'); },
  async sendFile() { throw new Error('File sending is not implemented in this sample'); },
  async sendMarkdown() { throw new Error('Markdown sending is not implemented in this sample'); },
  async sendRich() { throw new Error('Rich messages are not implemented in this sample'); },
  async sendButtons() { throw new Error('Buttons are not implemented in this sample'); },
  async sendTextWithSequence({ target, text }) { return this.sendText({ target, text }); },
  async getUserInfo({ user_id }) { return { user_id, nickname: '', avatar: '', extra: {} }; },
  async getGroupInfo({ group_id }) { return { group_id, name: '', member_count: 0, extra: {} }; },
  async atUser({ group_id, user_id }) { return this.sendText({ target: `group_${group_id}`, text: `@${user_id}` }); },
  async deleteMessage() { throw new Error('Message deletion is not implemented in this sample'); },
  async mute() { throw new Error('Mute is not implemented in this sample'); },
};

runAdapter(adapter);
