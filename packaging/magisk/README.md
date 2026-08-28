# Magisk / KernelSU 模块使用说明

构建产物 `sing-box-observability-<版本>-module-arm64.zip` 是可直接安装的 Magisk / KernelSU 通用模块，不要解压后手动复制。

## 安装

1. 确认设备架构为 `arm64-v8a`，并已安装可用的 Magisk 或 KernelSU。
2. 打开 root 管理器的模块页面，选择“从本地安装”。
3. 选择模块 ZIP，安装完成后重启设备。
4. 在设备浏览器打开 `http://127.0.0.1:9095`。

在 KernelSU 管理器中，还可以直接点击模块的“WebUI”按钮。入口页会确认服务已启动，然后在管理器 WebView 内显示完整面板。

首次安装会创建：

```text
/data/adb/sing-box-observability/config.yaml
/data/adb/sing-box-observability/data/
```

模块升级不会覆盖这两个位置。

## 配置

编辑 `/data/adb/sing-box-observability/config.yaml`：

- `singbox.base_url`：通常为 `http://127.0.0.1:9090`
- `singbox.token`：上游 observability 监听器密钥
- `storage.retention`：历史数据保留时间，例如 `168h`、`720h`、`2160h`
- `server.listen`：设备本地使用时保持 `127.0.0.1:9095`
- `console.auth_token`：仅回环访问时允许留空

修改后，可点击模块页面中的“操作”按钮重启服务，也可以执行：

```sh
su -c /data/adb/modules/sing-box-observability/bin/sing-box-observabilityctl restart
```

## 状态与日志

```sh
su -c /data/adb/modules/sing-box-observability/bin/sing-box-observabilityctl status
su -c '/data/adb/modules/sing-box-observability/bin/sing-box-observabilityctl logs 100'
curl http://127.0.0.1:9095/readyz
```

运行日志位于 `/data/adb/sing-box-observability/sing-box-observability.log`。

## 更新与卸载

更新时直接在 Magisk App 安装新版 ZIP 并重启。现有配置和 SQLite 数据会被保留。

从 Magisk 卸载模块会停止服务，但同样保留配置与历史数据，避免误删。如需彻底清理，请在确认无需历史记录后执行：

```sh
su -c 'rm -rf /data/adb/sing-box-observability'
```

此操作不可恢复。
