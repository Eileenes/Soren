# 部署和使用 Bot 模板分享

Soren 现在支持「创建 Bot 模板 → 分享 HTTPS 链接 → 导入自己的 Soren」。服务只存储配置及发布版本；聊天、模型连接、工具凭据和私人记忆继续留在各自的应用中。服务无需模型 API Key。

## 1. 使用流程

1. 管理员部署下面的独立模板服务，将服务地址告诉使用者。
2. 每个人在「设置 → Bot 模板服务」填写同一个根地址，例如 `https://templates.example.com`。只导入的人无需发布凭据。
3. 发布者另外填写管理员给自己的发布凭据，点击「保存模板服务」，可用「验证发布身份」核对名称。凭据存储在系统钥匙串中，不写入浏览器缓存，也不返回给界面。
4. 在侧栏右键 Bot，选择「复制 Bot」，即可创建可分享的模板；「我的 Bot」列表的「复制 Bot」使用相同流程。也可点击页面顶部「创建 Bot 模板」选择 Bot，或使用「配置 Bot」底部的模板入口。预览完整配置，填写模板名称、简介，确认发布后显示头像、名称和链接卡片，点击链接右侧图标复制。「复制 Bot」不会在自己的列表里直接新建本地副本。
5. 接收者打开链接，网页展示相同的 Bot 卡片并尝试唤起 Soren。浏览器可能要求允许打开应用，或需点击「在 Soren 中打开」。Soren 收到链接后自动加载头像、名称和链接卡片，点击「导入到我的 Soren」才会创建 Bot。唤起失败时，在「我的 Bot → 导入模板」粘贴 HTTPS 链接。
6. 导入得到独立副本，使用接收者的默认模型，初始对话与记忆为空。协作及所有工具动作默认禁用；先在「应用连接」将本机连接分配给该 Bot，再在「配置 Bot → 工具与协作」开启需要的动作。

首版使用「持有链接即可查看和导入」的访问模式，没有公共目录或搜索。链接可被转发。发布凭据仅用于发布、列出及管理自己的模板，不应共享给其他发布者。这是独立的模板发布身份，已有的 OAuth 配置功能仍未接入真实登录。

## 2. Docker Compose 部署

准备一台安装 Docker Compose 的服务器，将域名解析到这台服务器，并开放 80/443 端口。以下命令在仓库根目录执行。

```sh
cp deploy/templates/.env.example deploy/templates/.env
cp deploy/templates/publishers.example.json deploy/templates/publishers.json
```

编辑 `.env`，把 `SOREN_TEMPLATE_DOMAIN` 改为实际域名（只填域名，不带 `https://`）。

```sh
docker compose --project-directory deploy/templates build templates
docker compose --project-directory deploy/templates run --rm --no-deps templates credential alice 张三
```

命令会输出一个发布者 JSON 对象及一条随机发布凭据。把 JSON 对象放入 `deploy/templates/publishers.json` 数组，把凭据交给该发布者。为每位发布者单独运行一次，使用不同 ID，并将所有对象放入同一个数组。文件内容形如：

```json
[
  { "id": "alice", "name": "张三", "tokenHash": "命令生成的64位SHA256值" },
  { "id": "bob", "name": "李四", "tokenHash": "另一条凭据对应的SHA256值" }
]
```

不要原样使用占位值。空数组、重复 ID 或无效哈希会阻止服务启动。`publishers.json` 中只保存凭据哈希，不保存原始凭据；该文件及 `.env` 已加入 Git 忽略。

```sh
docker compose --project-directory deploy/templates up -d
curl https://你的模板域名/health
```

健康接口返回 `{"status":"ok"}` 后，在 Soren 中配置该 HTTPS 根地址。Caddy 自动管理 TLS 证书；SQLite 数据保存在 `template_data` Docker 卷中。服务内部端口 8090 不直接映射到主机。

如果服务器已有占用 80/443 的反向代理，复用现有代理并将模板域名转发到 `templates:8090`，不要同时启动两个监听同一端口的 Caddy。若与已有 Soren relay 共同部署，应将它们接入共同的 Docker 网络，并在同一个边缘代理中添加模板站点。

## 3. 不使用 Docker / 本地开发

需要仓库规定版本的 Go。在仓库根目录构建独立二进制：

```sh
mkdir -p .soren/template-dev
cd backend
go build -o ../.soren/template-dev/soren-templates ./cmd/templates
cd ..
.soren/template-dev/soren-templates credential alice 本机测试
```

按命令输出建立 `.soren/template-dev/publishers.json`，然后启动：

```sh
TEMPLATE_PUBLIC_URL=http://127.0.0.1:8090 \
TEMPLATE_LISTEN=127.0.0.1:8090 \
TEMPLATE_DATABASE=.soren/template-dev/templates.db \
TEMPLATE_PUBLISHERS_FILE=.soren/template-dev/publishers.json \
.soren/template-dev/soren-templates
```

Soren 设置中填写 `http://127.0.0.1:8090`。HTTP 仅允许回环地址，跨电脑使用应配置真实 HTTPS 域名。发布者 JSON 每次启动读取，修改后需要重启服务。

| 环境变量 | 用途 / 默认值 |
| --- | --- |
| `TEMPLATE_PUBLIC_URL` | 必填，对外 HTTPS 根地址；本机调试可用 HTTP 回环地址 |
| `TEMPLATE_LISTEN` | 监听地址，默认 `127.0.0.1:8090`；容器为 `0.0.0.0:8090` |
| `TEMPLATE_DATABASE` | 默认 `data/templates.db`；容器为 `/data/templates.db` |
| `TEMPLATE_PUBLISHERS_FILE` | 默认 `publishers.json`；容器为 `/config/publishers.json` |

## 4. 修改、停用与删除

- 职责与边界在「配置 Bot」中编辑，每条可移除；保存生成新的本地版本。单聊、任务及群聊共用这些规则。版本记录可预览并恢复职责，恢复不会重新开放已撤销的工具或连接授权。
- 工具逐项选择「禁用」「每次确认」「允许」。应用对发邮件、付费生成等操作的固定审批仍然有效。未配置在策略内的新动作默认拒绝；协作同时受发起者和执行者的策略限制。
- 「我的 Bot → 我的模板」可以复制链接、用本地 Bot 配置发布新版、停用/启用及永久删除模板。停用作用于该模板所有版本。
- 每个发布版本不可变，链接固定指向对应版本。修改本地 Bot 不会改变旧模板，发布新版也不会改变旧链接。接收者副本不自动更新。
- 停用或删除模板后，服务拒绝后续下载；已经下载或导入的内容无法收回。本地 Bot 的归档不会自动停用模板，需在「我的模板」中单独操作。
- 本地 Bot 仍使用可恢复的归档；本次未新增永久清除本地 Bot 私有数据的入口。模板删除与本地数据清理是两个独立操作。

## 5. 运维与边界

发布凭据遗失时，为同一个发布者 ID 重新生成凭据并替换哈希，然后重启服务；模板归属不变，旧凭据立即失效。删除发布者配置会撤销其管理权限，但不会自动停用该发布者已发布的模板。如需停用内容，应先停用模板再撤销凭据。

服务是单实例 SQLite 部署，不要为同一数据库启动多个写实例。升级前停止模板服务并备份数据卷；同时保留发布者配置，以免丢失模板管理身份。桌面数据库与模板服务数据库应分开备份。

模板最大 256 KB，格式版本为 1。只打包显式白名单字段；正文可能包含作者自己输入的敏感内容，因此发布前必须人工预览。接收者始终从设置中的固定服务下载，不接受分享链接指定任意服务器；下载不携带发布凭据，不跟随 HTTP 重定向，也不会执行模板文本、安装插件或自动授予权限。

原生链接形如 `soren://templates/import?id=…&revision=…&service=…`，其中 service 只用于与已配置地址核对，不能改变下载服务器。macOS 应使用打包后的 `.app` 注册协议；Linux/Windows 接入了协议注册与单实例处理，需要在目标平台发布包中另行验证。

组织内保密分享、OAuth 发布登录、公开 Bot 市场和自动合并模板更新不在首版中。如果需要组织内访问控制，应同时给网页和模板内容 API 增加认证，不能只隐藏链接。
