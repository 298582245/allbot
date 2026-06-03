<template>
  <div class="plugins page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="plugins-header page-header">
          <div>
            <span class="title">插件列表</span>
          </div>
          <el-button type="primary" size="small" @click="openCreateDialog">
            <el-icon><Plus /></el-icon>
            新建插件
          </el-button>
        </div>
      </template>

      <div class="plugins-content" v-loading="loading">
      <div class="plugin-grid" v-if="paginatedPlugins.length > 0">
        <div
          class="plugin-card"
          v-for="plugin in paginatedPlugins"
          :key="plugin.id"
        >
          <div class="plugin-card-header">
            <span class="plugin-name">{{ plugin.name }}</span>
            <el-tag
              :type="plugin.enabled ? 'success' : 'info'"
              size="small"
            >
              {{ plugin.enabled ? '已启用' : '已禁用' }}
            </el-tag>
          </div>

          <div class="plugin-card-body">
            <div class="plugin-info-row">
              <span class="label">版本：</span>
              <span>{{ plugin.version }}</span>
            </div>
            <div class="plugin-info-row">
              <span class="label">运行时：</span>
              <el-tag size="small">{{ plugin.runtime }}</el-tag>
            </div>
            <div class="plugin-info-row">
              <span class="label">指令：</span>
              <code class="trigger-text">{{ plugin.trigger || '无' }}</code>
            </div>
            <div class="plugin-info-row">
              <span class="label">优先级：</span>
              <el-tag size="small" type="warning">{{ plugin.priority ?? 0 }}</el-tag>
            </div>
            <div class="plugin-info-row">
              <span class="label">平台：</span>
              <span class="platforms">
                <el-tag
                  v-for="platform in plugin.platforms"
                  :key="platform"
                  size="small"
                  type="info"
                >
                  {{ getPlatformName(platform) }}
                </el-tag>
                <span v-if="!plugin.platforms || plugin.platforms.length === 0" style="color: #999">无</span>
              </span>
            </div>
            <div class="plugin-info-row" v-if="plugin.error">
              <span class="label">错误：</span>
              <span style="color: #f56c6c">{{ plugin.error }}</span>
            </div>
          </div>

          <div class="plugin-card-footer">
            <el-button
              v-if="plugin.enabled"
              type="warning"
              size="small"
              @click="handleDisable(plugin)"
            >
              禁用
            </el-button>
            <el-button
              v-else
              type="success"
              size="small"
              @click="handleEnable(plugin)"
            >
              启用
            </el-button>
            <el-button type="primary" size="small" @click="handleReload(plugin)">
              重载
            </el-button>
            <el-button type="info" size="small" @click="handleConfig(plugin)">
              配置
            </el-button>
            <el-button size="small" @click="handleEditCode(plugin)">
              代码
            </el-button>
            <el-button type="danger" size="small" @click="handleDelete(plugin)">
              删除
            </el-button>
          </div>
        </div>
      </div>

      <el-empty v-if="!loading && plugins.length === 0" description="暂无插件" />
    </div>

      <div class="plugins-pagination">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="plugins.length"
          layout="total, prev, pager, next"
          background
        />
      </div>
    </el-card>

    <!-- 配置编辑对话框 -->
    <el-dialog
      v-model="configDialogVisible"
      title="插件配置"
      width="840px"
      @close="handleConfigDialogClose"
    >
      <el-tabs v-model="configActiveTab" class="plugin-config-tabs">
        <el-tab-pane label="插件配置" name="base">
          <el-form :model="currentConfig" label-width="120px">
            <el-form-item label="插件名称">
              <el-input v-model="currentConfig.name" />
            </el-form-item>
            <el-form-item label="版本">
              <el-input v-model="currentConfig.version" />
            </el-form-item>
            <el-form-item label="运行时">
              <el-select v-model="currentConfig.runtime">
                <el-option label="Python" value="python" />
                <el-option label="Node.js" value="nodejs" />
              </el-select>
            </el-form-item>
            <el-form-item label="入口文件">
              <el-input v-model="currentConfig.entry" />
            </el-form-item>
            <el-form-item label="触发规则">
              <el-input v-model="currentConfig.trigger" placeholder="正则表达式" />
            </el-form-item>
            <el-form-item label="优先级">
              <el-input-number v-model="currentConfig.priority" :step="1" />
              <div class="field-tip">支持负数。匹配多个插件时，数字越大优先级越高。</div>
            </el-form-item>
            <el-form-item label="支持平台">
              <el-checkbox-group v-model="currentConfig.platforms">
                <el-checkbox
                  v-for="option in pluginPlatformOptions"
                  :key="option.value"
                  :label="option.value"
                >
                  {{ option.label }}
                </el-checkbox>
              </el-checkbox-group>
            </el-form-item>
            <el-form-item label="允许机器人">
              <el-select
                v-model="currentConfig.allowed_adapter_ids"
                multiple
                clearable
                collapse-tags
                placeholder="默认全部机器人可用"
                style="width: 100%"
              >
                <el-option
                  v-for="adapter in adapters"
                  :key="String(adapter.id)"
                  :label="getAdapterLabel(adapter)"
                  :value="String(adapter.id)"
                />
              </el-select>
              <div class="field-tip">不选择表示所有机器人都可以触发该插件。</div>
            </el-form-item>
            <el-form-item label="启用状态">
              <el-switch v-model="currentConfig.enabled" />
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="访问控制" name="access">
          <el-form :model="currentConfig" label-width="120px">
            <el-form-item label="沿用系统配置">
              <el-switch v-model="currentConfig.access_control.inherit_system" />
              <div class="field-tip">开启后使用系统权限控制；关闭后使用当前插件自己的配置。</div>
            </el-form-item>
            <template v-if="!currentConfig.access_control.inherit_system">
              <el-form-item label="白名单群">
                <el-select v-model="currentConfig.access_control.whitelist_groups" multiple filterable allow-create default-first-option placeholder="群 ID，可输入多个" style="width: 100%" />
              </el-form-item>
              <el-form-item label="屏蔽群消息">
                <el-select v-model="currentConfig.access_control.blocked_groups" multiple filterable allow-create default-first-option placeholder="群 ID，可输入多个" style="width: 100%" />
              </el-form-item>
              <el-form-item label="白名单 ID">
                <el-select v-model="currentConfig.access_control.whitelist_user_ids" multiple filterable allow-create default-first-option placeholder="用户 ID，可输入多个" style="width: 100%" />
              </el-form-item>
              <el-form-item label="黑名单 ID">
                <el-select v-model="currentConfig.access_control.blocked_user_ids" multiple filterable allow-create default-first-option placeholder="用户 ID，可输入多个" style="width: 100%" />
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>

        <el-tab-pane v-if="userConfigFields.length > 0" label="用户配置" name="user">
          <el-form :model="currentConfig.user_config" label-width="120px">
            <el-form-item
              v-for="field in userConfigFields"
              :key="field.key"
              :label="field.label || field.key"
              :required="Boolean(field.required)"
            >
              <el-switch
                v-if="field.type === 'boolean' || field.type === 'bool'"
                v-model="currentConfig.user_config[field.key]"
              />
              <el-input-number
                v-else-if="field.type === 'number'"
                v-model="currentConfig.user_config[field.key]"
                :step="1"
                style="width: 220px"
              />
              <el-select
                v-else-if="field.type === 'select'"
                v-model="currentConfig.user_config[field.key]"
                style="width: 220px"
              >
                <el-option v-for="option in configSelectOptions(field)" :key="option.value" :label="option.label" :value="option.value" />
              </el-select>
              <el-input
                v-else
                v-model="currentConfig.user_config[field.key]"
                :type="field.type === 'textarea' ? 'textarea' : 'text'"
                :rows="field.type === 'textarea' ? 3 : undefined"
                :placeholder="field.placeholder || ''"
              />
              <div v-if="field.description" class="field-tip">{{ field.description }}</div>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="configDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveConfig">保存</el-button>
      </template>
    </el-dialog>

    <!-- 新建插件对话框 -->
    <el-dialog
      v-model="createDialogVisible"
      title="新建插件"
      width="1200px"
      @close="handleCreateDialogClose"
    >
      <div class="create-dialog-body">
        <div class="create-dialog-left">
          <el-tabs v-model="createActiveTab" class="plugin-config-tabs">
            <el-tab-pane label="基础信息" name="base">
              <el-form :model="createForm" label-width="120px">
                <el-form-item label="插件模板" required>
                  <el-radio-group v-model="createForm.template">
                    <el-radio-button v-for="template in pluginTemplates" :key="template.id" :label="template.id">
                      {{ template.name }}
                    </el-radio-button>
                  </el-radio-group>
                  <div class="field-tip">{{ selectedTemplateDescription }}</div>
                </el-form-item>
                <el-form-item label="本地模板">
                  <div class="preset-row">
                    <el-select v-model="selectedCreatePreset" clearable placeholder="选择已保存模板" style="min-width: 220px">
                      <el-option v-for="item in createPresets" :key="item.name" :label="item.name" :value="item.name" />
                    </el-select>
                    <el-button @click="loadCreatePreset">加载模板</el-button>
                    <el-button @click="saveCreatePreset">保存为模板</el-button>
                    <el-button type="danger" plain @click="deleteCreatePreset">删除模板</el-button>
                  </div>
                  <div class="field-tip">模板和草稿只保存在当前浏览器 localStorage。</div>
                </el-form-item>
                <el-form-item label="插件名称" required>
                  <el-input v-model="createForm.name" placeholder="例如：定时示例插件" />
                </el-form-item>
                <el-form-item label="插件目录 ID">
                  <el-input v-model="createForm.id" placeholder="可选，留空后按插件名称自动生成" />
                  <div class="field-tip">仅支持英文、数字、下划线和短横线，目录已存在时不能重复创建。</div>
                </el-form-item>
                <el-form-item label="版本">
                  <el-input v-model="createForm.version" />
                </el-form-item>
                <el-form-item label="运行时" required>
                  <el-select v-model="createForm.runtime" style="width: 220px" :disabled="isAccountQLTemplate">
                    <el-option label="Node.js" value="nodejs" />
                    <el-option label="Python" value="python" />
                  </el-select>
                  <div v-if="isAccountQLTemplate" class="field-tip">账号青龙模板会按模板自动选择运行时。</div>
                </el-form-item>
                <el-form-item label="触发规则" :required="!isAccountQLTemplate">
                  <el-input v-if="!isAccountQLTemplate" v-model="createForm.trigger" placeholder="正则表达式，例如：^测试$" />
                  <el-input v-else :model-value="accountQLTriggerPreview" readonly />
                  <div v-if="isAccountQLTemplate" class="field-tip">触发规则由后端按指令前缀和指令集合自动生成并转义。</div>
                </el-form-item>
                <el-form-item label="优先级">
                  <el-input-number v-model="createForm.priority" :step="1" />
                  <div class="field-tip">匹配多个插件时，数字越大优先级越高。</div>
                </el-form-item>
                <el-form-item label="支持平台">
                  <el-checkbox-group v-model="createForm.platforms">
                    <el-checkbox
                      v-for="option in pluginPlatformOptions"
                      :key="option.value"
                      :label="option.value"
                    >
                      {{ option.label }}
                    </el-checkbox>
                  </el-checkbox-group>
                </el-form-item>
                <el-form-item label="启用状态">
                  <el-switch v-model="createForm.enabled" />
                </el-form-item>
              </el-form>
            </el-tab-pane>

            <el-tab-pane v-if="!isAccountQLTemplate" label="用户配置" name="user">
              <div class="create-config-header">
                <span>开发者可定义配置键，创建后会自动写入插件模板代码。</span>
                <el-button type="primary" size="small" @click="addCreateConfigField">添加配置</el-button>
              </div>
              <div v-if="createForm.user_config_schema.length > 0" class="create-config-list">
                <div
                  v-for="(field, index) in createForm.user_config_schema"
                  :key="index"
                  class="create-config-row"
                >
                  <el-input v-model="field.key" placeholder="键名，如 cron" />
                  <el-input v-model="field.description" placeholder="描述，如 定时表达式" />
                  <el-input v-model="field.default" placeholder="默认值，如 0 */30 * * * *" />
                  <el-button type="danger" plain @click="removeCreateConfigField(index)">删除</el-button>
                </div>
              </div>
              <el-empty v-else description="暂无用户配置字段" />
            </el-tab-pane>

            <el-tab-pane v-if="isAccountQLTemplate" label="青龙配置" name="account">
              <el-form :model="createForm.account_ql" label-width="150px">
                <el-form-item label="指令前缀" required>
                  <el-input v-model="createForm.account_ql.prefix" placeholder="例如：粉象" />
                </el-form-item>
                <el-form-item label="账号表名" required>
                  <el-input v-model="createForm.account_ql.table_name" placeholder="例如：fxsh_accounts" />
                  <div class="field-tip">只能使用英文、数字和下划线，且不能以数字开头。</div>
                </el-form-item>
                <el-form-item label="青龙变量名" required>
                  <el-input v-model="createForm.account_ql.env_name" placeholder="例如：FXSH_COOKIE" />
                  <div class="field-tip">需要符合环境变量名规则。</div>
                </el-form-item>
                <el-form-item label="青龙脚本语言" required>
                  <el-select v-model="createForm.account_ql.script_runtime" style="width: 220px" @change="handleCreateScriptRuntimeChange">
                    <el-option label="Node.js 脚本" value="nodejs" />
                    <el-option label="Python 脚本" value="python" />
                  </el-select>
                  <div class="field-tip">脚本语言只决定青龙任务脚本，和上面的插件语言互不绑定。</div>
                </el-form-item>
                <el-form-item label="青龙脚本路径" required>
                  <el-input v-model="createForm.account_ql.task_script" :placeholder="createForm.account_ql.script_runtime === 'python' ? 'scripts/task.py' : 'scripts/task.js'" />
                  <div class="field-tip">插件目录内相对路径，{{ createForm.account_ql.script_runtime === 'python' ? '仅支持 .py。' : '仅支持 .js、.mjs、.cjs。' }}</div>
                </el-form-item>
                <el-form-item label="授权价格">
                  <el-input-number v-model="createForm.account_ql.auth_price_per_month" :min="0" :step="1" />
                </el-form-item>
                <el-form-item label="运行定时 cron">
                  <el-input v-model="createForm.account_ql.cron" />
                </el-form-item>
                <el-form-item label="运行超时秒数">
                  <el-input-number v-model="createForm.account_ql.run_wait_timeout" :min="1" :step="60" />
                </el-form-item>
                <el-form-item label="启用 CK 检测">
                  <el-switch v-model="createForm.account_ql.enable_ck_check" />
                </el-form-item>
                <el-form-item v-if="createForm.account_ql.enable_ck_check" label="CK 检测 cron">
                  <el-input v-model="createForm.account_ql.ck_check_cron" />
                </el-form-item>
                <el-form-item label="启用过期检测">
                  <el-switch v-model="createForm.account_ql.enable_expire_check" />
                </el-form-item>
                <template v-if="createForm.account_ql.enable_expire_check">
                  <el-form-item label="过期检测 cron">
                    <el-input v-model="createForm.account_ql.expire_check_cron" />
                  </el-form-item>
                  <el-form-item label="提醒天数">
                    <el-input v-model="createForm.account_ql.expire_notify_days" placeholder="7,3,1,0" />
                  </el-form-item>
                  <el-form-item label="删除天数">
                    <el-input-number v-model="createForm.account_ql.expire_delete_after_days" :min="-1" :step="1" />
                    <div class="field-tip">-1 表示不自动删除；0 表示到期后检测到就删除。</div>
                  </el-form-item>
                </template>
              </el-form>
            </el-tab-pane>

            <el-tab-pane v-if="isAccountQLTemplate" label="自定义代码" name="code">
              <div class="code-editor-section">
                <div class="code-editor-title">登录解析代码</div>
                <div class="field-tip">必须定义 {{ isPythonAccountQLTemplate ? 'def parse_input(raw, ctx)' : 'function parseInput(raw, ctx)' }}。</div>
                <div ref="parseInputEditorContainer" class="create-code-editor"></div>
              </div>
              <div class="code-editor-section">
                <div class="code-editor-title">查询代码</div>
                <div class="field-tip">必须定义 {{ isPythonAccountQLTemplate ? 'async def query(account, ctx, index)' : 'async function query(account, ctx, index)' }}。</div>
                <div ref="queryEditorContainer" class="create-code-editor"></div>
              </div>
              <div v-if="createForm.account_ql.enable_ck_check" class="code-editor-section">
                <div class="code-editor-title">CK 检测代码</div>
                <div class="field-tip">必须定义 {{ isPythonAccountQLTemplate ? 'async def check_ck(account, ctx)' : 'async function checkCk(account, ctx)' }}。</div>
                <div ref="checkCkEditorContainer" class="create-code-editor"></div>
              </div>
            </el-tab-pane>

            <el-tab-pane v-if="isAccountQLTemplate" label="自定义指令" name="routes">
              <div class="create-config-header">
                <span>自定义指令会同时写入 trigger 和 SDK routes。</span>
                <el-button type="primary" size="small" @click="addCreateRoute">添加指令</el-button>
              </div>
              <div v-if="createForm.account_ql.routes.length > 0" class="route-list">
                <div v-for="(route, index) in createForm.account_ql.routes" :key="route.id" class="route-item">
                  <div class="route-row">
                    <el-input v-model="route.command" placeholder="指令，如 提现" />
                    <el-input v-model="route.function_name" :placeholder="isPythonAccountQLTemplate ? 'custom_route' : 'customRoute'" />
                    <el-input v-model="route.description" placeholder="描述" />
                    <el-button type="danger" plain @click="removeCreateRoute(index)">删除</el-button>
                  </div>
                  <el-input v-model="route.code" type="textarea" :rows="9" :placeholder="defaultRouteCode(route.function_name || defaultRouteFunctionName(index))" />
                </div>
              </div>
              <el-empty v-else description="暂无自定义指令" />
            </el-tab-pane>
          </el-tabs>
        </div>
        <div class="create-preview-panel" v-loading="createPreviewLoading">
          <div class="create-preview-title">生成预览</div>
          <div class="create-preview-summary" v-if="createPreviewData || createForm.name || createForm.trigger">
            <div>插件运行时：{{ createPreviewData?.runtime || createPreviewData?.normalized?.runtime || createForm.runtime || '-' }}</div>
            <div v-if="isAccountQLTemplate">脚本运行时：{{ createPreviewData?.normalized?.script_runtime || createPreviewData?.metadata?.script_runtime || createForm.account_ql.script_runtime || '-' }}</div>
            <div>入口：{{ createPreviewData?.entry || createPreviewData?.normalized?.entry || '-' }}</div>
            <div v-if="isAccountQLTemplate">脚本：{{ createPreviewData?.normalized?.task_script || createPreviewData?.metadata?.task_script || createForm.account_ql.task_script || '-' }}</div>
            <div>触发：{{ createPreviewData?.trigger || createPreviewData?.normalized?.trigger || createForm.trigger || '-' }}</div>
            <div v-if="createPreviewData?.commands?.length">指令：{{ createPreviewData.commands.join(' / ') }}</div>
          </div>
          <div v-if="createPreviewIssues.length" class="preview-issues">
            <div v-for="(issue, index) in createPreviewIssues" :key="index" class="preview-issue">{{ issue.message }}</div>
          </div>
          <el-collapse v-if="createPreviewFiles.length" class="preview-files">
            <el-collapse-item v-for="file in createPreviewFiles" :key="file.path" :title="file.path + '（' + file.role + '，' + file.bytes + ' bytes）'" :name="file.path">
              <pre>{{ file.content }}</pre>
            </el-collapse-item>
          </el-collapse>
          <pre v-else>{{ createPreview }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="createSaving" @click="saveCreatedPlugin">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="createResultVisible" title="插件创建结果" width="820px" @close="createResult = null">
      <div v-if="createResult" class="create-result">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="插件 ID">{{ createResult.plugin_id || createResult.id }}</el-descriptions-item>
          <el-descriptions-item label="模板">{{ createResult.template }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ createResult.template_version }}</el-descriptions-item>
          <el-descriptions-item label="插件运行时">{{ createResult.runtime }}</el-descriptions-item>
          <el-descriptions-item label="入口">{{ createResult.entry }}</el-descriptions-item>
          <el-descriptions-item v-if="createResult.metadata?.script_runtime" label="脚本运行时">{{ createResult.metadata.script_runtime }}</el-descriptions-item>
          <el-descriptions-item v-if="createResult.metadata?.task_script" label="青龙脚本">{{ createResult.metadata.task_script }}</el-descriptions-item>
          <el-descriptions-item label="触发规则">{{ createResult.trigger }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="createResult.commands?.length" class="create-result-section">
          <div class="create-result-title">指令</div>
          <el-tag v-for="command in createResult.commands" :key="command" size="small">{{ command }}</el-tag>
        </div>
        <div class="create-result-section">
          <div class="create-result-title">生成文件</div>
          <el-table :data="createResult.files || []" size="small" border>
            <el-table-column prop="path" label="文件" min-width="220" />
            <el-table-column prop="role" label="类型" width="110" />
            <el-table-column prop="bytes" label="大小" width="110" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.error ? 'danger' : 'success'" size="small">{{ row.error ? '失败' : '成功' }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="create-result-section">
          <div class="create-result-title">诊断</div>
          <el-table :data="createResult.diagnostics || []" size="small" border>
            <el-table-column prop="step" label="步骤" width="150" />
            <el-table-column prop="target" label="目标" width="180" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="row.ok ? 'success' : 'danger'" size="small">{{ row.ok ? '成功' : '失败' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="说明" min-width="220" />
          </el-table>
        </div>
      </div>
      <template #footer>
        <el-button @click="createResultVisible = false">留在列表</el-button>
        <el-button @click="continueCreatePlugin">继续创建</el-button>
        <el-button type="primary" @click="viewCreatedPluginCode">查看代码</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getAdapters, getPlugins, controlPlugin, deletePlugin, getPluginTemplates, previewCreatePlugin, validateCreatePlugin, createPlugin } from '@/api'
import request from '@/utils/request'
import { EditorView, basicSetup } from 'codemirror'
import { javascript } from '@codemirror/lang-javascript'
import { python } from '@codemirror/lang-python'
import { oneDark } from '@codemirror/theme-one-dark'

const router = useRouter()

const loading = ref(false)
const plugins = ref([])
const adapters = ref([])
const currentPage = ref(1)
const pageSize = 9
const pluginDefaultPlatforms = ['qq', 'qq_office', 'telegram']
const pluginPlatformOptions = [
  { label: 'QQ', value: 'qq' },
  { label: 'QQ 官方机器人', value: 'qq_office' },
  { label: '微信', value: 'wechat' },
  { label: 'Telegram', value: 'telegram' }
]
const pluginPlatformNames = Object.fromEntries(pluginPlatformOptions.map(option => [option.value, option.label]))
const configDialogVisible = ref(false)
const createDialogVisible = ref(false)
const createSaving = ref(false)
const createPreviewLoading = ref(false)
const createResultVisible = ref(false)
const createResult = ref(null)
const createActiveTab = ref('base')
const currentPluginId = ref('')
const configActiveTab = ref('base')
const createForm = ref(createEmptyPluginForm())
const parseInputEditorContainer = ref(null)
const queryEditorContainer = ref(null)
const checkCkEditorContainer = ref(null)
const createDraftKey = 'allbot:create-plugin-draft:v3'
const createDraftLegacyKey = 'allbot:create-plugin-draft:v2'
const createPresetKey = 'allbot:create-plugin-presets:v3'
const createPresetLegacyKey = 'allbot:create-plugin-presets:v2'
const selectedCreatePreset = ref('')
const createPresets = ref([])
const pluginTemplates = ref(defaultPluginTemplates())
const createPreview = shallowRef('')
const createPreviewData = shallowRef(null)
let createDraftTimer = null
let createPreviewTimer = null
let parseInputEditorView = null
let queryEditorView = null
let checkCkEditorView = null
let lastAccountQLDefaults = { prefix: '', table_name: '', env_name: '', task_script: 'scripts/task.js', script_runtime: 'nodejs' }
let lastAccountQLRuntime = 'nodejs'
let lastAccountQLScriptRuntime = 'nodejs'
let accountQLScriptRuntimeTouched = false
const currentConfig = ref({
  name: '',
  version: '',
  runtime: 'python',
  entry: '',
  trigger: '',
  priority: 0,
  platforms: [],
  allowed_adapter_ids: [],
  access_control: createAccessControl(true),
  user_config_schema: [],
  user_config: {},
  enabled: true
})

const userConfigFields = computed(() => {
  return Array.isArray(currentConfig.value.user_config_schema)
    ? currentConfig.value.user_config_schema.filter(field => field && field.key)
    : []
})

function configSelectOptions(field) {
  if (Array.isArray(field.options) && field.options.length) {
    return field.options.map(option => {
      if (option && typeof option === 'object') return { label: option.label || option.value, value: option.value }
      return { label: String(option), value: String(option) }
    }).filter(option => option.value)
  }
  if (field.key === 'script_runtime') {
    return [{ label: 'Node.js 脚本', value: 'nodejs' }, { label: 'Python 脚本', value: 'python' }]
  }
  return [{ label: String(field.default || ''), value: String(field.default || '') }].filter(option => option.value)
}

const isAccountQLTemplate = computed(() => createForm.value.template === 'nodejs_account_ql' || createForm.value.template === 'python_account_ql')
const isPythonAccountQLTemplate = computed(() => createForm.value.template === 'python_account_ql')
const accountQLRuntime = computed(() => isPythonAccountQLTemplate.value ? 'python' : 'nodejs')
const selectedTemplateDescription = computed(() => {
  const template = pluginTemplates.value.find(item => item.id === createForm.value.template)
  if (!template) return '选择用于生成插件代码的后端模板。'
  const features = Array.isArray(template.features) && template.features.length ? ` 支持：${template.features.join('、')}。` : ''
  return `${template.description || '选择用于生成插件代码的后端模板。'}${features}`
})
const createPreviewFiles = computed(() => Array.isArray(createPreviewData.value?.files) ? createPreviewData.value.files : [])
const createPreviewIssues = computed(() => [
  ...(Array.isArray(createPreviewData.value?.errors) ? createPreviewData.value.errors : []),
  ...(Array.isArray(createPreviewData.value?.warnings) ? createPreviewData.value.warnings : [])
])

const accountQLCommands = computed(() => {
  const accountQL = createForm.value.account_ql || {}
  const commands = ['登录', '账号', '管理', '查询', '运行', '一键运行', '签到', '删除', '授权', '帮助']
  if (accountQL.enable_ck_check) commands.push('CK检测')
  if (accountQL.enable_expire_check) commands.push('过期检测')
  ;(accountQL.routes || []).forEach((route) => {
    const command = String(route.command || '').trim()
    if (command) commands.push(command)
  })
  return commands
})

const accountQLTriggerPreview = computed(() => {
  if (createPreviewData.value?.trigger) return createPreviewData.value.trigger
  const prefix = String(createForm.value.account_ql?.prefix || '前缀').trim() || '前缀'
  return `^(${escapeRegExp(prefix)})(${accountQLCommands.value.map(escapeRegExp).join('|')})$`
})


const paginatedPlugins = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return plugins.value.slice(start, start + pageSize)
})

const loadPlugins = async () => {
  loading.value = true
  try {
    plugins.value = await getPlugins()
  } catch (error) {
    console.error('加载插件失败:', error)
  } finally {
    loading.value = false
  }
}

const loadAdapters = async () => {
  try {
    adapters.value = await getAdapters()
  } catch (error) {
    console.error('加载机器人失败:', error)
  }
}

const loadPluginTemplates = async () => {
  try {
    const templates = await getPluginTemplates()
    if (Array.isArray(templates) && templates.length) {
      pluginTemplates.value = templates
    }
  } catch (error) {
    pluginTemplates.value = defaultPluginTemplates()
  }
}

const getPlatformName = (platform) => pluginPlatformNames[platform] || platform

const getAdapterLabel = (adapter) => {
  const name = adapter.remark || `${getPlatformName(adapter.platform)} #${adapter.id}`
  return `${name}（${getPlatformName(adapter.platform)} / ID ${adapter.id}）`
}

const handleEnable = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'enable')
    ElMessage.success(`插件 ${plugin.name} 已启用`)
    await loadPlugins()
  } catch (error) {
    console.error('启用插件失败:', error)
    ElMessage.error('启用插件失败')
  }
}

const handleDisable = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'disable')
    ElMessage.success(`插件 ${plugin.name} 已禁用`)
    await loadPlugins()
  } catch (error) {
    console.error('禁用插件失败:', error)
    ElMessage.error('禁用插件失败')
  }
}

const handleReload = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'reload')
    ElMessage.success(`插件 ${plugin.name} 已重新加载`)
    await loadPlugins()
  } catch (error) {
    console.error('重新加载插件失败:', error)
    ElMessage.error('重新加载插件失败')
  }
}

const handleConfig = async (plugin) => {
  try {
    currentPluginId.value = plugin.id
    const config = await request.get(`/plugins/config/${plugin.id}`)
    config.platforms = Array.isArray(config.platforms) ? config.platforms : []
    config.allowed_adapter_ids = config.allowed_adapter_ids || []
    config.priority = Number(config.priority || 0)
    config.access_control = normalizeAccessControl(config.access_control, true)
    config.user_config_schema = Array.isArray(config.user_config_schema) ? config.user_config_schema : []
    config.user_config = normalizeUserConfig(config.user_config_schema, config.user_config)
    currentConfig.value = config
    configActiveTab.value = 'base'
    configDialogVisible.value = true
  } catch (error) {
    console.error('获取插件配置失败:', error)
    ElMessage.error('获取插件配置失败')
  }
}

const handleEditCode = (plugin) => {
  router.push(`/plugins/${plugin.id}/edit`)
}

const viewCreatedPluginCode = () => {
  const pluginID = createResult.value?.plugin_id || createResult.value?.id
  if (!pluginID) return
  createResultVisible.value = false
  router.push(`/plugins/${pluginID}/edit`)
}

const continueCreatePlugin = async () => {
  createResultVisible.value = false
  clearCreateDraft()
  await nextTick()
  openCreateDialog()
}

const openCreateDialog = () => {
  destroyCreateEditors()
  resetAccountQLDefaultState()
  const draft = loadCreateDraft()
  createResult.value = null
  createForm.value = draft || createEmptyPluginForm()
  accountQLScriptRuntimeTouched = Boolean(draft?.account_ql?.script_runtime)
  normalizeCreateFormShape(createForm.value)
  if (isAccountQLTemplate.value) ensureAccountQLDefaults()
  createActiveTab.value = 'base'
  createDialogVisible.value = true
  updateCreatePreview()
  if (draft) ElMessage.info('已恢复上次未创建的本地草稿')
}

const addCreateConfigField = () => {
  createForm.value.user_config_schema.push({
    key: '',
    description: '',
    default: ''
  })
}

const removeCreateConfigField = (index) => {
  createForm.value.user_config_schema.splice(index, 1)
}

const saveCreatedPlugin = async () => {
  try {
    syncCreateEditorCode()
    const payload = normalizeCreatePayload(createForm.value)
    createSaving.value = true
    const validation = await validateCreatePlugin(payload)
    if (!validation?.ok) {
      createPreviewData.value = { ...(createPreviewData.value || {}), errors: validation?.errors || [], warnings: validation?.warnings || [] }
      showCreateValidationIssues(validation?.errors || [{ message: '创建配置校验未通过', tab: 'base' }])
      return
    }
    const result = await createPlugin(payload)
    ElMessage.success('插件创建成功')
    createResult.value = result
    createResultVisible.value = true
    clearCreateDraft()
    createDialogVisible.value = false
    await loadPlugins()
  } catch (error) {
    console.error('创建插件失败:', error)
    ElMessage.error(error?.response?.data?.error || '创建插件失败')
  } finally {
    createSaving.value = false
  }
}

async function showCreateValidationIssues(issues) {
  const first = Array.isArray(issues) && issues.length ? issues[0] : { message: '创建配置校验未通过', tab: 'base' }
  const tabMap = { ql: 'account', base: 'base', code: 'code', routes: 'routes', user: 'user' }
  createActiveTab.value = tabMap[first.tab] || first.tab || 'base'
  ElMessage.warning(first.message || '创建配置校验未通过')
  if (createActiveTab.value === 'code') {
    await nextTick()
    ensureCreateEditors()
  }
}

const handleCreateDialogClose = () => {
  if (!createResultVisible.value) saveCreateDraft()
  createSaving.value = false
  updateCreatePreview.latestRequestID = 0
  createPreviewLoading.value = false
  createPreviewData.value = null
  createPreview.value = ''
  createActiveTab.value = 'base'
  destroyCreateEditors()
  resetAccountQLDefaultState()
  createForm.value = createEmptyPluginForm()
}

const saveConfig = async () => {
  try {
    currentConfig.value.priority = Number(currentConfig.value.priority || 0)
    currentConfig.value.access_control = normalizeAccessControl(currentConfig.value.access_control, true)
    await request.put(`/plugins/config/${currentPluginId.value}`, currentConfig.value)
    ElMessage.success('配置已保存并生效')
    configDialogVisible.value = false
    await loadPlugins()
  } catch (error) {
    console.error('保存配置失败:', error)
    ElMessage.error('保存配置失败')
  }
}

const handleConfigDialogClose = () => {
  configActiveTab.value = 'base'
  currentConfig.value = {
    name: '',
    version: '',
    runtime: 'python',
    entry: '',
    trigger: '',
    priority: 0,
    platforms: [],
    allowed_adapter_ids: [],
    access_control: createAccessControl(true),
    user_config_schema: [],
    user_config: {},
    enabled: true
  }
}

function createAccessControl(inheritSystem = false) {
  return {
    inherit_system: inheritSystem,
    whitelist_groups: [],
    blocked_groups: [],
    whitelist_user_ids: [],
    blocked_user_ids: []
  }
}

function defaultPluginTemplates() {
  return [
    { id: 'basic', name: '普通插件', runtime: 'nodejs', version: '3.0.0', description: '生成基础 Node.js 或 Python 插件骨架', features: ['基础触发正则', '用户配置', '空依赖'], defaults: { runtime: 'nodejs', version: '1.0.0', platforms: [...pluginDefaultPlatforms] } },
    { id: 'nodejs_account_ql', name: 'Node.js 青龙账号插件', runtime: 'nodejs', version: '3.0.0', description: '生成 Node.js 青龙账号插件、任务脚本和账号授权配置', features: ['账号登录', '账号查询', '青龙脚本运行', 'CK 检测', '自定义指令'], defaults: { runtime: 'nodejs', script_runtime: 'nodejs', version: '1.0.0', task_script: 'scripts/task.js' } },
    { id: 'python_account_ql', name: 'Python 青龙账号插件', runtime: 'python', version: '3.0.0', description: '生成 Python 青龙账号插件、任务脚本和账号授权配置', features: ['账号登录', '账号查询', '青龙脚本运行', 'CK 检测', '自定义指令'], defaults: { runtime: 'python', script_runtime: 'python', version: '1.0.0', task_script: 'scripts/task.py' } }
  ]
}

function createEmptyPluginForm() {
  return {
    id: '',
    name: '',
    version: '1.0.0',
    runtime: 'nodejs',
    trigger: '',
    priority: 0,
    platforms: [...pluginDefaultPlatforms],
    enabled: true,
    template: 'basic',
    user_config_schema: [],
    account_ql: {
      prefix: '',
      table_name: '',
      env_name: '',
      task_script: 'scripts/task.js',
      script_runtime: 'nodejs',
      auth_price_per_month: 0,
      cron: '0 8 * * *',
      enable_ck_check: true,
      ck_check_cron: '25 9 * * *',
      check_ck_code: defaultCheckCkCode('nodejs'),
      enable_expire_check: false,
      expire_check_cron: '15 9 * * *',
      expire_notify_days: '7,3,1,0',
      expire_delete_after_days: -1,
      run_wait_timeout: 7200,
      parse_input_code: defaultParseInputCode('nodejs'),
      query_code: defaultQueryCode('nodejs'),
      routes: []
    }
  }
}

function normalizeCreatePayload(form) {
  if (form.template === 'nodejs_account_ql' || form.template === 'python_account_ql') {
    const runtime = form.template === 'python_account_ql' ? 'python' : 'nodejs'
    const accountQL = form.account_ql || {}
    return {
      id: String(form.id || '').trim(),
      name: String(form.name || '').trim(),
      version: String(form.version || '1.0.0').trim(),
      runtime,
      template: form.template,
      priority: Number(form.priority || 0),
      platforms: Array.isArray(form.platforms) ? form.platforms : [],
      enabled: Boolean(form.enabled),
      account_ql: {
        prefix: String(accountQL.prefix || '').trim(),
        table_name: String(accountQL.table_name || '').trim(),
        env_name: String(accountQL.env_name || '').trim(),
        task_script: String(accountQL.task_script || '').trim(),
        script_runtime: normalizeScriptRuntime(accountQL.script_runtime, accountQL.task_script, runtime),
        auth_price_per_month: Math.max(0, Number(accountQL.auth_price_per_month || 0)),
        cron: String(accountQL.cron || '').trim(),
        enable_ck_check: Boolean(accountQL.enable_ck_check),
        ck_check_cron: String(accountQL.ck_check_cron || '').trim(),
        check_ck_code: String(accountQL.check_ck_code || '').trim(),
        enable_expire_check: Boolean(accountQL.enable_expire_check),
        expire_check_cron: String(accountQL.expire_check_cron || '').trim(),
        expire_notify_days: String(accountQL.expire_notify_days || '').trim(),
        expire_delete_after_days: Number(accountQL.expire_delete_after_days ?? -1),
        run_wait_timeout: Math.max(1, Number(accountQL.run_wait_timeout || 7200)),
        parse_input_code: String(accountQL.parse_input_code || '').trim(),
        query_code: String(accountQL.query_code || '').trim(),
        routes: (accountQL.routes || []).map((route, index) => ({
          command: String(route.command || '').trim(),
          function_name: String(route.function_name || defaultRouteFunctionName(index, runtime)).trim(),
          description: String(route.description || '').trim(),
          code: String(route.code || '').trim()
        })).filter(route => route.command || route.code)
      }
    }
  }
  const schema = []
  const userConfig = {}
  const seen = new Set()
  ;(form.user_config_schema || []).forEach((field) => {
    const key = normalizeConfigKey(field.key)
    if (!key || seen.has(key)) return
    seen.add(key)
    const defaultValue = field.default ?? ''
    schema.push({
      key,
      label: key,
      type: 'text',
      default: defaultValue,
      description: String(field.description || '').trim()
    })
    userConfig[key] = defaultValue
  })
  return {
    id: String(form.id || '').trim(),
    name: String(form.name || '').trim(),
    version: String(form.version || '1.0.0').trim(),
    runtime: form.runtime,
    template: 'basic',
    trigger: String(form.trigger || '').trim(),
    priority: Number(form.priority || 0),
    platforms: Array.isArray(form.platforms) ? form.platforms : [],
    enabled: Boolean(form.enabled),
    user_config_schema: schema,
    user_config: userConfig
  }
}

function defaultParseInputCode(runtime = accountQLRuntime.value) {
  if (runtime === 'python') {
    return `def parse_input(raw, ctx):
    value = str(raw or '').strip()
    if not value:
        raise RuntimeError('账号 CK 不能为空')
    return {
        "env_value": value,
        "unique_key": value,
        "display_name": value[:8],
    }`
  }
  return `function parseInput(raw, ctx) {
  const value = String(raw || '').trim();
  if (!value) throw new Error('账号 CK 不能为空');
  return {
    envValue: value,
    uniqueKey: value,
    displayName: value.slice(0, 8)
  };
}`
}

function defaultQueryCode(runtime = accountQLRuntime.value) {
  if (runtime === 'python') {
    return `async def query(account, ctx, index):
    return {
        "状态": account.get("status") or "active",
    }`
  }
  return "async function query(account, ctx, index) {\n  return `${index + 1}. ${account.account_name}｜${account.status || 'active'}`;\n}"
}

function defaultCheckCkCode(runtime = accountQLRuntime.value) {
  if (runtime === 'python') {
    return `async def check_ck(account, ctx):
    return {
        "valid": True,
        "reason": "",
    }`
  }
  return `async function checkCk(account, ctx) {
  return {
    valid: true,
    reason: ''
  };
}`
}

function defaultRouteFunctionName(index, runtime = accountQLRuntime.value) {
  return runtime === 'python' ? `custom_route_${index + 1}` : `customRoute${index + 1}`
}

function defaultRouteCode(functionName = '', runtime = accountQLRuntime.value) {
  const name = functionName || defaultRouteFunctionName(0, runtime)
  if (runtime === 'python') {
    return `async def ${name}(ctx, helpers):
    accounts = await helpers.list_mine({"status": "active"})
    await ctx.reply(f"账号数：{len(accounts)}")`
  }
  return `async function ${name}(ctx, helpers) {
  const accounts = await helpers.listMine({ status: 'active' });
  return ctx.reply(` + '`账号数：${accounts.length}`' + `);
}`
}

function resetAccountQLDefaultState() {
  lastAccountQLDefaults = { prefix: '', table_name: '', env_name: '', task_script: 'scripts/task.js', script_runtime: 'nodejs' }
  lastAccountQLRuntime = 'nodejs'
  lastAccountQLScriptRuntime = 'nodejs'
  accountQLScriptRuntimeTouched = false
}

function templateDefaultScriptRuntime(template = createForm.value.template) {
  return template === 'python_account_ql' ? 'python' : 'nodejs'
}

function handleCreateScriptRuntimeChange() {
  accountQLScriptRuntimeTouched = true
  ensureAccountQLDefaults()
}

function ensureAccountQLDefaults() {
  const form = createForm.value
  const accountQL = form.account_ql
  if (!accountQL) return
  const runtime = accountQLRuntime.value
  const scriptRuntime = accountQLScriptRuntimeTouched
    ? normalizeScriptRuntime(accountQL.script_runtime, accountQL.task_script, runtime)
    : templateDefaultScriptRuntime(form.template)
  accountQL.script_runtime = scriptRuntime
  const baseID = sanitizePluginID(form.id || form.name) || 'plugin'
  const envBase = sanitizeEnvBase(baseID)
  const ext = scriptRuntime === 'python' ? 'py' : 'js'
  const defaults = {
    prefix: String(form.name || form.id || '').trim(),
    table_name: `${baseID.replace(/-/g, '_')}_accounts`,
    env_name: `${envBase}_CK`,
    task_script: `scripts/${baseID}_task.${ext}`,
    script_runtime: scriptRuntime
  }
  if (!accountQL.prefix || accountQL.prefix === lastAccountQLDefaults.prefix) accountQL.prefix = defaults.prefix
  if (!accountQL.table_name || accountQL.table_name === lastAccountQLDefaults.table_name) accountQL.table_name = defaults.table_name
  if (!accountQL.env_name || accountQL.env_name === lastAccountQLDefaults.env_name) accountQL.env_name = defaults.env_name
  if (!accountQL.task_script || accountQL.task_script === lastAccountQLDefaults.task_script || isDefaultTaskScript(accountQL.task_script, baseID) || lastAccountQLScriptRuntime !== scriptRuntime) accountQL.task_script = defaults.task_script
  syncDefaultRuntimeCode(runtime)
  lastAccountQLDefaults = defaults
  lastAccountQLRuntime = runtime
  lastAccountQLScriptRuntime = scriptRuntime
  form.runtime = runtime
}

function syncDefaultRuntimeCode(runtime) {
  const accountQL = createForm.value.account_ql
  if (!accountQL) return
  const previousRuntime = lastAccountQLRuntime || (runtime === 'python' ? 'nodejs' : 'python')
  if (!accountQL.parse_input_code || accountQL.parse_input_code === defaultParseInputCode(previousRuntime)) accountQL.parse_input_code = defaultParseInputCode(runtime)
  if (!accountQL.query_code || accountQL.query_code === defaultQueryCode(previousRuntime)) accountQL.query_code = defaultQueryCode(runtime)
  if (!accountQL.check_ck_code || accountQL.check_ck_code === defaultCheckCkCode(previousRuntime)) accountQL.check_ck_code = defaultCheckCkCode(runtime)
}

function isDefaultTaskScript(value, baseID) {
  return value === `scripts/${baseID}_task.js` || value === `scripts/${baseID}_task.py` || value === 'scripts/task.js' || value === 'scripts/task.py'
}

function normalizeScriptRuntime(value, taskScript = '', fallback = 'nodejs') {
  const runtime = String(value || '').trim().toLowerCase()
  if (runtime === 'node' || runtime === 'js' || runtime === 'javascript') return 'nodejs'
  if (runtime === 'nodejs' || runtime === 'python') return runtime
  const ext = String(taskScript || '').trim().toLowerCase().split('?')[0]
  if (ext.endsWith('.py')) return 'python'
  if (ext.endsWith('.js') || ext.endsWith('.mjs') || ext.endsWith('.cjs')) return 'nodejs'
  return fallback === 'python' ? 'python' : 'nodejs'
}

function sanitizePluginID(value) {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]/g, '_')
    .replace(/^[_-]+|[_-]+$/g, '')
}

function sanitizeEnvBase(value) {
  const normalized = String(value || '')
    .trim()
    .toUpperCase()
    .replace(/[^A-Z0-9_]/g, '_')
    .replace(/^_+|_+$/g, '')
  if (!normalized) return 'PLUGIN'
  return /^[A-Z_]/.test(normalized) ? normalized : `PLUGIN_${normalized}`
}

function createCodeEditor(container, code, onUpdate) {
  if (!container) return null
  return new EditorView({
    doc: String(code || ''),
    extensions: [
      basicSetup,
      accountQLRuntime.value === 'python' ? python() : javascript(),
      oneDark,
      EditorView.lineWrapping,
      EditorView.updateListener.of((update) => {
        if (update.docChanged && typeof onUpdate === 'function') {
          onUpdate(update.state.doc.toString())
          scheduleCreatePreviewUpdate()
          scheduleCreateDraftSave()
        }
      })
    ],
    parent: container
  })
}

function ensureCreateEditors() {
  if (!isAccountQLTemplate.value || createActiveTab.value !== 'code') return
  const accountQL = createForm.value.account_ql
  if (!accountQL) return
  if (!accountQL.enable_ck_check && checkCkEditorView) {
    checkCkEditorView.destroy()
    checkCkEditorView = null
  }
  if (!parseInputEditorContainer.value || !queryEditorContainer.value || (accountQL.enable_ck_check && !checkCkEditorContainer.value)) return
  if (!parseInputEditorView) {
    parseInputEditorView = createCodeEditor(parseInputEditorContainer.value, accountQL.parse_input_code, (code) => {
      if (createForm.value.account_ql) createForm.value.account_ql.parse_input_code = code
    })
  }
  if (!queryEditorView) {
    queryEditorView = createCodeEditor(queryEditorContainer.value, accountQL.query_code, (code) => {
      if (createForm.value.account_ql) createForm.value.account_ql.query_code = code
    })
  }
  if (accountQL.enable_ck_check && !checkCkEditorView) {
    checkCkEditorView = createCodeEditor(checkCkEditorContainer.value, accountQL.check_ck_code, (code) => {
      if (createForm.value.account_ql) createForm.value.account_ql.check_ck_code = code
    })
  }
}

function syncCreateEditorCode() {
  const accountQL = createForm.value.account_ql
  if (!accountQL) return
  if (parseInputEditorView) accountQL.parse_input_code = parseInputEditorView.state.doc.toString()
  if (queryEditorView) accountQL.query_code = queryEditorView.state.doc.toString()
  if (checkCkEditorView) accountQL.check_ck_code = checkCkEditorView.state.doc.toString()
}

function destroyCreateEditors() {
  syncCreateEditorCode()
  if (parseInputEditorView) {
    parseInputEditorView.destroy()
    parseInputEditorView = null
  }
  if (queryEditorView) {
    queryEditorView.destroy()
    queryEditorView = null
  }
  if (checkCkEditorView) {
    checkCkEditorView.destroy()
    checkCkEditorView = null
  }
}

function normalizeConfigKey(value) {
  return String(value || '')
    .trim()
    .replace(/[^A-Za-z0-9_]/g, '_')
    .replace(/_+/g, '_')
    .replace(/^_+|_+$/g, '')
}

function normalizeAccessControl(value, defaultInherit = false) {
  const source = value && typeof value === 'object' ? value : {}
  const list = (items) => Array.isArray(items) ? items.map(item => String(item).trim()).filter(Boolean) : []
  return {
    inherit_system: Object.prototype.hasOwnProperty.call(source, 'inherit_system') ? Boolean(source.inherit_system) : defaultInherit,
    whitelist_groups: list(source.whitelist_groups),
    blocked_groups: list(source.blocked_groups),
    whitelist_user_ids: list(source.whitelist_user_ids),
    blocked_user_ids: list(source.blocked_user_ids)
  }
}

const normalizeUserConfig = (schema, values) => {
  const result = { ...(values && typeof values === 'object' && !Array.isArray(values) ? values : {}) }
  schema.forEach((field) => {
    if (!field || !field.key || Object.prototype.hasOwnProperty.call(result, field.key)) return
    if (Object.prototype.hasOwnProperty.call(field, 'default')) {
      result[field.key] = field.default
      return
    }
    if (field.type === 'boolean' || field.type === 'bool') {
      result[field.key] = false
    } else if (field.type === 'number') {
      result[field.key] = 0
    } else {
      result[field.key] = ''
    }
  })
  return result
}

function normalizeCreateFormShape(form) {
  const defaults = createEmptyPluginForm()
  const hasScriptRuntime = Boolean(form.account_ql && form.account_ql.script_runtime)
  form.account_ql = { ...defaults.account_ql, ...(form.account_ql || {}) }
  form.account_ql.routes = Array.isArray(form.account_ql.routes) ? form.account_ql.routes.map((route, index) => ({ id: route.id || `${Date.now()}_${index}`, command: route.command || '', function_name: route.function_name || '', description: route.description || '', code: route.code || '' })) : []
  form.account_ql.script_runtime = normalizeScriptRuntime(hasScriptRuntime ? form.account_ql.script_runtime : '', form.account_ql.task_script, templateDefaultScriptRuntime(form.template))
  form.platforms = Array.isArray(form.platforms) ? form.platforms : [...defaults.platforms]
  form.user_config_schema = Array.isArray(form.user_config_schema) ? form.user_config_schema : []
  if (form.template === 'python_account_ql') form.runtime = 'python'
  if (form.template === 'nodejs_account_ql') form.runtime = 'nodejs'
}

function addCreateRoute() {
  const index = createForm.value.account_ql.routes.length
  const functionName = defaultRouteFunctionName(index)
  createForm.value.account_ql.routes.push({
    id: `${Date.now()}_${index}`,
    command: '',
    function_name: functionName,
    description: '',
    code: defaultRouteCode(functionName)
  })
}

function removeCreateRoute(index) {
  createForm.value.account_ql.routes.splice(index, 1)
}

function loadCreateDraft() {
  try {
    const raw = localStorage.getItem(createDraftKey) || localStorage.getItem(createDraftLegacyKey)
    if (!raw) return null
    const draft = JSON.parse(raw)
    if ((draft?.schemaVersion === 3 || draft?.schemaVersion === 2) && draft.form) {
      if (draft.schemaVersion === 2) saveMigratedCreateDraft(draft.form)
      return draft.form
    }
    return null
  } catch (error) {
    ElMessage.warning('读取本地草稿失败')
    return null
  }
}

function saveCreateDraft() {
  if (!createDialogVisible.value) return
  syncCreateEditorCode()
  try {
    localStorage.setItem(createDraftKey, JSON.stringify({ schemaVersion: 3, savedAt: new Date().toISOString(), form: createForm.value }))
  } catch (error) {
    ElMessage.warning('保存本地草稿失败')
  }
}

function saveMigratedCreateDraft(form) {
  try {
    localStorage.setItem(createDraftKey, JSON.stringify({ schemaVersion: 3, savedAt: new Date().toISOString(), form }))
  } catch (error) {
    ElMessage.warning('迁移本地草稿失败')
  }
}

function scheduleCreateDraftSave() {
  if (createDraftTimer) clearTimeout(createDraftTimer)
  createDraftTimer = setTimeout(() => {
    createDraftTimer = null
    saveCreateDraft()
  }, 500)
}

async function updateCreatePreview() {
  if (!createDialogVisible.value) return
  syncCreateEditorCode()
  const payload = normalizeCreatePayload(createForm.value)
  if (!canRequestCreatePreview(payload)) {
    createPreviewData.value = null
    createPreview.value = '填写插件名称后生成后端预览。'
    createPreviewLoading.value = false
    return
  }
  const requestID = Symbol('create-preview')
  updateCreatePreview.latestRequestID = requestID
  createPreviewLoading.value = true
  try {
    const result = await previewCreatePlugin(payload)
    if (updateCreatePreview.latestRequestID !== requestID) return
    createPreviewData.value = result
    createPreview.value = formatCreatePreviewFallback(result)
  } catch (error) {
    if (updateCreatePreview.latestRequestID !== requestID) return
    const data = error?.response?.data
    if (data && (Array.isArray(data.errors) || data.normalized)) {
      createPreviewData.value = data
      createPreview.value = formatCreatePreviewFallback(data)
    } else {
      createPreviewData.value = null
      createPreview.value = '生成预览失败，请检查配置后重试。'
    }
  } finally {
    if (updateCreatePreview.latestRequestID === requestID) createPreviewLoading.value = false
  }
}

function scheduleCreatePreviewUpdate() {
  if (createPreviewTimer) clearTimeout(createPreviewTimer)
  createPreviewTimer = setTimeout(() => {
    createPreviewTimer = null
    updateCreatePreview()
  }, 200)
}

function clearCreateDraft() {
  try {
    localStorage.removeItem(createDraftKey)
    localStorage.removeItem(createDraftLegacyKey)
  } catch (error) {
    ElMessage.warning('清理本地草稿失败')
  }
}

function loadCreatePresets() {
  try {
    const raw = localStorage.getItem(createPresetKey) || localStorage.getItem(createPresetLegacyKey)
    const list = raw ? JSON.parse(raw) : []
    if (!Array.isArray(list)) return []
    if (!localStorage.getItem(createPresetKey) && localStorage.getItem(createPresetLegacyKey)) {
      localStorage.setItem(createPresetKey, JSON.stringify(list))
    }
    return list
  } catch (error) {
    return []
  }
}

function persistCreatePresets() {
  try {
    localStorage.setItem(createPresetKey, JSON.stringify(createPresets.value))
  } catch (error) {
    ElMessage.warning('保存本地模板失败')
  }
}

async function saveCreatePreset() {
  syncCreateEditorCode()
  const name = await ElMessageBox.prompt('请输入模板名称', '保存为模板', { inputValue: createForm.value.name || '青龙账号模板' }).then(({ value }) => value).catch(() => '')
  if (!name) return
  const preset = { name, savedAt: new Date().toISOString(), form: JSON.parse(JSON.stringify(createForm.value)) }
  const index = createPresets.value.findIndex(item => item.name === name)
  if (index >= 0) createPresets.value.splice(index, 1, preset)
  else createPresets.value.push(preset)
  selectedCreatePreset.value = name
  persistCreatePresets()
  ElMessage.success('模板已保存')
}

function loadCreatePreset() {
  const preset = createPresets.value.find(item => item.name === selectedCreatePreset.value)
  if (!preset) {
    ElMessage.warning('请选择要加载的模板')
    return
  }
  destroyCreateEditors()
  createForm.value = JSON.parse(JSON.stringify(preset.form))
  accountQLScriptRuntimeTouched = Boolean(createForm.value.account_ql?.script_runtime)
  normalizeCreateFormShape(createForm.value)
  if (isAccountQLTemplate.value) ensureAccountQLDefaults()
  updateCreatePreview()
  scheduleCreateDraftSave()
  ElMessage.success('模板已加载')
}

function deleteCreatePreset() {
  const index = createPresets.value.findIndex(item => item.name === selectedCreatePreset.value)
  if (index < 0) {
    ElMessage.warning('请选择要删除的模板')
    return
  }
  createPresets.value.splice(index, 1)
  selectedCreatePreset.value = ''
  persistCreatePresets()
  ElMessage.success('模板已删除')
}

function canRequestCreatePreview(payload) {
  return Boolean(payload?.name || payload?.id)
}

function formatCreatePreviewFallback(result) {
  if (!result) return ''
  const normalized = result.normalized || {}
  const lines = [
    `模板：${result.template || normalized.template || '-'}`,
    `runtime：${result.runtime || normalized.runtime || '-'}`,
    `入口：${result.entry || normalized.entry || '-'}`,
    `触发：${result.trigger || normalized.trigger || '-'}`
  ]
  if (Array.isArray(result.commands) && result.commands.length) lines.push(`指令：${result.commands.join(' / ')}`)
  if (Array.isArray(result.errors) && result.errors.length) lines.push('', '错误：', ...result.errors.map(issue => `- ${issue.message}`))
  if (Array.isArray(result.warnings) && result.warnings.length) lines.push('', '警告：', ...result.warnings.map(issue => `- ${issue.message}`))
  if (Array.isArray(result.files)) {
    for (const file of result.files) {
      lines.push('', `${file.path}：`, file.content || '')
    }
  }
  return lines.join('\n')
}

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

watch(() => createForm.value.template, async (template, previousTemplate) => {
  if (template === 'nodejs_account_ql' || template === 'python_account_ql') {
    const previousDefault = templateDefaultScriptRuntime(previousTemplate)
    if (!accountQLScriptRuntimeTouched || createForm.value.account_ql?.script_runtime === previousDefault) {
      accountQLScriptRuntimeTouched = false
      createForm.value.account_ql.script_runtime = templateDefaultScriptRuntime(template)
    }
    createForm.value.runtime = template === 'python_account_ql' ? 'python' : 'nodejs'
    destroyCreateEditors()
    ensureAccountQLDefaults()
    if (createActiveTab.value === 'user') createActiveTab.value = 'account'
    if (createActiveTab.value === 'code') {
      await nextTick()
      ensureCreateEditors()
    }
  } else {
    destroyCreateEditors()
    if (createActiveTab.value === 'account' || createActiveTab.value === 'code' || createActiveTab.value === 'routes') createActiveTab.value = 'base'
  }
})

watch(() => [createForm.value.id, createForm.value.name], () => {
  if (isAccountQLTemplate.value) ensureAccountQLDefaults()
})

watch(() => createForm.value.account_ql?.script_runtime, () => {
  if (isAccountQLTemplate.value) ensureAccountQLDefaults()
})

watch(createActiveTab, async (tab) => {
  if (tab === 'code' && isAccountQLTemplate.value) {
    await nextTick()
    ensureCreateEditors()
  } else {
    destroyCreateEditors()
  }
})

watch(() => createForm.value.account_ql?.enable_ck_check, async () => {
  if (createActiveTab.value === 'code') {
    await nextTick()
    ensureCreateEditors()
  }
})

watch(createForm, () => {
  scheduleCreatePreviewUpdate()
  scheduleCreateDraftSave()
}, { deep: true })

const handleDelete = async (plugin) => {
  await ElMessageBox.confirm(
    `确定要删除插件 "${plugin.name}" 吗？`,
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )

  try {
    await deletePlugin(plugin.id)
    ElMessage.success(`插件 ${plugin.name} 已删除`)
    await loadPlugins()
  } catch (error) {
    console.error('删除插件失败:', error)
    ElMessage.error('删除插件失败')
  }
}

onMounted(() => {
  createPresets.value = loadCreatePresets()
  loadPluginTemplates()
  loadPlugins()
  loadAdapters()
})

onBeforeUnmount(() => {
  if (createDraftTimer) clearTimeout(createDraftTimer)
  if (createPreviewTimer) clearTimeout(createPreviewTimer)
  destroyCreateEditors()
})
</script>

<style scoped>
.plugins,
.page-shell {
  width: 100%;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.page-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: hidden;
}

.plugins-header,
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.plugins-header .title {
  font-size: 18px;
  font-weight: bold;
}

.plugins-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}

.plugin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.plugin-card {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  background: #fff;
  transition: box-shadow 0.2s;
  min-height: 220px;
}

.plugin-card:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.plugin-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
}

.plugin-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.plugin-card-body {
  flex: 1;
  margin-bottom: 12px;
}

.plugin-info-row {
  display: flex;
  align-items: center;
  margin-bottom: 6px;
  font-size: 13px;
  color: #606266;
}

.plugin-info-row .label {
  color: #909399;
  min-width: 50px;
  flex-shrink: 0;
}

.trigger-text {
  background: #f5f7fa;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 12px;
  color: #606266;
  word-break: break-all;
}

.platforms .el-tag {
  margin-right: 4px;
}

.field-tip {
  width: 100%;
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
  color: #909399;
}

.create-config-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  color: #606266;
  font-size: 13px;
}

.create-dialog-body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 430px;
  gap: 16px;
  min-height: 640px;
}

.create-dialog-left {
  min-width: 0;
}

.create-preview-panel {
  min-width: 0;
  max-height: 680px;
  overflow: auto;
  padding: 12px;
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  background: #0f172a;
  color: #d1e7ff;
}

.create-preview-title {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #fff;
}

.create-preview-panel pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
}

.create-preview-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 10px;
  color: #dbeafe;
  font-size: 12px;
}

.preview-issues {
  margin-bottom: 10px;
  padding: 8px;
  border-radius: 4px;
  background: rgba(248, 113, 113, 0.16);
  color: #fecaca;
  font-size: 12px;
}

.preview-files :deep(.el-collapse-item__wrap),
.preview-files :deep(.el-collapse-item__header) {
  background: transparent;
  color: #d1e7ff;
}

.preview-files :deep(.el-collapse-item__content) {
  color: #d1e7ff;
}

.create-result-section {
  margin-top: 16px;
}

.create-result-title {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.create-result-section .el-tag {
  margin-right: 6px;
  margin-bottom: 6px;
}

.preset-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.create-config-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.create-config-row {
  display: grid;
  grid-template-columns: 1fr 1.4fr 1.4fr auto;
  gap: 8px;
  align-items: center;
}

.route-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.route-item {
  padding: 12px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  background: #fafafa;
}

.route-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1.2fr auto;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

.code-editor-section {
  margin-bottom: 16px;
}

.code-editor-title {
  margin-bottom: 4px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.create-code-editor {
  height: 220px;
  margin-top: 8px;
  overflow: auto;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.create-code-editor :deep(.cm-editor) {
  min-height: 220px;
}

.plugin-card-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 10px;
  border-top: 1px solid #f0f0f0;
}

.plugins-pagination {
  flex-shrink: 0;
  padding-top: 12px;
  display: flex;
  justify-content: center;
  border-top: 1px solid #ebeef5;
}

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }

  .page-card {
    height: 100%;
    min-height: 100%;
    border-radius: 10px;
  }

  .plugins-header {
    align-items: stretch;
    gap: 10px;
    flex-direction: column;
  }

  .plugins-header .title {
    font-size: 16px;
  }

  .plugin-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }

  .plugin-card {
    min-height: auto;
    padding: 14px;
  }

  .plugin-card-header {
    align-items: flex-start;
    gap: 8px;
  }

  .plugin-card-footer .el-button {
    margin-left: 0;
  }

  .create-config-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .create-dialog-body {
    grid-template-columns: minmax(0, 1fr);
  }

  .create-preview-panel {
    max-height: 360px;
  }

  .create-config-row,
  .route-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .plugins-content {
    -webkit-overflow-scrolling: touch;
  }

  .plugins-pagination {
    overflow-x: auto;
    justify-content: flex-start;
  }

  .plugins-content::-webkit-scrollbar,
  .plugins-pagination::-webkit-scrollbar {
    display: none;
  }
}
</style>
