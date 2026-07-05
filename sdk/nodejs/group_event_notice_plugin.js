const { runDirect } = require('./allbot_direct');

const eventReplies = {
  GROUP_MEMBER_ADD: (ctx) => `欢迎新成员加入群聊：${displayMember(ctx)}`,
  GROUP_MEMBER_REMOVE: (ctx) => `成员已退出群聊：${displayMember(ctx)}`,
  GROUP_ADD_ROBOT: () => '机器人已加入本群，群事件通知插件已启用。'
};

runDirect(async (ctx) => {
  const eventName = ctx.event?.name || ctx.meta('event_name') || ctx.content;
  const buildReply = eventReplies[eventName];
  if (!buildReply) return;
  await ctx.reply(buildReply(ctx));
});

function displayMember(ctx) {
  return ctx.event?.memberOpenid || ctx.event?.member_openid || ctx.userId || '未知成员';
}
