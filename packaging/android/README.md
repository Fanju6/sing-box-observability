# Android ARM64 使用说明

## 包内文件

- `sing-box-observability`：已嵌入当前前端的 Android ARM64 二进制
- `config.yaml`：服务、上游、采集器和 SQLite 示例配置
- `sing-box-observabilityctl`：启动、停止、重启、状态和日志工具
- `service.d.sh`：可选的 Magisk 开机脚本
- `LICENSE`、`NOTICE`、`THIRD_PARTY_LICENSES.txt`：项目与第三方许可
- `FRONTEND-LICENSES.txt`、`GO-LICENSES.txt`：锁定依赖的完整许可汇总
- `FRONTEND-LICENSES.json`、`GO-MODULES.txt`：不含本机路径的依赖清单
- `BUILD-MANIFEST.txt`：版本、提交、构建时间、目标和前端摘要
- `SHA256SUMS.txt`：包内文件校验和

设备需要 root，建议安装到 `/data/adb/sing-box-observability`。

## 1. 检查设备

```sh
getprop ro.product.cpu.abi
```

本包只支持 `arm64-v8a`。安装前验证完整性：

```sh
sha256sum -c SHA256SUMS.txt
```

## 2. 启用 sing-box observability

sing-box 必须提供 observability API v1、`/capabilities`、cursor 分页和事件 ID。典型实验配置如下，实际字段以所编译的 sing-box 源码为准：

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "replace-with-a-secret"
    },
    "observability": {
      "enabled": true,
      "recent_connections": 2000,
      "recent_ttl": "1h",
      "top_k_size": 100,
      "expose_sensitive": false
    }
  }
}
```

测试端点：

```sh
export SING_BOX_TOKEN='<填写上游监听器密钥>'
curl -H "Authorization: Bearer ${SING_BOX_TOKEN}" \
  http://127.0.0.1:9090/observability/v1/capabilities
unset SING_BOX_TOKEN
```

响应必须包含 `apiVersion: 1`、`cursorPagination: true` 以及所需的指标、连接和事件端点。不支持旧协议回退。

## 3. 安装

把构建目录复制到设备，然后执行：

```sh
su
mkdir -p /data/adb/sing-box-observability/data
cp sing-box-observability config.yaml sing-box-observabilityctl \
  /data/adb/sing-box-observability/
chmod 700 /data/adb/sing-box-observability
chmod 755 /data/adb/sing-box-observability/sing-box-observability
chmod 755 /data/adb/sing-box-observability/sing-box-observabilityctl
chmod 600 /data/adb/sing-box-observability/config.yaml
chmod 700 /data/adb/sing-box-observability/data
```

编辑 `/data/adb/sing-box-observability/config.yaml`：

- `singbox.base_url`：通常为 `http://127.0.0.1:9090`
- `singbox.token`：上游监听器密钥；未配置密钥时留空
- `storage.path`：本地 SQLite 历史路径
- `storage.retention`：历史保留时间
- `server.listen`：设备本地使用时保持 `127.0.0.1:9095`
- `console.auth_token`：仅回环访问时允许留空

如果修改了监听端口，还要把 `SBOBS_READY_URL` 设置为对应的回环 `/readyz` 地址。密钥优先使用 `SBOBS_SINGBOX_TOKEN` 和 `SBOBS_CONSOLE_AUTH_TOKEN` 环境变量覆盖。

## 4. 启动和使用

```sh
/data/adb/sing-box-observability/sing-box-observabilityctl start
/data/adb/sing-box-observability/sing-box-observabilityctl status
```

在同一台 Android 设备的浏览器中打开 `http://127.0.0.1:9095`。

常用命令：

```sh
# 查看版本
/data/adb/sing-box-observability/sing-box-observability -version

# 查看最近 100 行日志
/data/adb/sing-box-observability/sing-box-observabilityctl logs 100

# 应用配置修改
/data/adb/sing-box-observability/sing-box-observabilityctl restart

# 停止
/data/adb/sing-box-observability/sing-box-observabilityctl stop

# 检查服务状态
curl http://127.0.0.1:9095/healthz
curl http://127.0.0.1:9095/readyz
curl http://127.0.0.1:9095/api/v1/meta
```

## 5. Magisk 开机启动

先确认手动启动成功，再安装开机脚本：

```sh
cp service.d.sh /data/adb/service.d/sing-box-observability.sh
chmod 755 /data/adb/service.d/sing-box-observability.sh
```

如果 sing-box 启动较晚，采集器会继续重试。

## 6. 升级

备份 `config.yaml` 并停止服务，校验新包后只替换 `sing-box-observability`、`sing-box-observabilityctl` 和可选的 `service.d.sh`，随后重启并检查 `/readyz`。不要替换 `data` 目录，除非明确要丢弃历史。

## 7. 隐私、远程访问与卸载

上游开启敏感字段后，SQLite 可能包含域名、地址、进程、用户和规则。配置与日志也可能包含运行信息，请保持安装目录权限为 `0700`。

默认只支持回环访问。其他设备需要连接时，应使用 HTTPS 反向代理、设置长随机 `console.auth_token`、启用 `server.secure_cookie: true`，并只信任必要代理。不建议直接通过明文 HTTP 暴露到局域网。

删除服务及全部历史：

```sh
/data/adb/sing-box-observability/sing-box-observabilityctl stop
rm -f /data/adb/service.d/sing-box-observability.sh
rm -rf /data/adb/sing-box-observability
```

最后一条命令不可恢复。
