const d=["nodejs_account_ql","python_account_ql"];function A(e){return d.includes(String(e||""))}function _(e,t="",r="nodejs"){const n=String(e||"").trim().toLowerCase();if(n==="node"||n==="js"||n==="javascript")return"nodejs";if(n==="nodejs"||n==="python")return n;const s=String(t||"").trim().toLowerCase().split("?")[0];return s.endsWith(".py")?"python":s.endsWith(".js")||s.endsWith(".mjs")||s.endsWith(".cjs")?"nodejs":r==="python"?"python":"nodejs"}function y(e="nodejs"){return e==="python"?`def parse_input(raw, ctx):
    value = str(raw or '').strip()
    if not value:
        raise RuntimeError('账号 CK 不能为空')
    return {
        "env_value": value,
        "unique_key": value,
        "display_name": value[:8],
    }`:`function parseInput(raw, ctx) {
  const value = String(raw || '').trim();
  if (!value) throw new Error('账号 CK 不能为空');
  return {
    envValue: value,
    uniqueKey: value,
    displayName: value.slice(0, 8)
  };
}`}function h(e="nodejs"){return e==="python"?`async def query(account, ctx, index):
    return {
        "状态": account.get("status") or "active",
    }`:"async function query(account, ctx, index) {\n  return `${index + 1}. ${account.account_name}｜${account.status || 'active'}`;\n}"}function m(e="nodejs"){return e==="python"?`async def after_run(ctx, accounts, result, helpers):
    if result.get("status") != "success":
        return
    # 示例：一键运行成功后给触发会话推送消息，可按业务条件改为 ctx.send_message 或 ctx.push
    await ctx.reply(f"一键运行完成，账号数：{len(accounts)}")`:`async function afterRun(ctx, accounts, result, helpers) {
  if (result?.status !== 'success') return;
  // 示例：一键运行成功后给触发会话推送消息，可按业务条件改为 ctx.sendMessage 或 ctx.push
  await ctx.reply(\`一键运行完成，账号数：\${accounts.length}\`);
}`}function g(e="nodejs"){return e==="python"?`async def check_ck(account, ctx):
    return {
        "valid": True,
        "reason": "",
    }`:`async function checkCk(account, ctx) {
  return {
    valid: true,
    reason: ''
  };
}`}function f(e=0,t="nodejs"){return t==="python"?`custom_route_${e+1}`:`customRoute${e+1}`}function v(e="",t="nodejs"){const r=e||f(0,t);return t==="python"?`async def ${r}(ctx, helpers):
    accounts = await helpers.list_mine({"status": "active"})
    await ctx.reply(f"账号数：{len(accounts)}")`:`async function ${r}(ctx, helpers) {
  const accounts = await helpers.listMine({ status: 'active' });
  return ctx.reply(\`账号数：\${accounts.length}\`);
}`}function b(e="nodejs"){const t=e==="python"?"python":"nodejs";return{prefix:"",table_name:"",env_name:"",task_script:`scripts/task.${t==="python"?"py":"js"}`,script_runtime:t,auth_price_per_month:0,cron:"0 8 * * *",wait_scheduled:!0,enable_after_run:!1,after_run_code:m(t),enable_ck_check:!0,ck_check_cron:"25 9 * * *",check_ck_code:g(t),enable_expire_check:!1,expire_check_cron:"15 9 * * *",expire_notify_days:"7,3,1,0",expire_delete_after_days:-1,run_wait_timeout:7200,parse_input_code:y(t),query_code:h(t),routes:[]}}function k(e={},t="nodejs"){const r=b(t),n=e&&typeof e=="object"?e:{},s=t==="python"?"python":"nodejs";return{...r,...n,script_runtime:_(n.script_runtime,n.task_script,s),auth_price_per_month:Math.max(0,Number(n.auth_price_per_month??r.auth_price_per_month)),run_wait_timeout:Math.max(1,Number(n.run_wait_timeout??r.run_wait_timeout)),wait_scheduled:n.wait_scheduled!==!1,enable_after_run:!!n.enable_after_run,enable_ck_check:n.enable_ck_check!==!1,enable_expire_check:!!n.enable_expire_check,expire_delete_after_days:Number(n.expire_delete_after_days??r.expire_delete_after_days),routes:Array.isArray(n.routes)?n.routes.map((a,o)=>({id:a.id||`${Date.now()}_${o}`,command:String(a.command||""),function_name:String(a.function_name||f(o,s)),description:String(a.description||""),code:String(a.code||"")})):[]}}function x(e={}){const t=["登录","账号","管理","查询","运行","一键运行","签到","删除","授权","帮助"];e.enable_ck_check&&t.push("CK检测"),e.enable_expire_check&&t.push("过期检测");for(const r of e.routes||[]){const n=String(r.command||"").trim();n&&t.push(n)}return t}function p(e){return String(e||"").replace(/[.*+?^${}()|[\]\\]/g,"\\$&")}function w(e={}){const t=String(e.prefix||"前缀").trim()||"前缀";return`^(${p(t)})(${x(e).map(p).join("|")})$`}const j={parse:"输入解析",parser:"输入解析",parse_input:"输入解析",login:"登录",query:"查询",ck_check:"CK 检测",ck:"CK 检测",check_ck:"CK 检测",auth:"授权",authorization:"授权",ql_registration:"青龙任务注册",registration:"青龙任务注册",ql_env:"青龙环境变量",env:"青龙环境变量",schedules:"定时任务",schedule:"定时任务",cron:"定时任务",routes:"自定义路由",route:"自定义路由",custom_route:"自定义路由",custom_routes:"自定义路由",after_run:"运行完成处理",afterrun:"运行完成处理",other:"其他"},l=["name","version","runtime_profile","priority","platforms","enabled","script_env"];function C(e={}){return Number(e==null?void 0:e.version)===2&&String((e==null?void 0:e.mode)||"").toLowerCase()==="hybrid"}function i(e){if(e!==void 0)return JSON.parse(JSON.stringify(e))}function S(e={},t=""){const r=e&&typeof e=="object"?i(e):{};return r.version=2,r.mode="hybrid",r.plugin=r.plugin&&typeof r.plugin=="object"?r.plugin:{},r.plugin.id=String(t||r.plugin.id||""),r.plugin.runtime=String(r.plugin.runtime||"").trim(),r.plugin.template=String(r.plugin.template||r.template||"").trim(),r.files=Array.isArray(r.files)?r.files:[],r.sections=Array.isArray(r.sections)?r.sections.map(n=>!n||typeof n!="object"||Array.isArray(n)?n:{...n,content:String(n.content??"")}):[],r}function $(e={}){return String((e==null?void 0:e.ownership)||"").trim().toLowerCase()!=="patchable"}function L(e){const t=String(e||"").trim().toLowerCase();return t==="referenced"?"引用（只读）":t==="preserved"?"保留（只读）":t==="generated"?"生成配置（只读）":t==="patchable"?"可映射编辑":t==="owned"?"模板管理":t||"未标注"}function N(e){const t=String(e||"").trim();return j[t.toLowerCase()]||t||"其他"}function O(e={},t={}){const r=Array.isArray(e.sections)?e.sections:[],n=Array.isArray(t.sections)?t.sections:[];return r.flatMap((s,a)=>{const o=n[a];return!o||String((s==null?void 0:s.content)??"")===String((o==null?void 0:o.content)??"")?[]:[{id:String((s==null?void 0:s.id)||a),label:o.label||(s==null?void 0:s.label)||(s==null?void 0:s.id)||`Section ${a+1}`,category:o.category||(s==null?void 0:s.category)||"other",path:o.path||(s==null?void 0:s.path)||""}]})}function T(e={},t={}){const r=e!=null&&e.plugin&&typeof e.plugin=="object"?e.plugin:{},n=t!=null&&t.plugin&&typeof t.plugin=="object"?t.plugin:{};return l.filter(s=>JSON.stringify(r[s])!==JSON.stringify(n[s])).map(s=>({key:s,initial:i(r[s]),current:i(n[s])}))}function E(e={},t={}){const r=Array.isArray(e.files)?e.files:[],n=Array.isArray(t.files)?t.files:[],s=Math.max(r.length,n.length),a=[];for(let o=0;o<s;o+=1){if(JSON.stringify(r[o])===JSON.stringify(n[o]))continue;const c=r[o],u=n[o];a.push({path:String((u==null?void 0:u.path)||(c==null?void 0:c.path)||""),initial:c,current:u})}return a}function R(e={},t={},r=[]){const n=i(e&&typeof e=="object"?e:{})||{},s=n.plugin&&typeof n.plugin=="object"&&!Array.isArray(n.plugin)?n.plugin:{};n.plugin={...s};for(const a of l)Object.prototype.hasOwnProperty.call(t,a)&&(n.plugin[a]=i(t[a]));return Array.isArray(n.sections)&&(n.sections=n.sections.map((a,o)=>{if(!a||typeof a!="object"||Array.isArray(a)||!Object.prototype.hasOwnProperty.call(a,"content"))return a;const c=Array.isArray(r)?r[o]:void 0;if(!c||typeof c!="object"||Array.isArray(c))return a;const u=String(c.content??"");return u===String(a.content??"")?a:{...a,content:u}})),n}export{f as a,y as b,b as c,v as d,h as e,m as f,g,w as h,A as i,C as j,S as k,i as l,k as m,_ as n,N as o,$ as p,L as q,R as r,T as s,O as t,E as u};
