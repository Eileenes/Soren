# IngressHub 内部部署：项目侧准备

适用于公司桌面端连接企业服务；个人本地空间保持原有行为。平台契约参考 `ops/ingresshub` main `3abbdfede8ff51d51949d769caedf0924abc40f0`（2026-10-15）。本文件与示例属于 Soren 业务仓库，不会自动部署，也不会修改平台仓库。

## 登录方式

设置 `ENTERPRISE_AUTH_MODE=ingresshub`。不需要申请独立 OAuth Client，也不需要设置 OAuth Client Secret、授权/Token/UserInfo 端点。缺省 `oauth` 继续支持独立部署。

流程：

1. 桌面用 `/.well-known/soren` 识别服务，`POST /auth/attempts` 领取五分钟有效的浏览器链接和独立轮询凭据。
2. 系统浏览器打开 `/auth/browser?ticket=...`，由平台执行公司 SSO。
3. 企业服务验证 `X-IngressHub-Profile` 的 Ed25519 签名、`iss`、`aud`、`iat`、`exp`、可选 `nbf` 和稳定 `sub`，检查试点名单。
4. 页面显示公司、当前账号及确认说明。员工点击确认后，`POST /auth/confirm` 再次验证平台身份、同源 Origin、浏览器 Cookie、一次性 state、相同用户及有效期，签发 Soren 会话。
5. 桌面凭独立轮询密钥取回会话，沿用现有账号确认和系统凭据库存储。业务请求只认可撤销的 Soren Bearer Token，不把平台 JWT 当成桌面会话，不转存平台 Cookie。

服务发现中的 `auth: oauth-browser` 保留为现有桌面协议标识，表示浏览器授权和桌面轮询流程；实际公司身份来源由服务端 `ENTERPRISE_AUTH_MODE` 决定，现有支持企业空间的客户端可兼容。

JWT 的签发者（issuer）固定为 `https://sso.app.chaitin.net`，JWKS 从同一网关的内网入口 `https://sso.app.in.chaitin.net/.well-known/jwks.json` 获取，避免集群访问公网 ALB 超时。内网取钥不改变 issuer，不接受内网域名作为新的签发者。目标 audience 从 `ENTERPRISE_PUBLIC_URL` 的 host 取得，不信任客户端提供的 Host/转发头。公钥缓存五分钟；未知 kid 可刷新、刷新请求有十秒间隔；失败后可恢复，过期缓存不放行。集群必须能访问内网 JWKS 地址，TLS 验证和 JWT 签名校验始终开启。

`ENTERPRISE_ALLOWED_SUBJECTS` 继续按稳定的 `sub` 配置试点名单。`*` 表示允许平台认证的所有员工，只有确定开放全员时才填写。平台只做身份认证；Soren 负责成员停用、会话、数据隔离。现有会话最长八小时，离职/停用未自动同步，管理员须执行 Soren 的 `disable` 命令撤销会话。

OAuth 与 IngressHub 使用不同身份来源命名空间。若已有 OAuth 企业数据，切换模式不会按姓名或邮箱自动合并，也不会承认旧来源会话；应先制定稳定用户 ID 映射、备份和迁移方案。新试点无需迁移。

## 平台路由契约（下一阶段提交）

示例：`deploy/enterprise/ingresshub.example.yaml`。它只供后续准备平台 MR，必须替换 owner、镜像、应用名、域名、模型及试点名单，并以当时的最新 IngressHub schema 校验。

| 路径 | 平台 SSO | Soren 保护 |
| --- | --- | --- |
| `/auth/browser`、`/auth/confirm` | 必须开启，禁止豁免 | 已验证平台 JWT、账号名单；确认还有 Cookie/state/Origin/有效期校验 |
| `/.well-known/soren` | 豁免 | 只返回非敏感服务信息 |
| `/auth/attempts` | 豁免 | 创建短期尝试，限流及数量上限，不签发业务会话 |
| `/auth/poll` | 豁免 | 五分钟独立轮询密钥、账号状态校验 |
| `/api/*` | 豁免 | 所有业务请求强制验证 Soren Bearer 会话及成员状态 |
| `/healthz` | 可豁免供外部运维检查 | 仅数据库存活状态，不返回配置或凭据 |

豁免仅跳过平台的浏览器 Cookie 登录，不跳过 Soren 认证。不要添加 `/*` 或 `/auth/*` 豁免。不要把此配置移入 `apps/internet`，平台禁止公网 authExempt；远程员工通过公司认可的内网接入方式连接。

## 镜像与数据

- 企业服务入口为 `backend/cmd/enterprise`，监听 `0.0.0.0:8091`。平台直接转发到业务进程，TLS 由平台负责，不部署 Compose 中的 Caddy 和 PostgreSQL。
- `databases: [{type: postgres, name: soren}]` 让平台注入 `DATABASE_URL`，业务进程无需改数据库连接逻辑。平台 PostgreSQL 16 需支持本库建 schema（平台创建的账号是库 owner）。
- 声明 `/data` 持久卷，保存员工文件。数据库备份必须覆盖全部 `workspace_*` schema，文件与凭据主密钥分别备份。
- **持久卷初始化**：镜像入口以 root 创建数据目录，仅对该目录执行 `chmod u+rwx` 和 `chown 10001:10001`，再通过 `su-exec` 切换为 UID/GID 10001 执行业务进程。新 PVC 即使为 root:root 也可自助初始化；不递归修改已有员工文件或 `lost+found`，业务进程不以 root 运行。若部署方强制非 root，必须提前准备可写卷；入口检查失败会明确报错。不要写平台 schema 不支持的 securityContext/initContainers。实际集群 PVC 权限仍需在部署后验证。
- 固定 `replicas: 1`。现有 PostgreSQL advisory lock 防止多个企业进程同时执行；挂卷时平台渲染 `Recreate`，升级有短暂中断，不能承诺无损滚动更新。
- `/healthz` 检查数据库，平台同时作为就绪/存活探针使用；数据库故障可能触发重启，应观察恢复和任务状态，不能只凭探针成功判断上线验收完成。
- 默认模型并发 8；平台默认资源上限 1 CPU/1 GiB，需要试点观测后找运维调整。每员工的运行时和数据库池尚无全局空间回收机制，本次适配不等于 1200 人容量验收。

## 构建与推送的边界

在业务仓库根目录：

```sh
npm run build:enterprise -- portus.in.chaitin.net/<project>/soren-enterprise:<version>
```

脚本使用 buildx 构建并加载 `linux/amd64` 镜像，强制显式版本标签，不会推送镜像。Dockerfile 在构建平台交叉编译 Go，入口初始化卷后以非 root 运行业务。Compose 显式使用 10001:10001，保留只读根文件系统和禁用全部 capabilities，Docker 新命名卷继承镜像内 `/data` 的属主。不要把真实 `.env`、token、数据库文件或凭据加入镜像；`.dockerignore` 仅允许后端源码、依赖清单及企业入口脚本。

项目代码准备好后，先推送到 `ye.tao/soren-bot`。由实际发布人确认 Portus 项目并单独执行镜像 push，再到 `ops/ingresshub` 的 `apps/intranet/<owner>/` 提交 YAML。owner 必须是实际发布人的公司用户名；业务仓库名称不能代替身份确认。平台 MR 合并 main 会自动部署。

自定义 Secret 例如 `<app>-secrets`，保存 `ENTERPRISE_VAULT_KEY`、模型 Key 等。不要命名为 `<app>-db`，它由平台生成。数据库 URL 不要在 env 或自定义 Secret 中重复覆盖。首次平台发布会先创建 Secret 空壳，此时业务进程因缺少密钥拒绝启动是预期的；填写 Secret 并重建 Pod 后再验收。

## 项目侧验证与上线验收

镜像卷权限回归（先构建镜像，脚本仅创建并清理临时 Docker 卷）：

```sh
bash scripts/test-enterprise-volume.sh portus.in.chaitin.net/<project>/soren-enterprise:<version>
```

覆盖 root:root 新卷初始化、业务命令 UID/GID 10001 且无有效 capabilities、已有目录权限保留、重启后文件保留、显式非 root 启动，以及只读卷拒绝启动。

本地自动化：

```sh
cd backend
go test -race ./...
go vet ./...
# 对专用临时 PostgreSQL 16 数据库运行，不可指向现有业务库：
SOREN_TEST_POSTGRES_URL='postgres://.../disposable_db?sslmode=disable' go test -race ./internal/enterprise ./internal/enterpriseclient
```

测试包含平台身份验签、错误 issuer/audience、过期/伪造凭据、JWKS 故障恢复与换钥、浏览器确认/CSRF/重放、双员工数据隔离、停用与退出，以及模拟平台 SSO → 真实 PostgreSQL → 桌面代理的完整流程。真实数据库测试在未提供专用 DSN 时跳过。

平台上线阶段仍需确认：真实 SSO/JWKS 可达、PVC 权限、Secret 注入、域名路由和 authExempt、两名真实员工、真实模型与连接器、系统凭据库、签名客户端、任务重启恢复、备份恢复、监控告警和试点容量。模拟身份服务不能替代这些验收。

## 本次项目侧验证记录（2026-10-15）

- 在专用临时 PostgreSQL 16 下执行 Go 全量 `go test -race ./...`，全部通过，包含两种登录模式的桌面代理集成流程。
- `go vet ./...`、构建脚本语法检查、`git diff --check` 通过。
- 使用项目构建脚本成功生成 `linux/amd64` 镜像；以 UID/GID 10001、只读根文件系统、无 Linux capabilities 启动，健康检查/服务发现成功，匿名业务请求被拒绝。测试模型凭据均为虚构值，未调用真实模型或向 Portus 推送。
