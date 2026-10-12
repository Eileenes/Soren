# Soren iOS 下载与发布

[简体中文](../README.md) · [English](../README.en.md)

**当前状态：尚未开放 iPhone 下载。** 已有 iOS 工程和模拟器验证，尚未提供 TestFlight 邀请链接。Mac 的「设置 → 关联设备」会显示当前状态；可安装的 iOS 版本发布后，这里会提供安装按钮和二维码。

**Current status: iPhone downloads are not available yet.** The iOS project and simulator validation are in place. A signed build and public TestFlight invitation are still pending. The Mac app's Settings → Linked devices section supports an installation button and QR code once distribution is configured.

## 用户如何安装和关联

1. 在 Mac 打开「设置 → 关联设备」，用 **iPhone 相机**扫描安装二维码。
2. 根据页面安装 TestFlight，再安装 Soren；正式上架后也可直接从 App Store 安装。
3. 在 Mac 开启手机连接，点击「关联手机」，生成另一个**配对二维码**。
4. 打开 **Soren iOS App** 扫描配对码，核对两端确认码，在 Mac 允许关联。

安装码只包含 Apple 下载页面；配对码用于关联你的 Mac，五分钟有效，不应公开分享。Mac 需要保持联网、运行且未休眠。无需 Soren/GitHub 账号。

## 发布方：从开发者账号到 TestFlight

### 1. 准备公共连接服务

按[部署指南](../deploy/README.md)上线 HTTPS 中继和移动入口。先验证真实 iPhone 和 Mac 在不同网络下的配对、任务及审批，不要只检查服务健康接口。

### 2. 创建 Apple 应用并签名

- 在 Xcode → Settings → Accounts 登录自己的 Apple Developer 账号，选择具有签名权限的团队。
- 在 Apple Developer 与 App Store Connect 创建应用，Bundle ID 必须与 `mobile/src-tauri/tauri.conf.json` 中的 `com.soren.mobile` 一致。如果该 ID 不属于你的团队，先统一修改为自己持有的 ID。
- 准备当前 App Store Connect 要求的 Xcode 版本，初始化工程：

```sh
npm --prefix frontend ci
npm --prefix mobile run ios:init
```

- 在生成的 Xcode 工程 `mobile/src-tauri/gen/apple/soren-mobile.xcodeproj` 中选择团队，使用自动签名，并核对相机用途说明和 Keychain 权限。生成目录不会提交到 Git。
- 使用实际中继地址构建（替换占位地址），Apple 团队也可以通过 `APPLE_DEVELOPMENT_TEAM` 环境变量配置：

```sh
VITE_SOREN_RELAY_URL=https://relay.your-domain.com \
  npm --prefix mobile run ios:build -- --export-method app-store-connect
```

构建版本号来自 `mobile/src-tauri/tauri.conf.json`。同一版本重复上传时，在 Xcode/构建配置中提高 build number；不能重复上传同一个版本和 build number。

### 3. 上传并取得安装链接

通过 Xcode Organizer 或 Apple Transporter 上传签名构建，在 App Store Connect 等待处理。填写测试说明、联系方式和隐私信息，创建外部测试群组，添加构建并按 Apple 要求提交 TestFlight Beta App Review。审核通过且群组接受测试者后开启 Public Link，复制 `https://testflight.apple.com/join/…` 邀请地址。

签名身份、证书密码与 App Store Connect 私钥只保存在本机 Keychain 或发布系统 Secrets 中。不要发在聊天里、写入源码或 `VITE_*`。只有公开下载链接和服务地址是公开构建配置。

### 4. 接入桌面入口

GitHub 仓库 → Settings → Secrets and variables → Actions → **Variables** 设置：

| Variable | 内容 |
| --- | --- |
| `SOREN_IOS_DOWNLOAD_URL` | 已可用的 TestFlight 公开邀请链接或 App Store 应用链接 |
| `SOREN_RELEASE_RELAY_URL` | 已部署的 HTTPS 中继 origin |
| `SOREN_RELEASE_MOBILE_URL` | 已部署的 HTTPS 移动入口 origin |

随后发布新的 Mac 版本。工作流会把公开链接注入桌面，入口自动显示安装按钮和二维码。当前采用构建时配置，已经安装的旧版本需要升级，不会远程静默修改下载链接。未配置链接时不显示安装二维码，错误链接会让发布检查失败。

本机构建时使用：

```sh
export SOREN_RELEASE_RELAY_URL=https://relay.your-domain.com
export SOREN_RELEASE_MOBILE_URL=https://mobile.your-domain.com
export VITE_SOREN_RELAY_URL="$SOREN_RELEASE_RELAY_URL"
# 将下面的值替换为审核通过、实际可用的公开邀请链接。
export VITE_SOREN_IOS_DOWNLOAD_URL=https://testflight.apple.com/join/YOURCODE
npm run release:check
npm run build:mac
```

仓库工作流目前自动发布 **macOS**。iOS 上传、Apple 审核与打开 TestFlight 公共邀请仍由拥有开发者账号的发布者完成；不把模拟器 ZIP、unsigned IPA 或仅供登记设备的 Ad Hoc IPA 当作公开 iPhone 安装包。

## 发布验收

- 使用普通测试者的 iPhone 打开公开链接，可以接受邀请、安装并启动。
- 桌面安装按钮与安装二维码指向同一个 Apple 页面。
- 真机扫码后两端确认码一致，必须在 Mac 批准。
- 跨网络发起任务、查看结果及审批成功；Mac 离线时显示离线，不伪造成功。
- Mac 撤销设备后，该手机不能继续访问。

参考：[Apple TestFlight 外部测试](https://developer.apple.com/help/app-store-connect/test-a-beta-version/invite-external-testers/)、[Tauri App Store 分发](https://v2.tauri.app/distribute/app-store/)、[iOS 签名](https://v2.tauri.app/distribute/sign/ios/)。
