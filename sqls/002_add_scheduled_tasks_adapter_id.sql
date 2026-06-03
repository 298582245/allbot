-- 定时任务增加机器人实例 ID，用于多机器人场景精确路由
ALTER TABLE scheduled_tasks ADD COLUMN adapter_id TEXT NOT NULL DEFAULT '';
