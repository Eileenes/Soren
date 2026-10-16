#!/bin/sh
set -eu

data_dir="${SOREN_DATA_DIR:-/data}"
case "$data_dir" in
  /*) ;;
  *) echo 'SOREN_DATA_DIR 必须是绝对路径' >&2; exit 1 ;;
esac
if [ "$data_dir" = / ] || [ -L "$data_dir" ]; then
  echo 'SOREN_DATA_DIR 不可为根目录或符号链接' >&2
  exit 1
fi

if [ "$(id -u)" = 0 ]; then
  # PVC 覆盖镜像目录后可能为 root:root。只初始化挂载根目录，保留内部文件权限。
  mkdir -p "$data_dir"
  chmod u+rwx "$data_dir"
  chown 10001:10001 "$data_dir"
  exec su-exec 10001:10001 "$0" "$@"
fi

if [ ! -d "$data_dir" ] || [ ! -w "$data_dir" ] || [ ! -x "$data_dir" ]; then
  echo 'SOREN_DATA_DIR 不可写，请初始化卷权限或允许入口以 root 初始化后降权' >&2
  exit 1
fi

# access 权限检查不足以发现所有只读挂载，实际创建并删除一个临时文件后才启动。
if ! probe="$(mktemp "$data_dir/.soren-write-check.XXXXXX")"; then
  echo 'SOREN_DATA_DIR 写入检查失败，请检查挂载是否只读或存储是否可用' >&2
  exit 1
fi
rm -- "$probe"

exec "$@"
