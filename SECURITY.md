# 安全与开发约定

## 凭据与提交

- 模型凭据保存在当前系统用户的 `~/.soren/provider.json`，权限为 `0600`；连接器凭据保存在系统 Keychain。手机设备私钥保存在 iOS Keychain，Mac 配对身份文件也为 `0600`。
- `.env`、数据库、日志、设备身份、私钥、签名文件、预览截图、生成工程和安装包不提交 Git。配置示例仅放占位符。
- 不要在 `VITE_*` 中存放任何秘密，这些变量会进入客户端代码。公开中继域名可以注入，模型 Key 不可以。
- 推送前检查暂存文件，并运行 Gitleaks。依赖锁文件必须随代码一起提交。

```sh
git diff --cached --check
gitleaks git --redact
cd backend && go test -race ./... && go vet ./...
# 在项目根目录执行：
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
npm --prefix frontend audit
cargo audit --file frontend/src-tauri/Cargo.lock
cargo audit --file mobile/src-tauri/Cargo.lock
```

## 运行边界

本地 API 仅接受回环地址和受信任应用来源；Bot ID 用于隔离资料，不是公网身份认证。不要直接暴露桌面 API。

手机通过加密中继联动，必须扫描临时配对信息并在 Mac 上确认。配对后的手机具有当前本地空间的任务和记忆访问权限。撤销设备后阻止新请求，已经接受的任务仍需单独取消。

模型和连接器 HTTP 客户端拒绝凭据请求重定向。远程 MCP 仅接受 HTTPS，拒绝私网地址；工具执行受 Bot 授权和操作确认约束。飞书、钉钉的 Webhook 模式必须配置校验凭据。

本地记录不等于磁盘加密。应启用系统磁盘加密并保护操作系统账号。当前移动链路尚未完成真机与公网生产验收。

发现问题时请私下联系仓库维护者，不要在公开 Issue 中贴 Key、配对链接、数据库或完整个人日志。
