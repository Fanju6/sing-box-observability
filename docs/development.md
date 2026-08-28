# 开发与测试

## 环境

- Go 1.26.6 或更高版本
- Node.js 22.12 或更高版本
- pnpm 11.19
- PowerShell 7（Windows 构建）
- Chromium（端到端测试）

安装前端依赖：

```sh
pnpm --dir src/web install --frozen-lockfile
```

## 接口契约

`contracts/openapi.yaml` 是唯一接口契约。保持 camelCase JSON 字段和既有错误信封。修改契约后必须重新生成类型：

```sh
pnpm --dir src/web typegen
git diff -- src/web/src/api/schema.d.ts
```

后端、前端和兼容性测试必须与生成类型一起更新。

## 前端设计约束

- 视觉规范以当前组件和 CSS 变量为准，继续保持官方 dashboard 的侧栏、排版密度和卡片层级。
- 桌面使用固定侧栏，移动端使用抽屉，不增加底部导航。
- 组件优先复用 `src/web/src/components/ui`，避免页面内重复定义样式。
- 用户可见文本必须同时存在简体中文和英文；默认文档语言为简体中文。
- 必须覆盖亮色/暗色、键盘焦点、减少动态效果、安全区、加载/空/错误/陈旧状态。
- 390 px、820 px 和 1440 px 是必测视口。
- 模拟数据只在显式设置 `VITE_MSW=true` 时启用，生产产物不得包含 `mockServiceWorker.js`。

界面中改编自 sing-box-dashboard 和 shadcn/ui 的部分必须保留源码头部归属，许可信息集中在 `NOTICE` 与 `THIRD_PARTY_LICENSES.txt`。

## 必跑检查

前端：

```sh
pnpm --dir src/web check
pnpm --dir src/web test:e2e
pnpm --dir src/web audit --prod --audit-level high
```

后端：

```sh
cd src/server
gofmt -w .
go mod tidy -diff
go mod verify
go vet ./...
go test ./...
go test -tags webui ./...
go test -race ./internal/collector ./internal/events
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

Windows 本机若没有 CGO/GCC，可以由 Linux CI 执行 race 测试，其余检查仍应在本机完成。

完整嵌入、Android 交叉编译和 Magisk / KernelSU 模块 ZIP 使用：

```powershell
./scripts/build-android.ps1
```

构建脚本会重新构建前端、替换 `src/server/internal/webui/dist`、运行 webui 标签测试、生成许可证清单、验证嵌入摘要，并校验模块 ZIP 和 KernelSU `webroot/index.html` 的必需入口。

## 提交要求

- 变更保持聚焦并说明用户可见影响。
- 行为变化必须补充测试。
- 不提交令牌、真实配置、数据库、日志、`node_modules`、前端构建、嵌入暂存或 release 产物。
- 不记录、返回或持久化上游 sing-box 令牌。
