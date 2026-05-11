<template>
  <div class="plugin-form">
    <el-card>
      <template #header>
        <span>{{ isEdit ? '编辑插件' : '创建插件' }}</span>
      </template>

      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px" style="max-width: 800px">
        <el-form-item label="插件名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入插件名称" />
        </el-form-item>

        <el-form-item label="插件标识" prop="slug">
          <el-input v-model="form.slug" placeholder="plugin-name" />
          <span class="form-tip">用于URL，只能包含小写字母、数字和连字符</span>
        </el-form-item>

        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            placeholder="请输入插件描述"
          />
        </el-form-item>

        <el-form-item label="版本" prop="version">
          <el-input v-model="form.version" placeholder="1.0.0" />
        </el-form-item>

        <el-form-item label="运行时" prop="runtime">
          <el-select v-model="form.runtime" placeholder="请选择运行时">
            <el-option label="Python" value="python" />
            <el-option label="Node.js" value="nodejs" />
          </el-select>
        </el-form-item>

        <el-form-item label="触发规则" prop="trigger">
          <el-input v-model="form.trigger" placeholder="正则表达式，如：天气.*" />
        </el-form-item>

        <el-form-item label="支持平台" prop="platforms">
          <el-checkbox-group v-model="form.platforms">
            <el-checkbox label="qq">QQ</el-checkbox>
            <el-checkbox label="wechat">微信</el-checkbox>
            <el-checkbox label="telegram">Telegram</el-checkbox>
          </el-checkbox-group>
        </el-form-item>

        <el-form-item label="定价类型" prop="price_type">
          <el-radio-group v-model="form.price_type">
            <el-radio label="one_time">一次性购买</el-radio>
            <el-radio label="subscription">订阅</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="价格" prop="price" v-if="form.price_type === 'one_time'">
          <el-input-number v-model="form.price" :min="0" :step="100" />
          <span class="form-tip">单位：分（100分 = 1元）</span>
        </el-form-item>

        <el-form-item label="月付价格" prop="monthly_price" v-if="form.price_type === 'subscription'">
          <el-input-number v-model="form.monthly_price" :min="0" :step="100" />
          <span class="form-tip">单位：分</span>
        </el-form-item>

        <el-form-item label="年付价格" prop="yearly_price" v-if="form.price_type === 'subscription'">
          <el-input-number v-model="form.yearly_price" :min="0" :step="100" />
          <span class="form-tip">单位：分</span>
        </el-form-item>

        <el-form-item label="插件文件" v-if="!isEdit">
          <el-upload
            :auto-upload="false"
            :on-change="handleFileChange"
            :limit="1"
            accept=".allbot,.zip"
          >
            <el-button type="primary">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">
                支持 .allbot 或 .zip 格式，文件大小不超过 50MB
              </div>
            </template>
          </el-upload>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            {{ isEdit ? '保存' : '创建' }}
          </el-button>
          <el-button @click="handleCancel">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getPlugin, createPlugin, updatePlugin } from '@/api'

const route = useRoute()
const router = useRouter()

const formRef = ref(null)
const submitting = ref(false)
const uploadFile = ref(null)

const isEdit = computed(() => !!route.params.id)

const form = ref({
  name: '',
  slug: '',
  description: '',
  version: '1.0.0',
  runtime: 'python',
  trigger: '',
  platforms: [],
  price_type: 'one_time',
  price: 0,
  monthly_price: 0,
  yearly_price: 0
})

const rules = {
  name: [{ required: true, message: '请输入插件名称', trigger: 'blur' }],
  slug: [
    { required: true, message: '请输入插件标识', trigger: 'blur' },
    { pattern: /^[a-z0-9-]+$/, message: '只能包含小写字母、数字和连字符', trigger: 'blur' }
  ],
  version: [{ required: true, message: '请输入版本号', trigger: 'blur' }],
  runtime: [{ required: true, message: '请选择运行时', trigger: 'change' }],
  trigger: [{ required: true, message: '请输入触发规则', trigger: 'blur' }],
  platforms: [{ required: true, message: '请选择至少一个平台', trigger: 'change' }]
}

const handleFileChange = (file) => {
  uploadFile.value = file.raw
}

const handleSubmit = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      if (isEdit.value) {
        await updatePlugin(route.params.id, form.value)
        ElMessage.success('插件更新成功')
      } else {
        await createPlugin(form.value)
        ElMessage.success('插件创建成功')
      }
      router.push('/plugins')
    } catch (error) {
      console.error('提交失败:', error)
      ElMessage.error('操作失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleCancel = () => {
  router.back()
}

onMounted(async () => {
  if (isEdit.value) {
    try {
      const plugin = await getPlugin(route.params.id)
      form.value = {
        ...plugin,
        platforms: JSON.parse(plugin.platforms || '[]')
      }
    } catch (error) {
      console.error('加载插件失败:', error)
      ElMessage.error('加载插件失败')
    }
  }
})
</script>

<style scoped>
.plugin-form {
  width: 100%;
}

.form-tip {
  margin-left: 10px;
  font-size: 12px;
  color: #909399;
}
</style>
