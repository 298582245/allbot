#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SERVICE=allbot
PORT=${ALLBOT_WEB_PORT:-3000}

cd "$SCRIPT_DIR"

git pull

echo "[1/5] 构建当前源码镜像（不使用缓存）"
docker compose build --no-cache "$SERVICE"

echo "[2/5] 停止旧容器"
docker compose stop "$SERVICE" 2>/dev/null || true

echo "[3/5] 清理持久化目录中的旧程序和 SDK"
docker compose run --rm --no-deps --entrypoint sh "$SERVICE" -c \
  'data_dir=${ALLBOT_DATA_DIR:-/data}; rm -f "$data_dir/allbot" "$data_dir/.allbot-image-sha256" "$data_dir/.allbot-sdk-image-sha256"; rm -rf "$data_dir/sdk" "$data_dir/.allbot-sdk-base"'

echo "[4/5] 使用当前镜像启动端口 ${PORT}"
ALLBOT_WEB_PORT="$PORT" docker compose up -d --force-recreate "$SERVICE"

echo "[5/5] 检查容器内已挂载的自定义适配器"
docker compose exec -T "$SERVICE" sh -c \
  'data_dir=${ALLBOT_DATA_DIR:-/data}; echo "适配器目录: $data_dir/adapters"; find "$data_dir/adapters" -mindepth 2 -maxdepth 2 -type f -name adapter.json -print'

docker compose ps "$SERVICE"
