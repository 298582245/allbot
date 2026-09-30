#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SERVICE=allbot
PORT=${ALLBOT_WEB_PORT:-3000}

cd "$SCRIPT_DIR"

echo "[1/4] 构建当前源码镜像（不使用缓存）"
docker compose build --no-cache "$SERVICE"

echo "[2/4] 停止旧容器"
docker compose stop "$SERVICE" 2>/dev/null || true

echo "[3/4] 清理持久化目录中的旧程序"
docker compose run --rm --no-deps --entrypoint sh "$SERVICE" -c \
  'rm -f /data/allbot /data/.allbot-image-sha256'

echo "[4/4] 使用当前镜像启动端口 ${PORT}"
ALLBOT_WEB_PORT="$PORT" docker compose up -d --force-recreate "$SERVICE"

docker compose ps "$SERVICE"
