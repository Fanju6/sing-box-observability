#!/system/bin/sh

MODDIR=${0%/*}
STATE_DIR=/data/adb/sing-box-observability

"$MODDIR/bin/sing-box-observabilityctl" stop >/dev/null 2>&1 || true
rm -f "$STATE_DIR/sing-box-observability.pid"

# 有意保留配置、日志和数据库，避免卸载或重装时丢失历史数据。
