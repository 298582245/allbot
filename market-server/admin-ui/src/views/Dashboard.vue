<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background-color: #409eff">
              <el-icon :size="30"><Box /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ stats.totalPlugins }}</div>
              <div class="stat-label">插件总数</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background-color: #67c23a">
              <el-icon :size="30"><Download /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ stats.totalDownloads }}</div>
              <div class="stat-label">总下载量</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background-color: #e6a23c">
              <el-icon :size="30"><Money /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">¥{{ stats.totalRevenue }}</div>
              <div class="stat-label">总收益</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background-color: #f56c6c">
              <el-icon :size="30"><ShoppingCart /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ stats.totalOrders }}</div>
              <div class="stat-label">订单总数</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>收益趋势</span>
          </template>
          <v-chart :option="revenueChartOption" style="height: 300px" />
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card>
          <template #header>
            <span>下载趋势</span>
          </template>
          <v-chart :option="downloadChartOption" style="height: 300px" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="24">
        <el-card>
          <template #header>
            <span>最近订单</span>
          </template>
          <el-table :data="recentOrders" style="width: 100%">
            <el-table-column prop="order_no" label="订单号" width="200" />
            <el-table-column prop="plugin_name" label="插件名称" />
            <el-table-column prop="amount" label="金额" width="120">
              <template #default="{ row }">
                ¥{{ (row.amount / 100).toFixed(2) }}
              </template>
            </el-table-column>
            <el-table-column prop="payment_status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.payment_status === 'paid' ? 'success' : 'warning'" size="small">
                  {{ row.payment_status === 'paid' ? '已支付' : '待支付' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" width="180" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import { Box, Download, Money, ShoppingCart } from '@element-plus/icons-vue'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

const stats = ref({
  totalPlugins: 0,
  totalDownloads: 0,
  totalRevenue: 0,
  totalOrders: 0
})

const recentOrders = ref([])

const revenueChartOption = ref({
  tooltip: {
    trigger: 'axis'
  },
  xAxis: {
    type: 'category',
    data: ['1月', '2月', '3月', '4月', '5月', '6月']
  },
  yAxis: {
    type: 'value'
  },
  series: [{
    data: [1200, 1800, 2400, 3200, 4100, 5000],
    type: 'line',
    smooth: true,
    itemStyle: {
      color: '#409eff'
    }
  }]
})

const downloadChartOption = ref({
  tooltip: {
    trigger: 'axis'
  },
  xAxis: {
    type: 'category',
    data: ['1月', '2月', '3月', '4月', '5月', '6月']
  },
  yAxis: {
    type: 'value'
  },
  series: [{
    data: [120, 180, 240, 320, 410, 500],
    type: 'line',
    smooth: true,
    itemStyle: {
      color: '#67c23a'
    }
  }]
})

onMounted(async () => {
  // 加载统计数据
  // 实际应用中从API获取
  stats.value = {
    totalPlugins: 12,
    totalDownloads: 1850,
    totalRevenue: 15000,
    totalOrders: 156
  }

  // 加载最近订单
  recentOrders.value = [
    {
      order_no: 'ORDER20260511001',
      plugin_name: '天气插件',
      amount: 9900,
      payment_status: 'paid',
      created_at: '2026-05-11 10:30:00'
    },
    {
      order_no: 'ORDER20260511002',
      plugin_name: '翻译插件',
      amount: 4900,
      payment_status: 'paid',
      created_at: '2026-05-11 11:15:00'
    }
  ]
})
</script>

<style scoped>
.dashboard {
  width: 100%;
}

.stat-card {
  cursor: pointer;
  transition: all 0.3s;
}

.stat-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 20px;
}

.stat-icon {
  width: 60px;
  height: 60px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 5px;
}
</style>
