# 安全策略

## 支持版本

首个稳定版发布前，只维护 `main` 分支的最新版本。

## 报告漏洞

不要为疑似漏洞创建公开 Issue。请使用 GitHub 私密 Security Advisory 报告，并说明受影响版本、影响、复现方法和可能的修复建议。不要附带真实 sing-box 令牌、连接记录或其他个人数据。

如果仓库尚未启用私密报告，请先私下联系维护者再公开细节。首个正式发布前，确认和修复时限为尽力而为。

## 部署建议

- 除非确实需要远程访问，否则保持 `server.listen` 为 `127.0.0.1`。
- 未认证的回环模式会拒绝非回环 Host，避免增加绕过该检查的代理配置。
- 非回环访问必须设置 `console.auth_token`，仅通过 HTTPS 反向代理暴露，并设置 `server.secure_cookie: true`。
- 密钥优先使用 `SBOBS_SINGBOX_TOKEN` 和 `SBOBS_CONSOLE_AUTH_TOKEN` 环境变量，不要写入公开配置。
- 限制配置、日志和 SQLite 目录权限；Android 控制脚本使用 `umask 077`。
- 上游开启 `expose_sensitive` 后，应把 SQLite 历史视为敏感数据。
- 不公开本地数据库、日志、真实配置、浏览器 trace 或从设备生成的 release 目录。
