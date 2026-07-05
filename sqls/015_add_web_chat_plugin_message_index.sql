CREATE INDEX IF NOT EXISTS idx_web_chat_messages_user_plugin_message
ON web_chat_messages(user_id, plugin_id, message_id);
