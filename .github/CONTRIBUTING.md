# 贡献指南

感谢参与改进 sing-box-observability。

## 范围

项目只支持只读的 sing-box `/observability/v1` API。代理切换、配置修改、连接关闭以及无关的 Clash/sing-box API 不在项目范围内。

## 开发准备

1. 安装 Go 1.26.6+、Node.js 22.12+ 和 pnpm 11.19。
2. 运行 `pnpm --dir src/web install --frozen-lockfile`。
3. 将 `src/server/config.example.yaml` 复制为 `src/server/config.local.yaml`。
4. 按根目录 `README.md` 启动模拟上游、后端和前端。

模拟数据必须显式设置 `VITE_MSW=true` 才会启用。任何 `VITE_*` 值都可能进入浏览器代码，禁止放入令牌或其他秘密。

## 契约与测试

`contracts/openapi.yaml` 是前后端契约。保持 camelCase JSON 字段和既有错误信封。修改后运行 `pnpm --dir src/web typegen`，并提交生成类型、后端测试和前端兼容性测试。

完整检查命令见 [开发与测试](../docs/development.md)。至少需要通过：

```sh
pnpm --dir src/web check
pnpm --dir src/web test:e2e

cd src/server
go vet ./...
go test ./...
go test -tags webui ./...
```

## Pull Request

- 保持变更聚焦并说明用户可见影响。
- 行为变化必须新增或更新测试。
- 中英文 UI 文案必须同步。
- 覆盖亮色/暗色、键盘焦点、减少动态效果、加载/空/错误/陈旧状态、移动安全区和 390 px 视口。
- 不提交令牌、真实配置、数据库、日志、`node_modules`、构建产物、浏览器 trace 或 release 目录。
- 不记录、返回或持久化上游 sing-box 令牌。

提交贡献即表示同意原创贡献采用 GPL-3.0-or-later；改编自 sing-box-dashboard 的部分还受 `THIRD_PARTY_LICENSES.txt` 中附加条件约束。
