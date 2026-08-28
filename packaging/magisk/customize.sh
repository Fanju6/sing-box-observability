#!/system/bin/sh

STATE_DIR=/data/adb/sing-box-observability
CONFIG_FILE="$STATE_DIR/config.yaml"
DEFAULT_CONFIG="$MODPATH/config/default.yaml"

if [ "$ARCH" != "arm64" ]; then
  abort "! 仅支持 arm64-v8a 设备，当前架构：$ARCH"
fi

ui_print "- 准备 sing-box Observability"

# 停止并移除旧的手动安装入口，配置和数据库保持不变。
if [ -x "$STATE_DIR/sing-box-observabilityctl" ]; then
  "$STATE_DIR/sing-box-observabilityctl" stop >/dev/null 2>&1 || true
fi
rm -f "$STATE_DIR/sing-box-observability"
rm -f "$STATE_DIR/sing-box-observabilityctl"
rm -f /data/adb/service.d/sing-box-observability.sh

mkdir -p "$STATE_DIR/data"
if [ ! -f "$CONFIG_FILE" ]; then
  cp "$DEFAULT_CONFIG" "$CONFIG_FILE" || abort "! 无法创建默认配置"
  ui_print "- 已创建配置：$CONFIG_FILE"
else
  ui_print "- 保留现有配置：$CONFIG_FILE"
fi

set_perm "$MODPATH/bin/sing-box-observability" 0 0 0755
set_perm "$MODPATH/bin/sing-box-observabilityctl" 0 0 0755
set_perm "$MODPATH/service.sh" 0 0 0755
set_perm "$MODPATH/action.sh" 0 0 0755
set_perm "$MODPATH/uninstall.sh" 0 0 0755
set_perm "$STATE_DIR" 0 0 0700
set_perm "$STATE_DIR/data" 0 0 0700
set_perm "$CONFIG_FILE" 0 0 0600

ui_print "- Web 面板：http://127.0.0.1:9095"
if [ "${KSU:-false}" = "true" ]; then
  ui_print "- KernelSU WebUI 已启用"
fi
ui_print "- 安装完成后请重启设备"
