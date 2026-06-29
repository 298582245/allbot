<template>
  <div class="script-tasks-page page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="card-header">
          <div>
            <div class="title-row">
              <span class="title">脚本任务</span>
              <el-button
                class="mobile-info-button"
                type="primary"
                link
                aria-label="查看脚本任务说明"
                @click="showPageDescription"
              >
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
            <el-input-number
              v-model="retentionDays"
              :min="0"
              :max="3650"
              controls-position="right"
            />
            <span class="retention-label">天后自动清理</span>
            <el-button :loading="savingCleanup" @click="saveCleanup"
              >保存清理</el-button
            >
            <el-switch v-model="autoRefresh" active-text="自动刷新" />
            <el-button :loading="loading" @click="loadItems">刷新</el-button>
          </div>
        </div>
      </template>

      <div class="search-bar desktop-search-bar">
        <el-input
          v-model="searchKeyword"
          clearable
          placeholder="搜索 UnionID / 插件 / 脚本路径 / 状态"
          @keyup.enter="searchItems"
          @clear="searchItems"
        />
        <el-input
          v-model="searchUnionId"
          clearable
          placeholder="按 UnionID 精确定位用户任务"
          @keyup.enter="searchItems"
          @clear="searchItems"
        />
        <el-select
          v-model="searchRunMode"
          clearable
          placeholder="运行模式"
          @change="searchItems"
        >
          <el-option label="全部授权账号" value="all_authorized" />
          <el-option label="单账号" value="single_account" />
          <el-option label="当前用户" value="current_user" />
          <el-option label="手动/其他" value="manual" />
        </el-select>
        <el-select
          v-model="searchRuntimeProfile"
          clearable
          placeholder="运行环境"
          @change="searchItems"
        >
          <el-option label="默认" :value="DEFAULT_RUNTIME_PROFILE_FILTER" />
          <el-option
            v-for="profile in runtimeProfileOptions"
            :key="profile.id"
            :label="runtimeProfileLabel(profile)"
            :value="profile.id"
          />
        </el-select>
        <el-select
          v-model="searchStatus"
          clearable
          placeholder="状态"
          @change="searchItems"
        >
          <el-option label="运行中" value="running" />
          <el-option label="暂停中" value="pausing" />
          <el-option label="已暂停" value="paused" />
          <el-option label="已完成" value="success" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-button type="primary" :loading="loading" @click="searchItems"
          >搜索</el-button
        >
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <div class="mobile-search-bar">
        <div class="mobile-search-main">
          <el-input
            v-model="searchKeyword"
            clearable
            placeholder="搜索脚本任务"
            @keyup.enter="searchItems"
            @clear="searchItems"
          />
          <el-button type="primary" :loading="loading" @click="searchItems"
            >搜索</el-button
          >
          <el-button @click="showMobileFilters = !showMobileFilters"
            >筛选</el-button
          >
        </div>
        <div v-show="showMobileFilters" class="mobile-filter-panel">
          <el-input
            v-model="searchUnionId"
            clearable
            placeholder="按 UnionID 精确定位用户任务"
            @keyup.enter="searchItems"
            @clear="searchItems"
          />
          <el-select
            v-model="searchRunMode"
            clearable
            placeholder="运行模式"
            @change="searchItems"
          >
            <el-option label="全部授权账号" value="all_authorized" />
            <el-option label="单账号" value="single_account" />
            <el-option label="当前用户" value="current_user" />
            <el-option label="手动/其他" value="manual" />
          </el-select>
          <el-select
            v-model="searchRuntimeProfile"
            clearable
            placeholder="运行环境"
            @change="searchItems"
          >
            <el-option label="默认" :value="DEFAULT_RUNTIME_PROFILE_FILTER" />
            <el-option
              v-for="profile in runtimeProfileOptions"
              :key="profile.id"
              :label="runtimeProfileLabel(profile)"
              :value="profile.id"
            />
          </el-select>
          <el-select
            v-model="searchStatus"
            clearable
            placeholder="状态"
            @change="searchItems"
          >
            <el-option label="运行中" value="running" />
            <el-option label="暂停中" value="pausing" />
            <el-option label="已暂停" value="paused" />
            <el-option label="已完成" value="success" />
            <el-option label="失败" value="failed" />
          </el-select>
          <el-button @click="resetSearch">重置筛选</el-button>
        </div>
      </div>

      <div class="table-area desktop-script-table">
        <el-table
          ref="desktopTableRef"
          v-loading="loading"
          class="desktop-data-table"
          :data="items"
          row-key="id"
          stripe
          height="100%"
          @scroll="syncDesktopActionScroll"
        >
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column
            prop="plugin_id"
            label="插件"
            min-width="130"
            show-overflow-tooltip
          />
          <el-table-column
            prop="script_path"
            label="脚本路径"
            min-width="240"
            show-overflow-tooltip
          />
          <el-table-column
            prop="run_mode"
            label="运行模式"
            min-width="130"
            show-overflow-tooltip
          />
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="statusType(row.status)">{{
                statusText(row.status)
              }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column
            prop="union_id"
            label="UnionID"
            min-width="180"
            show-overflow-tooltip
          />
          <el-table-column
            label="运行环境"
            min-width="130"
            show-overflow-tooltip
          >
            <template #default="{ row }">{{
              displayRuntimeProfile(row.runtime_profile)
            }}</template>
          </el-table-column>
          <el-table-column label="运行总数" width="100" align="center">
            <template #default="{ row }">{{
              formatCount(row.run_total)
            }}</template>
          </el-table-column>
          <el-table-column label="失败总数" width="100" align="center">
            <template #default="{ row }">{{
              formatCount(row.failed_total)
            }}</template>
          </el-table-column>
          <el-table-column label="开始时间" min-width="170">
            <template #default="{ row }">{{
              formatTime(row.started_at)
            }}</template>
          </el-table-column>
          <el-table-column label="结束时间" min-width="170">
            <template #default="{ row }">{{
              isRunning(row.status) ? "-" : formatTime(row.finished_at)
            }}</template>
          </el-table-column>
        </el-table>
        <div class="desktop-action-panel">
          <div class="desktop-action-header">操作</div>
          <div ref="desktopActionBodyRef" class="desktop-action-body">
            <div v-for="row in items" :key="row.id" class="desktop-action-row">
              <el-button size="small" @click="openLog(row)">日志</el-button>
              <el-button
                size="small"
                :disabled="!isRunning(row.status)"
                :loading="pausingId === row.id"
                @click="pauseItem(row)"
                >暂停</el-button
              >
              <el-button
                size="small"
                type="danger"
                :loading="deletingId === row.id"
                @click="deleteItem(row)"
                >删除</el-button
              >
            </div>
          </div>
        </div>

        <el-empty
          v-if="!loading && items.length === 0"
          description="暂无脚本任务"
        />
      </div>

      <div class="mobile-script-list" v-loading="loading">
        <div v-for="row in items" :key="row.id" class="mobile-task-card">
          <div class="mobile-row-title">
            <span>#{{ row.id }}</span>
            <el-tag size="small" :type="statusType(row.status)">{{
              statusText(row.status)
            }}</el-tag>
          </div>
          <div class="mobile-row-desc">{{ row.script_path || "-" }}</div>
          <div class="mobile-field-grid">
            <div>
              <span>插件</span><strong>{{ row.plugin_id || "-" }}</strong>
            </div>
            <div>
              <span>模式</span><strong>{{ row.run_mode || "-" }}</strong>
            </div>
            <div>
              <span>运行环境</span
              ><strong>{{ displayRuntimeProfile(row.runtime_profile) }}</strong>
            </div>
            <div>
              <span>UnionID</span><strong>{{ row.union_id || "-" }}</strong>
            </div>
            <div>
              <span>运行总数</span
              ><strong>{{ formatCount(row.run_total) }}</strong>
            </div>
            <div>
              <span>失败总数</span
              ><strong>{{ formatCount(row.failed_total) }}</strong>
            </div>
            <div>
              <span>开始时间</span
              ><strong>{{ formatTime(row.started_at) }}</strong>
            </div>
            <div>
              <span>结束时间</span
              ><strong>{{
                isRunning(row.status) ? "-" : formatTime(row.finished_at)
              }}</strong>
            </div>
          </div>
          <div class="mobile-table-actions">
            <el-button size="small" @click="openLog(row)">日志</el-button>
            <el-button
              size="small"
              :disabled="!isRunning(row.status)"
              :loading="pausingId === row.id"
              @click="pauseItem(row)"
              >暂停</el-button
            >
            <el-button
              size="small"
              type="danger"
              :loading="deletingId === row.id"
              @click="deleteItem(row)"
              >删除</el-button
            >
          </div>
        </div>
        <el-empty
          v-if="!loading && items.length === 0"
          description="暂无脚本任务"
        />
      </div>
      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="loadItems"
          @size-change="handlePageSizeChange"
        />
      </div>
    </el-card>

    <el-dialog
      v-model="logDialogVisible"
      :title="logTitle"
      width="80%"
      top="5vh"
      destroy-on-close
    >
      <div v-loading="logLoading" class="log-dialog-body">
        <div v-if="currentLog" class="detail-meta">
          <el-tag size="small" :type="statusType(currentLog.status)">{{
            statusText(currentLog.status)
          }}</el-tag>
          <span>插件：{{ currentLog.plugin_id || "-" }}</span>
          <span>脚本：{{ currentLog.script_path || "-" }}</span>
          <span>模式：{{ currentLog.run_mode || "-" }}</span>
          <span
            >运行环境：{{
              displayRuntimeProfile(currentLog.runtime_profile)
            }}</span
          >
          <span>用户：{{ currentLog.union_id || "-" }}</span>
          <span>运行总数：{{ formatCount(currentLog.run_total) }}</span>
          <span>失败总数：{{ formatCount(currentLog.failed_total) }}</span>
        </div>
        <div class="log-section">
          <div class="log-title">输出日志</div>
          <pre>{{ currentLog?.output || "暂无输出" }}</pre>
        </div>
        <div v-if="currentLog?.error" class="log-section">
          <div class="log-title">错误信息</div>
          <pre class="error-log">{{ currentLog.error }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="logDialogVisible = false">关闭</el-button>
        <el-button :loading="logLoading" @click="refreshCurrentLog"
          >刷新日志</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { InfoFilled } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import request from "@/utils/request";

const loading = ref(false);
const logLoading = ref(false);
const pausingId = ref(0);
const deletingId = ref(0);
const savingCleanup = ref(false);
const autoRefresh = ref(true);
const showMobileFilters = ref(false);
const items = ref([]);
const desktopTableRef = ref(null);
const desktopActionBodyRef = ref(null);
const currentLog = ref(null);
const logDialogVisible = ref(false);
const retentionDays = ref(0);
const searchKeyword = ref("");
const searchUnionId = ref("");
const searchRunMode = ref("");
const searchRuntimeProfile = ref("");
const searchStatus = ref("");
const runtimeProfiles = ref([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const pageDescription =
  "查看插件提交的 Node.js / Python 脚本任务；日志放在弹窗里滚动查看，后台不提供手动启动。";
const DEFAULT_RUNTIME_PROFILE_FILTER = "__default__";
let refreshTimer = 0;

const logTitle = computed(() =>
  currentLog.value ? `脚本任务日志 #${currentLog.value.id}` : "脚本任务日志"
);
const runtimeProfileOptions = computed(() =>
  runtimeProfiles.value.filter(
    (profile) => profile && profile.id && profile.enabled !== false
  )
);

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, "脚本任务说明", {
    confirmButtonText: "知道了",
    type: "info",
  });
};

watch([total, pageSize], () => {
  const maxPage = Math.max(1, Math.ceil(total.value / pageSize.value));
  if (page.value > maxPage) page.value = maxPage;
});

watch(autoRefresh, (enabled) => {
  stopTimer();
  if (enabled) startTimer();
});

const loadItems = async () => {
  loading.value = true;
  try {
    const params = new URLSearchParams();
    if (searchKeyword.value.trim())
      params.set("keyword", searchKeyword.value.trim());
    if (searchUnionId.value.trim())
      params.set("union_id", searchUnionId.value.trim());
    if (searchRunMode.value) params.set("run_mode", searchRunMode.value);
    if (
      searchRuntimeProfile.value &&
      searchRuntimeProfile.value !== DEFAULT_RUNTIME_PROFILE_FILTER
    )
      params.set("runtime_profile", searchRuntimeProfile.value);
    if (searchStatus.value) params.set("status", searchStatus.value);
    params.set("page", String(page.value));
    params.set("page_size", String(pageSize.value));
    const query = params.toString();
    const result = await request.get(
      `/script-tasks${query ? `?${query}` : ""}`
    );
    items.value = Array.isArray(result) ? result : result.items || [];
    total.value = Array.isArray(result)
      ? items.value.length
      : Number(result.total || 0);
    if (!Array.isArray(result) && typeof result.retention_days === "number")
      retentionDays.value = result.retention_days;
  } finally {
    loading.value = false;
  }
};

const loadRuntimeProfiles = async () => {
  try {
    const data = await request.get("/runtime-profiles");
    runtimeProfiles.value = Array.isArray(data) ? data : [];
  } catch {
    runtimeProfiles.value = [];
  }
};

const searchItems = async () => {
  page.value = 1;
  await loadItems();
};

const handlePageSizeChange = async () => {
  page.value = 1;
  await loadItems();
};

const resetSearch = async () => {
  searchKeyword.value = "";
  searchUnionId.value = "";
  searchRunMode.value = "";
  searchRuntimeProfile.value = "";
  searchStatus.value = "";
  await searchItems();
};

const saveCleanup = async () => {
  savingCleanup.value = true;
  try {
    const result = await request.post(
      `/script-tasks?action=cleanup&days=${retentionDays.value}`
    );
    ElMessage.success(
      `${result.message || "脚本任务清理设置已保存"}，已清理 ${
        result.removed || 0
      } 条`
    );
    await loadItems();
  } finally {
    savingCleanup.value = false;
  }
};

const openLog = async (item) => {
  currentLog.value = { ...item, output: "" };
  logDialogVisible.value = true;
  await loadLog(item.id);
};

const refreshCurrentLog = async () => {
  if (!currentLog.value?.id) return;
  await loadLog(currentLog.value.id);
};

const loadLog = async (id) => {
  logLoading.value = true;
  try {
    currentLog.value = await request.get(`/script-tasks/${id}`);
  } finally {
    logLoading.value = false;
  }
};

const pauseItem = async (item) => {
  pausingId.value = item.id;
  try {
    const result = await request.put(`/script-tasks/${item.id}?action=pause`);
    ElMessage.success(result.message || "脚本任务暂停请求已发送");
    await loadItems();
  } finally {
    pausingId.value = 0;
  }
};

const deleteItem = async (item) => {
  await ElMessageBox.confirm(
    `确定删除脚本任务 #${item.id} 吗？运行中的任务会先尝试暂停。`,
    "删除确认",
    { type: "warning" }
  );
  deletingId.value = item.id;
  try {
    const result = await request.delete(`/script-tasks/${item.id}`);
    ElMessage.success(result.message || "脚本任务已删除");
    await loadItems();
  } finally {
    deletingId.value = 0;
  }
};

const syncDesktopActionScroll = ({ scrollTop }) => {
  if (desktopActionBodyRef.value)
    desktopActionBodyRef.value.scrollTop = scrollTop || 0;
};

const isRunning = (status) => ["running", "pausing"].includes(status);

const statusText = (status) =>
  ({
    running: "运行中",
    pausing: "暂停中",
    paused: "已暂停",
    success: "已完成",
    failed: "失败",
  }[status] ||
  status ||
  "-");

const statusType = (status) =>
  ({
    running: "primary",
    pausing: "warning",
    paused: "warning",
    success: "success",
    failed: "danger",
  }[status] || "info");

const runtimeProfileLabel = (profile) =>
  `${profile.name || profile.id}${profile.default ? "（默认）" : ""}`;

const displayRuntimeProfile = (profileID) => {
  const id = String(profileID || "").trim();
  if (!id) return "默认";
  const profile = runtimeProfiles.value.find((item) => item.id === id);
  return profile ? runtimeProfileLabel(profile) : id;
};

const formatTime = (value) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1970) return "-";
  return date.toLocaleString();
};

const formatCount = (value) => Number(value || 0).toLocaleString();

const startTimer = () => {
  refreshTimer = window.setInterval(async () => {
    if (!loading.value) await loadItems();
    if (
      logDialogVisible.value &&
      currentLog.value?.id &&
      isRunning(currentLog.value.status) &&
      !logLoading.value
    )
      await loadLog(currentLog.value.id);
  }, 5000);
};

const stopTimer = () => {
  if (refreshTimer) window.clearInterval(refreshTimer);
  refreshTimer = 0;
};

onMounted(async () => {
  await loadRuntimeProfiles();
  await loadItems();
  if (autoRefresh.value) startTimer();
});

onBeforeUnmount(stopTimer);
</script>

<style scoped>
.page-shell {
  height: 100%;
  min-height: 0;
}
.page-card {
  height: 100%;
  display: flex;
  flex-direction: column;
}
.page-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: hidden;
}
.script-tasks-page {
  padding: 0;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
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
  line-height: 1.5;
}
.header-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.header-actions > * {
  min-width: 0;
}
.retention-label {
  color: #606266;
  font-size: 13px;
  white-space: nowrap;
}
.search-bar {
  display: grid;
  grid-template-columns: minmax(220px, 1.4fr) minmax(220px, 1.2fr) 150px 150px 130px auto auto;
  gap: 10px;
  align-items: center;
  flex-shrink: 0;
}
.search-bar > * {
  min-width: 0;
}
.search-bar :deep(.el-input),
.search-bar :deep(.el-select),
.search-bar :deep(.el-button) {
  width: 100%;
}
.mobile-search-bar {
  display: none;
}
.table-area {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}
.table-area .el-empty {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
.desktop-script-table {
  padding-right: 250px;
}
.desktop-data-table {
  width: 100%;
}
.desktop-action-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 4;
  width: 250px;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-left: 1px solid #ebeef5;
  box-shadow: -6px 0 10px rgba(15, 23, 42, 0.06);
}
.desktop-action-header {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 12px;
  color: #909399;
  font-weight: 600;
  border-bottom: 1px solid #ebeef5;
  background: #fff;
}
.desktop-action-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}
.desktop-action-row {
  height: 48px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-bottom: 1px solid #ebeef5;
  box-sizing: border-box;
}
.desktop-action-row .el-button + .el-button {
  margin-left: 0;
}
.mobile-script-list {
  display: none;
}
.pagination-bar {
  display: flex;
  justify-content: flex-end;
  flex-shrink: 0;
  min-width: 0;
}
.pagination-bar :deep(.el-pagination) {
  max-width: 100%;
}
.log-dialog-body {
  min-height: 480px;
}
.detail-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  color: #606266;
}
.log-section {
  margin-top: 12px;
}
.log-title {
  margin-bottom: 6px;
  font-weight: 600;
  color: #303133;
}
pre {
  height: 58vh;
  overflow: auto;
  padding: 12px;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  background: #1f2937;
  color: #f9fafb;
  border-radius: 6px;
  font-family: Consolas, Monaco, monospace;
}
.error-log {
  height: 160px;
  background: #451a1a;
  color: #fee2e2;
}
@media (max-width: 768px) {
  .page-shell {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }
  .page-card {
    height: auto;
    min-height: 100%;
  }
  .page-card :deep(.el-card__body) {
    overflow: visible;
  }
  .card-header {
    flex-direction: column;
  }
  .mobile-info-button {
    display: inline-flex;
  }
  .subtitle {
    display: none;
  }
  .header-actions {
    width: 100%;
    display: grid;
    grid-template-columns: 1fr;
    align-items: stretch;
  }
  .header-actions :deep(.el-input-number),
  .header-actions :deep(.el-button) {
    width: 100%;
  }
  .header-actions :deep(.el-switch) {
    justify-self: flex-start;
  }
  .retention-label {
    white-space: normal;
  }
  .desktop-search-bar {
    display: none;
  }
  .mobile-search-bar {
    display: grid;
    gap: 8px;
    flex-shrink: 0;
  }
  .mobile-search-main {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: 8px;
    align-items: center;
  }
  .mobile-search-main :deep(.el-button) {
    padding-inline: 10px;
  }
  .mobile-filter-panel {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 8px;
    padding: 10px;
    border: 1px solid #ebeef5;
    border-radius: 10px;
    background: #fafafa;
  }
  .mobile-filter-panel :deep(.el-input),
  .mobile-filter-panel :deep(.el-select),
  .mobile-filter-panel :deep(.el-button) {
    width: 100%;
  }
  .desktop-script-table {
    display: none;
  }
  .mobile-script-list {
    flex: 1 1 auto;
    min-height: 240px;
    display: grid;
    gap: 10px;
    overflow: visible;
    padding-bottom: 8px;
  }
  .mobile-task-card {
    width: 100%;
    min-width: 0;
    box-sizing: border-box;
    padding: 12px 10px;
    border: 1px solid #ebeef5;
    border-radius: 10px;
    background: #fff;
  }
  .mobile-row-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    font-weight: 600;
  }
  .mobile-row-desc {
    margin-top: 6px;
    color: #606266;
    font-size: 12px;
    word-break: break-word;
    overflow-wrap: anywhere;
  }
  .mobile-field-grid {
    margin-top: 10px;
    display: grid;
    gap: 7px;
    font-size: 12px;
  }
  .mobile-field-grid > div {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 10px;
    min-width: 0;
  }
  .mobile-field-grid span {
    color: #909399;
    flex-shrink: 0;
  }
  .mobile-field-grid strong {
    min-width: 0;
    color: #303133;
    font-weight: 500;
    text-align: right;
    word-break: break-word;
    overflow-wrap: anywhere;
  }
  .mobile-table-actions {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px solid #f0f2f5;
  }
  .mobile-table-actions .el-button {
    width: 100%;
    min-width: 0;
    margin-left: 0;
    padding-inline: 8px;
  }
  .pagination-bar {
    justify-content: flex-start;
    width: 100%;
    overflow-x: auto;
    padding-bottom: 2px;
    flex-shrink: 0;
  }
  .pagination-bar :deep(.el-pagination) {
    flex-wrap: nowrap;
    min-width: max-content;
  }
  .mobile-script-list::-webkit-scrollbar,
  .pagination-bar::-webkit-scrollbar {
    display: none;
  }
  .script-tasks-page :deep(.el-dialog) {
    width: 94vw !important;
  }
}
</style>
