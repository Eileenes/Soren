# Soren 公共连接服务部署

这是产品方的部署步骤。普通用户只安装 Soren、配置模型并扫码，不填写服务器地址，不需要 GitHub 登录或额外组网工具。未部署公共服务不影响 Mac 本地使用。

本轮已实现配对协议、独立中继、Mac 网关与 iOS 工程，详情见 `docs/MOBILE-040-DELIVERY.md`。此目录没有默认公网服务或预填密钥。

## 本地协议验证

分别运行三个终端（在项目根目录）：

```sh
cd backend
RELAY_LISTEN=127.0.0.1:8090 RELAY_ALLOWED_ORIGINS=http://localhost:1420,http://127.0.0.1:1420 go run ./cmd/relay
```

```sh
cd backend
SOREN_DATA_DIR=/tmp/soren-mobile-dev SERVER_PORT=8088 SOREN_MOBILE_DEV=1 SOREN_RELAY_URL=http://127.0.0.1:8090 SOREN_MOBILE_URL=http://localhost:1420 go run ./cmd/server
```

```sh
VITE_API_BASE_URL=http://127.0.0.1:8088 npm --prefix frontend run dev
```

打开 `http://localhost:1420` 的设置 → 关联设备。开启连接，首次最长约 25 秒就绪，然后生成二维码。用另一个浏览器窗口打开配对链接，在 Mac 页面核对确认码并批准。手机网页测试端可提交任务，Mac 页面可查看同一个任务。本例 localhost 仅适用于开发机浏览器/模拟器，不能当作真机可访问地址。

## 产品方公网部署

准备两个域名，解析到可运行 Docker Compose 的主机：一个中继域名，一个移动网页入口。开放 80/443 供 Caddy 提供 HTTPS。不要开放桌面 Go 的本地 8080 端口。

1. 将 `deploy/relay/.env.example` 复制为同目录 `.env`，填写实际域名。
2. 构建与域名匹配的前端：

```sh
VITE_SOREN_RELAY_URL=https://relay.example.com npm --prefix frontend run build
```

3. 在产品方服务器项目目录执行：

```sh
docker compose --env-file deploy/relay/.env -f deploy/relay/compose.yaml up -d --build
```

配置提供单实例中继和 Caddy，普通用户不执行这条命令。部署后检查 HTTPS `/healthz`，并验证跨网络真实设备链路；不要仅根据健康检查就宣称手机联动验收通过。

4. 为用户构建内置公共地址的 Mac 安装包：

```sh
SOREN_RELEASE_RELAY_URL=https://relay.example.com SOREN_RELEASE_MOBILE_URL=https://mobile.example.com VITE_SOREN_RELAY_URL=https://relay.example.com npm run build:mac
```

5. 构建 iOS：

```sh
npm --prefix mobile run ios:init
VITE_SOREN_RELAY_URL=https://relay.example.com npm --prefix mobile run ios:build -- --export-method release-testing
```

需要配置 Apple 开发团队及签名。模拟器构建用 `--target aarch64-sim --archive-only --ci`，不能作为真机安装包分发。保留 Xcode 模拟器签名流程及 `project.yml` 中的 Keychain 权限；`--no-sign` 会使安全存储不可用。`example.com` 为文档占位地址，必须替换。发行移动客户端固定允许产品方的中继来源，不接受二维码注入任意远程服务。

## 运维与安全边界

- 采用 HTTPS 长轮询，反向代理需允许至少 85 秒响应等待。中继不落盘业务正文。Caddy 默认未开启访问日志，不要另行记录请求体或认证头。
- 主节点认证令牌保存在 Mac，不在二维码中。它的 SHA-256 是路由 ID；仅持有路由 ID 不能冒充 Mac 的中继连接。
- 中继对请求体、并发和队列限额；公共运营仍需边缘 IP 限流、容量监控与滥用处理。主节点注册不依赖账号，因此不能用“没有用户注册入口”替代抗滥用措施。
- 单实例内存路由，重启不影响设备身份，但在途请求可能失败。不要直接加随机多副本；扩容需主节点路由分片或共享连接目录。
- 公共 Web 入口属于可信代码分发面。正式客户端使用签名内置代码；不能把“传输加密”当作能抵御恶意网页代码更新。
- Mac 下线或超时时返回离线/待核实，禁止云端冒充任务完成。长期失去 Mac 且没有本地备份时不能靠中继恢复数据。
