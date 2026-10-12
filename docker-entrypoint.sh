#!/bin/sh
set -e

DATA_DIR="${ALLBOT_DATA_DIR:-/data}"
IMAGE_DIR="${ALLBOT_IMAGE_DIR:-/opt/allbot}"
APP_BIN="${DATA_DIR}/allbot"
APP_IMAGE_HASH="${DATA_DIR}/.allbot-image-sha256"
DATA_SDK_DIR="${DATA_DIR}/sdk"
APP_SDK_HASH="${DATA_DIR}/.allbot-sdk-image-sha256"
SDK_BASE_DIR="${DATA_DIR}/.allbot-sdk-base"
IMAGE_BIN="${IMAGE_DIR}/allbot"
IMAGE_HASH="${IMAGE_DIR}/allbot.sha256"
IMAGE_SDK_DIR="${IMAGE_DIR}/sdk"
IMAGE_SDK_HASH="${IMAGE_DIR}/sdk.sha256"
IMAGE_OPENAPIS_DIR="${IMAGE_DIR}/openapis"
UPGRADE_REQUEST="${DATA_DIR}/runtime/update/upgrade.json"
RESTART_REQUEST="${ALLBOT_DOCKER_RESTART_REQUEST:-${DATA_DIR}/runtime/restart/restart.json}"

mkdir -p "${DATA_DIR}/plugins" "${DATA_DIR}/runtime" "${DATA_DIR}/openapis" "${DATA_DIR}/logs" "${DATA_DIR}/backups"

init_app_bin() {
    image_hash=""
    old_image_hash=""
    current_hash=""
    if [ -f "${IMAGE_HASH}" ]; then
        image_hash=$(cat "${IMAGE_HASH}")
    fi
    if [ -f "${APP_IMAGE_HASH}" ]; then
        old_image_hash=$(cat "${APP_IMAGE_HASH}")
    fi
    if [ -f "${APP_BIN}" ]; then
        current_hash=$(sha256sum "${APP_BIN}" | cut -d ' ' -f 1)
    fi

    if [ ! -f "${APP_BIN}" ] || { [ -n "${old_image_hash}" ] && [ "${current_hash}" = "${old_image_hash}" ] && [ "${image_hash}" != "${old_image_hash}" ]; }; then
        cp "${IMAGE_BIN}" "${APP_BIN}"
        chmod 0755 "${APP_BIN}"
        if [ -n "${image_hash}" ]; then
            printf '%s\n' "${image_hash}" > "${APP_IMAGE_HASH}"
        fi
    fi
}

sdk_hash() {
    sdk_dir="$1"
    if [ ! -d "${sdk_dir}" ]; then
        return 1
    fi
    (cd "${sdk_dir}" && find . -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -d ' ' -f 1)
}

sdk_image_files() {
    (cd "${IMAGE_SDK_DIR}" && find . -type f -print)
}

copy_image_sdk_base() {
    rm -rf "${SDK_BASE_DIR}"
    mkdir -p "${SDK_BASE_DIR}"
    cp -a "${IMAGE_SDK_DIR}/." "${SDK_BASE_DIR}/"
}

copy_current_sdk_base() {
    rm -rf "${SDK_BASE_DIR}"
    mkdir -p "${SDK_BASE_DIR}"
    cp -a "${DATA_SDK_DIR}/." "${SDK_BASE_DIR}/"
}

copy_image_sdk() {
    rm -rf "${DATA_SDK_DIR}"
    mkdir -p "${DATA_SDK_DIR}"
    cp -a "${IMAGE_SDK_DIR}/." "${DATA_SDK_DIR}/"
    copy_image_sdk_base
    if [ -f "${IMAGE_SDK_HASH}" ]; then
        cat "${IMAGE_SDK_HASH}" > "${APP_SDK_HASH}"
    fi
}

bootstrap_untrusted_sdk() {
    echo "检测到已有 /data/sdk 但缺少可用的官方基线，保留当前 SDK，并将镜像版本写入 .update 文件。" >&2
    while IFS= read -r relative_path; do
        image_file="${IMAGE_SDK_DIR}/${relative_path#./}"
        current_file="${DATA_SDK_DIR}/${relative_path#./}"
        if [ ! -f "${current_file}" ]; then
            mkdir -p "$(dirname "${current_file}")"
            cp "${image_file}" "${current_file}"
        elif ! cmp -s "${current_file}" "${image_file}"; then
            cp "${image_file}" "${current_file}.update"
        fi
    done <<EOF
$(sdk_image_files)
EOF
    copy_image_sdk_base
}

merge_sdk_file() {
    relative_path="$1"
    image_file="${IMAGE_SDK_DIR}/${relative_path#./}"
    base_file="${SDK_BASE_DIR}/${relative_path#./}"
    current_file="${DATA_SDK_DIR}/${relative_path#./}"
    merge_file="${current_file}.allbot-merge.$$"

    mkdir -p "$(dirname "${current_file}")"
    if [ ! -f "${current_file}" ]; then
        cp "${image_file}" "${current_file}"
        return 0
    fi
    if [ ! -f "${base_file}" ]; then
        if ! cmp -s "${current_file}" "${image_file}"; then
            cp "${image_file}" "${current_file}.update"
        fi
        return 0
    fi
    if cmp -s "${current_file}" "${base_file}"; then
        cp "${image_file}" "${current_file}"
        return 0
    fi
    if cmp -s "${base_file}" "${image_file}"; then
        return 0
    fi

    if command -v diff3 >/dev/null 2>&1 && diff3 -m "${current_file}" "${base_file}" "${image_file}" > "${merge_file}"; then
        mv "${merge_file}" "${current_file}"
        echo "已合并 SDK 更新：${relative_path}"
    else
        rm -f "${merge_file}"
        cp "${image_file}" "${current_file}.update"
        echo "SDK 文件存在冲突，保留用户版本并生成：${current_file}.update" >&2
    fi
}

remove_deleted_sdk_files() {
    if [ ! -d "${SDK_BASE_DIR}" ]; then
        return 0
    fi
    while IFS= read -r relative_path; do
        base_file="${SDK_BASE_DIR}/${relative_path#./}"
        image_file="${IMAGE_SDK_DIR}/${relative_path#./}"
        current_file="${DATA_SDK_DIR}/${relative_path#./}"
        if [ -f "${image_file}" ] || [ ! -f "${current_file}" ]; then
            continue
        fi
        if cmp -s "${current_file}" "${base_file}"; then
            rm -f "${current_file}"
        else
            echo "官方 SDK 已删除 ${relative_path}，但当前文件有用户修改，已保留：${current_file}" >&2
        fi
    done <<EOF
$(cd "${SDK_BASE_DIR}" && find . -type f -print)
EOF
}

init_sdk() {
    image_sdk_hash=""
    old_sdk_hash=""
    current_sdk_hash=""
    if [ -f "${IMAGE_SDK_HASH}" ]; then
        image_sdk_hash=$(cat "${IMAGE_SDK_HASH}")
    fi
    if [ -f "${APP_SDK_HASH}" ]; then
        old_sdk_hash=$(cat "${APP_SDK_HASH}")
    fi

    if [ ! -d "${DATA_SDK_DIR}" ] || [ -z "$(find "${DATA_SDK_DIR}" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; then
        copy_image_sdk
        return 0
    fi

    current_sdk_hash=$(sdk_hash "${DATA_SDK_DIR}")

    if [ -z "${image_sdk_hash}" ]; then
        echo "镜像缺少 SDK 指纹，跳过 SDK 同步。" >&2
        return 0
    fi

    if [ ! -d "${SDK_BASE_DIR}" ] || [ -z "$(find "${SDK_BASE_DIR}" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; then
        if [ "${current_sdk_hash}" = "${image_sdk_hash}" ]; then
            copy_image_sdk_base
        elif [ -n "${old_sdk_hash}" ] && [ "${current_sdk_hash}" = "${old_sdk_hash}" ]; then
            copy_current_sdk_base
        else
            bootstrap_untrusted_sdk
            printf '%s\n' "${image_sdk_hash}" > "${APP_SDK_HASH}"
            return 0
        fi
    fi

    if [ "${old_sdk_hash}" = "${image_sdk_hash}" ]; then
        printf '%s\n' "${image_sdk_hash}" > "${APP_SDK_HASH}"
        return 0
    fi

    remove_deleted_sdk_files
    while IFS= read -r relative_path; do
        merge_sdk_file "${relative_path}"
    done <<EOF
$(sdk_image_files)
EOF
    copy_image_sdk_base
    printf '%s\n' "${image_sdk_hash}" > "${APP_SDK_HASH}"
}

init_app_bin
init_sdk

if [ -z "$(find "${DATA_DIR}/openapis" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; then
    cp -a "${IMAGE_OPENAPIS_DIR}/." "${DATA_DIR}/openapis/"
fi

apply_restart_if_requested() {
    if [ ! -f "${RESTART_REQUEST}" ]; then
        return 1
    fi

    restart_exports=$(python3 - "${RESTART_REQUEST}" <<'PY'
import json
import shlex
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as source:
    values = json.load(source)

required = (
    "ALLBOT_IGNORE_RESTART_MESSAGE_KEY",
    "ALLBOT_RESTART_NOTIFY_PLATFORM",
    "ALLBOT_RESTART_NOTIFY_TARGET",
    "ALLBOT_RESTART_STARTED_AT_NS",
)
for key in required:
    if not str(values.get(key, "")).strip():
        raise ValueError(f"Docker 重启请求缺少 {key}")

for key, value in values.items():
    if not key.startswith("ALLBOT_RESTART_") and key != "ALLBOT_IGNORE_RESTART_MESSAGE_KEY":
        continue
    print(f"export {key}={shlex.quote(str(value))}")
PY
    ) || {
        echo "Docker 重启请求无效：${RESTART_REQUEST}" >&2
        rm -f "${RESTART_REQUEST}"
        return 1
    }

    eval "${restart_exports}"
    export ALLBOT_RESTARTED=1
    export ALLBOT_RESTART_DELAY_MS=2000
    rm -f "${RESTART_REQUEST}"
    echo "AllBot Docker 重启请求已接管"
    return 0
}

apply_update_if_requested() {
    if [ ! -f "${UPGRADE_REQUEST}" ]; then
        return 1
    fi

    new_path=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8")).get("newPath", ""))' "${UPGRADE_REQUEST}")
    backup_path=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8")).get("backupPath", ""))' "${UPGRADE_REQUEST}")
    from_version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8")).get("fromVersion", ""))' "${UPGRADE_REQUEST}")
    to_version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8")).get("toVersion", ""))' "${UPGRADE_REQUEST}")
    notification_exports=$(python3 - "${UPGRADE_REQUEST}" <<'PY'
import json
import shlex
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as source:
    values = json.load(source)

notification = values.get("notification") or {}
fields = {
    "ALLBOT_IGNORE_UPDATE_MESSAGE_KEY": notification.get("messageKey", ""),
    "ALLBOT_UPDATE_NOTIFY_PLATFORM": notification.get("platform", ""),
    "ALLBOT_UPDATE_NOTIFY_ADAPTER_ID": notification.get("adapterId", ""),
    "ALLBOT_UPDATE_NOTIFY_USER_ID": notification.get("userId", ""),
    "ALLBOT_UPDATE_NOTIFY_GROUP_ID": notification.get("groupId", ""),
    "ALLBOT_UPDATE_NOTIFY_TARGET": notification.get("target", ""),
    "ALLBOT_UPDATE_NOTIFY_STARTED_AT_NS": notification.get("startedAtNs", ""),
}
for key, value in fields.items():
    if str(value).strip():
        print(f"export {key}={shlex.quote(str(value))}")
PY
    ) || {
        echo "Docker 更新请求无效：${UPGRADE_REQUEST}" >&2
        rm -f "${UPGRADE_REQUEST}"
        return 1
    }

    if [ -z "${new_path}" ] || [ ! -f "${new_path}" ]; then
        echo "升级请求无效：新程序不存在 ${new_path}" >&2
        rm -f "${UPGRADE_REQUEST}"
        return 1
    fi

    if [ -z "${backup_path}" ]; then
        backup_path="${DATA_DIR}/runtime/update/backup/allbot.bak"
    fi

    mkdir -p "$(dirname "${backup_path}")"
    rm -f "${backup_path}"
    if [ -f "${APP_BIN}" ]; then
        mv "${APP_BIN}" "${backup_path}"
    fi
    mv "${new_path}" "${APP_BIN}"
    chmod 0755 "${APP_BIN}"
    if [ -f "${IMAGE_HASH}" ]; then
        cp "${IMAGE_HASH}" "${APP_IMAGE_HASH}"
    else
        rm -f "${APP_IMAGE_HASH}"
    fi
    rm -f "${UPGRADE_REQUEST}"

    export ALLBOT_UPDATED=1
    export ALLBOT_UPDATED_FROM="${from_version}"
    export ALLBOT_UPDATED_TO="${to_version}"
    export ALLBOT_RESTARTED=1
    export ALLBOT_RESTART_DELAY_MS=2000
    eval "${notification_exports}"
    echo "AllBot Docker 更新已应用：${from_version} -> ${to_version}"
    return 0
}

clear_transient_startup_env() {
    unset ALLBOT_UPDATED ALLBOT_UPDATED_FROM ALLBOT_UPDATED_TO
    unset ALLBOT_RESTARTED ALLBOT_RESTART_DELAY_MS ALLBOT_PARENT_PID
    unset ALLBOT_IGNORE_RESTART_MESSAGE_KEY
    unset ALLBOT_IGNORE_UPDATE_MESSAGE_KEY
    unset ALLBOT_RESTART_NOTIFY_PLATFORM ALLBOT_RESTART_NOTIFY_ADAPTER_ID
    unset ALLBOT_RESTART_NOTIFY_USER_ID ALLBOT_RESTART_NOTIFY_GROUP_ID
    unset ALLBOT_RESTART_NOTIFY_TARGET ALLBOT_RESTART_STARTED_AT_NS
    unset ALLBOT_UPDATE_NOTIFY_PLATFORM ALLBOT_UPDATE_NOTIFY_ADAPTER_ID
    unset ALLBOT_UPDATE_NOTIFY_USER_ID ALLBOT_UPDATE_NOTIFY_GROUP_ID
    unset ALLBOT_UPDATE_NOTIFY_TARGET ALLBOT_UPDATE_NOTIFY_STARTED_AT_NS
}

terminate_child() {
    if [ -n "${child_pid:-}" ]; then
        kill "${child_pid}" 2>/dev/null || true
        wait "${child_pid}" 2>/dev/null || true
    fi
    exit 143
}

trap terminate_child TERM INT

cd "${DATA_DIR}"

while true; do
    "${APP_BIN}" --plugins="${DATA_DIR}/plugins" "$@" &
    child_pid=$!
    clear_transient_startup_env
    set +e
    wait "${child_pid}"
    status=$?
    set -e
    child_pid=""

    if apply_update_if_requested; then
        continue
    fi
    if apply_restart_if_requested; then
        continue
    fi

    exit "${status}"
done
