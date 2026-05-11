<template>
  <div class="orders">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>订单管理</span>
          <el-input
            v-model="searchText"
            placeholder="搜索订单号"
            style="width: 300px"
            clearable
            @input="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>
      </template>

      <el-table :data="filteredOrders" v-loading="loading" style="width: 100%">
        <el-table-column prop="order_no" label="订单号" width="220" />
        <el-table-column prop="plugin_id" label="插件ID" width="100" />
        <el-table-column prop="amount" label="金额" width="120">
          <template #default="{ row }">
            ¥{{ (row.amount / 100).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column prop="license_type" label="授权类型" width="120">
          <template #default="{ row }">
            <el-tag size="small">
              {{ row.license_type === 'one_time' ? '一次性' : '订阅' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="payment_method" label="支付方式" width="120">
          <template #default="{ row }">
            {{ getPaymentMethodText(row.payment_method) }}
          </template>
        </el-table-column>
        <el-table-column prop="payment_status" label="支付状态" width="120">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.payment_status)" size="small">
              {{ getStatusText(row.payment_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="handleView(row)">
              查看
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && orders.length === 0" description="暂无订单" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { getOrders } from '@/api'

const loading = ref(false)
const orders = ref([])
const searchText = ref('')

const filteredOrders = computed(() => {
  if (!searchText.value) return orders.value
  return orders.value.filter(order =>
    order.order_no.toLowerCase().includes(searchText.value.toLowerCase())
  )
})

const loadOrders = async () => {
  loading.value = true
  try {
    orders.value = await getOrders()
  } catch (error) {
    console.error('加载订单失败:', error)
  } finally {
    loading.value = false
  }
}

const getPaymentMethodText = (method) => {
  const textMap = {
    alipay: '支付宝',
    wechat: '微信支付',
    stripe: 'Stripe'
  }
  return textMap[method] || method
}

const getStatusType = (status) => {
  const typeMap = {
    pending: 'warning',
    paid: 'success',
    failed: 'danger',
    refunded: 'info'
  }
  return typeMap[status] || 'info'
}

const getStatusText = (status) => {
  const textMap = {
    pending: '待支付',
    paid: '已支付',
    failed: '支付失败',
    refunded: '已退款'
  }
  return textMap[status] || status
}

const handleSearch = () => {
  // 搜索逻辑已通过computed实现
}

const handleView = (order) => {
  ElMessage.info(`查看订单: ${order.order_no}`)
}

onMounted(() => {
  loadOrders()
})
</script>

<style scoped>
.orders {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
