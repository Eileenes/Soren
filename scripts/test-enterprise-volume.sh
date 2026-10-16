#!/usr/bin/env bash
set -euo pipefail

# 只使用临时 Docker 卷与测试数据，不连接集群或读取实际密钥。
image="${1:?Usage: bash scripts/test-enterprise-volume.sh IMAGE}"
volume="soren-volume-test-$$"
docker volume create "$volume" >/dev/null
trap 'docker volume rm "$volume" >/dev/null' EXIT
run=(docker run --rm --platform linux/amd64 --mount "type=volume,src=$volume,dst=/data")

# 模拟平台新卷的 root:root 挂载根，同时保留一个不同属主的已有目录。
"${run[@]}" --user 0:0 --entrypoint sh "$image" -ec '
  chown 0:0 /data
  chmod 755 /data
  mkdir /data/keep
  chown 10002:10002 /data/keep
  chmod 700 /data/keep
'

# 未初始化时，强制非 root 必须明确失败，不能假装就绪。
if "${run[@]}" --user 10001:10001 --entrypoint /usr/local/bin/soren-entrypoint "$image" true; then
  echo 'FAIL: 未初始化的卷不应允许非 root 启动' >&2
  exit 1
fi

# 初始化与降权：根文件系统只读，实际业务命令 UID/GID 正确且无有效 capabilities。
"${run[@]}" --read-only --security-opt no-new-privileges:true \
  --entrypoint /usr/local/bin/soren-entrypoint "$image" sh -ec '
  test "$(id -u)" = 10001
  test "$(id -g)" = 10001
  grep -Eq "^CapEff:[[:space:]]*0+$" /proc/self/status
  mkdir /data/workspaces
  echo persisted > /data/workspaces/probe
'

# 再次初始化不修改历史文件属主/权限，重启后文件仍在。
"${run[@]}" --entrypoint /usr/local/bin/soren-entrypoint "$image" sh -ec '
  test "$(stat -c %u:%g /data)" = 10001:10001
  test "$(stat -c %u:%g /data/keep)" = 10002:10002
  test "$(stat -c %a /data/keep)" = 700
  test "$(cat /data/workspaces/probe)" = persisted
'

# Compose 的显式非 root / 禁用 capabilities 模式仍可使用已经准备好的卷。
"${run[@]}" --user 10001:10001 --cap-drop ALL --read-only \
  --security-opt no-new-privileges:true \
  --entrypoint /usr/local/bin/soren-entrypoint "$image" sh -ec '
  test "$(id -u)" = 10001
  echo nonroot >> /data/workspaces/probe
'

# 无法写入的只读卷要阻止启动。
if docker run --rm --platform linux/amd64 --user 10001:10001 \
  --mount "type=volume,src=$volume,dst=/data,readonly" \
  --entrypoint /usr/local/bin/soren-entrypoint "$image" true; then
  echo 'FAIL: 只读卷不应通过启动检查' >&2
  exit 1
fi
echo 'PASS: 新卷初始化、降权、持久化、已有目录保护及非 root 兼容性'
