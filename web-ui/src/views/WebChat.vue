<template>
  <div class="web-chat-page">
    <section v-if="!session" class="auth-card">
      <div class="brand">
        <h1>AllBot Web 聊天室</h1>
        <p>使用网页账号进入固定私聊和插件独立对话，也可以绑定已有平台身份。</p>
      </div>
      <el-tabs v-model="authMode" stretch>
        <el-tab-pane label="登录" name="login">
          <el-segmented v-model="loginMode" :options="loginModeOptions" class="login-mode-tabs" />
          <el-form v-if="loginMode === 'password'" label-position="top" @submit.prevent>
            <el-form-item label="账号或邮箱">
              <el-input v-model="loginForm.login" placeholder="请输入账号或邮箱" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="loginForm.password" type="password" show-password placeholder="请输入密码" @keyup.enter="handleLogin" />
            </el-form-item>
            <el-button type="primary" :loading="loading" class="full-button" @click="handleLogin">登录</el-button>
          </el-form>
          <el-form v-else-if="loginMode === 'email'" label-position="top" @submit.prevent>
            <el-form-item label="邮箱">
              <div class="inline-row">
                <el-input v-model="emailLoginForm.email" placeholder="请输入注册邮箱" />
                <el-button :loading="emailLoginCodeLoading" @click="sendEmailLoginCode">发送验证码</el-button>
              </div>
            </el-form-item>
            <el-form-item label="验证码">
              <el-input v-model="emailLoginForm.code" placeholder="6 位数字验证码" @keyup.enter="handleEmailLogin" />
            </el-form-item>
            <el-button type="primary" :loading="loading" class="full-button" @click="handleEmailLogin">验证码登录</el-button>
          </el-form>
          <el-form v-else label-position="top" @submit.prevent>
            <el-form-item label="Web 用户名">
              <div class="inline-row">
                <el-input v-model="platformLoginForm.username" placeholder="请输入 Web 用户名" />
                <el-button :loading="platformLoginCodeLoading" @click="sendPlatformLoginCode">获取验证码</el-button>
              </div>
            </el-form-item>
            <el-form-item label="接收平台">
              <el-select v-model="platformLoginForm.adapter_id" class="full-button" placeholder="请选择正在运行的平台" @change="handlePlatformChange">
                <el-option v-for="item in platformLoginPlatforms" :key="item.id" :label="platformOptionLabel(item)" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="验证码">
              <el-input v-model="platformLoginForm.code" placeholder="6 位数字验证码" @keyup.enter="handlePlatformLogin" />
            </el-form-item>
            <el-button type="primary" :loading="loading" class="full-button" @click="handlePlatformLogin">平台验证码登录</el-button>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="注册" name="register">
          <el-form label-position="top" @submit.prevent>
            <el-form-item label="邮箱">
              <div class="inline-row">
                <el-input v-model="registerForm.email" placeholder="用于接收验证码" />
                <el-button :loading="codeLoading" @click="sendCode">发送验证码</el-button>
              </div>
            </el-form-item>
            <el-form-item label="验证码">
              <el-input v-model="registerForm.code" placeholder="6 位数字验证码" />
            </el-form-item>
            <el-form-item label="账号">
              <el-input v-model="registerForm.username" placeholder="3-32 位字母、数字、下划线" />
            </el-form-item>
            <el-form-item label="昵称">
              <el-input v-model="registerForm.display_name" placeholder="可选" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="registerForm.password" type="password" show-password placeholder="至少 8 位" />
            </el-form-item>
            <el-form-item label="绑定码（可选）">
              <el-input v-model="registerForm.bind_code" placeholder="已有平台私聊机器人获取绑定码后填写" />
            </el-form-item>
            <el-button type="primary" :loading="loading" class="full-button" @click="handleRegister">注册并进入聊天室</el-button>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="找回密码" name="reset">
          <el-form label-position="top" @submit.prevent>
            <el-form-item label="邮箱">
              <div class="inline-row">
                <el-input v-model="resetForm.email" placeholder="请输入注册邮箱" />
                <el-button :loading="resetCodeLoading" @click="sendResetCode">发送验证码</el-button>
              </div>
            </el-form-item>
            <el-form-item label="验证码">
              <el-input v-model="resetForm.code" placeholder="6 位数字验证码" />
            </el-form-item>
            <el-form-item label="新密码">
              <el-input v-model="resetForm.password" type="password" show-password placeholder="至少 8 位" />
            </el-form-item>
            <el-form-item label="确认新密码">
              <el-input v-model="resetForm.confirmPassword" type="password" show-password placeholder="请再次输入新密码" @keyup.enter="handleResetPassword" />
            </el-form-item>
            <el-button type="primary" :loading="loading" class="full-button" @click="handleResetPassword">重置密码</el-button>
          </el-form>
        </el-tab-pane>
      </el-tabs>
      <nav class="auth-bottom-tabs">
        <button :class="{ active: authMode === 'login' }" type="button" @click="authMode = 'login'">
          <el-icon><User /></el-icon>
          <span>登录</span>
        </button>
        <button :class="{ active: authMode === 'register' }" type="button" @click="authMode = 'register'">
          <el-icon><EditPen /></el-icon>
          <span>注册</span>
        </button>
        <button :class="{ active: authMode === 'reset' }" type="button" @click="authMode = 'reset'">
          <el-icon><Lock /></el-icon>
          <span>找回密码</span>
        </button>
      </nav>
    </section>

    <section v-else class="chat-shell">
      <aside v-if="!isMobile" class="plugin-panel desktop-panel">
        <div class="panel-title">会话列表</div>
        <button class="plugin-item private-item" :class="{ active: activeSessionId === privateSessionId }" type="button" @click="selectSession(privateSessionId)">
          <div class="plugin-row">
            <strong>固定私聊</strong>
            <span v-if="unreadMap[privateSessionId]" class="unread">{{ unreadMap[privateSessionId] }}</span>
          </div>
          <span class="plugin-description">接收机器人主动消息，也可以发送内置函数</span>
          <em class="message-count">{{ messageCountMap[privateSessionId] || 0 }} 条消息</em>
        </button>
        <el-input v-model="pluginKeyword" clearable placeholder="搜索插件、关键词或快捷指令" class="plugin-search" />
        <el-empty v-if="filteredPlugins.length === 0" description="暂无匹配插件" :image-size="80" />
        <div
          v-for="item in filteredPlugins"
          :key="item.id"
          class="plugin-item"
          :class="{ active: item.id === activeSessionId }"
          role="button"
          tabindex="0"
          @click="selectSession(item.id)"
          @keydown.enter.prevent="selectSession(item.id)"
          @keydown.space.prevent="selectSession(item.id)"
        >
          <div class="plugin-row">
            <strong>{{ item.title || item.name || item.id }}</strong>
            <span v-if="unreadMap[item.id]" class="unread">{{ unreadMap[item.id] }}</span>
          </div>
          <span class="plugin-description" :class="{ expanded: expandedPluginDescriptions.has(item.id) }">{{ item.description || '点击进入插件对话' }}</span>
          <button
            v-if="item.description"
            class="description-toggle"
            type="button"
            :aria-expanded="expandedPluginDescriptions.has(item.id)"
            @click.stop="togglePluginDescription(item.id)"
          >{{ expandedPluginDescriptions.has(item.id) ? '收起描述' : '展开描述' }}</button>
          <em class="message-count">{{ messageCountMap[item.id] || 0 }} 条消息</em>
          <div v-if="item.quick_actions?.length" class="quick-preview">
            <em v-for="action in item.quick_actions.slice(0, 3)" :key="action.label + action.text">{{ action.label }}</em>
          </div>
        </div>
      </aside>

      <main v-if="isMobile && mobileView === 'sessions'" class="plugin-panel mobile-session-panel">
        <header class="mobile-session-header">
          <h2>会话列表</h2>
          <el-dropdown trigger="click" @command="handleMobileMenuCommand">
            <button class="mobile-menu-button" type="button" aria-label="打开会话菜单">
              <el-icon><MoreFilled /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="bind">绑定</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </header>
        <el-input v-model="pluginKeyword" clearable placeholder="搜索插件、关键词或快捷指令" class="plugin-search mobile-plugin-search" />
        <button class="plugin-item private-item" :class="{ active: activeSessionId === privateSessionId }" type="button" @click="selectSession(privateSessionId)">
          <div class="plugin-row">
            <strong>固定私聊</strong>
            <span v-if="unreadMap[privateSessionId]" class="unread">{{ unreadMap[privateSessionId] }}</span>
          </div>
          <span class="plugin-description">接收机器人主动消息，也可以发送内置函数</span>
          <em class="message-count">{{ messageCountMap[privateSessionId] || 0 }} 条消息</em>
        </button>
        <el-empty v-if="filteredPlugins.length === 0" description="暂无匹配插件" :image-size="80" />
        <div
          v-for="item in filteredPlugins"
          :key="item.id"
          class="plugin-item"
          :class="{ active: item.id === activeSessionId }"
          role="button"
          tabindex="0"
          @click="selectSession(item.id)"
          @keydown.enter.prevent="selectSession(item.id)"
          @keydown.space.prevent="selectSession(item.id)"
        >
          <div class="plugin-row">
            <strong>{{ item.title || item.name || item.id }}</strong>
            <span v-if="unreadMap[item.id]" class="unread">{{ unreadMap[item.id] }}</span>
          </div>
          <span class="plugin-description" :class="{ expanded: expandedPluginDescriptions.has(item.id) }">{{ item.description || '点击进入插件对话' }}</span>
          <button
            v-if="item.description"
            class="description-toggle"
            type="button"
            :aria-expanded="expandedPluginDescriptions.has(item.id)"
            @click.stop="togglePluginDescription(item.id)"
          >{{ expandedPluginDescriptions.has(item.id) ? '收起描述' : '展开描述' }}</button>
          <em class="message-count">{{ messageCountMap[item.id] || 0 }} 条消息</em>
          <div v-if="item.quick_actions?.length" class="quick-preview">
            <em v-for="action in item.quick_actions.slice(0, 3)" :key="action.label + action.text">{{ action.label }}</em>
          </div>
        </div>
      </main>

      <main v-show="!isMobile || mobileView === 'chat'" class="chat-main">
        <header class="chat-header">
          <div class="chat-header-main">
            <button v-if="isMobile" class="mobile-back-button" type="button" aria-label="返回会话列表" @click="showMobileSessions">
              <el-icon><ArrowLeft /></el-icon>
            </button>
            <div class="chat-header-text">
              <h2>{{ activeSessionTitle }}</h2>
              <p>{{ activeSessionDescription }}</p>
            </div>
          </div>
          <div class="header-actions desktop-header-actions">
            <el-button @click="settingsDrawer = true">绑定</el-button>
            <el-button @click="handleLogout">退出</el-button>
          </div>
        </header>

        <div ref="messageListRef" class="message-list" @click="handleMessageListClick">
          <el-empty v-if="sessionLoading" description="正在加载会话" />
          <el-empty v-else-if="messages.length === 0" :description="activeSessionId === privateSessionId ? '开始固定私聊吧' : '开始当前插件对话吧'" />
          <article v-for="msg in sessionLoading ? [] : messages" :key="msg.message_id" class="message" :class="msg.direction">
            <div class="bubble">
              <div class="meta">{{ msg.direction === 'in' ? '我' : '机器人' }} · {{ formatTime(msg.created_at) }}</div>
              <div v-if="msg.message_type === 'markdown'" class="markdown-block" v-html="renderMarkdown(msg.content)"></div>
              <el-image v-else-if="msg.message_type === 'image' && safeImageURL(msg.image_url)" :src="msg.image_url" :preview-src-list="[msg.image_url]" preview-teleported fit="contain" class="chat-image" @load="scrollBottom" />
              <div v-else-if="msg.message_type === 'rich'" class="rich-message">
                <template v-if="parseRich(msg.rich_json).length > 0">
                  <template v-for="(part, index) in parseRich(msg.rich_json)" :key="index">
                    <div v-if="part.type === 'markdown'" class="markdown-block" v-html="renderMarkdown(part.markdown)"></div>
                    <el-image v-else-if="part.type === 'image' && safeImageURL(part.url)" :src="part.url" :preview-src-list="[part.url]" preview-teleported fit="contain" class="chat-image" @load="scrollBottom" />
                    <div v-else-if="part.type === 'plugin_card' && part.plugin" class="plugin-switch-card" @click="switchToPlugin(part.action?.plugin_id || part.plugin.id)">
                      <div class="plugin-switch-card__label">建议切换到</div>
                      <div class="plugin-switch-card__title">{{ part.plugin.title || part.plugin.name || part.plugin.id }}</div>
                      <div
                        class="plugin-switch-card__desc"
                        :class="{ expanded: expandedSwitchDescriptions.has(pluginSwitchDescriptionKey(msg, part, index)) }"
                        role="button"
                        tabindex="0"
                        :aria-expanded="expandedSwitchDescriptions.has(pluginSwitchDescriptionKey(msg, part, index))"
                        @click.stop="toggleSwitchDescription(pluginSwitchDescriptionKey(msg, part, index))"
                        @keydown.enter.stop.prevent="toggleSwitchDescription(pluginSwitchDescriptionKey(msg, part, index))"
                        @keydown.space.stop.prevent="toggleSwitchDescription(pluginSwitchDescriptionKey(msg, part, index))"
                      >{{ part.plugin.description || '点击进入插件对话' }}</div>
                      <div v-if="part.plugin.quick_actions?.length" class="plugin-switch-card__actions">
                        <span v-for="action in part.plugin.quick_actions.slice(0, 3)" :key="action.label + action.text">{{ action.label }}</span>
                      </div>
                      <el-button size="small" type="primary" @click.stop="switchToPlugin(part.action?.plugin_id || part.plugin.id)">{{ part.action?.label || '切换到此插件' }}</el-button>
                    </div>
                    <p v-else>{{ part.text || part.markdown || part.url }}</p>
                  </template>
                </template>
                <p v-else>{{ msg.content }}</p>
              </div>
              <div v-else-if="msg.message_type === 'buttons'" class="button-message">
                <p v-if="msg.content">{{ msg.content }}</p>
                <div v-for="(row, rowIndex) in parseButtons(msg.rich_json)" :key="rowIndex" class="button-row">
                  <el-button v-for="button in row" :key="button.value || button.text" size="small" @click="sendQuick(button.value || button.text)">{{ button.text }}</el-button>
                </div>
                <p v-if="!msg.content && parseButtons(msg.rich_json).length === 0">{{ msg.content || msg.rich_json }}</p>
              </div>
              <p v-else>{{ msg.content || msg.image_url }}</p>
            </div>
          </article>
        </div>

        <footer class="composer">
          <div v-if="activeQuickActions.length" class="quick-actions-shell composer-quick-actions">
            <button v-show="showQuickScrollButtons" class="quick-scroll-button" type="button" aria-label="向左滚动快捷指令" :disabled="quickScrollAtStart" @click="scrollQuickActions(-1)">
              <el-icon><ArrowLeft /></el-icon>
            </button>
            <div ref="quickActionsScrollRef" class="quick-actions-scroll" @scroll="updateQuickScrollState">
              <div ref="quickActionsListRef" class="quick-actions-list">
                <el-button v-for="action in activeQuickActions" :key="action.label + action.text" size="small" @click="sendQuick(action.text)">{{ action.label }}</el-button>
              </div>
            </div>
            <button v-show="showQuickScrollButtons" class="quick-scroll-button" type="button" aria-label="向右滚动快捷指令" :disabled="quickScrollAtEnd" @click="scrollQuickActions(1)">
              <el-icon><ArrowRight /></el-icon>
            </button>
          </div>
          <div class="composer-row">
            <el-input v-model="content" type="textarea" :rows="3" :placeholder="composerPlaceholder" @keydown.ctrl.enter.prevent="sendMessage" />
            <el-button type="primary" :loading="sending" class="send-button" @click="sendMessage">发送</el-button>
          </div>
        </footer>
      </main>
    </section>

    <el-drawer v-model="settingsDrawer" title="绑定已有平台身份" direction="rtl" size="360px">
      <p class="bind-tip">在钉钉、飞书、Telegram 等平台私聊机器人发送“绑定码”，再把验证码填到这里。</p>
      <el-input v-model="bindCode" placeholder="请输入绑定码" />
      <el-button type="primary" class="full-button bind-button" @click="bindCodeSubmit">绑定</el-button>
    </el-drawer>

    <el-dialog v-model="codeDialogVisible" :title="codeDialogTitle" width="78vw" class="code-dialog">
      <pre class="expanded-code"><code>{{ expandedCodeContent }}</code></pre>
      <template #footer>
        <el-button @click="copyExpandedCode">复制全部</el-button>
        <el-button type="primary" @click="codeDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowLeft, ArrowRight, EditPen, Lock, MoreFilled, User } from '@element-plus/icons-vue'
import {
  bindWebChatCode,
  getWebChatMe,
  getWebChatMessageCounts,
  getWebChatMessages,
  getWebChatPlatforms,
  getWebChatPlugins,
  loginWebChat,
  loginWebChatByEmailCode,
  loginWebChatByPlatformCode,
  logoutWebChat,
  markWebChatRead,
  registerWebChat,
  resetWebChatPassword,
  sendWebChatEmailCode,
  sendWebChatMessage,
  sendWebChatPlatformCode
} from '@/api'

const privateSessionId = '__private__'
const privateQuickActions = [
  { label: 'myid', text: 'myid' },
  { label: '积分充值', text: '积分充值' },
  { label: '我的平台', text: '我的平台' },
  { label: '绑定码', text: '绑定码' },
  { label: 'version', text: 'version' }
]

const authMode = ref('login')
const loginMode = ref('password')
const loginModeOptions = [{ label: '密码登录', value: 'password' }, { label: '邮箱验证码', value: 'email' }, { label: '平台验证码', value: 'platform' }]
const loading = ref(false)
const codeLoading = ref(false)
const resetCodeLoading = ref(false)
const emailLoginCodeLoading = ref(false)
const platformLoginCodeLoading = ref(false)
const sending = ref(false)
const sessionLoading = ref(false)
const session = ref(null)
const csrfToken = ref('')
const messages = ref([])
const plugins = ref([])
const platformLoginPlatforms = ref([])
const activeSessionId = ref(privateSessionId)
const loadedSessionId = ref('')
const isMobile = ref(false)
const mobileView = ref('sessions')
const pluginKeyword = ref('')
const unreadMap = reactive({})
const messageCountMap = reactive({ [privateSessionId]: 0 })
const lastMessageIdMap = reactive({ [privateSessionId]: 0 })
const sortedPluginIds = ref([])
const content = ref('')
const bindCode = ref('')
const settingsDrawer = ref(false)
const codeDialogVisible = ref(false)
const expandedCodeContent = ref('')
const expandedCodeLang = ref('')
const messageListRef = ref(null)
const quickActionsScrollRef = ref(null)
const quickActionsListRef = ref(null)
const showQuickScrollButtons = ref(false)
const quickScrollAtStart = ref(true)
const quickScrollAtEnd = ref(false)
const expandedPluginDescriptions = reactive(new Set())
const expandedSwitchDescriptions = reactive(new Set())
let eventSource = null
let mobileMediaQuery = null
let quickActionsResizeObserver = null
let quickActionsViewportWidth = 0
let initializationVersion = 0
let sessionRequestVersion = 0
let readRequestVersion = 0

const loginForm = reactive({ login: '', password: '' })
const emailLoginForm = reactive({ email: '', code: '' })
const platformLoginForm = reactive({ adapter_id: '', platform: '', username: '', code: '' })
const registerForm = reactive({ email: '', code: '', username: '', display_name: '', password: '', bind_code: '' })
const resetForm = reactive({ email: '', code: '', password: '', confirmPassword: '' })

const activePlugin = computed(() => activeSessionId.value === privateSessionId ? null : plugins.value.find((item) => item.id === activeSessionId.value))
const activeQuickActions = computed(() => activeSessionId.value === privateSessionId ? privateQuickActions : (activePlugin.value?.quick_actions || []))
const activeSessionTitle = computed(() => activePlugin.value?.title || activePlugin.value?.name || (activeSessionId.value === privateSessionId ? '固定私聊' : '请选择会话'))
const activeSessionDescription = computed(() => activePlugin.value?.description || (activeSessionId.value === privateSessionId ? '接收机器人主动消息，可使用 myid、积分充值、我的平台、绑定码、version 等内置函数' : `${session.value?.user?.email || ''} · ${session.value?.user?.union_id || ''}`))
const composerPlaceholder = computed(() => activePlugin.value?.placeholder || '输入内置函数或普通文本，按 Ctrl+Enter 发送')
const codeDialogTitle = computed(() => expandedCodeLang.value ? `代码块 · ${expandedCodeLang.value}` : '代码块')
const filteredPlugins = computed(() => {
  const keyword = pluginKeyword.value.trim().toLowerCase()
  const orderMap = new Map(sortedPluginIds.value.map((id, index) => [id, index]))
  return plugins.value
    .filter((item) => !keyword || pluginSearchText(item).includes(keyword))
    .slice()
    .sort((a, b) => {
      const orderDiff = (orderMap.get(a.id) ?? Number.MAX_SAFE_INTEGER) - (orderMap.get(b.id) ?? Number.MAX_SAFE_INTEGER)
      if (orderDiff !== 0) return orderDiff
      return sessionTitle(a).localeCompare(sessionTitle(b), 'zh-Hans-CN')
    })
})

onMounted(async () => {
  mobileMediaQuery = window.matchMedia('(max-width: 820px)')
  isMobile.value = mobileMediaQuery.matches
  mobileMediaQuery.addEventListener('change', handleBreakpointChange)
  quickActionsResizeObserver = new ResizeObserver(handleQuickActionsResize)
  try {
    const data = await getWebChatMe()
    setSession(data)
  } catch {
    session.value = null
    await loadPlatformLoginPlatforms()
  }
})

onBeforeUnmount(() => {
  initializationVersion += 1
  sessionRequestVersion += 1
  readRequestVersion += 1
  closeEvents()
  mobileMediaQuery?.removeEventListener('change', handleBreakpointChange)
  quickActionsResizeObserver?.disconnect()
})

watch(() => [activeSessionId.value, activeQuickActions.value.map((item) => `${item.label}:${item.text}`).join('|'), mobileView.value], resetQuickActionsScroll)

function setSession(data) {
  const currentInitializationVersion = ++initializationVersion
  sessionRequestVersion += 1
  readRequestVersion += 1
  closeEvents()
  session.value = data
  csrfToken.value = data.csrf_token
  sending.value = false
  sessionLoading.value = false
  activeSessionId.value = privateSessionId
  loadedSessionId.value = ''
  messages.value = []
  expandedPluginDescriptions.clear()
  expandedSwitchDescriptions.clear()
  mobileView.value = isMobile.value ? 'sessions' : 'chat'
  initializeSession(currentInitializationVersion)
}

async function initializeSession(currentInitializationVersion) {
  const isCurrentSession = () => Boolean(session.value) && initializationVersion === currentInitializationVersion
  try {
    const loadedPlugins = await getWebChatPlugins()
    if (!isCurrentSession()) return
    plugins.value = Array.isArray(loadedPlugins) ? loadedPlugins : []
    refreshPluginOrder()
  } catch (error) {
    if (!isCurrentSession()) return
    plugins.value = []
    refreshPluginOrder()
    ElMessage.error(error.message || '加载插件列表失败')
  }

  try {
    const counts = await getWebChatMessageCounts()
    if (!isCurrentSession()) return
    for (const item of Array.isArray(counts) ? counts : []) updateSessionStats(item)
    refreshPluginOrder()
  } catch (error) {
    if (isCurrentSession()) ElMessage.error(error.message || '加载消息统计失败')
  }

  if (!isCurrentSession()) return
  openEvents()
  if (!isMobile.value && loadedSessionId.value !== activeSessionId.value && !sessionLoading.value) {
    await selectSession(activeSessionId.value)
  }
}

async function loadPlatformLoginPlatforms() {
  try {
    platformLoginPlatforms.value = await getWebChatPlatforms()
    if (!platformLoginForm.adapter_id && platformLoginPlatforms.value.length > 0) {
      platformLoginForm.adapter_id = platformLoginPlatforms.value[0].id
      platformLoginForm.platform = platformLoginPlatforms.value[0].platform
    }
  } catch {
    platformLoginPlatforms.value = []
  }
}

async function selectSession(sessionId) {
  refreshPluginOrder()
  const key = sessionKey(sessionId)
  const previousSessionId = activeSessionId.value
  const previousMobileView = mobileView.value
  const requestVersion = ++sessionRequestVersion
  activeSessionId.value = key
  sessionLoading.value = true
  if (isMobile.value) mobileView.value = 'chat'
  const params = { after_id: 0, limit: 50 }
  if (key !== privateSessionId) params.plugin_id = key
  try {
    const loadedMessages = await getWebChatMessages(params)
    if (requestVersion !== sessionRequestVersion || !session.value || activeSessionId.value !== key) return false
    messages.value = loadedMessages
    loadedSessionId.value = key
    sessionLoading.value = false
    messageCountMap[key] = Math.max(messageCountMap[key] || 0, loadedMessages.length)
    const latestID = latestMessageID(loadedMessages)
    if (latestID > (lastMessageIdMap[key] || 0)) lastMessageIdMap[key] = latestID
    if (isMobile.value) mobileView.value = 'chat'
    await markActiveSessionRead(key, requestVersion)
    if (requestVersion !== sessionRequestVersion) return false
    scrollBottom()
    return true
  } catch (error) {
    if (requestVersion === sessionRequestVersion) {
      sessionLoading.value = false
      activeSessionId.value = loadedSessionId.value || previousSessionId
      if (isMobile.value) mobileView.value = previousMobileView
      ElMessage.error(error.message || '加载会话失败')
    }
    return false
  } finally {
    if (requestVersion === sessionRequestVersion) sessionLoading.value = false
  }
}

async function switchToPlugin(pluginId) {
  if (!pluginId) return
  const currentSession = session.value
  const currentInitializationVersion = initializationVersion
  try {
    let target = plugins.value.find((item) => item.id === pluginId)
    if (!target) {
      const loadedPlugins = await getWebChatPlugins()
      if (session.value !== currentSession || initializationVersion !== currentInitializationVersion) return
      plugins.value = Array.isArray(loadedPlugins) ? loadedPlugins : []
      refreshPluginOrder()
      target = plugins.value.find((item) => item.id === pluginId)
    }
    if (!target) {
      ElMessage.warning('该插件当前不可用')
      return
    }
    await selectSession(pluginId)
  } catch (error) {
    if (session.value === currentSession && initializationVersion === currentInitializationVersion) {
      ElMessage.error(error.message || '切换插件失败')
    }
  }
}

function showMobileSessions() {
  if (!isMobile.value) return
  mobileView.value = 'sessions'
}

function togglePluginDescription(pluginId) {
  toggleExpandedDescription(expandedPluginDescriptions, pluginId)
}

function toggleSwitchDescription(key) {
  toggleExpandedDescription(expandedSwitchDescriptions, key)
}

function toggleExpandedDescription(items, key) {
  if (items.has(key)) {
    items.delete(key)
  } else {
    items.add(key)
  }
}

function pluginSwitchDescriptionKey(message, part, index) {
  return `${message.message_id}:${part.plugin.id}:${index}`
}

function handleMobileMenuCommand(command) {
  if (command === 'bind') {
    settingsDrawer.value = true
  } else if (command === 'logout') {
    handleLogout()
  }
}

function handleBreakpointChange(event) {
  const previousMobileView = mobileView.value
  isMobile.value = event.matches
  if (event.matches) {
    mobileView.value = loadedSessionId.value === activeSessionId.value ? 'chat' : 'sessions'
  } else {
    mobileView.value = 'chat'
    if (previousMobileView === 'sessions' && session.value && !sessionLoading.value && loadedSessionId.value !== activeSessionId.value) {
      selectSession(activeSessionId.value)
    }
  }
}

function isChatVisible(key) {
  return Boolean(session.value) && loadedSessionId.value === key && activeSessionId.value === key && (!isMobile.value || mobileView.value === 'chat')
}

function resetQuickActionsScroll() {
  nextTick(() => {
    const scrollElement = quickActionsScrollRef.value
    quickActionsResizeObserver?.disconnect()
    if (!scrollElement) {
      showQuickScrollButtons.value = false
      quickScrollAtStart.value = true
      quickScrollAtEnd.value = false
      return
    }
    scrollElement.scrollLeft = 0
    quickActionsViewportWidth = scrollElement.clientWidth
    quickActionsResizeObserver?.observe(scrollElement)
    if (quickActionsListRef.value) quickActionsResizeObserver?.observe(quickActionsListRef.value)
    requestAnimationFrame(updateQuickActionsOverflow)
  })
}

function handleQuickActionsResize() {
  const element = quickActionsScrollRef.value
  if (element && element.clientWidth !== quickActionsViewportWidth) {
    quickActionsViewportWidth = element.clientWidth
    element.scrollLeft = 0
  }
  updateQuickActionsOverflow()
}

function updateQuickActionsOverflow() {
  const element = quickActionsScrollRef.value
  if (!element) return
  showQuickScrollButtons.value = element.scrollWidth > element.clientWidth + 1
  updateQuickScrollState()
}

function updateQuickScrollState() {
  const element = quickActionsScrollRef.value
  if (!element) return
  quickScrollAtStart.value = element.scrollLeft <= 1
  quickScrollAtEnd.value = element.scrollLeft >= element.scrollWidth - element.clientWidth - 1
}

function scrollQuickActions(direction) {
  const element = quickActionsScrollRef.value
  if (!element) return
  element.scrollBy({ left: direction * Math.max(element.clientWidth * 0.75, 120), behavior: 'smooth' })
}

async function sendCode() {
  codeLoading.value = true
  try {
    await sendWebChatEmailCode(registerForm.email)
    ElMessage.success('验证码已发送')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    codeLoading.value = false
  }
}

async function sendEmailLoginCode() {
  emailLoginCodeLoading.value = true
  try {
    await sendWebChatEmailCode(emailLoginForm.email, 'login')
    ElMessage.success('验证码已发送')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    emailLoginCodeLoading.value = false
  }
}

async function sendResetCode() {
  resetCodeLoading.value = true
  try {
    await sendWebChatEmailCode(resetForm.email, 'reset_password')
    ElMessage.success('验证码已发送')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    resetCodeLoading.value = false
  }
}

async function sendPlatformLoginCode() {
  const payload = buildPlatformLoginPayload()
  if (!payload) return
  platformLoginCodeLoading.value = true
  try {
    await sendWebChatPlatformCode(payload)
    ElMessage.success('如果账号和平台绑定可用，验证码将发送到对应平台私聊，请注意查收。')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    platformLoginCodeLoading.value = false
  }
}

async function handleRegister() {
  loading.value = true
  try {
    const data = await registerWebChat(registerForm)
    setSession(data)
    ElMessage.success('注册成功')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

async function handleResetPassword() {
  const email = resetForm.email.trim()
  const code = resetForm.code.trim()
  const password = resetForm.password.trim()
  const confirmPassword = resetForm.confirmPassword.trim()
  if (!email || !code || !password || !confirmPassword) {
    ElMessage.warning('请完整填写邮箱、验证码和新密码')
    return
  }
  if (password.length < 8 || password.length > 128) {
    ElMessage.warning('密码长度必须为 8 到 128 位')
    return
  }
  if (password !== confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  loading.value = true
  try {
    await resetWebChatPassword({ email, code, password })
    loginForm.login = email
    loginForm.password = ''
    emailLoginForm.email = email
    emailLoginForm.code = ''
    resetForm.code = ''
    resetForm.password = ''
    resetForm.confirmPassword = ''
    authMode.value = 'login'
    ElMessage.success('密码已重置，请登录')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

async function handleLogin() {
  loading.value = true
  try {
    const data = await loginWebChat(loginForm)
    setSession(data)
    ElMessage.success('登录成功')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

async function handleEmailLogin() {
  const email = emailLoginForm.email.trim()
  const code = emailLoginForm.code.trim()
  if (!email || !code) {
    ElMessage.warning('请填写邮箱和验证码')
    return
  }
  loading.value = true
  try {
    const data = await loginWebChatByEmailCode({ email, code })
    setSession(data)
    ElMessage.success('登录成功')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

async function handlePlatformLogin() {
  const payload = buildPlatformLoginPayload()
  if (!payload) return
  const code = platformLoginForm.code.trim()
  if (!code) {
    ElMessage.warning('请填写平台验证码')
    return
  }
  loading.value = true
  try {
    const data = await loginWebChatByPlatformCode({ ...payload, code })
    setSession(data)
    ElMessage.success('登录成功')
  } catch (error) {
    ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

async function handleLogout() {
  const logoutSession = session.value
  const logoutToken = csrfToken.value
  initializationVersion += 1
  sessionRequestVersion += 1
  readRequestVersion += 1
  closeEvents()
  try {
    await logoutWebChat(logoutToken)
  } catch {}
  if (session.value !== logoutSession) return
  session.value = null
  csrfToken.value = ''
  sending.value = false
  sessionLoading.value = false
  messages.value = []
  loadedSessionId.value = ''
  plugins.value = []
  sortedPluginIds.value = []
  expandedPluginDescriptions.clear()
  expandedSwitchDescriptions.clear()
  activeSessionId.value = privateSessionId
  mobileView.value = 'sessions'
  clearReactiveMap(unreadMap)
  clearReactiveMap(messageCountMap)
  clearReactiveMap(lastMessageIdMap)
  messageCountMap[privateSessionId] = 0
  lastMessageIdMap[privateSessionId] = 0
  await loadPlatformLoginPlatforms()
}

async function bindCodeSubmit() {
  try {
    const data = await bindWebChatCode(bindCode.value, csrfToken.value)
    session.value.user = data.user
    bindCode.value = ''
    settingsDrawer.value = false
    ElMessage.success('绑定成功')
  } catch (error) {
    ElMessage.error(error.message)
  }
}

async function sendQuick(value) {
  content.value = value
  await sendMessage()
}

async function sendMessage() {
  if (sending.value) return
  const payload = buildPayload()
  if (!payload) return
  const key = activeSessionId.value
  const currentSession = session.value
  const draft = {
    message_id: `local_${Date.now()}`,
    direction: 'in',
    message_type: 'text',
    content: payload.content,
    plugin_id: key === privateSessionId ? '' : key,
    created_at: new Date().toISOString()
  }
  messages.value.push(draft)
  messageCountMap[key] = (messageCountMap[key] || 0) + 1
  content.value = ''
  scrollBottom()
  sending.value = true
  try {
    const saved = await sendWebChatMessage(payload, csrfToken.value)
    if (session.value !== currentSession) return
    if (loadedSessionId.value === key) {
      const index = messages.value.findIndex((item) => item.message_id === draft.message_id)
      if (index >= 0) messages.value[index] = saved
    }
    if (saved?.message_id) lastMessageIdMap[key] = Math.max(lastMessageIdMap[key] || 0, Number(saved.message_id) || 0)
    if (isChatVisible(key)) await markActiveSessionRead(key)
  } catch (error) {
    if (session.value !== currentSession) return
    if (loadedSessionId.value === key) {
      messages.value = messages.value.filter((item) => item.message_id !== draft.message_id)
      content.value = payload.content
    }
    messageCountMap[key] = Math.max((messageCountMap[key] || 1) - 1, 0)
    ElMessage.error(error.message)
  } finally {
    sending.value = false
  }
}

function buildPayload() {
  const text = content.value.trim()
  if (!text) return null
  const payload = { type: 'text', content: text }
  if (activeSessionId.value !== privateSessionId) payload.plugin_id = activeSessionId.value
  return payload
}

function buildPlatformLoginPayload() {
  const adapter = platformLoginPlatforms.value.find((item) => item.id === platformLoginForm.adapter_id)
  if (!adapter) {
    ElMessage.warning('请选择正在运行的平台实例')
    return null
  }
  const username = platformLoginForm.username.trim()
  if (!username) {
    ElMessage.warning('请填写 Web 用户名')
    return null
  }
  platformLoginForm.platform = adapter.platform
  return {
    adapter_id: adapter.id,
    platform: adapter.platform,
    username
  }
}

function handlePlatformChange(value) {
  const adapter = platformLoginPlatforms.value.find((item) => item.id === value)
  platformLoginForm.platform = adapter?.platform || ''
}

function platformOptionLabel(item) {
  const title = item.display_name || item.platform
  const remark = item.remark ? ` / ${item.remark}` : ''
  return `${title}${remark} / #${item.id}`
}

function handleMessageListClick(event) {
  const button = event.target?.closest?.('.markdown-code-expand')
  if (!button) return
  const codeBlock = button.closest('.markdown-code')
  const code = codeBlock?.querySelector('code')
  expandedCodeContent.value = code?.textContent || ''
  expandedCodeLang.value = code?.dataset?.lang || ''
  codeDialogVisible.value = true
}

async function copyExpandedCode() {
  await copyText(expandedCodeContent.value, '代码已复制')
}

async function copyText(text, message) {
  if (!text) {
    ElMessage.warning('没有可复制的代码内容')
    return
  }
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    copyByFallback(text)
  }
  ElMessage.success(message)
}

function copyByFallback(text) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'readonly')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

function openEvents() {
  closeEvents()
  eventSource = new EventSource('/api/open/web-chat/events')
  eventSource.addEventListener('message', (event) => {
    const msg = JSON.parse(event.data)
    const key = sessionKey(msg.plugin_id)
    messageCountMap[key] = (messageCountMap[key] || 0) + 1
    if (msg.message_id) lastMessageIdMap[key] = Math.max(lastMessageIdMap[key] || 0, Number(msg.message_id) || 0)
    if (isChatVisible(key)) {
      if (!messages.value.some((item) => item.message_id === msg.message_id)) {
        messages.value.push(msg)
        scrollBottom()
      }
      markActiveSessionRead(key)
      return
    }
    unreadMap[key] = (unreadMap[key] || 0) + 1
    refreshPluginOrder()
  })
}

function closeEvents() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

function sessionKey(pluginId) {
  return pluginId || privateSessionId
}

function pluginIdFromSessionKey(key) {
  return key === privateSessionId ? '' : key
}

function updateSessionStats(item) {
  const key = sessionKey(item.plugin_id)
  messageCountMap[key] = item.count || 0
  unreadMap[key] = item.unread_count || 0
  lastMessageIdMap[key] = item.last_message_id || 0
}

async function markActiveSessionRead(key, sessionVersion = sessionRequestVersion) {
  if (!isChatVisible(key)) return
  const currentSession = session.value
  const currentReadVersion = ++readRequestVersion
  try {
    const state = await markWebChatRead({ plugin_id: pluginIdFromSessionKey(key) }, csrfToken.value)
    if (session.value !== currentSession || sessionVersion !== sessionRequestVersion || currentReadVersion !== readRequestVersion || !isChatVisible(key)) return
    updateSessionStats(state)
    unreadMap[key] = 0
  } catch (error) {
    if (session.value === currentSession && sessionVersion === sessionRequestVersion && currentReadVersion === readRequestVersion) ElMessage.error(error.message || '更新已读状态失败')
  }
}

function latestMessageID(items) {
  return items.reduce((max, item) => Math.max(max, Number(item.message_id) || 0), 0)
}

function clearReactiveMap(map) {
  for (const key of Object.keys(map)) delete map[key]
}

function sessionTitle(item) {
  return item?.title || item?.name || item?.id || ''
}

function refreshPluginOrder() {
  sortedPluginIds.value = plugins.value
    .slice()
    .sort((a, b) => {
      const unreadDiff = (unreadMap[b.id] || 0) - (unreadMap[a.id] || 0)
      if (unreadDiff !== 0) return unreadDiff
      return sessionTitle(a).localeCompare(sessionTitle(b), 'zh-Hans-CN')
    })
    .map((item) => item.id)
}

function pluginSearchText(item) {
  return [
    item.id,
    item.name,
    item.title,
    item.description,
    ...(item.keywords || []),
    ...(item.quick_actions || []).flatMap((action) => [action.label, action.text])
  ].join(' ').toLowerCase()
}

function parseRich(value) {
  try {
    const parsed = JSON.parse(value || '[]')
    if (Array.isArray(parsed)) return parsed
    if (Array.isArray(parsed.parts)) return parsed.parts
    return []
  } catch {
    return []
  }
}

function parseButtons(value) {
  try {
    const parsed = JSON.parse(value || '[]')
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function renderMarkdown(value) {
  const lines = String(value || '').replace(/\r\n/g, '\n').split('\n')
  const html = []
  const codeLines = []
  let listOpen = false
  let codeOpen = false
  let codeLang = ''
  const closeList = () => {
    if (listOpen) {
      html.push('</ul>')
      listOpen = false
    }
  }
  const closeCode = () => {
    if (codeOpen) {
      html.push(`<pre class="markdown-code"><button class="markdown-code-expand" type="button">展开</button><code${codeLang ? ` data-lang="${escapeAttribute(codeLang)}"` : ''}>${escapeHTML(codeLines.join('\n'))}</code></pre>`)
      codeLines.length = 0
      codeOpen = false
      codeLang = ''
    }
  }
  for (const line of lines) {
    const text = line.trim()
    if (text.startsWith('```')) {
      if (codeOpen) {
        closeCode()
      } else {
        closeList()
        codeOpen = true
        codeLang = text.slice(3).trim()
      }
    } else if (codeOpen) {
      codeLines.push(line)
    } else if (text === '') {
      closeList()
      html.push('<br>')
    } else if (text.startsWith('### ')) {
      closeList()
      html.push(`<h3>${formatMarkdownInline(text.slice(4))}</h3>`)
    } else if (text.startsWith('## ')) {
      closeList()
      html.push(`<h2>${formatMarkdownInline(text.slice(3))}</h2>`)
    } else if (text.startsWith('# ')) {
      closeList()
      html.push(`<h1>${formatMarkdownInline(text.slice(2))}</h1>`)
    } else if (text.startsWith('- ') || text.startsWith('* ')) {
      if (!listOpen) {
        html.push('<ul>')
        listOpen = true
      }
      html.push(`<li>${formatMarkdownInline(text.slice(2))}</li>`)
    } else {
      closeList()
      html.push(`<p>${formatMarkdownInline(text)}</p>`)
    }
  }
  closeCode()
  closeList()
  return html.join('')
}

function formatMarkdownInline(value) {
  return formatMarkdownLinks(escapeHTML(value))
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
}

function formatMarkdownLinks(value) {
  return value.replace(/\[([^\]]+)\]\(([^\s)]+)\)/g, (match, text, href) => {
    const decodedHref = decodeHTML(href)
    if (!safeLinkURL(decodedHref)) return match
    return `<a href="${escapeAttribute(decodedHref)}" target="_blank" rel="noopener noreferrer">${text}</a>`
  })
}

function safeLinkURL(value) {
  const url = String(value || '').trim().toLowerCase()
  return url.startsWith('http://') || url.startsWith('https://') || url.startsWith('/api/open/images/')
}

function escapeHTML(value) {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeAttribute(value) {
  return escapeHTML(value).replace(/`/g, '&#96;')
}

function decodeHTML(value) {
  return String(value || '')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
}

function safeImageURL(value) {
  const url = (value || '').trim()
  return url.startsWith('http://') || url.startsWith('https://') || url.startsWith('/api/open/images/')
}

function formatTime(value) {
  return value ? new Date(value).toLocaleString() : ''
}

function scrollBottom() {
  nextTick(() => {
    scrollMessageListToBottom()
    requestAnimationFrame(() => {
      scrollMessageListToBottom()
      setTimeout(scrollMessageListToBottom, 120)
    })
  })
}

function scrollMessageListToBottom() {
  if (messageListRef.value) messageListRef.value.scrollTop = messageListRef.value.scrollHeight
}
</script>

<style scoped>
.web-chat-page { min-height: 100vh; background: linear-gradient(135deg, #dff3ff 0%, #ffffff 52%, #eef5ff 100%); padding: 28px; color: #1f2937; }
.auth-card { max-width: 460px; margin: 6vh auto; padding: 28px; border-radius: 24px; background: rgba(255,255,255,.92); box-shadow: 0 20px 60px rgba(59,130,246,.18); }
.brand h1 { margin: 0 0 8px; font-size: 28px; }
.brand p, .chat-header p, .bind-tip { color: #64748b; }
.inline-row { display: flex; gap: 10px; width: 100%; }
.full-button { width: 100%; }
.login-mode-tabs { width: 100%; margin-bottom: 18px; }
.auth-bottom-tabs { display: none; }
.chat-shell { max-width: 1280px; height: calc(100vh - 56px); min-height: 0; margin: 0 auto; display: grid; grid-template-columns: 320px minmax(0, 1fr); gap: 18px; overflow: hidden; }
.plugin-panel, .chat-main { background: rgba(255,255,255,.94); border: 1px solid rgba(148,163,184,.25); border-radius: 22px; box-shadow: 0 18px 48px rgba(15,23,42,.10); }
.plugin-panel { padding: 18px; overflow: auto; }
.panel-title { font-weight: 700; margin-bottom: 14px; }
.plugin-search { margin-bottom: 12px; }
.plugin-item { width: 100%; text-align: left; border: 1px solid #e2e8f0; background: #fff; border-radius: 14px; padding: 12px; margin-bottom: 10px; cursor: pointer; }
.plugin-item.active { border-color: #2563eb; background: #eff6ff; }
.private-item { border-color: #bfdbfe; background: #f8fbff; }
.plugin-row { display: flex; justify-content: space-between; gap: 8px; }
.plugin-row strong { min-width: 0; overflow-wrap: anywhere; }
.plugin-description { display: -webkit-box; margin-top: 6px; max-height: 34px; overflow: hidden; color: #64748b; font-size: 12px; line-height: 17px; overflow-wrap: anywhere; word-break: break-word; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.plugin-description.expanded { display: block; max-height: none; overflow: visible; -webkit-line-clamp: unset; }
.description-toggle { display: inline-flex; margin-top: 4px; padding: 0; border: 0; background: transparent; color: #2563eb; font: inherit; font-size: 12px; line-height: 18px; cursor: pointer; }
.message-count { display: block; margin-top: 6px; color: #2563eb; font-size: 12px; font-style: normal; }
.unread { display: block; flex-shrink: 0; min-width: 20px; height: 20px; padding: 0 6px; border-radius: 999px; background: #ef4444; color: #fff; text-align: center; font-size: 12px; line-height: 20px; }
.quick-preview { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
.quick-preview em { font-style: normal; font-size: 12px; color: #2563eb; background: #dbeafe; border-radius: 999px; padding: 2px 8px; }
.chat-main { display: flex; min-height: 0; overflow: hidden; flex-direction: column; }
.chat-header { display: flex; flex: 0 0 auto; justify-content: space-between; align-items: center; gap: 12px; padding: 18px 22px; border-bottom: 1px solid #e2e8f0; }
.chat-header-main { display: flex; align-items: center; min-width: 0; gap: 10px; }
.chat-header-text { min-width: 0; }
.chat-header h2 { margin: 0 0 4px; }
.chat-header p { margin: 0; font-size: 12px; word-break: break-all; }
.header-actions { display: flex; gap: 8px; }
.mobile-session-panel { display: none; }
.mobile-back-button, .mobile-menu-button { display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; width: 36px; height: 36px; padding: 0; border: 0; border-radius: 10px; background: #eff6ff; color: #2563eb; cursor: pointer; }
.quick-actions-shell { display: flex; align-items: center; min-width: 0; }
.quick-actions-scroll { flex: 1; min-width: 0; overflow-x: auto; overflow-y: hidden; scrollbar-width: none; -ms-overflow-style: none; }
.quick-actions-scroll::-webkit-scrollbar { display: none; }
.quick-actions-list { display: flex; align-items: center; gap: 8px; width: max-content; min-width: 100%; white-space: nowrap; }
.quick-actions-list .el-button { flex-shrink: 0; margin-left: 0; }
.quick-scroll-button { display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; width: 28px; height: 28px; padding: 0; border: 0; border-radius: 8px; background: transparent; color: #2563eb; cursor: pointer; }
.quick-scroll-button:hover:not(:disabled) { background: #eff6ff; }
.quick-scroll-button:disabled { opacity: .3; cursor: default; }
.composer-quick-actions { flex: 0 0 auto; max-height: 32px; padding: 0 0 2px; overflow: hidden; }
.message-list { flex: 1 1 0; min-height: 0; overflow: auto; padding: 20px; }
.message { display: flex; margin-bottom: 14px; }
.message.in { justify-content: flex-end; }
.bubble { max-width: min(720px, 82%); padding: 12px 14px; border-radius: 18px; background: #f1f5f9; white-space: pre-wrap; word-break: break-word; }
.message.in .bubble { background: #2563eb; color: #fff; }
.message.in .meta { color: rgba(255,255,255,.75); }
.meta { font-size: 12px; color: #64748b; margin-bottom: 6px; }
.markdown-block { margin: 0; padding: 12px; background: rgba(15,23,42,.08); border-radius: 12px; white-space: normal; line-height: 1.7; }
.markdown-block :deep(h1), .markdown-block :deep(h2), .markdown-block :deep(h3), .markdown-block :deep(p), .markdown-block :deep(ul) { margin: 0 0 8px; }
.markdown-block :deep(h1) { font-size: 24px; }
.markdown-block :deep(h2) { font-size: 20px; }
.markdown-block :deep(h3) { font-size: 17px; }
.markdown-block :deep(ul) { padding-left: 20px; }
.markdown-block :deep(a) { color: #2563eb; text-decoration: underline; word-break: break-all; }
.markdown-block :deep(code) { padding: 2px 5px; border-radius: 5px; background: rgba(15,23,42,.12); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.markdown-block :deep(.markdown-code) { position: relative; margin: 0 0 8px; padding: 36px 12px 12px; overflow-x: auto; border-radius: 10px; background: rgba(15,23,42,.88); color: #e5e7eb; white-space: pre; }
.markdown-block :deep(.markdown-code code) { padding: 0; background: transparent; color: inherit; }
.markdown-block :deep(.markdown-code-expand) { position: absolute; top: 8px; right: 8px; z-index: 1; border: 1px solid rgba(226,232,240,.32); border-radius: 999px; padding: 3px 9px; background: rgba(15,23,42,.76); color: #f8fafc; font-size: 12px; line-height: 1.4; cursor: pointer; }
.markdown-block :deep(.markdown-code-expand:hover) { background: rgba(30,41,59,.95); border-color: rgba(226,232,240,.58); }
.expanded-code { max-height: 62vh; margin: 0; padding: 16px; overflow: auto; border-radius: 12px; background: #0f172a; color: #e5e7eb; white-space: pre; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; line-height: 1.65; }
.expanded-code code { padding: 0; background: transparent; color: inherit; font: inherit; }
.markdown-block :deep(:last-child) { margin-bottom: 0; }
.chat-image { max-width: 320px; max-height: 320px; border-radius: 12px; display: block; cursor: zoom-in; }
.chat-image :deep(.el-image__inner) { max-width: 320px; max-height: 320px; border-radius: 12px; }
.plugin-switch-card { display: grid; gap: 8px; min-width: min(360px, 100%); padding: 14px; border: 1px solid #bfdbfe; border-radius: 16px; background: #eff6ff; cursor: pointer; }
.plugin-switch-card__label { color: #2563eb; font-size: 12px; font-weight: 700; }
.plugin-switch-card__title { color: #0f172a; font-size: 16px; font-weight: 700; }
.plugin-switch-card__desc { display: -webkit-box; max-height: 40px; overflow: hidden; color: #475569; font-size: 13px; line-height: 20px; overflow-wrap: anywhere; word-break: break-word; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.plugin-switch-card__desc[role="button"] { cursor: pointer; }
.plugin-switch-card__desc.expanded { display: block; max-height: none; overflow: visible; -webkit-line-clamp: unset; }
.plugin-switch-card__actions { display: flex; gap: 6px; flex-wrap: wrap; }
.plugin-switch-card__actions span { color: #2563eb; background: #dbeafe; border-radius: 999px; padding: 2px 8px; font-size: 12px; }
.button-message { min-width: min(420px, 100%); }
.button-row { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
.button-row .el-button { flex: 1 1 100%; min-height: 38px; margin-left: 0; justify-content: center; font-size: 15px; }
.composer { display: grid; flex: 0 0 auto; gap: 10px; border-top: 1px solid #e2e8f0; padding: 14px; background: #fff; }
.composer-row { display: grid; min-height: 0; grid-template-columns: minmax(0, 1fr) 88px; gap: 10px; align-items: stretch; }
.send-button { height: 100%; min-height: 78px; }
.bind-button { margin-top: 16px; }
@media (max-width: 820px) {
  .web-chat-page { position: fixed; inset: 0; width: 100%; height: auto; min-height: 0; padding: 0; overflow: hidden; }
  .auth-card { height: 100%; margin: 0; min-height: 0; max-height: none; padding-bottom: calc(86px + env(safe-area-inset-bottom)); overflow-y: auto; border-radius: 0; box-shadow: none; -webkit-overflow-scrolling: touch; }
  .auth-card :deep(.el-tabs__header) { display: none; }
  .auth-bottom-tabs { position: fixed; left: 0; right: 0; bottom: 0; z-index: 20; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 4px; padding: 6px 8px calc(6px + env(safe-area-inset-bottom)); background: var(--bg-sidebar); border-top: 1px solid var(--border-on-dark); box-shadow: 0 -4px 16px rgba(0,0,0,.16); }
  .auth-bottom-tabs button { width: 100%; min-width: 0; height: 52px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px; border: 0; border-radius: var(--radius-md); background: transparent; color: var(--text-on-dark-muted); font: inherit; font-size: 12px; transition: all var(--transition-normal); }
  .auth-bottom-tabs .el-icon { font-size: 18px; }
  .auth-bottom-tabs span { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .auth-bottom-tabs button.active { background: var(--brand-500); color: #fff; box-shadow: 0 2px 8px var(--bg-sidebar-active-glow); }
  .chat-shell { width: 100%; height: 100%; min-height: 0; grid-template-columns: minmax(0, 1fr); gap: 0; }
  .desktop-panel { display: none; }
  .mobile-session-panel { display: block; height: 100%; min-height: 0; padding: 16px; overflow-y: auto; border: 0; border-radius: 0; -webkit-overflow-scrolling: touch; }
  .mobile-session-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
  .mobile-session-header h2 { margin: 0; font-size: 20px; }
  .mobile-plugin-search { width: 100%; }
  .chat-main { position: fixed; inset: 0; width: auto; height: auto; min-height: 0; border-radius: 0; border: 0; }
  .chat-header { padding: 12px; align-items: center; }
  .desktop-header-actions { display: none; }
  .chat-header-main { width: 100%; }
  .chat-header-text { flex: 1; }
  .chat-header h2, .chat-header p { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .bubble { max-width: 90%; }
  .composer { flex: 0 0 auto; padding: 8px 12px calc(8px + env(safe-area-inset-bottom)); }
  .composer-quick-actions { height: 30px; max-height: 30px; }
  .composer-row { height: 52px; grid-template-columns: minmax(0, 1fr) 72px; }
  .composer-row :deep(.el-textarea), .composer-row :deep(.el-textarea__inner) { height: 52px; min-height: 52px !important; max-height: 52px; }
  .send-button { height: 52px; min-height: 52px; }
  .inline-row { flex-direction: column; align-items: stretch; }
  .markdown-block :deep(.markdown-code-expand) { padding: 2px 7px; font-size: 11px; }
  .expanded-code { max-height: 56vh; padding: 12px; font-size: 12px; }
}
</style>
