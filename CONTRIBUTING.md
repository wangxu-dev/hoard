# 贡献指南

感谢你关注 Hoard。Issue、文档改进、测试反馈和代码贡献都欢迎。

## 提交 Issue

提交前请先搜索已有 Issue。问题报告请尽量包含：

- 操作系统和版本。
- Hoard 版本或提交号。
- 清晰的复现步骤。
- 预期行为和实际行为。
- 相关的脱敏日志或错误信息。

请不要在 Issue、截图或日志中包含 Cookie、令牌、账号 ID、昵称、数据库文件或完整媒体地址。

## 开发环境

安装 Go、Bun 和项目所需的桌面运行依赖后，在项目根目录执行：

```sh
bun install --frozen-lockfile
bun run generate
bun run dev
```

只修改前端时可以使用：

```sh
bun run dev:web
```

## 提交前检查

根据修改范围运行相关检查。完整检查命令为：

```sh
gofmt -l .
go test ./...
go vet ./...
bun run generate
bun run typecheck
bun run build:web
```

`gofmt -l .` 不应输出 Go 文件。涉及构建配置或应用打包时，再运行 `bun run build`。

GitHub Actions 在提交到 `main` 的 PR 和 `main` 更新时运行 `Checks`：检查受 Git 跟踪的 Go 文件格式，运行 Go 测试与静态分析，再生成绑定、检查 TypeScript 并验证前端构建。CI 使用 macOS runner，不启动应用窗口，不生成安装包或发布 Release。真实账号登录和各系统桌面行为仍需人工验证。

## 分支与提交

每个独立需求从最新 `main` 创建单独分支，通过 Pull Request 合并。`main` 禁止直接推送、强制推送和删除。外部贡献者可以先 Fork 仓库，再从自己的分支提交 PR。

提交信息使用 gitmoji 和 Conventional Commits，例如：

```text
🐛 fix(sync): handle expired cursor
✨ feat(web): add source filter
🧪 test(store): cover account isolation
```

提交应保持范围清晰，避免把无关格式化或生成文件混入同一变更。

## 代码约定

- Go 包按领域放在 `internal/` 下，错误向上返回时保留上下文。
- 前端通过 `web/src/lib/api.ts` 访问后端绑定。
- 用户可见文案放在 i18n locale 文件中，并同步维护 `zh-CN` 和 `en`。
- 界面组件优先沿用项目现有的 shadcn、Lucide 和 token 体系。
- 日志使用 `[hoard:<模块>]` 前缀，不记录 Cookie、令牌、账号 ID 或完整媒体 URL。

## 平台来源与测试数据

来源接入涉及平台规则和个人账号数据。测试请使用虚构 ID、测试 URL 或 fixture，不要提交真实账号数据。涉及真实账号的行为验证应在本地完成，并在提交前清理日志和临时文件。

## Pull Request

Pull Request 请说明：

- 修改解决的问题。
- 影响的模块和用户流程。
- 已运行的检查或测试。
- 尚未验证的部分和已知限制。

保持 PR 小而聚焦，方便审查和回滚。
