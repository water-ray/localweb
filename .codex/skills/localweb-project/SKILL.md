---
name: localweb-project
description: 维护 localweb 的 Codex 项目上下文。用于修改 Go 服务结构、认证转发功能、配置文件、测试、构建脚本、文档和发布流程。
---

# localweb Project

## 入口

- 先读 `README.md`。
- 主程序入口：`cmd/localweb/main.go`。
- 关键源码目录：`internal/config/`、`internal/auth/`、`internal/server/`。
- 示例配置：`config.example.json`。
- 临时输出：`temp/`。
- 最终产物：`Bin/localweb/`。

## 技术栈

- 主要语言：Go。
- 应用框架：Go 标准库 `net/http`。
- 包管理器：Go modules。
- 运行命令：`go run ./cmd/localweb -config config.json`。
- 构建命令：`python scripts/build/build_localweb.py`。
- 测试命令：`go test ./...`。

## 工作流

1. 确认用户请求属于配置、认证、代理、部署、测试还是文档。
2. 读取 `README.md` 和相关 `internal/` 包。
3. 保持 Go 标准库优先，只有明确必要时再引入第三方依赖。
4. 修改后运行 `gofmt` 和 `go test ./...`。
5. 配置结构或行为变化时，同步更新 `README.md` 和 `config.example.json`。

## 命名与规范

- 项目 slug：`localweb`。
- Go 包名使用小写英文，目录按职责拆分。
- 配置 JSON 字段使用 snake_case。
- JWT 中的 route claim 必须按路由隔离，避免一个路径的登录态复用于另一个路径。
- 根目录 `README.md` 用于项目说明；长期开发规范以 `AGENTS.md` 和 `.codex/skills/` 为准。

## 约束

- 代理目标默认只允许本机服务，默认 `target_host` 为 `127.0.0.1`。
- 普通 HTTP 请求按请求验证 JWT；WebSocket 和长连接在连接建立前验证。
- 子路径代理默认开启末尾斜杠跳转、后端重定向改写和 Cookie Path 改写。
- 密码配置可用明文、环境变量或 SHA-256 哈希；生产环境优先使用环境变量或后续更强哈希方案。
- 不把构建产物提交到源码目录，产物输出到 `Bin/localweb/`。
