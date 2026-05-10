@echo off
chcp 65001 >nul
echo ========================================
echo AllBot 自动安装脚本
echo ========================================
echo.

REM 检查管理员权限
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [错误] 请以管理员身份运行此脚本
    pause
    exit /b 1
)

REM 1. 检查并安装 Python
echo [1/5] 检查 Python...
python --version >nul 2>&1
if %errorlevel% neq 0 (
    echo Python 未安装，正在下载...
    powershell -Command "Invoke-WebRequest -Uri 'https://www.python.org/ftp/python/3.11.0/python-3.11.0-amd64.exe' -OutFile 'python-installer.exe'"
    echo 正在安装 Python...
    python-installer.exe /quiet InstallAllUsers=1 PrependPath=1
    del python-installer.exe
    echo Python 安装完成
) else (
    echo Python 已安装
)

REM 2. 检查并安装 Node.js
echo.
echo [2/5] 检查 Node.js...
node --version >nul 2>&1
if %errorlevel% neq 0 (
    echo Node.js 未安装，正在下载...
    powershell -Command "Invoke-WebRequest -Uri 'https://nodejs.org/dist/v20.11.0/node-v20.11.0-x64.msi' -OutFile 'node-installer.msi'"
    echo 正在安装 Node.js...
    msiexec /i node-installer.msi /quiet
    del node-installer.msi
    echo Node.js 安装完成
) else (
    echo Node.js 已安装
)

REM 3. 创建运行时目录
echo.
echo [3/5] 创建运行时环境...
if not exist "runtime" mkdir runtime
if not exist "plugins" mkdir plugins

REM 4. 初始化 Python 虚拟环境
echo.
echo [4/5] 初始化 Python 环境...
if not exist "runtime\.venv" (
    python -m venv runtime\.venv
    echo Python 虚拟环境创建成功
)

REM 安装基础依赖
echo 正在安装 Python 基础依赖...
runtime\.venv\Scripts\pip.exe install grpcio grpcio-tools protobuf

REM 5. 初始化 Node.js 环境
echo.
echo [5/5] 初始化 Node.js 环境...
if not exist "runtime\package.json" (
    echo {"name":"allbot-runtime","version":"1.0.0","dependencies":{}} > runtime\package.json
)

echo 正在安装 Node.js 基础依赖...
cd runtime
call npm install @grpc/grpc-js @grpc/proto-loader
cd ..

REM 创建配置文件
if not exist "config.yml" (
    echo 正在创建配置文件...
    (
        echo # AllBot 配置文件
        echo.
        echo # 管理员账号
        echo admin:
        echo   username: admin
        echo   password: admin123  # 首次启动后请修改
        echo.
        echo # Web UI 配置
        echo web:
        echo   port: 3000
        echo   host: 0.0.0.0
        echo.
        echo # QQ 平台配置
        echo qq:
        echo   api_url: http://localhost:5700
        echo   enabled: false
        echo.
        echo # 插件目录
        echo plugins:
        echo   dir: ./plugins
    ) > config.yml
)

echo.
echo ========================================
echo 安装完成！
echo ========================================
echo.
echo 启动命令：allbot.exe
echo Web UI：http://localhost:3000
echo 默认账号：admin / admin123
echo.
echo 注意：首次启动后请修改管理员密码
echo ========================================
pause
