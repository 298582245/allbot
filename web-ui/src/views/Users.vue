<template>
  <div class="users-page">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">用户管理</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看用户管理说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">统一查看跨平台身份、积分与账号状态。</div>
          </div>
          <el-button :loading="activeLoading" @click="refreshActiveView">刷新</el-button>
        </div>
      </template>

      <el-tabs v-model="activeView" class="view-tabs">
        <el-tab-pane label="按 UnionID" name="union">
          <div class="toolbar union-toolbar">
            <el-input v-model="unionQuery.keyword" clearable placeholder="搜索 UnionID 或平台用户 ID" @keyup.enter="searchUnionUsers" />
            <el-select v-model="unionQuery.platform" clearable placeholder="全部平台" @change="searchUnionUsers">
              <el-option v-for="item in platformOptions" :key="item.platform" :label="item.displayName" :value="item.platform" />
            </el-select>
            <el-select v-model="unionQuery.disabled" clearable placeholder="全部状态" @change="searchUnionUsers">
              <el-option label="正常" :value="false" />
              <el-option label="已封禁" :value="true" />
            </el-select>
            <el-button type="primary" @click="searchUnionUsers">查询</el-button>
          </div>

          <div class="desktop-table table-wrap" v-loading="unionLoading">
            <el-table :data="unionUsers" border stripe height="100%" empty-text="暂无用户">
              <el-table-column prop="union_id" label="UnionID" min-width="210" show-overflow-tooltip />
              <el-table-column label="积分" width="105" align="right">
                <template #default="{ row }">{{ formatPoints(row.points) }}</template>
              </el-table-column>
              <el-table-column label="平台" min-width="190">
                <template #default="{ row }">
                  <div class="tag-list">
                    <el-tag v-for="platform in row.platforms" :key="platform" size="small" effect="plain">{{ platformDisplayName(platform) }}</el-tag>
                    <span v-if="!row.platforms?.length" class="muted">无平台账号</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="平台 / 账号" width="110" align="center">
                <template #default="{ row }">{{ Number(row.platform_count || 0) }} / {{ accountCount(row) }}</template>
              </el-table-column>
              <el-table-column label="状态" width="125">
                <template #default="{ row }">
                  <div class="tag-list vertical">
                    <el-tag :type="statusTagType(row)">{{ statusText(row) }}</el-tag>
                    <el-tag v-if="hasDuplicate(row)" size="small" type="warning">绑定异常</el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="更新时间" min-width="170">
                <template #default="{ row }">{{ formatTime(row.updated_at || row.created_at) }}</template>
              </el-table-column>
              <el-table-column label="操作" width="230" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openDetail(row)">详情</el-button>
                  <el-button link type="warning" @click="openPointsDialog(row)">调整积分</el-button>
                  <el-button link :type="row.disabled ? 'success' : 'danger'" :loading="statusUnionId === row.union_id" @click="toggleUserStatus(row)">
                    {{ row.disabled ? '解除封禁' : '整体封禁' }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="mobile-card-list" v-loading="unionLoading">
            <article v-for="row in unionUsers" :key="row.union_id" class="mobile-user-card">
              <div class="card-head">
                <strong>{{ row.union_id }}</strong>
                <div class="tag-list"><el-tag size="small" :type="statusTagType(row)">{{ statusText(row) }}</el-tag><el-tag v-if="hasDuplicate(row)" size="small" type="warning">绑定异常</el-tag></div>
              </div>
              <div class="platform-strip"><el-tag v-for="platform in row.platforms" :key="platform" size="small" effect="plain">{{ platformDisplayName(platform) }}</el-tag></div>
              <div class="card-fields">
                <div><span>积分</span><b>{{ formatPoints(row.points) }}</b></div>
                <div><span>平台 / 账号</span><b>{{ Number(row.platform_count || 0) }} / {{ accountCount(row) }}</b></div>
                <div><span>更新时间</span><b>{{ formatTime(row.updated_at || row.created_at) }}</b></div>
              </div>
              <div class="card-actions">
                <el-button size="small" type="primary" @click="openDetail(row)">详情</el-button>
                <el-button size="small" type="warning" @click="openPointsDialog(row)">积分</el-button>
                <el-button size="small" :type="row.disabled ? 'success' : 'danger'" :loading="statusUnionId === row.union_id" @click="toggleUserStatus(row)">{{ row.disabled ? '解封' : '封禁' }}</el-button>
              </div>
            </article>
            <el-empty v-if="!unionLoading && unionUsers.length === 0" description="暂无用户" />
          </div>

          <StdPagination v-model:current-page="unionPage" v-model:page-size="unionPageSize" :total="unionTotal" :page-sizes="[10, 20, 50, 100]" @current-change="loadUnionUsers" @size-change="handleUnionPageSize" />
        </el-tab-pane>

        <el-tab-pane label="按平台" name="platform">
          <div class="toolbar platform-toolbar">
            <el-input v-model="platformQuery.keyword" clearable placeholder="搜索平台用户 ID 或 UnionID" @keyup.enter="searchPlatformAccounts" />
            <el-select v-model="platformQuery.disabled" clearable placeholder="全部状态" @change="searchPlatformAccounts">
              <el-option label="正常" :value="false" />
              <el-option label="已封禁" :value="true" />
            </el-select>
            <el-button type="primary" @click="searchPlatformAccounts">查询</el-button>
          </div>

          <el-select v-model="selectedPlatform" class="mobile-platform-select" placeholder="选择平台" @change="changePlatform">
            <el-option v-for="item in platformOptions" :key="item.platform" :label="item.displayName" :value="item.platform" />
          </el-select>

          <div class="platform-workspace">
            <aside class="platform-master">
              <button v-for="item in platformOptions" :key="item.platform" type="button" :class="{ active: selectedPlatform === item.platform }" @click="changePlatform(item.platform)">
                <span>{{ item.displayName }}</span>
              </button>
              <el-empty v-if="platformOptions.length === 0" :image-size="48" description="暂无平台" />
            </aside>
            <div class="platform-detail">
              <div class="desktop-table table-wrap" v-loading="accountLoading">
                <el-table :data="platformAccounts" border stripe height="100%" empty-text="请选择平台或该平台暂无账号">
                  <el-table-column prop="user_id" label="平台用户 ID" min-width="190" show-overflow-tooltip />
                  <el-table-column prop="union_id" label="UnionID" min-width="210" show-overflow-tooltip />
                  <el-table-column label="积分" width="110" align="right"><template #default="{ row }">{{ formatPoints(row.points) }}</template></el-table-column>
                  <el-table-column label="状态" width="125">
                    <template #default="{ row }"><div class="tag-list vertical"><el-tag :type="statusTagType(row)">{{ statusText(row) }}</el-tag><el-tag v-if="row.duplicate_platform" size="small" type="warning">绑定异常</el-tag></div></template>
                  </el-table-column>
                  <el-table-column label="更新时间" min-width="170"><template #default="{ row }">{{ formatTime(row.updated_at || row.created_at) }}</template></el-table-column>
                  <el-table-column label="操作" width="90" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">详情</el-button></template></el-table-column>
                </el-table>
              </div>
              <div class="mobile-card-list" v-loading="accountLoading">
                <article v-for="row in platformAccounts" :key="accountKey(row)" class="mobile-user-card">
                  <div class="card-head"><strong>{{ row.user_id || '-' }}</strong><div class="tag-list"><el-tag size="small" :type="statusTagType(row)">{{ statusText(row) }}</el-tag><el-tag v-if="row.duplicate_platform" size="small" type="warning">绑定异常</el-tag></div></div>
                  <div class="card-fields"><div><span>平台</span><b>{{ platformDisplayName(row.platform) }}</b></div><div><span>UnionID</span><b>{{ row.union_id }}</b></div><div><span>积分</span><b>{{ formatPoints(row.points) }}</b></div></div>
                  <div class="card-actions single"><el-button size="small" type="primary" @click="openDetail(row)">查看 UnionID 详情</el-button></div>
                </article>
                <el-empty v-if="!accountLoading && platformAccounts.length === 0" description="暂无平台账号" />
              </div>
              <StdPagination v-model:current-page="accountPage" v-model:page-size="accountPageSize" :total="accountTotal" :page-sizes="[10, 20, 50, 100]" @current-change="loadPlatformAccounts" @size-change="handleAccountPageSize" />
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-drawer v-model="detailVisible" title="用户详情" size="62%" class="user-detail-drawer" @closed="resetDetail">
      <div v-loading="detailLoading" class="detail-body">
        <section class="detail-section">
          <div class="section-head"><h3>用户信息</h3><div v-if="detailUser"><el-button type="warning" @click="openPointsDialog(detailUser)">调整积分</el-button><el-button :type="detailUser.disabled ? 'success' : 'danger'" @click="toggleUserStatus(detailUser)">{{ detailUser.disabled ? '解除封禁' : '整体封禁' }}</el-button></div></div>
          <div v-if="detailUser" class="detail-grid"><div><span>UnionID</span><b>{{ detailUser.union_id }}</b></div><div><span>积分</span><b>{{ formatPoints(detailUser.points) }}</b></div><div><span>整体状态</span><b>{{ statusText(detailUser) }}</b></div><div><span>平台 / 账号</span><b>{{ Number(detailUser.platform_count || 0) }} / {{ accountCount(detailUser) }}</b></div><div><span>创建时间</span><b>{{ formatTime(detailUser.created_at) }}</b></div><div><span>更新时间</span><b>{{ formatTime(detailUser.updated_at) }}</b></div></div>
          <div v-if="hasDuplicate(detailUser)" class="anomaly-note">检测到同一平台绑定了多个账号：{{ duplicatePlatformNames(detailUser) }}</div>
        </section>
        <section class="detail-section">
          <h3>全部平台账号</h3>
          <div class="account-list"><div v-for="account in detailAccounts" :key="accountKey(account)" class="account-item"><div><div class="account-title"><strong>{{ platformDisplayName(account.platform) }}</strong><el-tag v-if="account.duplicate_platform" size="small" type="warning">绑定异常</el-tag></div><span>平台用户 ID</span></div><code>{{ account.user_id || '-' }}</code></div><el-empty v-if="detailAccounts.length === 0" description="暂无关联账号" /></div>
        </section>
        <section class="detail-section transaction-section">
          <h3>积分流水</h3>
          <el-table :data="transactions" border empty-text="暂无积分流水">
            <el-table-column label="时间" min-width="165"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
            <el-table-column label="变动" width="100" align="right"><template #default="{ row }"><span :class="Number(row.delta) >= 0 ? 'points-plus' : 'points-minus'">{{ formatDelta(row.delta) }}</span></template></el-table-column>
            <el-table-column prop="balance_after" label="变动后" width="100" align="right" />
            <el-table-column prop="source" label="来源" min-width="140" />
            <el-table-column prop="description" label="说明" min-width="180" show-overflow-tooltip />
          </el-table>
          <StdPagination v-model:current-page="transactionPage" :page-size="transactionPageSize" :total="transactionTotal" @current-change="() => loadTransactions()" />
        </section>
      </div>
    </el-drawer>

    <el-dialog v-model="pointsDialogVisible" title="调整用户积分" width="520px" class="points-dialog">
      <el-form :model="pointsForm" label-width="90px">
        <el-form-item label="UnionID"><el-input :model-value="pointsTarget?.union_id" disabled /></el-form-item>
        <el-form-item label="当前积分"><strong>{{ formatPoints(pointsTarget?.points) }}</strong></el-form-item>
        <el-form-item label="调整数量" required><el-input-number v-model="pointsForm.delta" :min="-999999999" :max="999999999" :step="1" controls-position="right" /><div class="field-tip">正数增加积分，负数扣减积分，不能填写 0。</div></el-form-item>
        <el-form-item label="调整说明"><el-input v-model="pointsForm.description" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="可填写本次调整的原因" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="pointsDialogVisible = false">取消</el-button><el-button type="primary" :loading="pointsSaving" @click="submitPointsAdjustment">确认调整</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'Users' })
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import { adjustUserPoints, getAdapterPlatforms, getUser, getUserAccounts, getUserPointTransactions, getUsers, updateUserStatus } from '@/api'
import StdPagination from '@/components/StdPagination.vue'

const activeView = ref('union')
const unionLoading = ref(false), accountLoading = ref(false), detailLoading = ref(false)
const unionUsers = ref([]), platformAccounts = ref([]), adapterPlatforms = ref([])
const unionPage = ref(1), unionPageSize = ref(20), unionTotal = ref(0)
const accountPage = ref(1), accountPageSize = ref(20), accountTotal = ref(0)
const unionQuery = reactive({ keyword: '', platform: '', disabled: '' })
const platformQuery = reactive({ keyword: '', disabled: '' })
const selectedPlatform = ref('')
const statusUnionId = ref('')
const detailVisible = ref(false), detailUser = ref(null), detailAccounts = ref([])
const transactions = ref([]), transactionPage = ref(1), transactionPageSize = 20, transactionTotal = ref(0)
const pointsDialogVisible = ref(false), pointsSaving = ref(false), pointsTarget = ref(null)
const pointsForm = reactive({ delta: 1, description: '' })
let unionRequestId = 0
let accountRequestId = 0
let detailRequestId = 0
const activeLoading = computed(() => activeView.value === 'union' ? unionLoading.value : accountLoading.value)
const platformOptions = computed(() => adapterPlatforms.value
  .map(item => ({ platform: String(item.platform || ''), displayName: String(item.display_name || item.displayName || item.platform || '') }))
  .filter(item => item.platform))
const platformNameMap = computed(() => Object.fromEntries(platformOptions.value.map(item => [item.platform, item.displayName])))

async function loadUnionUsers() {
  const requestId = ++unionRequestId
  unionLoading.value = true
  try {
    const data = await getUsers(buildParams(unionQuery, unionPage.value, unionPageSize.value))
    if (requestId !== unionRequestId) return
    unionUsers.value = resultItems(data)
    unionTotal.value = resultTotal(data, unionUsers.value)
    if (repairPage(unionPage, unionPageSize.value, unionTotal.value)) return loadUnionUsers()
  } finally {
    if (requestId === unionRequestId) unionLoading.value = false
  }
}

async function loadPlatformAccounts() {
  const requestId = ++accountRequestId
  const platform = selectedPlatform.value
  if (!platform) {
    platformAccounts.value = []
    accountTotal.value = 0
    return
  }
  accountLoading.value = true
  try {
    const data = await getUserAccounts({
      ...buildParams(platformQuery, accountPage.value, accountPageSize.value),
      platform
    })
    if (requestId !== accountRequestId || platform !== selectedPlatform.value) return
    platformAccounts.value = resultItems(data)
    accountTotal.value = resultTotal(data, platformAccounts.value)
    if (repairPage(accountPage, accountPageSize.value, accountTotal.value)) return loadPlatformAccounts()
  } finally {
    if (requestId === accountRequestId) accountLoading.value = false
  }
}

function searchUnionUsers() {
  unionPage.value = 1
  loadUnionUsers()
}

function searchPlatformAccounts() {
  accountPage.value = 1
  loadPlatformAccounts()
}

function changePlatform(platform) {
  selectedPlatform.value = platform
  searchPlatformAccounts()
}

function handleUnionPageSize() {
  unionPage.value = 1
  loadUnionUsers()
}

function handleAccountPageSize() {
  accountPage.value = 1
  loadPlatformAccounts()
}

function refreshActiveView() {
  return activeView.value === 'union' ? loadUnionUsers() : loadPlatformAccounts()
}

async function openDetail(row) {
  const unionId = String(row?.union_id || '').trim()
  if (!unionId) return ElMessage.warning('该账号尚未关联 UnionID')
  const requestId = ++detailRequestId
  detailVisible.value = true
  detailLoading.value = true
  transactionPage.value = 1
  try {
    const detail = await getUser(unionId)
    if (requestId !== detailRequestId) return
    detailUser.value = detail
    detailAccounts.value = Array.isArray(detail?.accounts) ? detail.accounts : []
    await loadTransactions(unionId, requestId)
  } finally {
    if (requestId === detailRequestId) detailLoading.value = false
  }
}

async function loadTransactions(targetUnionId = detailUser.value?.union_id, requestId = detailRequestId) {
  if (!targetUnionId) return
  const data = await getUserPointTransactions(targetUnionId, {
    limit: transactionPageSize,
    offset: (transactionPage.value - 1) * transactionPageSize
  })
  if (requestId !== detailRequestId || detailUser.value?.union_id !== targetUnionId) return
  transactions.value = resultItems(data)
  transactionTotal.value = resultTotal(data, transactions.value)
  if (repairPage(transactionPage, transactionPageSize, transactionTotal.value)) return loadTransactions(targetUnionId, requestId)
}

function resetDetail() {
  detailRequestId++
  detailUser.value = null
  detailAccounts.value = []
  transactions.value = []
  transactionTotal.value = 0
}

function openPointsDialog(row) {
  pointsTarget.value = row
  pointsForm.delta = 1
  pointsForm.description = ''
  pointsDialogVisible.value = true
}

async function submitPointsAdjustment() {
  const delta = Number(pointsForm.delta)
  const description = pointsForm.description.trim()
  if (!Number.isInteger(delta) || delta === 0) return ElMessage.warning('调整数量必须是非零整数')
  pointsSaving.value = true
  try {
    await adjustUserPoints(pointsTarget.value.union_id, { delta, description })
    ElMessage.success('积分调整成功')
    pointsDialogVisible.value = false
    await refreshAfterChange(pointsTarget.value.union_id)
  } finally {
    pointsSaving.value = false
  }
}

async function toggleUserStatus(row) {
  const unionId = String(row?.union_id || '').trim()
  if (!unionId) return ElMessage.warning('该账号尚未关联 UnionID')
  const disabled = Boolean(row.disabled)
  if (!disabled) {
    await ElMessageBox.confirm(`确定整体封禁用户「${unionId}」吗？该 UnionID 关联的全部平台账号都将被拦截。`, '整体封禁确认', {
      type: 'warning',
      confirmButtonText: '确认封禁',
      cancelButtonText: '取消'
    })
  }
  statusUnionId.value = unionId
  try {
    await updateUserStatus(unionId, { disabled: !disabled })
    ElMessage.success(disabled ? '已解除整体封禁' : '已整体封禁用户')
    await refreshAfterChange(unionId)
  } finally {
    statusUnionId.value = ''
  }
}

async function refreshAfterChange(unionId) {
  await Promise.all([
    loadUnionUsers(),
    activeView.value === 'platform' ? loadPlatformAccounts() : Promise.resolve()
  ])
  if (detailVisible.value && detailUser.value?.union_id === unionId) await openDetail({ union_id: unionId })
}

function buildParams(query, page, size) {
  const params = { limit: size, offset: (page - 1) * size }
  if (query.keyword.trim()) params.keyword = query.keyword.trim()
  if (query.platform) params.platform = query.platform
  if (typeof query.disabled === 'boolean') params.disabled = query.disabled
  return params
}

function resultItems(data) {
  return Array.isArray(data?.items) ? data.items : []
}

function resultTotal(data, items) {
  return Number(data?.total ?? items.length)
}

function repairPage(page, size, total) {
  const lastPage = Math.max(1, Math.ceil(total / size))
  if (page.value <= lastPage) return false
  page.value = lastPage
  return true
}

function statusText(row) {
  return row?.disabled ? '已封禁' : '正常'
}

function statusTagType(row) {
  return row?.disabled ? 'danger' : 'success'
}

function accountCount(row) {
  return Number(row?.account_count ?? row?.accounts?.length ?? 0)
}

function hasDuplicate(row) {
  return Array.isArray(row?.duplicate_platforms) && row.duplicate_platforms.length > 0
}

function duplicatePlatformNames(row) {
  return (row?.duplicate_platforms || []).map(platformDisplayName).join('、')
}

function accountKey(row) {
  return String(row?.id || `${row?.platform || ''}:${row?.user_id || ''}`)
}

function platformDisplayName(platform) {
  return platformNameMap.value[platform] || platform || '-'
}

function formatPoints(value) {
  return Number(value || 0).toLocaleString('zh-CN')
}

function formatDelta(value) {
  const number = Number(value || 0)
  return number > 0 ? `+${number}` : String(number)
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN')
}

function showPageDescription() {
  ElMessageBox.alert('按 UnionID 可管理统一身份；按平台可定位具体账号。封禁始终作用于整个 UnionID。', '用户管理说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

watch(activeView, value => {
  if (value === 'platform' && selectedPlatform.value && platformAccounts.value.length === 0) loadPlatformAccounts()
})

onMounted(async () => {
  const platforms = await getAdapterPlatforms().catch(() => [])
  adapterPlatforms.value = Array.isArray(platforms) ? platforms : []
  selectedPlatform.value = platformOptions.value[0]?.platform || ''
  await loadUnionUsers()
})
</script>

<style scoped>
.users-page { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.page-header, .section-head, .card-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-md); }
.title-row { display: flex; align-items: center; gap: var(--space-xs); }
.title { color: var(--text-primary); font-size: 18px; font-weight: 600; }
.subtitle, .field-tip { margin-top: var(--space-xs); color: var(--text-tertiary); font-size: 12px; line-height: 1.5; }
.mobile-info-button, .mobile-card-list, .mobile-platform-select { display: none; }
.view-tabs { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.view-tabs :deep(.el-tabs__header) { flex-shrink: 0; }
.view-tabs :deep(.el-tabs__content) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.view-tabs :deep(.el-tab-pane) { flex: 1; min-height: 0; display: flex; flex-direction: column; width: 100%; overflow: hidden; }
.toolbar { flex-shrink: 0; display: grid; grid-template-columns: minmax(260px, 1fr) 180px auto; gap: var(--space-sm); margin-bottom: var(--space-md); }
.union-toolbar { grid-template-columns: minmax(240px, 1fr) 160px 160px auto; }
.table-wrap { flex: 1; min-height: 0; overflow: hidden; }
.tag-list, .platform-strip, .account-title { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-xs); }
.tag-list.vertical { align-items: flex-start; flex-direction: column; }
.platform-strip { margin-top: 10px; }
.muted { color: var(--text-tertiary); font-size: 12px; }
.platform-workspace { flex: 1; min-height: 0; display: grid; grid-template-columns: 180px minmax(0, 1fr); gap: var(--space-md); }
.platform-master { min-height: 0; padding: var(--space-sm); overflow-y: auto; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); }
.platform-master button { width: 100%; padding: 10px 12px; border: 0; border-radius: var(--radius-sm); color: var(--text-secondary); background: transparent; text-align: left; cursor: pointer; transition: all var(--transition-fast); }
.platform-master button:hover { background: var(--bg-surface-hover); }
.platform-master button.active { color: var(--brand-600); background: var(--brand-50); font-weight: 600; }
.platform-detail { min-width: 0; min-height: 0; display: flex; flex-direction: column; }
.detail-body { display: grid; gap: var(--space-md); }
.detail-section { padding: var(--space-md); border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); }
.detail-section h3 { margin: 0 0 var(--space-md); color: var(--text-primary); font-size: 15px; }
.section-head h3 { margin: 0; }
.detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-sm); }
.detail-grid > div { min-width: 0; padding: 10px; border-radius: var(--radius-sm); background: var(--bg-surface-hover); }
.detail-grid span, .account-item span { display: block; margin-bottom: var(--space-xs); color: var(--text-tertiary); font-size: 12px; }
.detail-grid b, .account-item code { color: var(--text-primary); word-break: break-all; }
.account-list { display: grid; gap: var(--space-sm); }
.account-item { display: flex; align-items: center; justify-content: space-between; gap: var(--space-md); padding: 10px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); }
.account-title { margin-bottom: var(--space-xs); }
.anomaly-note { margin-top: var(--space-md); padding: 10px 12px; border: 1px solid var(--color-warning); border-radius: var(--radius-sm); color: var(--text-secondary); background: var(--color-warning-light); font-size: 13px; }
.points-plus { color: var(--color-success); font-weight: 600; }
.points-minus { color: var(--color-danger); font-weight: 600; }

@media (max-width: 768px) {
  .users-page { height: auto; min-height: 100%; }
  .page-card { height: auto; min-height: 100%; }
  .page-card :deep(.el-card__header) { padding: 10px 12px; }
  .page-card :deep(.el-card__body) { display: block; overflow: visible; padding: 10px 12px 16px; }
  .title { font-size: 16px; }
  .subtitle { display: none; }
  .mobile-info-button { display: inline-flex; }
  .view-tabs { display: block; min-height: auto; overflow: visible; }
  .view-tabs :deep(.el-tabs__header) { margin-bottom: 10px; }
  .view-tabs :deep(.el-tabs__content) { display: block; min-height: auto; overflow: visible; }
  .view-tabs :deep(.el-tab-pane) { display: block; min-height: auto; overflow: visible; }
  .toolbar, .union-toolbar { grid-template-columns: minmax(0, 1fr) auto; margin-bottom: var(--space-sm); }
  .toolbar .el-select { width: 100%; }
  .toolbar > :first-child { grid-column: 1 / -1; }
  .union-toolbar > :nth-child(2) { grid-column: 1; }
  .union-toolbar > :nth-child(3) { grid-column: 1; }
  .union-toolbar > :last-child { grid-column: 2; grid-row: 2 / 4; height: 100%; }
  .desktop-table, .platform-master { display: none; }
  .mobile-card-list { display: grid; min-height: auto; align-content: start; gap: var(--space-sm); overflow: visible; padding-bottom: var(--space-sm); }
  .mobile-user-card { min-width: 0; padding: 12px; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); box-shadow: var(--shadow-xs); }
  .card-head { align-items: flex-start; }
  .card-head strong { min-width: 0; color: var(--text-primary); word-break: break-all; }
  .card-head .tag-list { flex-shrink: 0; justify-content: flex-end; }
  .card-fields { display: grid; gap: 7px; margin-top: 10px; font-size: 12px; }
  .card-fields > div { display: flex; justify-content: space-between; gap: 10px; min-width: 0; }
  .card-fields span { flex-shrink: 0; color: var(--text-tertiary); }
  .card-fields b { min-width: 0; color: var(--text-primary); text-align: right; word-break: break-all; }
  .card-actions { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--space-sm); margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border-subtle); }
  .card-actions.single { grid-template-columns: 1fr; }
  .card-actions .el-button { width: 100%; margin-left: 0; }
  .mobile-platform-select { display: block; width: 100%; margin-bottom: var(--space-sm); }
  .platform-workspace, .platform-detail { display: block; min-height: auto; }
  :global(.user-detail-drawer) { width: 94vw !important; }
  :global(.points-dialog) { width: 94vw !important; margin-top: 8vh !important; }
  .detail-grid { grid-template-columns: 1fr; }
  .section-head { align-items: flex-start; flex-direction: column; }
  .section-head > div { display: grid; grid-template-columns: 1fr 1fr; width: 100%; gap: var(--space-sm); }
  .section-head .el-button { width: 100%; margin-left: 0; }
  .account-item { align-items: flex-start; flex-direction: column; }
  .transaction-section { overflow-x: auto; }
  .transaction-section .el-table { min-width: 620px; }
  :global(.points-dialog .el-form-item) { display: block; }
  :global(.points-dialog .el-form-item__label) { width: 100% !important; justify-content: flex-start; padding-bottom: var(--space-xs); }
  :global(.points-dialog .el-form-item__content) { margin-left: 0 !important; }
}
</style>
