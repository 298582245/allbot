# AllBot 开发者后台

基于 Vue 3 + Element Plus 的现代化开发者管理后台。

## 功能特性

- 📊 **仪表盘** - 实时统计数据和图表展示
- 🔌 **插件管理** - 创建、编辑、删除插件
- 💰 **订单管理** - 查看和管理订单
- 📈 **数据分析** - 收益、下载量等多维度分析
- ⚙️ **系统设置** - 基本信息、密码修改、支付配置

## 技术栈

- **Vue 3** - 渐进式JavaScript框架
- **Element Plus** - 企业级UI组件库
- **Vue Router** - 官方路由管理器
- **Pinia** - 轻量级状态管理
- **Axios** - HTTP客户端
- **ECharts** - 数据可视化图表库
- **Vite** - 下一代前端构建工具

## 快速开始

### 安装依赖

```bash
cd admin-ui
npm install
```

### 开发模式

```bash
npm run dev
```

访问 http://localhost:5173

### 构建生产版本

```bash
npm run build
```

构建产物输出到 `../static/admin` 目录。

## 项目结构

```
admin-ui/
├── src/
│   ├── api/              # API接口
│   ├── assets/           # 静态资源
│   ├── components/       # 公共组件
│   ├── layouts/          # 布局组件
│   ├── router/           # 路由配置
│   ├── stores/           # 状态管理
│   ├── utils/            # 工具函数
│   ├── views/            # 页面组件
│   ├── App.vue           # 根组件
│   └── main.js           # 入口文件
├── index.html            # HTML模板
├── package.json          # 项目配置
└── vite.config.js        # Vite配置
```

## 页面说明

### 登录页面 (Login.vue)
- JWT Token认证
- 表单验证
- 自动跳转

### 仪表盘 (Dashboard.vue)
- 统计卡片（插件数、下载量、收益、订单数）
- 收益趋势图表
- 下载趋势图表
- 最近订单列表

### 插件管理 (Plugins.vue)
- 插件列表展示
- 创建/编辑/删除插件
- 状态标签显示
- 价格格式化

### 插件表单 (PluginForm.vue)
- 完整的插件信息表单
- 运行时选择（Python/Node.js）
- 平台多选（QQ/微信/Telegram）
- 定价类型（一次性/订阅）
- 文件上传

### 订单管理 (Orders.vue)
- 订单列表
- 搜索功能
- 支付状态显示
- 支付方式显示

### 数据分析 (Analytics.vue)
- 收益分析图表
- 下载量分析图表
- 插件销售排行
- 支付方式分布
- 时间范围切换

### 系统设置 (Settings.vue)
- 基本信息管理
- 密码修改
- 支付配置（支付宝/微信/Stripe）

## API集成

后台通过Axios与市场服务器API通信：

```javascript
// 请求拦截器 - 自动添加Token
request.interceptors.request.use(config => {
  const authStore = useAuthStore()
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }
  return config
})

// 响应拦截器 - 统一错误处理
request.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response?.status === 401) {
      // Token过期，跳转登录
      authStore.logout()
      router.push('/login')
    }
    return Promise.reject(error)
  }
)
```

## 路由守卫

```javascript
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login')
  } else if (to.path === '/login' && authStore.isAuthenticated) {
    next('/')
  } else {
    next()
  }
})
```

## 状态管理

使用Pinia进行状态管理：

```javascript
// auth store
export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))
  const isAuthenticated = ref(!!token.value)

  const login = async (username, password) => {
    // 登录逻辑
  }

  const logout = () => {
    // 登出逻辑
  }

  return { token, user, isAuthenticated, login, logout }
})
```

## 环境变量

创建 `.env` 文件配置环境变量：

```env
VITE_API_BASE_URL=http://localhost:8000
```

## 部署

### 开发环境

```bash
npm run dev
```

### 生产环境

1. 构建项目：
```bash
npm run build
```

2. 构建产物会输出到 `../static/admin` 目录

3. 配置市场服务器提供静态文件服务

## 浏览器支持

- Chrome >= 87
- Firefox >= 78
- Safari >= 14
- Edge >= 88

## 开发规范

- 使用 Composition API
- 使用 `<script setup>` 语法
- 组件命名使用 PascalCase
- 文件命名使用 PascalCase
- 使用 ESLint 进行代码检查

## 常见问题

### Q: 登录后Token过期怎么办？
A: 系统会自动检测401状态码，清除Token并跳转到登录页面。

### Q: 如何修改API地址？
A: 修改 `vite.config.js` 中的 `proxy` 配置或 `.env` 文件中的 `VITE_API_BASE_URL`。

### Q: 图表不显示？
A: 确保已正确安装 `echarts` 和 `vue-echarts` 依赖。

## 许可证

MIT License
