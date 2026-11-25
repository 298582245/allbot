'use strict';

const path = require('node:path');
const sdkCandidates = [
  path.resolve(__dirname, '../../../sdk/nodejs/allbot_adapter'),
  path.resolve(__dirname, '../../sdk/nodejs/allbot_adapter'),
  path.resolve(process.cwd(), 'sdk/nodejs/allbot_adapter'),
];
const sdkPath = sdkCandidates.find((candidate) => {
  try {
    require.resolve(candidate);
    return true;
  } catch {
    return false;
  }
});
if (!sdkPath) throw new Error('找不到 sdk/nodejs/allbot_adapter.js，请在 AllBot 项目目录中运行');
const { runAdapter } = require(sdkPath);

const DEFAULT_API_BASE = 'https://www.kookapp.cn/api/v3';
const API_TIMEOUT_MS = 3500;
const API_MAX_RETRIES = 3;

class KookAdapter {
  constructor() {
    this.running = false;
    this.socket = null;
    this.config = null;
    this.emit = null;
    this.lastSequence = 0;
    this.sessionId = '';
    this.heartbeatTimer = null;
    this.heartbeatTimeout = null;
    this.reconnectDelayTimer = null;
    this.reconnectDelayResolve = null;
    this.eventQueue = Promise.resolve();
  }

  async start(config, emit) {
    if (!config.bot_token) throw new Error('Bot Token 不能为空');
    if (typeof WebSocket !== 'function') throw new Error('KOOK 适配器需要 Node.js 22 或更新版本');

    this.config = {
      token: String(config.bot_token).trim(),
      apiBase: String(config.api_base_url || DEFAULT_API_BASE).trim().replace(/\/$/, ''),
    };
    this.emit = emit;
    this.running = true;

    const connection = await this.connect();
    this.reconnectTask = this.reconnectLoop(connection.closed);
    console.error('KOOK Gateway 已连接');
  }

  async stop() {
    this.running = false;
    this.reconnectDelayResolve?.();
    this.clearHeartbeat();
    this.socket?.close();
    this.socket = null;
    await this.reconnectTask;
  }

  async reconnectLoop(closed) {
    let delay = 2000;
    while (this.running) {
      if (closed) await closed;
      if (!this.running) break;
      await this.waitForReconnectDelay(delay);
      if (!this.running) break;
      try {
        const connection = await this.connect();
        closed = connection.closed;
        delay = 2000;
        console.error('KOOK Gateway 已重新连接');
      } catch (error) {
        closed = null;
        console.error(`KOOK Gateway 重连失败: ${error.message}`);
        delay = Math.min(delay * 2, 60000);
      }
    }
  }

  waitForReconnectDelay(milliseconds) {
    return new Promise((resolve) => {
      const finish = () => {
        clearTimeout(this.reconnectDelayTimer);
        this.reconnectDelayTimer = null;
        this.reconnectDelayResolve = null;
        resolve();
      };
      this.reconnectDelayResolve = finish;
      this.reconnectDelayTimer = setTimeout(finish, milliseconds);
    });
  }

  async connect() {
    const gateway = await this.api('/gateway/index?compress=0');
    const gatewayUrl = new URL(gateway.url);
    if (this.sessionId) {
      gatewayUrl.searchParams.set('resume', '1');
      gatewayUrl.searchParams.set('sn', String(this.lastSequence));
      gatewayUrl.searchParams.set('session_id', this.sessionId);
    }

    return new Promise((resolve, reject) => {
      let closeConnection;
      const closed = new Promise((finish) => { closeConnection = finish; });
      const socket = new WebSocket(gatewayUrl);
      this.socket = socket;
      let settled = false;
      const handshakeTimeout = setTimeout(() => {
        finish(new Error('等待 KOOK Gateway 握手超时'));
        socket.close();
      }, 15000);

      const finish = (error) => {
        if (settled) return;
        settled = true;
        clearTimeout(handshakeTimeout);
        if (error) reject(error);
        else resolve({ closed });
      };

      socket.addEventListener('message', (event) => {
        let packet;
        try {
          packet = JSON.parse(String(event.data));
        } catch {
          return;
        }

        if (packet.s === 1) {
          if (packet.d?.code && packet.d.code !== 0) {
            if (this.sessionId) {
              this.sessionId = '';
              this.lastSequence = 0;
            }
            finish(new Error(`KOOK Gateway 握手失败: ${packet.d.code}`));
            socket.close();
            return;
          }
          if (packet.d?.session_id) this.sessionId = packet.d.session_id;
          this.startHeartbeat();
          finish();
          return;
        }

        if (packet.s === 0) {
          this.eventQueue = this.eventQueue
            .then(() => this.handleEvent(packet.d))
            .then(() => {
              if (Number.isFinite(packet.sn)) this.lastSequence = Math.max(this.lastSequence, packet.sn);
            })
            .catch((error) => console.error(`处理 KOOK 事件失败: ${error.message}`));
          return;
        }

        if (packet.s === 3) {
          clearTimeout(this.heartbeatTimeout);
          this.heartbeatTimeout = null;
          return;
        }

        if (packet.s === 5) {
          this.sessionId = '';
          this.lastSequence = 0;
          socket.close();
        }

        if (packet.s === 6 && packet.d?.session_id) this.sessionId = packet.d.session_id;
      });

      socket.addEventListener('error', () => finish(new Error('KOOK Gateway WebSocket 连接失败')));
      socket.addEventListener('close', () => {
        closeConnection();
        this.clearHeartbeat();
        if (this.socket === socket) this.socket = null;
        if (!settled) finish(new Error('KOOK Gateway 在握手完成前关闭'));
      });
    });
  }

  startHeartbeat() {
    this.clearHeartbeat();
    const ping = () => {
      if (!this.running || this.socket?.readyState !== WebSocket.OPEN) return;
      this.socket.send(JSON.stringify({ s: 2, sn: this.lastSequence }));
      this.heartbeatTimeout = setTimeout(() => this.socket?.close(), 6000);
      this.heartbeatTimer = setTimeout(ping, 30000 + Math.floor(Math.random() * 10001) - 5000);
    };
    this.heartbeatTimer = setTimeout(ping, 30000);
  }

  clearHeartbeat() {
    clearTimeout(this.heartbeatTimer);
    clearTimeout(this.heartbeatTimeout);
    this.heartbeatTimer = null;
    this.heartbeatTimeout = null;
  }

  async handleEvent(event) {
    if (!event || event.type === 255 || !event.author_id) return;
    const channelType = String(event.channel_type || '').toUpperCase();
    if (channelType !== 'GROUP' && channelType !== 'PERSON') return;
    if (event.extra?.author?.bot) return;

    const author = event.extra?.author || {};
    const groupId = channelType === 'GROUP' ? String(event.target_id || '') : '';
    const userId = String(event.author_id);
    const content = String(event.content || '');
    const mediaType = ({ 2: 'image', 3: 'video', 4: 'file', 8: 'audio' })[Number(event.type)];
    const mediaUrl = mediaType ? String(event.content || event.extra?.attachments?.url || '') : '';
    const metadata = {
      reply_target: groupId ? `group_${groupId}` : `user_${userId}`,
      kook_channel_type: channelType,
      kook_target_id: String(event.target_id || ''),
      kook_guild_id: String(event.extra?.guild_id || ''),
    };
    if (event.extra?.chat_code) metadata.kook_chat_code = String(event.extra.chat_code);

    await this.emit({
      id: String(event.msg_id || ''),
      user_id: userId,
      nickname: String(author.nickname || author.username || ''),
      group_id: groupId,
      content,
      medias: mediaUrl ? [{ type: mediaType, url: mediaUrl }] : [],
      media_only: Boolean(mediaUrl && !content),
      metadata,
    });
  }

  async api(endpoint, options = {}) {
    for (let attempt = 0; attempt <= API_MAX_RETRIES; attempt += 1) {
      let retryable = true;
      try {
        const response = await fetch(`${this.config.apiBase}${endpoint}`, {
          ...options,
          signal: AbortSignal.timeout(API_TIMEOUT_MS),
          headers: {
            Authorization: `Bot ${this.config.token}`,
            ...(options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
            ...options.headers,
          },
        });
        const transientStatus = response.status === 408 || response.status === 429 || response.status >= 500;
        retryable = response.ok || transientStatus;
        const result = await response.json();
        retryable = transientStatus;
        if (!response.ok || result.code !== 0) {
          throw new Error(result.message || `KOOK API 请求失败 (${response.status})`);
        }
        return result.data || {};
      } catch (error) {
        if (!retryable || attempt === API_MAX_RETRIES) throw error;
        console.error(`KOOK API ${endpoint} 请求失败，重试 ${attempt + 1}/${API_MAX_RETRIES}: ${error.message}`);
        await new Promise((resolve) => setTimeout(resolve, 300 * (2 ** attempt)));
      }
    }
  }

  parseTarget(target) {
    const value = String(target || '');
    if (value.startsWith('group_')) return { channelType: 'GROUP', id: value.slice(6) };
    if (value.startsWith('user_')) return { channelType: 'PERSON', id: value.slice(5) };
    throw new Error('KOOK 发送目标格式无效');
  }

  async sendToTarget(target, content, type = 9) {
    const parsed = this.parseTarget(target);
    const endpoint = parsed.channelType === 'GROUP' ? '/message/create' : '/direct-message/create';
    return this.api(endpoint, {
      method: 'POST',
      body: JSON.stringify({ target_id: parsed.id, type, content }),
    });
  }

  async sendText({ target, text }) {
    await this.sendToTarget(target, text, 9);
  }

  async sendMarkdown({ target, markdown }) {
    await this.sendToTarget(target, markdown, 9);
  }

  async sendRich({ target, message }) {
    const modules = [];
    for (const part of message.parts || []) {
      if (part.type === 'image' && part.url) {
        modules.push({
          type: 'container',
          elements: [{ type: 'image', src: await this.uploadMedia(part.url), alt: part.alt || '' }],
        });
      } else if (part.type === 'text' || part.type === 'markdown') {
        const content = part.type === 'markdown' ? part.markdown : part.text;
        if (String(content || '').trim()) {
          modules.push({
            type: 'section',
            text: { type: part.type === 'markdown' ? 'kmarkdown' : 'plain-text', content },
          });
        }
      }
    }
    if (!modules.length) {
      if (message.fallback_text) await this.sendText({ target, text: message.fallback_text });
      return;
    }
    await this.sendToTarget(target, JSON.stringify([{ type: 'card', theme: 'secondary', size: 'lg', modules }]), 10);
  }

  async uploadFile(bytes, filename, contentType = 'application/octet-stream') {
    const form = new FormData();
    form.append('file', new Blob([bytes], { type: contentType }), filename);
    const data = await this.api('/asset/create', { method: 'POST', body: form });
    if (!data.url) throw new Error('KOOK 上传接口没有返回资源地址');
    return data.url;
  }

  async uploadUrl(url) {
    const response = await fetch(url);
    if (!response.ok) throw new Error(`下载媒体失败 (${response.status})`);
    const bytes = Buffer.from(await response.arrayBuffer());
    const filename = path.basename(new URL(url).pathname) || 'media';
    return this.uploadFile(bytes, filename, response.headers.get('content-type') || 'application/octet-stream');
  }

  async uploadMedia(urlOrPath) {
    const isUrl = /^https?:\/\//i.test(String(urlOrPath));
    if (isUrl) {
      return this.uploadUrl(String(urlOrPath));
    }
    const fs = require('node:fs/promises');
    const filePath = path.resolve(String(urlOrPath));
    return this.uploadFile(await fs.readFile(filePath), path.basename(filePath));
  }

  async sendMedia(target, urlOrPath, type) {
    const uploadedUrl = await this.uploadMedia(urlOrPath);
    await this.sendToTarget(target, uploadedUrl, type);
  }

  async sendImage({ target, url }) { await this.sendMedia(target, url, 2); }
  async sendAudio({ target, url }) { await this.sendMedia(target, url, 8); }
  async sendVideo({ target, url }) { await this.sendMedia(target, url, 3); }
  async sendFile({ target, path: filePath }) { await this.sendMedia(target, filePath, 4); }

  async atUser({ group_id, user_id }) {
    await this.sendText({ target: `group_${group_id}`, text: `(met)${user_id}(met)` });
  }

  async getUserInfo({ user_id }) {
    const query = new URLSearchParams({ user_id });
    const user = await this.api(`/user/view?${query}`);
    return { user_id: String(user.id || user_id), nickname: String(user.nickname || user.username || ''), avatar: String(user.avatar || ''), extra: {} };
  }

  async getGroupInfo({ group_id }) {
    const query = new URLSearchParams({ guild_id: group_id });
    const guild = await this.api(`/guild/view?${query}`);
    return { group_id: String(guild.id || group_id), name: String(guild.name || ''), member_count: 0, extra: {} };
  }
}

runAdapter(new KookAdapter());
