# 内部 OAuth2 IdP 配置

桌面端入口：设置 → 通用设置 → OAuth 登录配置。

当前功能负责校验、保存、读取与更新配置。保存成功表示本地配置可用，不表示凭据已获 IdP 验证，也不会发起登录、创建会话或限制本地空间访问。

## 配置项

| 字段 | 默认值或要求 |
| --- | --- |
| 使用环境 | 本地开发；可选择测试、生产。目前保存一组配置 |
| Authorization Endpoint | `https://auth.chaitin.net/oauth2/auth` |
| Token Endpoint | `https://auth.chaitin.net/oauth2/token` |
| UserInfo Endpoint | `https://auth.chaitin.net/userinfo` |
| Client ID | 从内部工单申请获得 |
| Client Secret | Basic / Post 必填；已有密钥可留空保留，输入新值可替换 |
| Redirect URI | 必填，与 IdP 注册地址完全一致；不预填尚未实现的回调 |
| Scope | `openid profile email`；必须包含 `openid`，使用空格分隔 |
| 客户端认证方式 | 默认 `client_secret_basic`；也支持 `client_secret_post`、`none` |

所有地址须使用 HTTPS。本地开发环境允许 `localhost`、`127.0.0.1`、`[::1]` 的 HTTP 地址；测试与生产环境仍要求 HTTPS。接口地址不能含查询参数，回调地址允许查询参数并保留原始拼写。地址均不允许账号、密码或片段。

更换环境、Client ID、任一 IdP 接口或认证方式，需要重新提供密钥。切换至 `none` 并保存，会移除之前保存的密钥。修改回调地址或 Scope 可以保留同一客户端的原密钥。保存前的更改可以撤销。

## 存储与接口

- Tauri 命令：`get_oauth_config`、`save_oauth_config`；不通过本地 HTTP 服务公开凭据配置接口。
- 存储位置：`SOREN_DATA_DIR/oauth.json`，未设置环境变量时为 `~/.soren/oauth.json`。
- 沿用桌面私有配置目录。macOS / Unix 目录权限为 `0700`，文件权限为 `0600`；这是本地文件存储，不是加密凭据库。
- 原子替换配置；校验或写入失败时保留原配置，损坏的旧配置不会被静默覆盖。
- 读取和保存响应仅包含 `settings`、`configured`、`hasClientSecret`，不会回传密钥。前端不将密钥写入 localStorage，保存成功后清空输入框。
- 运行时文件 `oauth.json*` 已加入 Git 忽略规则。测试只使用独立临时目录和虚构凭据。

保存请求：

```ts
invoke('save_oauth_config', {
  config: {
    settings: {
      environment: 'development',
      authorizationEndpoint: 'https://auth.chaitin.net/oauth2/auth',
      tokenEndpoint: 'https://auth.chaitin.net/oauth2/token',
      userInfoEndpoint: 'https://auth.chaitin.net/userinfo',
      clientId: '申请获得的 ID',
      redirectUri: '在 IdP 中注册的地址',
      scope: 'openid profile email',
      clientAuthMethod: 'client_secret_basic',
    },
    // 首次保存或轮换时提供；更新同一客户端时省略以保留。
    clientSecret: '由用户在界面填写',
  },
});
```

## 后续登录接入

当前尚未实现授权入口、回调处理、令牌交换、登录状态保持和退出登录。后续应复用成熟 OAuth 客户端库，使用每次独立且有过期时间的 state 并在回调校验，按注册的认证方式交换授权码，再调用 UserInfo。用户唯一标识使用文档约定的 `id`，不能使用可变的 `username` 或 `email`。令牌应保留在原生侧或服务端，不返回前端持久化。

文档未说明 PKCE、OIDC discovery 与 JWKS 的支持情况，后续接入真实登录时需要与 IdP 确认；当前配置保存不会据此假定端点或发出验证请求。对于桌面公开客户端，应确认 IdP 支持 PKCE 后再启用实际授权流程。
