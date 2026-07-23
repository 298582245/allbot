<template>
  <div class="scheduled-tasks page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">定时任务</span>
              <el-button
                class="mobile-info-button"
                type="primary"
                link
                aria-label="查看定时任务说明"
                @click="showPageDescription"
              >
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <el-button type="primary" @click="openDialog()">新增任务</el-button>
        </div>
      </template>

      <div class="task-toolbar">
        <div class="filter-actions">
          <el-input
            v-model="searchKeyword"
            clearable
            placeholder="搜索名称、备注、来源"
            class="task-search"
          />
          <el-radio-group
            v-model="statusFilter"
            size="small"
            class="status-filter"
          >
            <el-radio-button label="all"
              >全部（{{ statusCounters.all }}）</el-radio-button
            >
            <el-radio-button label="enabled"
              >开启（{{ statusCounters.enabled }}）</el-radio-button
            >
            <el-radio-button label="disabled"
              >关闭（{{ statusCounters.disabled }}）</el-radio-button
            >
          </el-radio-group>
        </div>
        <div class="batch-actions">
          <span class="selected-count">已选 {{ selectedItems.length }} 项</span>
          <el-button
            size="small"
            type="success"
            :disabled="!hasSelection || !!batchAction"
            :loading="batchAction === 'enable'"
            @click="batchToggleEnabled(true)"
            >开启</el-button
          >
          <el-button
            size="small"
            type="warning"
            :disabled="!hasSelection || !!batchAction"
            :loading="batchAction === 'disable'"
            @click="batchToggleEnabled(false)"
            >停止</el-button
          >
          <el-button
            size="small"
            type="danger"
            :disabled="!hasSelection || !!batchAction"
            :loading="batchAction === 'delete'"
            @click="deleteSelectedItems"
            >删除</el-button
          >
        </div>
      </div>

      <div class="table-area desktop-task-table">
        <el-table
          ref="desktopTableRef"
          v-loading="loading"
          :data="pagedItems"
          border
          stripe
          height="100%"
          row-key="id"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="50" reserve-selection />
          <el-table-column label="ID" width="90">
            <template #default="{ row }">
              <span class="id-with-pin"><span class="pin-id">{{ row.id
                }}</span><svg v-if="row.pinned" class="pin-icon" viewBox="0 0 24 24" width="14" height="14"><path d="M16 9V4l1-1V2H7v1l1 1v5l-2 2v2h5v6l1 1 1-1v-6h5v-2l-2-2z" fill="currentColor"/></svg></span>
            </template>
          </el-table-column>
          <el-table-column label="名称" min-width="180">
            <template #default="{ row }">
              <div class="task-name">
                {{ row.name || row.task_key || `任务 ${row.id}` }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="备注" min-width="220" show-overflow-tooltip>
            <template #default="{ row }">{{ row.description || "-" }}</template>
          </el-table-column>
          <el-table-column label="来源" width="150">
            <template #default="{ row }">
              <el-tag :type="row.source === 'plugin' ? 'success' : 'info'">{{
                sourceName(row.source)
              }}</el-tag>
              <div v-if="row.plugin_id" class="plugin-id">
                {{ row.plugin_id }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="row.enabled ? 'success' : 'info'">{{
                row.enabled ? "启用" : "禁用"
              }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="表达式" width="190" show-overflow-tooltip>
            <template #default="{ row }">{{ row.cron }}</template>
          </el-table-column>
          <el-table-column label="伪造身份" min-width="220">
            <template #default="{ row }">
              <div>{{ taskIdentityText(row) }}</div>
              <div class="task-desc">
                {{ row.group_id ? `群 ${row.group_id}` : "私聊" }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="上次执行" width="180">
            <template #default="{ row }">{{
              formatTime(row.last_run_at)
            }}</template>
          </el-table-column>
          <el-table-column label="下次执行" width="180">
            <template #default="{ row }">{{
              formatTime(row.next_run_at)
            }}</template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <el-button
                size="small"
                type="success"
                :loading="runningId === row.id"
                @click="runItem(row)"
                >执行</el-button
              >
              <el-button size="small" @click="openDialog(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteItem(row)"
                >删除</el-button
              >
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-task-cards" v-loading="loading">
        <article
          v-for="row in pagedItems"
          :key="row.id"
          class="mobile-task-card"
          :class="{ selected: isSelected(row) }"
        >
          <div class="mobile-task-head">
            <el-checkbox
              :model-value="isSelected(row)"
              @change="checked => toggleTaskSelection(row, checked)"
            />
            <div class="mobile-task-title">
              <strong>{{ row.name || row.task_key || `任务 ${row.id}` }}</strong>
              <span class="mobile-task-id">
                ID <span class="id-with-pin"><span class="pin-id">{{ row.id }}</span><svg v-if="row.pinned" class="pin-icon" viewBox="0 0 24 24" width="14" height="14"><path d="M16 9V4l1-1V2H7v1l1 1v5l-2 2v2h5v6l1 1 1-1v-6h5v-2l-2-2z" fill="currentColor"/></svg></span>
              </span>
            </div>
            <el-tag :type="row.enabled ? 'success' : 'info'" effect="plain">
              {{ row.enabled ? "启用" : "禁用" }}
            </el-tag>
          </div>
          <div class="mobile-task-fields">
            <div><span>备注</span><strong>{{ row.description || "-" }}</strong></div>
            <div><span>来源</span><strong>{{ sourceName(row.source) }}</strong></div>
            <div v-if="row.plugin_id"><span>插件</span><strong>{{ row.plugin_id }}</strong></div>
            <div><span>表达式</span><strong>{{ row.cron }}</strong></div>
            <div><span>身份</span><strong>{{ taskIdentityText(row) }}</strong></div>
            <div><span>会话</span><strong>{{ row.group_id ? `群 ${row.group_id}` : "私聊" }}</strong></div>
            <div><span>上次执行</span><strong>{{ formatTime(row.last_run_at) }}</strong></div>
            <div><span>下次执行</span><strong>{{ formatTime(row.next_run_at) }}</strong></div>
          </div>
          <div class="mobile-task-actions">
            <el-button size="small" type="success" :loading="runningId === row.id" @click="runItem(row)">启动</el-button>
            <el-button size="small" @click="openDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteItem(row)">删除</el-button>
          </div>
        </article>
        <el-empty v-if="!loading && pagedItems.length === 0" description="暂无定时任务" />
      </div>

      <StdPagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="filteredItems.length"
        :page-sizes="[10, 20, 50, 100]"
      />
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="form.id ? '编辑定时任务' : '新增定时任务'"
      width="680px"
      @closed="clearUserAccountCandidates"
    >
      <el-form :model="form" label-width="110px">
        <el-form-item label="任务名称">
          <el-input v-model="form.name" placeholder="例如：每日天气推送" />
        </el-form-item>
        <el-form-item label="任务 Key">
          <el-input
            v-model="form.task_key"
            :disabled="form.source === 'plugin'"
            placeholder="插件任务用于更新同一条定时"
          />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="置顶">
          <el-switch v-model="form.pinned" />
        </el-form-item>
        <el-form-item label="定时表达式">
          <el-input
            v-model="form.cron"
            type="textarea"
            :rows="3"
            placeholder="每行一个表达式；支持 5/6 位、@once。例如：15 30 8 * * *"
          />
          <span class="hint"
            >多个表达式按行填写；@once
            表示只允许手动启动，不自动定时执行。</span
          >
        </el-form-item>
        <el-form-item label="平台">
          <el-select
            v-model="form.platform"
            filterable
            allow-create
            default-first-option
            placeholder="选择或输入平台"
            style="width: 100%"
            @change="handlePlatformChange"
          >
            <el-option
              v-for="option in adapterPlatformOptions"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="机器人实例">
          <el-select
            v-model="form.adapter_id"
            clearable
            filterable
            placeholder="不选则使用该平台第一个启用机器人"
            style="width: 100%"
          >
            <el-option
              v-for="adapter in filteredAdapterOptions"
              :key="adapter.value"
              :label="adapter.label"
              :value="adapter.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="用户 ID">
          <el-select
            v-model="form.user_id"
            filterable
            remote
            allow-create
            clearable
            default-first-option
            :loading="userAccountLoading"
            :debounce="300"
            :remote-method="searchUserAccounts"
            placeholder="选择或输入用户 ID"
            style="width: 100%"
            @visible-change="handleUserSelectVisibleChange"
          >
            <el-option
              v-for="account in userAccountCandidates"
              :key="`${account.platform || form.platform}:${account.user_id}`"
              :label="account.user_id"
              :value="account.user_id"
            >
              <div class="user-account-option">
                <span>{{ account.user_id }}</span>
                <span v-if="account.union_id" class="user-account-union">
                  UnionID：{{ account.union_id }}
                </span>
              </div>
            </el-option>
          </el-select>
          <span class="hint">
            {{ form.platform ? "输入关键字搜索当前平台账号；未注册用户也可直接填写。" : "请先选择平台，再搜索平台账号。" }}
          </span>
        </el-form-item>
        <el-form-item label="群 ID">
          <el-input v-model="form.group_id" placeholder="留空表示私聊消息" />
        </el-form-item>
        <el-form-item label="消息内容">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="4"
            placeholder="要触发的插件指令内容"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveItem"
          >保存</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'ScheduledTasks' })
import { computed, nextTick, onMounted, reactive, ref, watch } from "vue";
import { InfoFilled } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import request from "@/utils/request";
import { getAdapterPlatforms, getAdapters, getUserAccounts } from "@/api";
import StdPagination from '@/components/StdPagination.vue'

const loading = ref(false);
const saving = ref(false);
const runningId = ref(0);
const dialogVisible = ref(false);
const items = ref([]);
const adapters = ref([]);
const userAccountCandidates = ref([]);
const userAccountLoading = ref(false);
const userAccountLoaded = ref(false);
let userAccountRequestSeq = 0;
const userAccountSearchLimit = 50;
const adapterPlatformFallback = [
  { label: "QQ", value: "qq" },
  { label: "QQ 官方机器人", value: "qq_office" },
  { label: "Telegram", value: "telegram" },
];
const adapterPlatformOptions = ref([...adapterPlatformFallback]);
const searchKeyword = ref("");
const statusFilter = ref("all");
const selectedItems = ref([]);
const batchAction = ref("");
const desktopTableRef = ref(null);
const syncingDesktopSelection = ref(false);
const page = ref(1);
const pageSize = ref(20);
const form = reactive(createEmptyForm());
const pageDescription =
  "定时伪造用户消息触发插件指令；插件声明的任务也可以在这里修改。";

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, "定时任务说明", {
    confirmButtonText: "知道了",
    type: "info",
  });
};

const statusCounters = computed(() => ({
  all: items.value.length,
  enabled: items.value.filter((item) => Boolean(item.enabled)).length,
  disabled: items.value.filter((item) => !item.enabled).length,
}));

const hasSelection = computed(() => selectedItems.value.length > 0);
const selectedIdSet = computed(() => new Set(selectedItems.value.map((item) => item.id)));
const adapterPlatformNames = computed(() =>
  Object.fromEntries(
    adapterPlatformOptions.value.map((option) => [option.value, option.label])
  )
);
const adapterById = computed(() =>
  Object.fromEntries(
    adapters.value.map((adapter) => [String(adapter.id), adapter])
  )
);

const filteredItems = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase();
  return items.value.filter((item) => {
    if (statusFilter.value === "enabled" && !item.enabled) return false;
    if (statusFilter.value === "disabled" && item.enabled) return false;
    if (!keyword) return true;

    const fields = [
      item.name,
      item.task_key,
      item.description,
      item.plugin_id,
      item.platform,
      getPlatformName(item.platform),
      item.adapter_id,
      adapterLabelById(item.adapter_id),
      sourceName(item.source),
    ];
    return fields.some((field) =>
      String(field || "")
        .toLowerCase()
        .includes(keyword)
    );
  });
});

const pagedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return filteredItems.value.slice(start, start + pageSize.value);
});

const adapterOptions = computed(() =>
  adapters.value.map((adapter) => ({
    label: adapterLabel(adapter),
    value: String(adapter.id),
    platform: adapter.platform,
  }))
);

const filteredAdapterOptions = computed(() =>
  adapterOptions.value.filter((adapter) => adapter.platform === form.platform)
);

watch([searchKeyword, statusFilter], () => {
  page.value = 1;
});

watch(items, () => {
  const currentIds = new Set(items.value.map((item) => item.id));
  selectedItems.value = selectedItems.value.filter((item) =>
    currentIds.has(item.id)
  );
  nextTick(syncDesktopSelection);
});

watch([filteredItems, pageSize], () => {
  const maxPage = Math.max(
    1,
    Math.ceil(filteredItems.value.length / pageSize.value)
  );
  if (page.value > maxPage) page.value = maxPage;
});

const loadItems = async () => {
  loading.value = true;
  try {
    const [taskItems, adapterItems, platformItems] = await Promise.all([
      request.get("/scheduled-tasks"),
      getAdapters(),
      getAdapterPlatforms().catch(() => []),
    ]);
    items.value = taskItems;
    adapters.value = adapterItems;
    adapterPlatformOptions.value = normalizeAdapterPlatforms(platformItems);
  } finally {
    loading.value = false;
  }
};

const openDialog = (item) => {
  Object.assign(form, createEmptyForm(), item ? { ...item } : {});
  if (!form.source) form.source = item?.source || "user";
  ensureSelectedAdapterMatchesPlatform();
  clearUserAccountCandidates();
  dialogVisible.value = true;
  searchUserAccounts(form.user_id);
};

const saveItem = async () => {
  saving.value = true;
  try {
    const payload = normalizeForm(form);
    if (payload.id)
      await request.put(`/scheduled-tasks/${payload.id}`, payload);
    else await request.post("/scheduled-tasks", payload);
    ElMessage.success("定时任务已保存");
    dialogVisible.value = false;
    await loadItems();
  } finally {
    saving.value = false;
  }
};

const deleteItem = async (item) => {
  await ElMessageBox.confirm(
    `确定删除定时任务「${item.name || item.id}」吗？`,
    "提示",
    { type: "warning" }
  );
  await request.delete(`/scheduled-tasks/${item.id}`);
  ElMessage.success("定时任务已删除");
  await loadItems();
};

const handleSelectionChange = (selection) => {
  if (syncingDesktopSelection.value) return;
  selectedItems.value = Array.isArray(selection) ? selection : [];
};

const isSelected = (row) => {
  return selectedIdSet.value.has(row?.id);
};

const toggleTaskSelection = (row, checked) => {
  if (!row?.id) return;
  if (checked) {
    if (!selectedIdSet.value.has(row.id)) {
      selectedItems.value = [...selectedItems.value, row];
    }
  } else {
    selectedItems.value = selectedItems.value.filter((item) => item.id !== row.id);
  }
  nextTick(syncDesktopSelection);
};

function syncDesktopSelection() {
  syncingDesktopSelection.value = true;
  desktopTableRef.value?.clearSelection();
  pagedItems.value.forEach((row) => {
    if (selectedIdSet.value.has(row.id)) desktopTableRef.value?.toggleRowSelection(row, true);
  });
  nextTick(() => {
    syncingDesktopSelection.value = false;
  });
}

const clearSelection = () => {
  desktopTableRef.value?.clearSelection();
  selectedItems.value = [];
};

const batchToggleEnabled = async (enabled) => {
  const actionName = enabled ? "启动" : "关闭";
  const pendingItems = selectedItems.value.filter(
    (item) => Boolean(item.enabled) !== enabled
  );
  if (pendingItems.length === 0) {
    ElMessage.info(`选中的任务已全部${actionName}`);
    return;
  }

  batchAction.value = enabled ? "enable" : "disable";
  try {
    await Promise.all(
      pendingItems.map((item) => {
        const payload = normalizeForm({ ...item, enabled });
        return request.put(`/scheduled-tasks/${item.id}`, payload);
      })
    );
    ElMessage.success(`已${actionName} ${pendingItems.length} 个定时任务`);
    clearSelection();
    await loadItems();
  } finally {
    batchAction.value = "";
  }
};

const deleteSelectedItems = async () => {
  const tasks = [...selectedItems.value];
  if (tasks.length === 0) return;

  await ElMessageBox.confirm(
    `确定删除选中的 ${tasks.length} 个定时任务吗？`,
    "提示",
    { type: "warning" }
  );
  batchAction.value = "delete";
  try {
    await Promise.all(
      tasks.map((item) => request.delete(`/scheduled-tasks/${item.id}`))
    );
    ElMessage.success(`已删除 ${tasks.length} 个定时任务`);
    clearSelection();
    await loadItems();
  } finally {
    batchAction.value = "";
  }
};

const handlePlatformChange = () => {
  ensureSelectedAdapterMatchesPlatform();
  clearUserAccountCandidates();
  searchUserAccounts(form.user_id);
};

const searchUserAccounts = async (query = "") => {
  const platform = normalizedPlatform();
  const requestSeq = ++userAccountRequestSeq;
  if (!platform) {
    userAccountCandidates.value = [];
    userAccountLoading.value = false;
    userAccountLoaded.value = false;
    return;
  }

  userAccountCandidates.value = [];
  userAccountLoading.value = true;
  userAccountLoaded.value = false;
  try {
    const data = await getUserAccounts({
      platform,
      keyword: String(query || "").trim(),
      limit: userAccountSearchLimit,
      offset: 0,
    });
    if (requestSeq !== userAccountRequestSeq || platform !== normalizedPlatform()) return;
    userAccountCandidates.value = normalizeUserAccounts(data?.items);
    userAccountLoaded.value = true;
  } catch {
    if (requestSeq === userAccountRequestSeq) userAccountCandidates.value = [];
  } finally {
    if (requestSeq === userAccountRequestSeq) userAccountLoading.value = false;
  }
};

const handleUserSelectVisibleChange = (visible) => {
  if (visible && normalizedPlatform() && !userAccountLoading.value && !userAccountLoaded.value) {
    searchUserAccounts(form.user_id);
  }
};

function clearUserAccountCandidates() {
  userAccountRequestSeq += 1;
  userAccountCandidates.value = [];
  userAccountLoading.value = false;
  userAccountLoaded.value = false;
}

function normalizedPlatform() {
  return String(form.platform || "").trim();
}

function normalizeUserAccounts(items) {
  if (!Array.isArray(items)) return [];
  const seen = new Set();
  return items.reduce((accounts, item) => {
    const userId = String(item?.user_id || "").trim();
    if (!userId) return accounts;
    const platform = String(item?.platform || normalizedPlatform()).trim();
    const key = `${platform}:${userId}`;
    if (seen.has(key)) return accounts;
    seen.add(key);
    accounts.push({
      ...item,
      platform,
      user_id: userId,
      union_id: String(item?.union_id || "").trim(),
    });
    return accounts;
  }, []);
}

const runItem = async (item) => {
  runningId.value = item.id;
  try {
    await request.put(`/scheduled-tasks/${item.id}?action=run`);
    ElMessage.success("已立即执行");
    await loadItems();
  } finally {
    runningId.value = 0;
  }
};

onMounted(loadItems);

function createEmptyForm() {
  return {
    id: 0,
    plugin_id: "",
    task_key: "",
    name: "",
    description: "",
    enabled: true,
    pinned: false,
    cron: "",
    platform: "qq",
    adapter_id: "",
    user_id: "",
    group_id: "",
    content: "",
    source: "user",
  };
}

function normalizeAdapterPlatforms(items) {
  const options = Array.isArray(items)
    ? items
        .filter((item) => item && item.platform)
        .map((item) => ({
          label: item.display_name || item.platform,
          value: item.platform,
        }))
    : [];
  const merged = [...options];
  const exists = new Set(merged.map((option) => option.value));
  adapterPlatformFallback.forEach((option) => {
    if (!exists.has(option.value)) merged.push(option);
  });
  return merged;
}

function normalizeForm(value) {
  return {
    ...value,
    id: Number(value.id || 0),
    enabled: Boolean(value.enabled),
    pinned: Boolean(value.pinned),
    plugin_id: String(value.plugin_id || "").trim(),
    task_key: String(value.task_key || "").trim(),
    name: String(value.name || "").trim(),
    description: String(value.description || "").trim(),
    cron: String(value.cron || "").trim(),
    platform: String(value.platform || "").trim(),
    adapter_id: String(value.adapter_id || "").trim(),
    user_id: String(value.user_id || "").trim(),
    group_id: String(value.group_id || "").trim(),
    content: String(value.content || "").trim(),
    source: value.source === "plugin" ? "plugin" : "user",
  };
}

function sourceName(source) {
  return source === "plugin" ? "插件声明" : "管理员添加";
}

function getPlatformName(platform) {
  return adapterPlatformNames.value[platform] || platform || "-";
}

function adapterLabel(adapter) {
  const name =
    adapter.remark ||
    adapter.description ||
    `${getPlatformName(adapter.platform)} #${adapter.id}`;
  return `（${getPlatformName(adapter.platform)} / ID ${adapter.id}）`;
}

function adapterLabelById(adapterId) {
  const adapter = adapterById.value[String(adapterId || "")];
  return adapter ? adapterLabel(adapter) : "";
}

function taskIdentityText(task) {
  const adapterLabelText = adapterLabelById(task.adapter_id);
  return (
    adapterLabelText ||
    `${getPlatformName(task.platform)}${
      task.adapter_id ? ` #${task.adapter_id}` : ""
    }`
  );
}

function ensureSelectedAdapterMatchesPlatform() {
  if (!form.adapter_id) return;
  const adapter = adapterById.value[String(form.adapter_id)];
  if (!adapter || adapter.platform !== form.platform) form.adapter_id = "";
}

function formatTime(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString();
}
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
  line-height: 1.5;
}
.task-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  flex-wrap: wrap;
}
.filter-actions,
.batch-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.task-search {
  width: 320px;
  max-width: 100%;
}
.selected-count {
  color: #909399;
  font-size: 13px;
  white-space: nowrap;
}
.table-area {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.mobile-task-cards {
  display: none;
}
.task-name {
  font-weight: 600;
}
.id-with-pin {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.pin-id {
  min-width: 2em;
  text-align: right;
}
.pin-icon {
  color: #6366f1;
  flex-shrink: 0;
}
.task-desc,
.plugin-id,
.hint {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}
.user-account-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}
.user-account-union {
  color: #909399;
  font-size: 12px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }
  .page-card {
    height: 100%;
  }
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .mobile-info-button {
    display: inline-flex;
  }
  .subtitle {
    display: none;
  }
  .title {
    font-size: 16px;
  }
  .page-header > .el-button {
    width: 100%;
    margin-left: 0;
  }
  .task-toolbar {
    align-items: stretch;
    justify-content: stretch;
  }
  .filter-actions,
  .batch-actions {
    width: 100%;
  }
  .task-search,
  .status-filter {
    width: 100%;
  }
  .status-filter :deep(.el-radio-button) {
    flex: 1;
  }
  .status-filter :deep(.el-radio-button__inner) {
    width: 100%;
  }
  .batch-actions {
    display: grid;
    grid-template-columns: 1fr repeat(3, minmax(64px, auto));
  }
  .desktop-task-table {
    display: none;
  }
  .mobile-task-cards {
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
    overflow-y: auto;
    padding-bottom: 8px;
    -webkit-overflow-scrolling: touch;
  }
  .mobile-task-card {
    padding: 12px;
    border: 1px solid #e4e7ed;
    border-radius: 10px;
    background: #fff;
    transition: border-color 0.2s, background 0.2s;
  }
  .mobile-task-card.selected {
    border-color: #6366f1;
    background: #f5f3ff;
  }
  .mobile-task-head {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 10px;
  }
  .mobile-task-title {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .mobile-task-title strong {
    font-size: 14px;
    font-weight: 600;
    color: #303133;
    word-break: break-word;
  }
  .mobile-task-id {
    font-size: 12px;
    color: #909399;
  }
  .mobile-task-fields {
    display: grid;
    gap: 6px;
    font-size: 12px;
    padding: 8px 0;
    border-top: 1px solid #f0f2f5;
  }
  .mobile-task-fields > div {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 10px;
    min-width: 0;
  }
  .mobile-task-fields span {
    color: #909399;
    flex-shrink: 0;
  }
  .mobile-task-fields strong {
    min-width: 0;
    color: #303133;
    font-weight: 500;
    text-align: right;
    word-break: break-word;
    overflow-wrap: anywhere;
  }
  .mobile-task-actions {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    padding-top: 10px;
    border-top: 1px solid #f0f2f5;
  }
  .mobile-task-actions .el-button {
    width: 100%;
    min-width: 0;
    margin-left: 0;
    padding-inline: 8px;
  }
  .scheduled-tasks :deep(.el-dialog) {
    width: 94vw !important;
  }
  .scheduled-tasks :deep(.el-form-item) {
    display: block;
  }
  .scheduled-tasks :deep(.el-form-item__label) {
    width: 100% !important;
    justify-content: flex-start;
    padding: 0 0 6px;
  }
  .scheduled-tasks :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }
  .mobile-task-cards::-webkit-scrollbar {
    display: none;
  }
}
</style>
