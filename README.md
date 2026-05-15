# localweb

`localweb` 是一个使用 Go 开发的轻量级 Web 认证转发网关。它运行在服务器的公开 HTTP 端口上，为只监听 `127.0.0.1:<port>` 的本机 Docker Web 服务增加一层路由级认证和密码保护。

项目基于 `work_codex` 基础框架初始化，开发规则、脚本和项目内 skill 仍保留在 `AGENTS.md`、`.codex/skills/`、`docs/` 和 `scripts/` 中。

## 项目目标

- 对外只暴露一个 HTTP 服务，例如 `0.0.0.0:80`。
- 按路径转发本机服务，例如 `/abc1` 转发到 `127.0.0.1:10203`。
- 每个转发路径可配置独立访问密码。
- 认证成功后签发 JWT，并通过 HttpOnly Cookie 缓存登录状态。
- 后端 Docker 应用只监听本机地址，不直接对公网开放。
- 支持常规 HTTP 请求，并预留 WebSocket、SSE 和长轮询转发能力。

## 配置文件

程序启动时读取 `config.json`。推荐使用 `routers` 数组；程序也兼容用户草案中的 `router` 数组别名。推荐使用 `http_headers` 对象；程序也兼容旧式 `httpheaders` 别名。

最小配置示例：

```json
{
  "port": 80,
  "jwt": {
    "secret": "please-change-this-secret",
    "ttl_seconds": 604800,
    "cookie_name": "localweb_token"
  },
  "routers": [
    {
      "path": "/abc1",
      "port": 10203,
      "password": "123456",
      "http_headers": {}
    },
    {
      "path": "/abc2",
      "port": 10204,
      "password": "123456",
      "http_headers": {}
    }
  ]
}
```

推荐配置示例：

```json
{
  "bind_host": "0.0.0.0",
  "port": 80,
  "jwt": {
    "secret": "please-change-this-secret",
    "ttl_seconds": 604800,
    "cookie_name": "localweb_token",
    "issuer": "localweb"
  },
  "security": {
    "login_path": "/_localweb/login",
    "cookie_secure": false,
    "cookie_same_site": "Lax",
    "rate_limit_per_minute": 30
  },
  "routers": [
    {
      "name": "app-abc1",
      "path": "/abc1",
      "target_host": "127.0.0.1",
      "port": 10203,
      "password": "123456",
      "strip_path_prefix": true,
      "force_trailing_slash": true,
      "rewrite_redirects": true,
      "rewrite_cookie_path": true,
      "websocket": true,
      "timeout_seconds": 60,
      "http_headers": {
        "X-Forwarded-Proto": "http"
      }
    }
  ]
}
```

字段说明：

- `bind_host`：监听地址，默认可设为 `0.0.0.0`。
- `port`：对外 HTTP 监听端口。Linux 下直接监听 `80` 端口可能需要 root 权限或 `CAP_NET_BIND_SERVICE`。
- `jwt.secret`：JWT 签名密钥，生产环境必须使用高强度随机值，后续可支持环境变量覆盖。
- `jwt.secret_env`：可选，从环境变量读取 JWT 签名密钥；配置后优先于 `jwt.secret`。
- `jwt.ttl_seconds`：登录有效期，默认建议 7 天。
- `security.cookie_secure`：使用 HTTPS 时应设为 `true`。
- `security.cookie_same_site`：Cookie SameSite 策略，支持 `Lax`、`Strict`、`None`。
- `security.rate_limit_per_minute`：登录接口限流，避免密码暴力尝试。
- `routers[].path`：公网访问路径前缀，必须以 `/` 开头。
- `routers[].target_host`：目标服务地址，默认 `127.0.0.1`。
- `routers[].port`：目标本机服务端口。
- `routers[].password`：当前路径访问密码。
- `routers[].password_env`：可选，从环境变量读取当前路径访问密码；配置后优先于 `password`。
- `routers[].password_sha256`：可选，使用密码的 SHA-256 十六进制摘要验证，避免明文密码直接落盘。
- `routers[].strip_path_prefix`：是否去掉路径前缀再转发。若为 `true`，`/abc1/api` 会转发为后端的 `/api`。
- `routers[].force_trailing_slash`：访问 `/abc1` 时自动跳到 `/abc1/`，避免相对资源被浏览器解析到站点根路径。默认开启。
- `routers[].rewrite_redirects`：把后端返回的 `Location: /xxx` 改写为 `Location: /abc1/xxx`。默认开启。
- `routers[].rewrite_cookie_path`：把后端 `Set-Cookie` 中的 `Path=/` 改写为当前路由路径。默认开启。
- `routers[].websocket`：是否允许 WebSocket Upgrade。未配置时默认允许。
- `routers[].http_headers`：转发到后端时追加或覆盖的请求头。

## 认证与转发流程

1. 用户访问 `/abc1` 或 `/abc1/...`。
2. 网关查找匹配的路由配置。
3. 如果请求没有携带有效 JWT，返回登录页面或跳转到 `security.login_path`。
4. 用户提交当前路径对应的密码。
5. 密码验证通过后，网关签发 JWT，JWT 中记录 `route_path`、签发时间和过期时间。
6. JWT 写入 HttpOnly Cookie，Cookie Path 建议设置为对应路由路径，例如 `/abc1`。
7. 后续访问同一路径时先验证 JWT，再反向代理到 `http://127.0.0.1:<port>`。
8. JWT 的 `route_path` 必须和当前路由匹配，避免 `/abc1` 的登录态被复用于 `/abc2`。

## 关于 JWT 和 TCP 连接

本项目建议按 HTTP 反向代理实现，不建议只在首次 TCP 连接上认证一次。

原因：

- 路径 `/abc1`、`/abc2` 属于 HTTP 请求信息，TCP 握手阶段看不到路径。
- HTTP/1.1 keep-alive 下，一个 TCP 连接可以连续发送多个 HTTP 请求，甚至可能访问不同路径。
- HTTP/2 下，一个 TCP 连接可以并发多条 stream，按 TCP 连接授权会混淆请求边界。
- 反向代理通常会维护两段连接：客户端到网关、网关到本机服务。认证通过后，网关仍需要负责数据中转，而不是把原始 TCP 连接直接交给后端服务。

建议策略：

- 普通 HTTP 请求：每个请求进入路由前验证 JWT。
- WebSocket、SSE、长轮询：在 Upgrade 或长连接建立前验证一次 JWT，验证通过后持续转发该连接的数据直到关闭。
- 如果未来需要"认证后纯 TCP 隧道"模式，应作为独立功能设计，例如 CONNECT 隧道或 WebSocket 隧道；该模式不适合当前按路径代理多个 Web 服务的设计。

## 路由规则

- 路由按最长路径前缀匹配，避免 `/abc` 抢先匹配 `/abc1`。
- `/abc1` 只能匹配 `/abc1` 和 `/abc1/...`，不能匹配 `/abc123`。
- 启动时会校验重复路径、非法路径、非法端口、空 JWT 密钥和空密码。
- 子路径代理可能会影响后端页面中的资源路径。当前已处理常见的 `/abc1` 到 `/abc1/` 跳转、后端重定向路径和 Cookie Path 改写。
- 如果后端页面硬编码了绝对资源路径，例如 `/assets/app.js`，优先让后端应用支持 base path；无法配置时建议使用独立子域名或端口代理。

## 目录结构

```text
.
├─ cmd/
│  └─ localweb/
│     └─ main.go
├─ internal/
│  ├─ auth/
│  ├─ config/
│  └─ server/
├─ config.example.json
├─ docs/
├─ scripts/
├─ Bin/
└─ temp/
```

## 使用方式

复制示例配置：

```powershell
Copy-Item config.example.json config.json
```

按需修改 `config.json` 后运行：

```powershell
go run ./cmd/localweb -config config.json
```

配置里的 `port` 如果是 `80`，在 Linux 上通常需要 root 权限、`CAP_NET_BIND_SERVICE`，或放在 Nginx/Caddy 等前置服务后面监听高端口。

## 开发命令

测试：

```powershell
go test ./...
```

编译所有平台（Windows + Linux amd64）：

```powershell
python scripts/build/build_localweb.py
```

编译输出结构：

```
Bin/
├─ localweb-windows-amd64/
│  ├─ localweb.exe
│  └─ config.example.json
└─ localweb-linux-amd64/
   ├─ localweb
   └─ config.example.json
```

VS Code 任务 `发布：编译LocalWeb` 会调用编译脚本，编译 Windows 和 Linux 版本，并自动复制 `config.example.json` 到各输出目录。

等价 Go 命令（Linux）：

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o Bin/localweb-linux-amd64/localweb ./cmd/localweb
```

等价 Go 命令（Windows）：

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o Bin/localweb-windows-amd64/localweb.exe ./cmd/localweb
```

## 当前能力

- Go module 和基础目录已初始化。
- 已实现 `config.json` 解析、默认值填充和启动校验。
- 已实现明文密码、环境变量密码和 SHA-256 密码摘要验证。
- 已实现 JWT 签发、JWT 校验和 HttpOnly Cookie 写入。
- 已实现路径级反向代理，支持 `strip_path_prefix`。
- 已实现路由根路径末尾斜杠跳转、后端重定向路径改写和 Cookie Path 改写。
- 已支持 WebSocket Upgrade 转发。
- 已实现登录限流、基础错误处理和代理错误日志。
- 已增加配置解析、JWT、路由匹配和登录代理流程测试。
- 已实现跨平台编译，支持 Windows 和 Linux amd64 版本。

## 后续计划

- 支持 bcrypt/argon2id 密码哈希。
- 增加更完整的访问日志和审计日志。
- 增加可选登出接口。
- 增加 systemd、Docker 和反向代理部署示例。
- 视需要支持响应内容重写，解决部分后端应用不支持 base path 的问题。

## 安全建议

- 生产环境不要长期使用明文 `password`，优先用 `password_env`，后续计划支持 bcrypt/argon2id 哈希。
- `jwt.secret` 不要提交到公开仓库，可通过环境变量或独立私有配置注入。
- 若公网使用，建议放在 HTTPS 后面，并开启 `cookie_secure`。
- 本项目只适合作为轻量认证层，不替代完整的身份系统、权限系统或零信任网关。
