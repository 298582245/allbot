<template>
  <div class="open-apis-page">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">开放接口</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看开放接口说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
            <el-input v-model="searchKeyword" class="header-search" clearable placeholder="搜索接口名称、路径、描述或运行时" />
            <el-button @click="openSettings">全局 IP 设置</el-button>
            <el-button :loading="loading" @click="loadItems">刷新</el-button>
            <el-button type="primary" @click="createItem">
              <el-icon><Plus /></el-icon>
              新增接口
            </el-button>
          </div>
        </div>
      </template>

      <div class="api-content" v-loading="loading">
        <div class="api-table-area desktop-api-table">
          <el-table :data="paginatedItems" row-key="id" stripe border height="100%" class="api-table">
            <el-table-column prop="name" label="接口名称" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.name || row.id || '-' }}</template>
            </el-table-column>
            <el-table-column label="启用状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="方法" width="96">
              <template #default="{ row }">
                <el-tag effect="plain">{{ normalizeMethod(row.method) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="路径" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <code>{{ displayPath(row) }}</code>
              </template>
            </el-table-column>
            <el-table-column label="Runtime" width="120">
              <template #default="{ row }">
                <el-tag type="warning" effect="plain">{{ runtimeLabel(row.runtime) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="运行环境" min-width="130" show-overflow-tooltip>
              <template #default="{ row }">{{ displayRuntimeProfile(row.runtime_profile) }}</template>
            </el-table-column>
            <el-table-column label="描述" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ row.description || '-' }}</template>
            </el-table-column>
            <el-table-column label="IP 策略" min-width="150">
              <template #default="{ row }">
                <el-tag :type="ipPolicyTagType(row)" effect="plain">{{ ipPolicyLabel(row) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="总调用" width="105" align="right">
              <template #default="{ row }">{{ formatCount(callStats(row).total) }}</template>
            </el-table-column>
            <el-table-column label="Token" width="110">
              <template #default="{ row }">
                <el-tag :type="hasToken(row) ? 'success' : 'info'">{{ hasToken(row) ? '已设置' : '未设置' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="290" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="primary" @click="editItem(row)">编辑</el-button>
                <el-button size="small" type="success" @click="openCalls(row)">调用数据</el-button>
                <el-dropdown trigger="click" @command="command => handleRowCommand(command, row)">
                  <el-button size="small">更多</el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-if="!isBuiltin(row)" command="file">文件</el-dropdown-item>
                      <el-dropdown-item command="copy">复制地址</el-dropdown-item>
                      <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="paginatedItems.length > 0" class="api-grid mobile-api-grid">
          <div v-for="row in paginatedItems" :key="itemId(row)" class="api-card-item">
            <div class="api-card-header">
              <span class="api-name">{{ row.name || row.id || '-' }}</span>
              <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
            </div>
            <div class="api-card-body">
              <div class="api-info-row">
                <span class="label">方法：</span>
                <el-tag effect="plain" size="small">{{ normalizeMethod(row.method) }}</el-tag>
              </div>
              <div class="api-info-row">
                <span class="label">路径：</span>
                <code class="path-text">{{ displayPath(row) }}</code>
              </div>
              <div class="api-info-row">
                <span class="label">Runtime：</span>
                <el-tag type="warning" effect="plain" size="small">{{ runtimeLabel(row.runtime) }}</el-tag>
              </div>
              <div class="api-info-row">
                <span class="label">运行环境：</span>
                <span class="value-text">{{ displayRuntimeProfile(row.runtime_profile) }}</span>
              </div>
              <div class="api-info-row">
                <span class="label">描述：</span>
                <span class="value-text">{{ row.description || '-' }}</span>
              </div>
              <div class="api-info-row">
                <span class="label">IP 策略：</span>
                <el-tag :type="ipPolicyTagType(row)" effect="plain" size="small">{{ ipPolicyLabel(row) }}</el-tag>
              </div>
              <div class="api-info-row">
                <span class="label">总调用：</span>
                <strong>{{ formatCount(callStats(row).total) }}</strong>
              </div>
              <div class="api-info-row">
                <span class="label">Token：</span>
                <el-tag :type="hasToken(row) ? 'success' : 'info'" size="small">{{ hasToken(row) ? '已设置' : '未设置' }}</el-tag>
              </div>
            </div>
            <div class="api-card-footer">
              <el-button size="small" type="primary" @click="editItem(row)">编辑</el-button>
              <el-button size="small" type="success" @click="openCalls(row)">调用数据</el-button>
              <el-dropdown class="mobile-more" trigger="click" @command="command => handleRowCommand(command, row)">
                <el-button size="small">更多</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="!isBuiltin(row)" command="file">文件</el-dropdown-item>
                    <el-dropdown-item command="copy">复制地址</el-dropdown-item>
                    <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>
        </div>

        <el-empty v-if="!loading && items.length === 0" description="暂无开放接口" />
        <el-empty v-else-if="!loading && filteredItems.length === 0" description="没有匹配的开放接口" />
      </div>

      <StdPagination
        v-if="items.length > 0"
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="filteredItems.length"
      />
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogMode === 'create' ? '新增开放接口' : '编辑开放接口'"
      width="560px"
      class="api-dialog"
      :close-on-click-modal="false"
    >
      <el-form :model="form" label-width="96px" class="dialog-form">
        <el-form-item label="接口名称" required>
          <el-input v-model="form.name" maxlength="60" show-word-limit placeholder="例如：自定义回复接口" />
        </el-form-item>
        <el-form-item label="接口路径" required>
          <el-input v-model="form.path" :disabled="dialogMode === 'edit'" placeholder="只输入单个词，例如 a">
            <template #prepend>/api/open/</template>
          </el-input>
          <div class="field-tip">只允许字母、数字、横线和下划线，不能输入 a/b、/a 或 /api/open/a。</div>
        </el-form-item>
        <el-form-item label="请求方法" required>
          <el-select v-model="form.method" style="width: 100%">
            <el-option v-for="method in httpMethods" :key="method" :label="method" :value="method" />
          </el-select>
        </el-form-item>
        <el-form-item label="是否开启">
          <el-switch v-model="form.enabled" active-text="开启" inactive-text="停用" />
        </el-form-item>
        <el-form-item label="运行语言" required>
          <el-radio-group v-model="form.runtime" :disabled="form.runtime === 'builtin'">
            <el-radio-button label="nodejs">Node.js</el-radio-button>
            <el-radio-button label="python">Python</el-radio-button>
            <el-radio-button v-if="form.runtime === 'builtin'" label="builtin">内置</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.runtime !== 'builtin'" label="运行环境">
          <el-select v-model="form.runtime_profile" clearable placeholder="使用默认运行环境" style="width: 100%">
            <el-option
              v-for="profile in runtimeProfilesBy(form.runtime)"
              :key="profile.id"
              :label="runtimeProfileLabel(profile)"
              :value="profile.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Token" required>
          <el-input
            v-model="form.token"
            type="password"
            show-password
            :placeholder="dialogMode === 'create' ? '必填，调用接口时需要 Token' : '留空则保留原 Token，填写则更新'"
          />
        </el-form-item>
        <el-form-item label="IP 白名单">
          <el-radio-group v-model="form.ipWhitelistMode">
            <el-radio-button label="inherit">继承全局</el-radio-button>
            <el-radio-button label="allow_all">允许全部</el-radio-button>
            <el-radio-button label="custom">自定义</el-radio-button>
          </el-radio-group>
          <el-select
            v-if="form.ipWhitelistMode === 'custom'"
            v-model="form.ipWhitelist"
            class="ip-value-input"
            multiple
            filterable
            allow-create
            default-first-option
            placeholder="输入 IPv4、IPv6 或 CIDR 后回车"
          />
          <div class="field-tip">{{ endpointPolicyTip }}</div>
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            maxlength="240"
            show-word-limit
            :rows="3"
            placeholder="说明这个接口的用途、入参或调用场景"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveDialog">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="settingsVisible"
      title="开放接口全局 IP 设置"
      width="620px"
      class="settings-dialog"
      :close-on-click-modal="false"
    >
      <div v-loading="settingsLoading">
        <el-alert type="info" :closable="false" show-icon class="settings-alert">
          <template #title>规则优先级：单接口 &gt; 全局 &gt; 默认 <code>*</code></template>
          单接口未配置时继承全局；全局配置不存在时默认允许全部。反向代理场景必须正确填写代理出口 IP 或网段。
        </el-alert>
        <el-form :model="settingsForm" label-width="128px" class="settings-form">
          <el-form-item label="全局白名单">
            <el-radio-group v-model="settingsForm.ipWhitelistMode">
              <el-radio-button label="allow_all">允许全部</el-radio-button>
              <el-radio-button label="custom">自定义白名单</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="settingsForm.ipWhitelistMode === 'custom'" label="IP / CIDR" required>
            <el-select
              v-model="settingsForm.ipWhitelist"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="输入 IPv4、IPv6 或 CIDR 后回车"
              style="width: 100%"
            />
            <div class="field-tip">自定义白名单不能为空；<code>*</code> 只能单独使用，最终格式校验以后端为准。</div>
          </el-form-item>
          <el-form-item label="可信代理">
            <el-select
              v-model="settingsForm.trustedProxies"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="输入代理 IP 或 CIDR 后回车"
              style="width: 100%"
            />
            <div class="field-tip">只有直连来源匹配该列表时才信任 X-Forwarded-For / X-Real-IP；留空表示不信任任何转发头。</div>
          </el-form-item>
          <el-form-item label="明细保留天数">
            <el-input-number v-model="settingsForm.callLogRetentionDays" :min="0" :max="3650" controls-position="right" />
            <div class="field-tip">0 表示不自动清理调用明细；累计统计永久保留。</div>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="settingsVisible = false">取消</el-button>
        <el-button type="primary" :loading="settingsSaving" :disabled="settingsLoading || !settingsLoaded" @click="saveSettings">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="callsVisible"
      :title="callsTitle"
      size="76%"
      class="calls-drawer"
      destroy-on-close
      @closed="resetCalls"
    >
      <div class="calls-body">
        <div class="stats-grid">
          <div><span>总调用</span><strong>{{ formatCount(callsSummary.total) }}</strong></div>
          <div><span>成功</span><strong class="success-text">{{ formatCount(callsSummary.success) }}</strong></div>
          <div><span>拒绝</span><strong class="warning-text">{{ formatCount(callsSummary.rejected) }}</strong></div>
          <div><span>失败</span><strong class="danger-text">{{ formatCount(callsSummary.failed) }}</strong></div>
          <div><span>最近调用</span><strong>{{ formatTime(callsSummary.last_called_at) }}</strong></div>
        </div>

        <div class="calls-filters">
          <el-select v-model="callsFilters.outcome" clearable placeholder="全部结果">
            <el-option label="成功" value="success" />
            <el-option label="IP 拒绝" value="ip_denied" />
            <el-option label="Token 拒绝" value="token_denied" />
            <el-option label="失败" value="failed" />
          </el-select>
          <el-input v-model="callsFilters.clientIp" clearable placeholder="客户端 IP（精确匹配）" @keyup.enter="searchCalls" />
          <el-input v-model="callsFilters.statusCode" clearable inputmode="numeric" placeholder="HTTP 状态码" @keyup.enter="searchCalls" />
          <el-date-picker
            v-model="callsFilters.timeRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
          />
          <el-button type="primary" :loading="callsLoading" @click="searchCalls">查询</el-button>
          <el-button @click="resetCallFilters">重置</el-button>
        </div>

        <div class="calls-table-wrap" v-loading="callsLoading">
          <el-table :data="callItems" border stripe height="100%" empty-text="暂无调用数据">
            <el-table-column label="调用时间" min-width="180"><template #default="{ row }">{{ formatTime(row.started_at || row.startedAt) }}</template></el-table-column>
            <el-table-column prop="method" label="方法" width="85" />
            <el-table-column label="路径" min-width="170" show-overflow-tooltip><template #default="{ row }">{{ displayCallPath(row) }}</template></el-table-column>
            <el-table-column label="客户端 IP" min-width="165" show-overflow-tooltip><template #default="{ row }">{{ row.client_ip || row.clientIp || '-' }}</template></el-table-column>
            <el-table-column label="HTTP" width="80" align="center"><template #default="{ row }">{{ row.status_code ?? row.statusCode ?? '-' }}</template></el-table-column>
            <el-table-column label="结果" width="110"><template #default="{ row }"><el-tag :type="outcomeTagType(row.outcome)" effect="plain">{{ outcomeLabel(row.outcome) }}</el-tag></template></el-table-column>
            <el-table-column label="耗时" width="100" align="right"><template #default="{ row }">{{ formatDuration(row.duration_ms ?? row.durationMs) }}</template></el-table-column>
          </el-table>
        </div>
        <div class="calls-footer">
          <span>筛选明细 {{ formatCount(callsTotal) }} 条，保留 {{ callsRetentionText }}</span>
          <StdPagination
            v-model:current-page="callsPage"
            v-model:page-size="callsPageSize"
            :total="callsTotal"
            :page-sizes="[10, 20, 50, 100]"
            @current-change="loadCalls"
            @size-change="handleCallsPageSize"
          />
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
defineOptions({ name: 'OpenApis' })
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Plus } from '@element-plus/icons-vue'
import {
  createOpenApi,
  deleteOpenApi,
  getOpenApiCalls,
  getOpenApiSettings,
  getOpenApis,
  getRuntimeProfiles,
  saveOpenApiSettings,
  updateOpenApi
} from '@/api'
import StdPagination from '@/components/StdPagination.vue'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const items = ref([])
const searchKeyword = ref('')
const runtimeProfiles = ref([])
const currentPage = ref(1)
const pageSize = 10
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref('')
const form = reactive(createEmptyForm())
const settingsVisible = ref(false)
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const settingsLoaded = ref(false)
const settingsForm = reactive(createEmptySettingsForm())
const callsVisible = ref(false)
const callsLoading = ref(false)
const callsTarget = ref(null)
const callItems = ref([])
const callsSummary = reactive(createEmptySummary())
const callsTotal = ref(0)
const callsRetentionDays = ref(30)
const callsPage = ref(1)
const callsPageSize = ref(20)
const callsFilters = reactive(createEmptyCallFilters())
let settingsRequestVersion = 0
let callsRequestVersion = 0
const singlePathPattern = /^[A-Za-z0-9_-]+$/
const httpMethods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']
const pageDescription = '独立管理对外 HTTP 接口、运行时、IP 访问策略和调用数据。'

const endpointPolicyTip = computed(() => {
  if (form.ipWhitelistMode === 'allow_all') return '此接口显式允许全部来源，将覆盖全局限制。'
  if (form.ipWhitelistMode === 'custom') return '自定义规则优先于全局；支持 IPv4、IPv6 和 CIDR，最终校验以后端为准。'
  if (dialogMode.value !== 'edit') return '未配置单接口规则时继承全局设置。'
  const current = items.value.find(row => itemId(row) === editingId.value)
  const effective = effectiveWhitelist(current || {})
  const source = String(current?.ip_whitelist_source || '') === 'default' ? '默认规则' : '全局规则'
  return `当前继承${source}：${effective.length ? effective.join('、') : '*'}`
})

const callsTitle = computed(() => `调用数据 - ${callsTarget.value?.name || itemId(callsTarget.value || {}) || '-'}`)
const callsRetentionText = computed(() => callsRetentionDays.value === 0 ? '永久' : `${callsRetentionDays.value} 天`)

const filteredItems = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return items.value
  return items.value.filter(row => getOpenApiSearchText(row).includes(keyword))
})

const paginatedItems = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredItems.value.slice(start, start + pageSize)
})

watch(searchKeyword, () => {
  currentPage.value = 1
})

watch(filteredItems, () => {
  const maxPage = Math.max(1, Math.ceil(filteredItems.value.length / pageSize))
  if (currentPage.value > maxPage) currentPage.value = maxPage
})

watch(() => form.runtime, (runtime) => {
  if (!form.runtime_profile) return
  const matched = runtimeProfiles.value.some(profile => profile.id === form.runtime_profile && profile.runtime === runtime)
  if (!matched) form.runtime_profile = ''
})

watch(settingsVisible, (visible) => {
  if (!visible) {
    settingsRequestVersion++
    settingsLoading.value = false
    settingsLoaded.value = false
  }
})

watch(callsVisible, (visible) => {
  if (!visible) {
    callsRequestVersion++
    callsLoading.value = false
  }
})

const loadItems = async () => {
  loading.value = true
  try {
    const result = await getOpenApis()
    items.value = normalizeListResult(result)
  } finally {
    loading.value = false
  }
}

const loadRuntimeProfiles = async () => {
  try {
    const data = await getRuntimeProfiles()
    runtimeProfiles.value = Array.isArray(data) ? data : []
  } catch {
    runtimeProfiles.value = []
  }
}

const runtimeProfilesBy = (runtime) => runtimeProfiles.value.filter(profile => profile.runtime === runtime && profile.enabled)

const runtimeProfileLabel = (profile) => `${profile.name || profile.id}${profile.default ? '（默认）' : ''}`

const displayRuntimeProfile = (profileID) => {
  const id = String(profileID || '').trim()
  if (!id) return '默认'
  const profile = runtimeProfiles.value.find(item => item.id === id)
  return profile ? runtimeProfileLabel(profile) : id
}

const normalizeListResult = (result) => {
  if (Array.isArray(result)) return result
  if (Array.isArray(result?.items)) return result.items
  if (Array.isArray(result?.apis)) return result.apis
  if (Array.isArray(result?.open_apis)) return result.open_apis
  if (Array.isArray(result?.openApis)) return result.openApis
  return []
}

const normalizeMethod = (method) => String(method || 'POST').toUpperCase()

const normalizePath = (path) => String(path || '').replace(/\\/g, '/').replace(/^\/api\/open\//, '').replace(/^\/+|\/+$/g, '').trim()

const normalizeRuntime = (runtime) => runtime === 'python' ? 'python' : runtime === 'builtin' ? 'builtin' : 'nodejs'

const runtimeLabel = (runtime) => {
  const normalized = normalizeRuntime(runtime)
  if (normalized === 'python') return 'Python'
  if (normalized === 'builtin') return '内置'
  return 'Node.js'
}

const resolvePath = (row) => row.path || row.url_path || row.urlPath || row.route || ''

const displayPath = (row) => {
  const normalized = normalizePath(resolvePath(row))
  return normalized ? `/api/open/${normalized}` : '/api/open'
}

const hasToken = (row) => {
  if (typeof row.has_token === 'boolean') return row.has_token
  if (typeof row.hasToken === 'boolean') return row.hasToken
  if (typeof row.token_set === 'boolean') return row.token_set
  if (typeof row.tokenSet === 'boolean') return row.tokenSet
  return Boolean(String(row.token || '').trim())
}

const normalizeSummary = (stats = {}) => ({
  total: Number(stats?.total ?? stats?.total_count ?? 0),
  success: Number(stats?.success ?? stats?.success_count ?? 0),
  rejected: Number(stats?.rejected ?? stats?.rejected_count ?? 0),
  failed: Number(stats?.failed ?? stats?.failed_count ?? 0),
  last_called_at: stats?.last_called_at || stats?.lastCalledAt || ''
})

const callStats = (row) => normalizeSummary(row?.call_stats || row?.callStats || {})

const configuredWhitelist = (row) => Array.isArray(row?.ip_whitelist)
  ? row.ip_whitelist
  : Array.isArray(row?.ipWhitelist) ? row.ipWhitelist : null
const effectiveWhitelist = (row) => Array.isArray(row?.effective_ip_whitelist)
  ? row.effective_ip_whitelist
  : Array.isArray(row?.effectiveIpWhitelist) ? row.effectiveIpWhitelist : []

const ipPolicyMode = (row) => {
  const mode = String(row?.ip_whitelist_mode || row?.ipWhitelistMode || '').trim()
  if (['inherit', 'allow_all', 'custom'].includes(mode)) return mode
  const configured = configuredWhitelist(row)
  if (!configured?.length) return 'inherit'
  return configured.length === 1 && configured[0] === '*' ? 'allow_all' : 'custom'
}

const ipPolicyLabel = (row) => {
  const mode = ipPolicyMode(row)
  if (mode === 'allow_all') return '允许全部'
  if (mode === 'custom') return `自定义（${configuredWhitelist(row)?.length || 0}）`
  return `继承${ipPolicySourceLabel(row)}`
}

const ipPolicySourceLabel = (row) => {
  const source = String(row?.ip_whitelist_source || row?.ipWhitelistSource || '').trim()
  if (source === 'default') return '默认'
  if (source === 'endpoint') return '单接口'
  return '全局'
}

const ipPolicyTagType = (row) => ipPolicyMode(row) === 'custom' ? 'warning' : ipPolicyMode(row) === 'allow_all' ? 'success' : 'info'
const formatCount = (value) => Math.max(0, Number(value) || 0).toLocaleString('zh-CN')

const isBuiltin = (row) => Boolean(String(row.builtin || '').trim())

const itemId = (row) => String(row.id || row.api_id || row.apiId || normalizePath(resolvePath(row)) || '')

const getOpenApiSearchText = (row) => [
  itemId(row),
  row.name,
  displayPath(row),
  normalizeMethod(row.method),
  runtimeLabel(row.runtime),
  row.runtime,
  row.runtime_profile,
  displayRuntimeProfile(row.runtime_profile),
  row.description,
  hasToken(row) ? '已设置 token' : '未设置 token',
  ipPolicyLabel(row),
  effectiveWhitelist(row).join(' '),
  formatCount(callStats(row).total),
  row.enabled ? '启用 开启' : '停用 关闭',
  row.builtin
].filter(value => value !== undefined && value !== null).join(' ').toLowerCase()

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '开放接口说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const createItem = () => {
  dialogMode.value = 'create'
  editingId.value = ''
  Object.assign(form, createEmptyForm())
  dialogVisible.value = true
}

const editItem = (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法编辑')
    return
  }
  dialogMode.value = 'edit'
  editingId.value = id
  Object.assign(form, {
    name: row.name || id,
    path: normalizePath(resolvePath(row)) || id,
    enabled: Boolean(row.enabled),
    runtime: normalizeRuntime(row.runtime),
    runtime_profile: row.runtime_profile || '',
    token: '',
    ipWhitelistMode: ipPolicyMode(row),
    ipWhitelist: ipPolicyMode(row) === 'custom' ? [...(configuredWhitelist(row) || [])] : [],
    description: row.description || '',
    method: normalizeMethod(row.method)
  })
  dialogVisible.value = true
}

const openFile = (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法打开文件')
    return
  }
  router.push(`/open-apis/${encodeURIComponent(id)}/edit`)
}

const handleRowCommand = (command, row) => {
  if (command === 'file') return openFile(row)
  if (command === 'copy') return copyAddress(row)
  if (command === 'delete') return deleteItem(row)
}

const openSettings = async () => {
  const requestVersion = ++settingsRequestVersion
  settingsVisible.value = true
  settingsLoaded.value = false
  settingsLoading.value = true
  try {
    const data = await getOpenApiSettings()
    if (requestVersion !== settingsRequestVersion || !settingsVisible.value) return
    const ipWhitelist = normalizeStringList(data?.ip_whitelist ?? data?.ipWhitelist)
    Object.assign(settingsForm, {
      ipWhitelistMode: ipWhitelist.length === 0 || (ipWhitelist.length === 1 && ipWhitelist[0] === '*') ? 'allow_all' : 'custom',
      ipWhitelist: ipWhitelist.includes('*') ? [] : ipWhitelist,
      trustedProxies: normalizeStringList(data?.trusted_proxies ?? data?.trustedProxies),
      callLogRetentionDays: Math.max(0, Number(data?.call_log_retention_days ?? data?.callLogRetentionDays ?? 30) || 0)
    })
    settingsLoaded.value = true
  } finally {
    if (requestVersion === settingsRequestVersion) settingsLoading.value = false
  }
}

const saveSettings = async () => {
  const ipWhitelist = settingsForm.ipWhitelistMode === 'allow_all' ? ['*'] : normalizeStringList(settingsForm.ipWhitelist)
  if (settingsForm.ipWhitelistMode === 'custom' && ipWhitelist.length === 0) {
    ElMessage.warning('自定义全局白名单至少需要填写一项')
    return
  }
  if (settingsForm.ipWhitelistMode === 'custom' && ipWhitelist.includes('*')) {
    ElMessage.warning('自定义全局白名单不能混用 *，如需全放行请选择“允许全部”')
    return
  }
  settingsSaving.value = true
  try {
    await saveOpenApiSettings({
      ip_whitelist: ipWhitelist,
      trusted_proxies: normalizeStringList(settingsForm.trustedProxies),
      call_log_retention_days: Math.max(0, Number(settingsForm.callLogRetentionDays) || 0)
    })
    ElMessage.success('开放接口全局 IP 设置已保存')
    settingsVisible.value = false
    await loadItems()
  } finally {
    settingsSaving.value = false
  }
}

const openCalls = (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法查看调用数据')
    return
  }
  callsRequestVersion++
  callsTarget.value = row
  callsVisible.value = true
  callsPage.value = 1
  Object.assign(callsFilters, createEmptyCallFilters())
  Object.assign(callsSummary, callStats(row))
  callItems.value = []
  callsTotal.value = 0
  loadCalls()
}

const loadCalls = async () => {
  const targetId = itemId(callsTarget.value || {})
  if (!targetId || !callsVisible.value) return
  const requestVersion = ++callsRequestVersion
  callsLoading.value = true
  try {
    const data = await getOpenApiCalls(targetId, buildCallParams())
    if (requestVersion !== callsRequestVersion || targetId !== itemId(callsTarget.value || {}) || !callsVisible.value) return
    callItems.value = Array.isArray(data?.items) ? data.items : []
    callsTotal.value = Math.max(0, Number(data?.total) || 0)
    callsRetentionDays.value = Math.max(0, Number(data?.retention_days ?? data?.retentionDays ?? 30) || 0)
    Object.assign(callsSummary, normalizeSummary(data?.summary))
    const maxPage = Math.max(1, Math.ceil(callsTotal.value / callsPageSize.value))
    if (callsPage.value > maxPage) {
      callsPage.value = maxPage
      return loadCalls()
    }
  } finally {
    if (requestVersion === callsRequestVersion) callsLoading.value = false
  }
}

const buildCallParams = () => {
  const params = {
    limit: callsPageSize.value,
    offset: (callsPage.value - 1) * callsPageSize.value
  }
  if (callsFilters.outcome) params.outcome = callsFilters.outcome
  if (callsFilters.clientIp.trim()) params.client_ip = callsFilters.clientIp.trim()
  if (String(callsFilters.statusCode).trim()) params.status_code = String(callsFilters.statusCode).trim()
  if (Array.isArray(callsFilters.timeRange) && callsFilters.timeRange.length === 2) {
    params.start = toApiTime(callsFilters.timeRange[0])
    params.end = toApiTime(callsFilters.timeRange[1])
  }
  return params
}

const searchCalls = () => {
  callsPage.value = 1
  loadCalls()
}

const resetCallFilters = () => {
  Object.assign(callsFilters, createEmptyCallFilters())
  searchCalls()
}

const handleCallsPageSize = () => {
  callsPage.value = 1
  loadCalls()
}

const resetCalls = () => {
  callsRequestVersion++
  callsLoading.value = false
  callsTarget.value = null
  callItems.value = []
  callsTotal.value = 0
  Object.assign(callsSummary, createEmptySummary())
}

const normalizeStringList = (values) => Array.from(new Set(
  (Array.isArray(values) ? values : [])
    .map(value => String(value || '').trim())
    .filter(Boolean)
))

const toApiTime = (value) => {
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

const formatTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString('zh-CN', { hour12: false })
}

const displayCallPath = (row) => {
  const path = normalizePath(row?.endpoint_path || row?.endpointPath || row?.path || '')
  return path ? `/api/open/${path}` : '/api/open'
}

const outcomeLabel = (outcome) => ({ success: '成功', ip_denied: 'IP 拒绝', token_denied: 'Token 拒绝', failed: '失败' }[outcome] || outcome || '-')
const outcomeTagType = (outcome) => outcome === 'success' ? 'success' : outcome === 'failed' ? 'danger' : 'warning'
const formatDuration = (value) => `${Math.max(0, Number(value) || 0).toLocaleString('zh-CN')} ms`

const saveDialog = async () => {
  const payload = buildPayload()
  if (!payload) return

  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await createOpenApi(payload)
      ElMessage.success('开放接口已创建')
    } else {
      await updateOpenApi(editingId.value, payload)
      ElMessage.success('开放接口已保存')
    }
    dialogVisible.value = false
    await loadItems()
  } finally {
    saving.value = false
  }
}

const buildPayload = () => {
  const name = String(form.name || '').trim()
  const validation = validatePath(form.path)
  if (!name) {
    ElMessage.warning('请输入接口名称')
    return null
  }
  if (!validation.ok) {
    ElMessage.warning(validation.message)
    return null
  }
  if (isPathDuplicated(validation.path)) {
    ElMessage.warning('接口路径已存在，请换一个路径名')
    return null
  }

  const runtime = normalizeRuntime(form.runtime)
  const token = String(form.token || '').trim()
  const currentItem = dialogMode.value === 'edit' ? items.value.find((row) => itemId(row) === editingId.value) : null
  if (!token && (dialogMode.value === 'create' || !hasToken(currentItem || {}) || Boolean(form.enabled))) {
    ElMessage.warning('请输入 Open API token')
    return null
  }
  const ipWhitelist = buildEndpointWhitelist()
  if (ipWhitelist === undefined) return null
  const payload = {
    id: dialogMode.value === 'create' ? validation.path : editingId.value,
    name,
    path: validation.path,
    method: normalizeMethod(form.method),
    enabled: Boolean(form.enabled),
    runtime,
    runtime_profile: runtime === 'builtin' ? '' : String(form.runtime_profile || '').trim(),
    ip_whitelist: ipWhitelist,
    description: String(form.description || '').trim(),
    entry: codeFileName(validation.path, runtime)
  }
  if (dialogMode.value === 'create' || token) payload.token = token
  return payload
}

const buildEndpointWhitelist = () => {
  if (form.ipWhitelistMode === 'inherit') return null
  if (form.ipWhitelistMode === 'allow_all') return ['*']
  const values = normalizeStringList(form.ipWhitelist)
  if (values.length === 0) {
    ElMessage.warning('自定义 IP 白名单至少需要填写一项')
    return undefined
  }
  if (values.includes('*')) {
    ElMessage.warning('自定义 IP 白名单不能混用 *，如需全放行请选择“允许全部”')
    return undefined
  }
  return values
}

const validatePath = (value) => {
  const path = String(value || '').trim()
  if (!path) return { ok: false, message: '请输入接口路径' }
  if (path.includes('/') || path.includes('\\')) {
    return { ok: false, message: '接口路径只支持单个词，例如 a，不能输入 a/b、/a 或 /api/open/a' }
  }
  if (!singlePathPattern.test(path)) {
    return { ok: false, message: '接口路径只能包含字母、数字、横线和下划线' }
  }
  return { ok: true, path }
}

const isPathDuplicated = (path) => {
  const currentId = editingId.value
  const normalized = path.toLowerCase()
  return items.value.some((row) => {
    if (dialogMode.value === 'edit' && itemId(row) === currentId) return false
    return normalizePath(resolvePath(row)).toLowerCase() === normalized
  })
}

const codeFileName = (path, runtime) => `${path}.${normalizeRuntime(runtime) === 'python' ? 'py' : 'js'}`

function createEmptyForm() {
  return {
    name: '',
    path: '',
    enabled: true,
    runtime: 'nodejs',
    runtime_profile: '',
    token: '',
    ipWhitelistMode: 'inherit',
    ipWhitelist: [],
    description: '',
    method: 'POST'
  }
}

function createEmptySettingsForm() {
  return {
    ipWhitelistMode: 'allow_all',
    ipWhitelist: [],
    trustedProxies: [],
    callLogRetentionDays: 30
  }
}

function createEmptySummary() {
  return { total: 0, success: 0, rejected: 0, failed: 0, last_called_at: '' }
}

function createEmptyCallFilters() {
  return { outcome: '', clientIp: '', statusCode: '', timeRange: [] }
}

const deleteItem = async (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法删除')
    return
  }
  try {
    await ElMessageBox.confirm(`确定删除开放接口「${row.name || id}」吗？`, '删除确认', { type: 'warning' })
  } catch {
    return
  }
  await deleteOpenApi(id)
  ElMessage.success('开放接口已删除')
  await loadItems()
}

const copyAddress = async (row) => {
  const address = displayPath(row)
  const url = `${window.location.origin}${address}`
  try {
    await navigator.clipboard.writeText(url)
  } catch {
    copyByFallback(url)
  }
  ElMessage.success('接口地址已复制')
}

const copyByFallback = (text) => {
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

onMounted(() => {
  loadRuntimeProfiles()
  loadItems()
})
</script>

<style scoped>
.open-apis-page {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.page-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.page-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.title {
  font-size: 18px;
  font-weight: 600;
}

.mobile-info-button {
  display: none;
  padding: 0;
  font-size: 16px;
}

.subtitle {
  margin-top: 6px;
  color: #909399;
  font-size: 13px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-search {
  width: 280px;
}

.api-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-bottom: 12px;
}

.api-table-area {
  height: 100%;
  min-height: 0;
}

.api-grid.mobile-api-grid {
  display: none;
}

.api-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.api-card-item {
  min-width: 0;
  min-height: 260px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  background: #fff;
  transition: box-shadow 0.2s;
}

.api-card-item:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.api-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
}

.api-name {
  min-width: 0;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  word-break: break-all;
}

.api-card-body {
  flex: 1;
}

.api-info-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 13px;
  color: #606266;
}

.api-info-row .label {
  min-width: 70px;
  flex-shrink: 0;
  color: #909399;
}

.value-text {
  min-width: 0;
  word-break: break-all;
}

.api-table code,
.path-text {
  max-width: 100%;
  padding: 4px 8px;
  border-radius: 6px;
  color: #1d4ed8;
  background: #eff6ff;
  font-family: "JetBrains Mono", "Cascadia Code", monospace;
  word-break: break-all;
  white-space: normal;
}

.api-card-footer {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid #f0f0f0;
}

.api-card-footer .el-button,
.mobile-more {
  width: 100%;
  margin-left: 0;
}

.mobile-more :deep(.el-button) {
  width: 100%;
}

.dialog-form {
  padding: 2px 4px 0;
}

.field-tip {
  width: 100%;
  margin-top: 6px;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
}

.ip-value-input {
  width: 100%;
  margin-top: 10px;
}

.settings-alert {
  margin-bottom: 18px;
  line-height: 1.6;
}

.settings-form {
  padding: 0 4px;
}

.calls-body {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(120px, 1fr)) minmax(200px, 1.5fr);
  gap: 12px;
}

.stats-grid > div {
  min-width: 0;
  padding: 14px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  background: #fafafa;
}

.stats-grid span {
  display: block;
  margin-bottom: 7px;
  color: #909399;
  font-size: 12px;
}

.stats-grid strong {
  color: #303133;
  font-size: 20px;
  word-break: break-all;
}

.stats-grid > div:last-child strong {
  font-size: 14px;
}

.success-text { color: #67c23a !important; }
.warning-text { color: #e6a23c !important; }
.danger-text { color: #f56c6c !important; }

.calls-filters {
  display: grid;
  grid-template-columns: 150px minmax(180px, 1fr) 140px minmax(320px, 1.6fr) auto auto;
  gap: 10px;
}

.calls-table-wrap {
  flex: 1;
  min-height: 260px;
}

.calls-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  color: #909399;
  font-size: 12px;
}

.calls-footer :deep(.std-pagination) {
  padding-top: 0;
  border-top: 0;
}

.api-dialog :deep(.el-dialog__header),
.settings-dialog :deep(.el-dialog__header) {
  padding-bottom: 12px;
  border-bottom: 1px solid #edf0f5;
}

.api-dialog :deep(.el-dialog__footer),
.settings-dialog :deep(.el-dialog__footer) {
  padding-top: 12px;
  border-top: 1px solid #edf0f5;
}

@media (max-width: 768px) {
  .open-apis-page {
    height: calc(100dvh - 52px - 76px - 24px);
    min-height: 0;
    overflow: hidden;
  }

  .page-card {
    height: 100%;
    min-height: 100%;
  }

  .page-card :deep(.el-card__body) {
    overflow: hidden;
  }

  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .title {
    font-size: 16px;
  }

  .mobile-info-button {
    display: inline-flex;
  }

  .subtitle {
    display: none;
  }

  .header-actions {
    width: 100%;
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .header-search,
  .header-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .api-content {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    padding-bottom: 12px;
  }

  .desktop-api-table {
    display: none;
  }

  .api-grid.mobile-api-grid {
    display: grid;
  }

  .api-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }

  .api-card-item {
    min-height: auto;
    padding: 14px;
  }

  .api-info-row {
    gap: 6px;
  }

  .api-info-row .label {
    min-width: 66px;
  }

  .api-card-footer {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .api-dialog :deep(.el-dialog),
  .settings-dialog :deep(.el-dialog) {
    width: 94vw !important;
  }

  .api-dialog :deep(.el-form-item),
  .settings-dialog :deep(.el-form-item) {
    display: block;
  }

  .api-dialog :deep(.el-form-item__label),
  .settings-dialog :deep(.el-form-item__label) {
    width: 100% !important;
    justify-content: flex-start;
    padding: 0 0 6px;
  }

  .api-dialog :deep(.el-form-item__content),
  .settings-dialog :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }

  :global(.calls-drawer) {
    width: 96vw !important;
  }

  :global(.calls-drawer .el-drawer__header) {
    margin-bottom: 12px;
  }

  :global(.calls-drawer .el-drawer__body) {
    padding: 0 12px 12px;
    overflow: hidden;
  }

  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .stats-grid > div:last-child {
    grid-column: 1 / -1;
  }

  .stats-grid > div {
    padding: 10px;
  }

  .stats-grid strong {
    font-size: 17px;
  }

  .calls-filters {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  .calls-filters .el-date-editor {
    grid-column: 1 / -1;
    width: 100%;
  }

  .calls-table-wrap {
    min-height: 240px;
    overflow-x: auto;
  }

  .calls-table-wrap :deep(.el-table) {
    min-width: 900px;
  }

  .calls-footer {
    align-items: flex-start;
    flex-direction: column;
    gap: 6px;
  }

  .calls-footer :deep(.std-pagination) {
    width: 100%;
  }

  .api-dialog :deep(.el-input-group__prepend) {
    padding: 0 10px;
  }

  .api-content::-webkit-scrollbar {
    display: none;
  }
}
</style>
