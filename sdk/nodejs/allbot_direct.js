/**
 * AllBot Node.js Direct SDK
 *
 * 插件调用 runDirect(handler) 后，会收到封装好的 Context。
 * Context 提供回复、监听、配置、账号、授权、脚本运行、定时任务等能力。
 */
const readline = require('readline');

class Context {
    constructor(data, rl) {
        this.pluginId = data.plugin_id || '';
        this.plugin_id = this.pluginId;
        this.platform = data.platform || '';
        this.adapterId = data.adapter_id || '';
        this.adapter_id = this.adapterId;
        this.userId = data.user_id || '';
        this.user_id = this.userId;
        this.unionId = data.union_id || '';
        this.union_id = this.unionId;
        this.points = Number(data.points || 0);
        this.pointsUnit = data.points_unit || '积分';
        this.points_unit = this.pointsUnit;
        this.groupId = data.group_id || '';
        this.group_id = this.groupId;
        this.content = data.content || '';
        this.text = this.content;
        this.messageId = data.message_id || '';
        this.message_id = this.messageId;
        this.admin = Boolean(data.is_admin);
        this.is_admin = this.admin;
        this.metadata = data.metadata || {};
        this.userConfig = data.user_config || {};
        this.user_config = this.userConfig;
        this.accessControl = data.access_control || {};
        this.access_control = this.accessControl;
        this._rl = rl;
        this._requestSeq = 0;
        this.db = new Database(this);
    }

    
    isGroup() {
        return Boolean(this.groupId);
    }

    
    isPrivate() {
        return !this.groupId;
    }

    
    chatId() {
        return this.groupId || this.userId;
    }

    
    isAdmin() {
        return this.admin;
    }

    
    args(command = '') {
        if (!command) return this.content.trim();
        return this.content.startsWith(command) ? this.content.slice(command.length).trim() : '';
    }

    
    async reply(text) {
        return this._send({ action: 'reply', text: String(text ?? '') });
    }

    
    async sendText(text) {
        return this.reply(text);
    }

    async sendMessage(options = {}) {
        return this._request({
            action: 'send_message',
            platform: String(options.platform || this.platform || ''),
            adapter_id: this._adapterIdFor(options),
            user_id: String(options.userId || options.user_id || this.userId || ''),
            group_id: String(options.groupId || options.group_id || ''),
            union_id: String(options.unionId || options.union_id || ''),
            text: String(options.text || options.content || '')
        }, 'send_message_response');
    }

    async send_message(options = {}) {
        return this.sendMessage(options);
    }

    
    async sendImage(imageUrl) {
        return this._send({ action: 'send_image', url: String(imageUrl ?? '') });
    }

    
    async send_image(imageUrl) {
        return this.sendImage(imageUrl);
    }

    
    async sendFile(filePath) {
        return this._send({ action: 'send_file', path: String(filePath ?? '') });
    }

    
    async send_file(filePath) {
        return this.sendFile(filePath);
    }

    
    async listen(timeout = 60) {
        this._send({ action: 'listen', timeout });

        return new Promise((resolve) => {
            const timer = setTimeout(() => resolve(''), (timeout + 5) * 1000);
            this._rl.once('line', (line) => {
                clearTimeout(timer);
                try {
                    const response = JSON.parse(line);
                    resolve(response.action === 'listen_response' ? (response.content || '') : '');
                } catch (error) {
                    resolve('');
                }
            });
        });
    }

    
    async setDataView(tableName, options = {}) {
        const action = {
            action: 'set_data_view',
            table_name: tableName,
            view_name: options.viewName || options.view_name || tableName,
            group_name: options.groupName || options.group_name || '请求失败',
            description: options.description || '',
            columns: options.columns || []
        };
        return this._send(action);
    }

    
    async set_data_view(tableName, viewName = '', groupName = '请求失败', description = '', columns = []) {
        return this.setDataView(tableName, { viewName, groupName, description, columns });
    }

    
    meta(key, defaultValue = '') {
        return Object.prototype.hasOwnProperty.call(this.metadata, key) ? this.metadata[key] : defaultValue;
    }

    
    config(key, defaultValue = '') {
        if (!key) return this.userConfig;
        return Object.prototype.hasOwnProperty.call(this.userConfig, key) ? this.userConfig[key] : defaultValue;
    }

    async getUnionId() {
        if (this.unionId) return this.unionId;
        const data = await this._request({ action: 'get_union_id' }, 'union_id_response');
        this.unionId = data.union_id || '';
        this.union_id = this.unionId;
        this.points = Number(data.points || 0);
        return this.unionId;
    }

    async get_union_id() {
        return this.getUnionId();
    }

    async consumePoints(amount, options = {}) {
        const data = await this._request({
            action: 'points_consume',
            union_id: String(options.unionId || options.union_id || this.unionId || ''),
            amount: Number(amount || 0)
        }, 'auth_response');
        this.points = Number(data.points || 0);
        return this.points;
    }

    async addPoints(amount, options = {}) {
        const data = await this._request({
            action: 'points_add',
            union_id: String(options.unionId || options.union_id || this.unionId || ''),
            amount: Number(amount || 0)
        }, 'auth_response');
        this.points = Number(data.points || 0);
        return this.points;
    }

    async add_points(amount, options = {}) {
        return this.addPoints(amount, options);
    }

    async consume_points(amount, options = {}) {
        return this.consumePoints(amount, options);
    }

    async setAccessControl(config = {}) {
        const data = await this._request({ action: 'set_access_control', access_control: normalizeAccessControl(config) }, 'access_control_response');
        this.accessControl = data || {};
        this.access_control = this.accessControl;
        return this.accessControl;
    }

    async set_access_control(config = {}) {
        return this.setAccessControl(config);
    }

    async listPlatformAdmins(options = {}) {
        const data = await this._request({
            action: 'list_platform_admins',
            platform: String(options.platform || '')
        }, 'platform_admins_response');
        return Array.isArray(data) ? data : [];
    }

    async list_platform_admins(options = {}) {
        return this.listPlatformAdmins(options);
    }

    async getPlatformAdmins(options = {}) {
        return this.listPlatformAdmins(options);
    }

    async get_platform_admins(options = {}) {
        return this.listPlatformAdmins(options);
    }

    async setScheduledTask(options = {}) {
        return this._request({
            action: 'set_scheduled_task',
            task_key: String(options.taskKey || options.task_key || options.name || ''),
            name: String(options.name || options.taskKey || options.task_key || ''),
            description: String(options.description || ''),
            enabled: options.enabled !== false,
            pinned: Boolean(options.pinned),
            cron: Array.isArray(options.cron) ? options.cron.join('\n') : String(options.cron || ''),
            platform: String(options.platform || this.platform),
            adapter_id: this._adapterIdFor(options),
            user_id: String(options.userId || options.user_id || this.userId || ''),
            group_id: String(options.groupId || options.group_id || this.groupId || ''),
            content: String(options.content || options.text || ''),
            max_count: Number(options.maxCount || options.max_count || 0)
        }, 'scheduled_task_response');
    }

    async set_scheduled_task(options = {}) {
        return this.setScheduledTask(options);
    }

    
    async fakeMessage(options = {}) {
        await this._request({
            action: 'fake_message',
            platform: options.platform || this.platform,
            adapter_id: this._adapterIdFor(options),
            user_id: String(options.userId || options.user_id || this.userId || ''),
            group_id: String(options.groupId || options.group_id || ''),
            content: String(options.content || options.text || '')
        }, 'fake_message_response');
        return true;
    }

    
    async fake_message(platform, userId, groupId, content, adapterId = '') {
        if (typeof platform === 'object' && platform !== null) return this.fakeMessage(platform);
        return this.fakeMessage({ platform, adapterId, userId, groupId, content });
    }

    
    async runScript(options = {}) {
        return this._request({
            action: 'run_script',
            runtime: String(options.runtime || 'nodejs'),
            script: String(options.script || options.path || ''),
            cwd: String(options.cwd || ''),
            env: normalizeEnv(options.env || {}),
            timeout: Number(options.timeout || 300),
            wait: Boolean(options.wait),
            run_mode: String(options.runMode || options.run_mode || ''),
            union_id: String(options.unionId || options.union_id || this.unionId || '')
        }, 'script_response', Number(options.timeout || 300) + 10);
    }

    
    async runQLScript(options = {}) {
        const envName = String(options.envName || options.env_name || '').trim();
        if (!envName) throw new Error('envName 不能为空');
        const accounts = Array.isArray(options.accounts) ? options.accounts : [];
        const env = { ...(options.env || {}) };
        env[envName] = accounts.map(item => item.env_value || item.envValue || '').filter(Boolean).join('\n');
        return this.runScript({ ...options, env });
    }

    async run_script(options = {}) {
        return this.runScript(options);
    }

    async run_ql_script(options = {}) {
        return this.runQLScript(options);
    }

    _adapterIdFor(options = {}) {
        return String(options.adapterId || options.adapter_id || this.adapterId || '');
    }


    _request(action, expectedAction = 'db_response', timeoutSeconds = 30) {
        const requestId = `${Date.now()}_${++this._requestSeq}`;
        this._send({ ...action, request_id: requestId });

        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error('请求失败')), Math.max(1, timeoutSeconds) * 1000);
            this._rl.once('line', (line) => {
                clearTimeout(timer);
                try {
                    const response = JSON.parse(line);
                    if (response.action !== expectedAction) {
                        reject(new Error('response action mismatch'));
                        return;
                    }
                    if (response.request_id !== requestId) {
                        reject(new Error('response request id mismatch'));
                        return;
                    }
                    if (!response.success) {
                        const requestError = new Error(response.error || '请求失败');
                        requestError.data = response.data || {};
                        reject(requestError);
                        return;
                    }
                    resolve(response.data);
                } catch (error) {
                    reject(error);
                }
            });
        });
    }

    _send(action) {
        process.stdout.write(JSON.stringify(action) + '\n');
        return true;
    }
}

function normalizeAccessControl(config = {}) {
    const list = (value) => Array.isArray(value) ? value.map(String).filter(Boolean) : [];
    return {
        inherit_system: Boolean(config.inherit_system ?? config.inheritSystem),
        whitelist_groups: list(config.whitelist_groups || config.whitelistGroups),
        blocked_groups: list(config.blocked_groups || config.blockedGroups),
        whitelist_user_ids: list(config.whitelist_user_ids || config.whitelistUserIds),
        blocked_user_ids: list(config.blocked_user_ids || config.blockedUserIds)
    };
}

class Database {
    constructor(ctx) {
        this.ctx = ctx;
    }

    
    async createTable(table, columns = []) {
        return this.ctx._request({ action: 'db_create_table', table, db_columns: normalizeColumns(columns) });
    }

    
    async setView(table, options = {}) {
        const realTable = `plugin_${this.ctx.pluginId}_${table}`;
        return this.ctx.setDataView(realTable, {
            viewName: options.viewName || options.view_name || table,
            groupName: options.groupName || options.group_name || '请求失败',
            description: options.description || '',
            columns: options.columns || []
        });
    }

    
    async query(table, options = {}) {
        const order = normalizeQueryOrder(options);
        return this.ctx._request({
            action: 'db_query',
            table,
            query: {
                table,
                where: options.where || '',
                args: options.args || [],
                filters: normalizeQueryFilters(options.filters || options.filter),
                order,
                order_by: options.orderBy || options.order_by || '',
                order_dir: options.orderDir || options.order_dir || '',
                limit: options.limit || 0,
                page: options.page || 1,
                size: options.size || options.pageSize || 20
            }
        });
    }

    
    async first(table, options = {}) {
        const result = await this.query(table, { ...options, limit: 1, size: 1 });
        return result.rows && result.rows.length > 0 ? result.rows[0] : null;
    }

    
    async insert(table, values = {}) {
        return this.ctx._request({ action: 'db_insert', table, values });
    }

    
    async update(table, rowId, values = {}) {
        await this.ctx._request({ action: 'db_update', table, row_id: Number(rowId), values });
        return true;
    }

    
    async delete(table, rowId) {
        await this.ctx._request({ action: 'db_delete', table, row_id: Number(rowId) });
        return true;
    }

    
    async clear(table) {
        await this.ctx._request({ action: 'db_clear', table });
        return true;
    }
}

function normalizeEnv(env = {}) {
    const result = {};
    for (const [key, value] of Object.entries(env)) {
        if (!key) continue;
        result[String(key)] = String(value ?? '');
    }
    return result;
}

function normalizeColumns(columns) {
    return columns.map((column) => {
        if (typeof column === 'string') return { name: column, type: 'TEXT' };
        return { name: column.name, type: column.type || 'TEXT', default: column.default || '' };
    });
}

function normalizeQueryFilters(filters) {
    if (!filters) return [];
    if (!Array.isArray(filters)) filters = [filters];
    return filters
        .filter((filter) => filter && typeof filter === 'object')
        .map((filter) => ({
            field: String(filter.field || filter.column || ''),
            op: String(filter.op || filter.operator || '='),
            value: filter.value,
            values: Array.isArray(filter.values) ? filter.values : []
        }));
}

function normalizeQueryOrder(options = {}) {
    const order = options.order;
    if (order && typeof order === 'object') {
        return {
            field: String(order.field || order.column || order.orderBy || order.order_by || ''),
            direction: String(order.direction || order.dir || order.orderDir || order.order_dir || '')
        };
    }
    if (options.orderBy || options.order_by) {
        return {
            field: String(options.orderBy || options.order_by || ''),
            direction: String(options.orderDir || options.order_dir || '')
        };
    }
    return String(order || '');
}


class HTTPResponse {
    constructor() {
        this.statusCode = 200;
        this.headers = { 'Content-Type': 'application/json; charset=utf-8' };
        this.body = '';
        this.jsonData = undefined;
    }

    status(code) {
        this.statusCode = Number(code) || 200;
        return this;
    }

    setHeader(key, value) {
        if (key) this.headers[String(key)] = String(value ?? '');
        return this;
    }

    json(data, statusCode) {
        if (statusCode) this.status(statusCode);
        this.jsonData = data;
        this.setHeader('Content-Type', 'application/json; charset=utf-8');
        return this;
    }

    sendJson(data, statusCode) {
        return this.json(data, statusCode);
    }

    send(body, statusCode) {
        if (statusCode) this.status(statusCode);
        this.body = typeof body === 'string' ? body : JSON.stringify(body ?? '');
        return this;
    }

    toAction() {
        const action = { action: 'http_response', status: this.statusCode, headers: this.headers, body: this.body };
        if (this.jsonData !== undefined) action.json = this.jsonData;
        return action;
    }
}

async function runOpenAPIAction(handler, data, rl) {
    const ctx = new Context(data, rl);
    const req = data.request || {};
    req.query = flattenSingleValue(req.query || {});
    req.headers = flattenSingleValue(req.headers || {});
    req.body = req.json || req.form || req.body || {};
    const res = new HTTPResponse();
    if (handler.length >= 4) await handler(ctx, req, res, ctx);
    else await handler(ctx, req, res);
    return res;
}

function flattenSingleValue(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return value;
    const result = {};
    for (const [key, item] of Object.entries(value)) {
        result[key] = Array.isArray(item) && item.length === 1 ? item[0] : item;
    }
    return result;
}

function runOpenAPI(handler) {
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
    let firstLine = true;

    rl.on('line', async (line) => {
        if (!firstLine) return;
        firstLine = false;

        try {
            const data = JSON.parse(line);
            const res = await runOpenAPIAction(handler, data, rl);
            process.stdout.write(JSON.stringify(res.toAction()) + '\n');
            process.exit(0);
        } catch (error) {
            process.stdout.write(JSON.stringify({ action: 'http_response', status: 500, headers: { 'Content-Type': 'application/json; charset=utf-8' }, json: { error: error.message } }) + '\n');
            process.exit(1);
        }
    });
}

function runDirect(handler) {
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
    let firstLine = true;

    rl.on('line', async (line) => {
        if (!firstLine) return;
        firstLine = false;

        try {
            const messageData = JSON.parse(line);
            const ctx = new Context(messageData, rl);
            await handler(ctx);
            process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
            process.exit(0);
        } catch (error) {
            process.stdout.write(JSON.stringify({ action: 'done', success: false, error: error.message }) + '\n');
            process.exit(1);
        }
    });
}

async function runAutoOpenAPI(entryPath) {
    const path = require('path');
    const fullPath = path.resolve(process.cwd(), entryPath);
    const pluginModule = require(fullPath);
    const handler = pluginModule.action || pluginModule.default || pluginModule;
    if (typeof handler !== 'function') throw new Error('Open API 插件必须导出 action(ctx, req, res) 函数');
    runOpenAPI(handler);
}

if (require.main === module && process.argv[2] === 'openapi') {
    runAutoOpenAPI(process.argv[3]).catch((error) => {
        process.stdout.write(JSON.stringify({ action: 'http_response', status: 500, headers: { 'Content-Type': 'application/json; charset=utf-8' }, json: { error: error.message } }) + '\n');
        process.exit(1);
    });
}

module.exports = { Context, Database, HTTPResponse, runDirect, runOpenAPI };
