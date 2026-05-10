#!/bin/bash
set -e

echo "========================================"
echo "AllBot 自动安装脚本"
echo "========================================"
echo ""

# 检测操作系统
OS="$(uname -s)"
case "${OS}" in
    Linux*)     MACHINE=Linux;;
    Darwin*)    MACHINE=Mac;;
    *)          MACHINE="UNKNOWN:${OS}"
esac

echo "检测到操作系统: ${MACHINE}"
echo ""

# 1. 检查并安装 Python
echo "[1/5] 检查 Python..."
if ! command -v python3 &> /dev/null; then
    echo "Python 未安装，正在安装..."
    if [ "${MACHINE}" = "Mac" ]; then
        if ! command -v brew &> /dev/null; then
            echo "正在安装 Homebrew..."
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        fi
        brew install python@3.11
    elif [ "${MACHINE}" = "Linux" ]; then
        sudo apt-get update
        sudo apt-get install -y python3.11 python3-pip python3-venv
    fi
    echo "Python 安装完成"
else
    echo "Python 已安装: $(python3 --version)"
fi

# 2. 检查并安装 Node.js
echo ""
echo "[2/5] 检查 Node.js..."
if ! command -v node &> /dev/null; then
    echo "Node.js 未安装，正在安装..."
    if [ "${MACHINE}" = "Mac" ]; then
        brew install node@20
    elif [ "${MACHINE}" = "Linux" ]; then
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt-get install -y nodejs
    fi
    echo "Node.js 安装完成"
else
    echo "Node.js 已安装: $(node --version)"
fi

# 3. 创建运行时目录
echo ""
echo "[3/5] 创建运行时环境..."
mkdir -p runtime
mkdir -p plugins

# 4. 初始化 Python 虚拟环境
echo ""
echo "[4/5] 初始化 Python 环境..."
if [ ! -d "runtime/.venv" ]; then
    python3 -m venv runtime/.venv
    echo "Python 虚拟环境创建成功"
fi

# 安装基础依赖
echo "正在安装 Python 基础依赖..."
runtime/.venv/bin/pip install grpcio grpcio-tools protobuf

# 5. 初始化 Node.js 环境
echo ""
echo "[5/5] 初始化 Node.js 环境..."
if [ ! -f "runtime/package.json" ]; then
    echo '{"name":"allbot-runtime","version":"1.0.0","dependencies":{}}' > runtime/package.json
fi

echo "正在安装 Node.js 基础依赖..."
cd runtime
npm install @grpc/grpc-js @grpc/proto-loader
cd ..

# 创建配置文件
if [ ! -f "config.yml" ]; then
    echo "正在创建配置文件..."
    cat > config.yml << 'EOF'
# AllBot 配置文件

# 管理员账号
admin:
  username: admin
  password: admin123  # 首次启动后请修改

# Web UI 配置
web:
  port: 3000
  host: 0.0.0.0

# QQ 平台配置
qq:
  api_url: http://localhost:5700
  enabled: false

# 插件目录
plugins:
  dir: ./plugins
EOF
fi

# 设置执行权限
if [ -f "allbot" ]; then
    chmod +x allbot
fi

echo ""
echo "========================================"
echo "安装完成！"
echo "========================================"
echo ""
echo "启动命令：./allbot"
echo "Web UI：http://localhost:3000"
echo "默认账号：admin / admin123"
echo ""
echo "注意：首次启动后请修改管理员密码"
echo "========================================"
