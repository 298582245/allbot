/**
 * AllBot Node.js SDK - HTTP Server Mode (兼容旧版)
 */

class Context {
    constructor(platform, user_id, group_id, content, message_id, plugin_id) {
        this.platform = platform;
        this.user_id = user_id;
        this.group_id = group_id;
        this.content = content;
        this.message_id = message_id;
        this.plugin_id = plugin_id;
    }

    async reply(text) {
        console.log(`[Reply] ${text}`);
        return true;
    }

    async send_image(image_url) {
        console.log(`[SendImage] ${image_url}`);
        return true;
    }

    async listen(timeout = 60) {
        // TODO: 实现连续对话
        return '';
    }
}

function startPlugin() {
    console.log('HTTP Server mode not implemented yet. Use --mode=direct instead.');
    process.exit(1);
}

module.exports = { Context, startPlugin };
