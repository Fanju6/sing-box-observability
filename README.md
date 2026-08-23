# sing-box-observability

面向 sing-box 专用 `/observability/v1` API 的非官方、只读可观测性面板。项目使用 Go 采集实时指标和连接事件，将短期历史保存到 SQLite，并由同一个 Android ARM64 二进制提供响应式 Web 界面。

## 功能

- 概述、流量趋势、活动/近期连接和排行
- 所有浏览器共享一条上游 SSE 连接
- 正确处理计数器重置、采样缺口和短期历史
- 根据 observability API v1 的 capabilities 动态启用功能
- 可选的控制台登录和同源 HttpOnly Cookie 会话
- 简体中文/英文、亮色/暗色、键盘操作和减少动态效果
- 桌面侧栏、移动抽屉以及 390 px 小屏适配
- 前端资源嵌入 Android ARM64 单文件二进制

## 项目结构

```text
src/
  server/                 Go API、采集器、SSE、SQLite 与嵌入式前端
  web/                    React、TypeScript、Vite 与 Tailwind CSS 前端
contracts/openapi.yaml    前后端接口契约
packaging/android/        Android 配置、控制脚本与安装说明
scripts/                  构建和第三方许可证收集脚本
docs/                     架构、开发和发布文档
```

## 兼容要求

上游 sing-box 必须提供 observability API v1、`/capabilities`、cursor 分页和事件 ID。项目根据 capabilities 响应协商能力，不兼容旧的 `/status` 能力推导或 offset 上游分页。

详细协议与边界见 [架构与接口](./docs/architecture.md)。

## 本地运行

需要 Go 1.26.6+、Node.js 22.12+ 和 pnpm 11.19；Windows 构建脚本还需要 PowerShell 7+。

先启动模拟上游：

```powershell
Set-Location src/server
go run ./cmd/fakeupstream -listen 127.0.0.1:9090 -scenario online
```

另开终端启动后端：

```powershell
Set-Location src/server
Copy-Item config.example.yaml config.local.yaml
go run ./cmd/sing-box-observability -config config.local.yaml
```

再启动前端：

```powershell
Set-Location src/web
corepack enable
pnpm install --frozen-lockfile
pnpm dev
```

打开 `http://127.0.0.1:5173`。Vite 会把 API、健康检查和 SSE 请求代理到 `127.0.0.1:9095`。仅在需要前端模拟数据时显式设置 `VITE_MSW=true`。

连接真实 sing-box 时，在 `src/server/config.local.yaml` 中设置 `singbox.base_url` 和 `singbox.token`，也可使用对应的 `SBOBS_*` 环境变量。不要提交真实令牌。

## Android 构建

Windows：

```powershell
./scripts/build-android.ps1
```

Linux 或 macOS：

```sh
sh ./scripts/build-android.sh
```

默认输出到 `release/android-arm64/`。构建脚本会重新构建前端、替换嵌入资源、交叉编译 Android/arm64，并生成依赖许可、构建清单和校验和。安装方式见 [Android 使用说明](./packaging/android/README.md)。

## 配置与安全

- 浏览器永远不会收到上游 sing-box 令牌。
- 服务默认只监听 `127.0.0.1:9095`。
- 非回环监听必须设置 `console.auth_token`，并建议放在 HTTPS 反向代理后。
- SQLite 可能保存域名、地址、进程、用户和规则，请保护数据目录并设置合理保留时间。
- 配置中的未知 YAML 字段会直接报错，避免拼写错误被静默忽略。

安全问题请阅读 [安全策略](./.github/SECURITY.md)。

## 开发与许可

- [文档索引](./docs/README.md)
- [开发与测试](./docs/development.md)
- [贡献指南](./.github/CONTRIBUTING.md)
- [发布检查](./docs/publishing.md)

项目原创代码采用 GPL-3.0-or-later。部分界面改编自 sing-box-dashboard，并受其附加命名条件约束；其他组件和字体适用各自许可证。完整信息见 [NOTICE](./NOTICE) 和 [THIRD_PARTY_LICENSES.txt](./THIRD_PARTY_LICENSES.txt)。
