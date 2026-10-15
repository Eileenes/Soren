# 内部 OAuth 登录配置

企业登录已接入独立 Go 后端。员工只需在桌面端的「使用空间」填写公司服务器根地址，在系统浏览器完成授权，并确认账号后进入公司空间。个人本地使用无需登录。

管理员配置与部署步骤见 [企业部署基础版](ENTERPRISE-DEPLOYMENT.md)，环境变量示例见 [deploy/enterprise/.env.example](../deploy/enterprise/.env.example)。

## 公司身份服务

- 注册服务端 OAuth 应用，回调地址为 `https://公司服务域名/auth/callback`。
- 配置 Authorization、Token、UserInfo 三个可信 HTTPS 端点，以及 Client ID 和 Client Secret。
- 必须支持授权码流程及 PKCE S256；支持 `client_secret_basic` 或 `client_secret_post`。
- 身份来自可信 UserInfo 响应的稳定 ID。默认字段为 `sub`；公司文档约定 `id` 时设置 `ENTERPRISE_SUBJECT_CLAIM=id`。用户名和邮箱不能作为成员主键。
- 通过 `ENTERPRISE_ALLOWED_SUBJECTS` 限定试点成员；只有 IdP 应用已限制合法员工访问时才使用 `*`。
- 应用密钥只放在企业服务器。桌面登录令牌留在系统凭据库，前端不接收令牌。

真实 IdP 的 PKCE、响应结构及应用准入策略仍须部署时验证。未实现 IdP refresh token／SCIM 同步；已有会话的有效期为 8 小时，管理员可停用 Soren 成员并撤销会话。

## 旧本地配置的处理

桌面端旧的 OAuth 配置表单、配置服务及 `get_oauth_config`、`save_oauth_config` Tauri 命令均已移除。OAuth 应用参数只由管理员在企业服务器配置，员工使用「连接公司服务器」和「使用公司账号登录」入口。

旧版本可能留下的 `oauth.json` 不再被读取或写入，也不会自动上传或迁移到服务器。本次代码清理不删除用户已有的本地文件；确认不再需要旧配置后，可自行删除该文件。它仍受 Git 忽略规则保护。
