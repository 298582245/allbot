# AllBot Market Server

开源的插件市场服务器模板，开发者可以一键部署自己的插件市场。

## 特性

- 用户认证和授权（JWT）
- 插件上传/下载管理
- 订单和支付系统
- 授权证书生成和验证
- RESTful API
- Docker 一键部署

## 快速开始

### 使用 Docker Compose（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/allbot/market-server
cd market-server

# 2. 配置环境变量（可选）
cp .env.example .env
# 编辑 .env 文件，设置 SECRET_KEY

# 3. 启动服务
docker-compose up -d

# 4. 访问 API
curl http://localhost:8000/health
```

### 手动安装

```bash
# 1. 安装依赖
pip install -r requirements.txt

# 2. 初始化数据库
python -c "from app.core.database import init_db; init_db()"

# 3. 启动服务
uvicorn app.main:app --host 0.0.0.0 --port 8000
```

## API 文档

启动服务后访问：http://localhost:8000/docs

### 主要端点

#### 认证
- `POST /token` - 登录获取 Token

#### 插件管理
- `GET /api/plugins` - 获取插件列表
- `GET /api/plugins/{id}` - 获取插件详情
- `POST /api/plugins` - 上传插件（需要认证）
- `GET /api/plugins/{id}/download` - 下载插件（需要认证）
- `POST /api/plugins/{id}/purchase` - 购买插件（需要认证）

#### 授权验证
- `POST /api/plugins/verify` - 验证授权证书

## 数据库 Schema

### 用户表（users）
- id, username, email, hashed_password
- role (admin/developer/user)
- is_active, created_at, updated_at

### 插件表（plugins）
- id, name, slug, description, version
- author_id, runtime, trigger, platforms
- price_type, price, monthly_price, yearly_price
- status, downloads, rating
- file_path, file_size
- created_at, updated_at

### 订单表（orders）
- id, order_no, user_id, plugin_id
- amount, license_type
- payment_method, payment_status, paid_at
- created_at, updated_at

### 授权表（licenses）
- id, license_key, plugin_id, user_id, device_id
- license_type, expires_at, is_active
- signature, created_at, last_verified_at

## 配置

### 环境变量

```bash
DATABASE_URL=postgresql://user:pass@localhost:5432/market
SECRET_KEY=your-secret-key-here
UPLOAD_DIR=./uploads
```

### 数据库

支持 PostgreSQL 和 SQLite：
- PostgreSQL（生产环境推荐）：`postgresql://user:pass@host:5432/dbname`
- SQLite（开发环境）：`sqlite:///./market.db`

## 支付集成

### 支付宝

```python
# TODO: 添加支付宝 SDK 配置
```

### 微信支付

```python
# TODO: 添加微信支付 SDK 配置
```

### Stripe

```python
# TODO: 添加 Stripe SDK 配置
```

## 部署

### Docker 部署

```bash
docker-compose up -d
```

### 生产环境建议

1. 使用 PostgreSQL 数据库
2. 设置强密码的 SECRET_KEY
3. 配置 HTTPS（使用 Nginx + Let's Encrypt）
4. 配置文件存储（MinIO / S3）
5. 配置 Redis 缓存（可选）
6. 配置日志收集
7. 配置监控和告警

## 开发

### 项目结构

```
market-server/
├── app/
│   ├── api/           # API 路由
│   ├── models/        # 数据库模型
│   ├── schemas/       # Pydantic schemas
│   ├── services/      # 业务逻辑
│   ├── core/          # 核心配置
│   └── main.py        # 应用入口
├── tests/             # 测试
├── scripts/           # 脚本
├── Dockerfile
├── docker-compose.yml
└── requirements.txt
```

### 运行测试

```bash
pytest tests/
```

## 许可证

MIT License
