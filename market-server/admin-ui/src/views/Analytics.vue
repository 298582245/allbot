<template>
  <div class="analytics">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>数据分析</span>
          <el-radio-group v-model="timeRange" @change="handleTimeRangeChange">
            <el-radio-button label="week">最近7天</el-radio-button>
            <el-radio-button label="month">最近30天</el-radio-button>
            <el-radio-button label="year">最近一年</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <el-row :gutter="20">
        <el-col :span="12">
          <div class="chart-container">
            <h4>收益分析</h4>
            <v-chart :option="revenueChartOption" style="height: 350px" />
          </div>
        </el-col>

        <el-col :span="12">
          <div class="chart-container">
            <h4>下载量分析</h4>
            <v-chart :option="downloadChartOption" style="height: 350px" />
          </div>
        </el-col>
      </el-row>

      <el-row :gutter="20" style="margin-top: 20px">
        <el-col :span="12">
          <div class="chart-container">
            <h4>插件销售排行</h4>
            <v-chart :option="pluginRankingOption" style="height: 350px" />
          </div>
        </el-col>

        <el-col :span="12">
          <div class="chart-container">
            <h4>支付方式分布</h4>
            <v-chart :option="paymentMethodOption" style="height: 350px" />
          </div>
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent
} from 'echarts/components'
import VChart from 'vue-echarts'

use([
  CanvasRenderer,
  LineChart,
  BarChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent
])

const timeRange = ref('month')

const revenueChartOption = ref({
  tooltip: {
    trigger: 'axis'
  },
  legend: {
    data: ['收益', '订单数']
  },
  xAxis: {
    type: 'category',
    data: ['1月', '2月', '3月', '4月', '5月', '6月']
  },
  yAxis: [
    {
      type: 'value',
      name: '收益（元）'
    },
    {
      type: 'value',
      name: '订单数'
    }
  ],
  series: [
    {
      name: '收益',
      type: 'line',
      data: [1200, 1800, 2400, 3200, 4100, 5000],
      smooth: true,
      itemStyle: {
        color: '#409eff'
      }
    },
    {
      name: '订单数',
      type: 'bar',
      yAxisIndex: 1,
      data: [12, 18, 24, 32, 41, 50],
      itemStyle: {
        color: '#67c23a'
      }
    }
  ]
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
    type: 'value',
    name: '下载量'
  },
  series: [{
    data: [120, 180, 240, 320, 410, 500],
    type: 'line',
    smooth: true,
    areaStyle: {
      color: 'rgba(103, 194, 58, 0.2)'
    },
    itemStyle: {
      color: '#67c23a'
    }
  }]
})

const pluginRankingOption = ref({
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      type: 'shadow'
    }
  },
  xAxis: {
    type: 'value'
  },
  yAxis: {
    type: 'category',
    data: ['翻译插件', '天气插件', 'ChatGPT插件', '图片处理', '音乐播放']
  },
  series: [{
    type: 'bar',
    data: [320, 280, 250, 180, 120],
    itemStyle: {
      color: '#e6a23c'
    }
  }]
})

const paymentMethodOption = ref({
  tooltip: {
    trigger: 'item'
  },
  legend: {
    orient: 'vertical',
    left: 'left'
  },
  series: [{
    type: 'pie',
    radius: '50%',
    data: [
      { value: 450, name: '支付宝' },
      { value: 320, name: '微信支付' },
      { value: 180, name: 'Stripe' }
    ],
    emphasis: {
      itemStyle: {
        shadowBlur: 10,
        shadowOffsetX: 0,
        shadowColor: 'rgba(0, 0, 0, 0.5)'
      }
    }
  }]
})

const handleTimeRangeChange = () => {
  // 实际应用中根据时间范围重新加载数据
  console.log('时间范围变更:', timeRange.value)
}
</script>

<style scoped>
.analytics {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chart-container {
  background: white;
  padding: 20px;
  border-radius: 4px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.chart-container h4 {
  margin: 0 0 15px 0;
  color: #303133;
}
</style>
