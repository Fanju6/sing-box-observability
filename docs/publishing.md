# 发布指南

## 身份与许可

- 项目名为 `sing-box-observability`，仓库预期地址为 `github.com/Fanju6/sing-box-observability`。
- 当前名称仍包含 `sing-box`。在公开发布前，应取得 sing-box-dashboard 附加命名条件所要求的许可，或完成独立名称及法律审查。
- 原创代码采用 GPL-3.0-or-later；改编和第三方部分继续适用 `NOTICE` 与 `THIRD_PARTY_LICENSES.txt` 中的条款。
- 非官方、无关联声明必须保留。
- 新增依赖或复制外部代码时，必须同步更新归属和许可证收集逻辑。

## Git 历史

旧私有历史包含已删除的内部规划文档和个人绝对路径。公开时建议从当前清洁工作树建立新的公开仓库和初始提交。若要保留历史，应先离线备份，再有意识地 squash 或重写；`.gitignore` 不会清除旧提交中的内容。

源码扫描未发现真实令牌、私钥、设备标识、数据库或二进制，但发布前仍需重新扫描当前快照和最终历史。

## 发布前检查

1. 确认 `git status` 只包含预期源码变更。
2. 执行 [开发与测试](./development.md) 中的全部检查。
3. 从清洁 checkout 运行 Android 构建脚本。
4. 确认生产前端和二进制不包含 `mockServiceWorker`、本地路径、令牌或测试凭据。
5. 校验 `SHA256SUMS.txt` 并检查 `BUILD-MANIFEST.txt`。
6. 检查 `FRONTEND-LICENSES.txt`、`GO-LICENSES.txt` 和 `THIRD_PARTY_LICENSES.txt` 是否随包发布。

## GitHub 设置

- 启用私密漏洞报告、Secret Scanning 和 Dependabot。
- 为 `main` 启用 `.github/workflows/ci.yml` 中的必需检查。
- 工作流只保留声明的最小权限。
- 仓库描述和 Topics 不得暗示官方关联。
- 补充当前桌面和移动端截图。

## 首个版本

从清洁提交创建签名的 `v0.x.y` 标签。Release 工作流会生成 Android ARM64 压缩包和校验文件，并创建 GitHub Release。发布前确认构建清单中的版本、提交和提交时间与标签一致。
