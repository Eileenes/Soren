# 发布 Soren

GitHub [Releases](https://github.com/Eileenes/Soren/releases) 提供安装包和 SHA-256 校验文件。当前发布平台为 macOS Apple Silicon，版本为开发预览版；本地签名，尚未 Apple 公证。iOS 通过 [TestFlight / App Store](IOS-DISTRIBUTION.md) 单独分发。

## 自动发布

1. 同步版本号：根目录、frontend、mobile 的 `package.json`，frontend 的锁文件，两个 Tauri 配置、Cargo.toml 和对应 Cargo.lock 中 Soren 自身版本。不要修改依赖版本。
2. 添加 `docs/releases/v版本号.md`，写明功能、限制和安装说明。
3. 执行 `npm run release:check`，提交并推送代码。
4. 创建并推送版本标签，例如下一个版本为 `0.4.3` 时：

```sh
git tag v0.4.3
git push origin v0.4.3
```

`.github/workflows/release.yml` 将检查版本和标签、扫描历史密钥、执行前端/Go/Rust 检查，构建 arm64 DMG，验证签名和磁盘镜像，生成校验文件，上传到草稿 Release 后再公开并设为 Latest，让仓库首页显示下载版本。安装包不进入 Git 历史。失败时不会发布不完整版本；已发布版本不会覆盖，修复后请递增版本号。

也可以在 Actions → Release macOS → Run workflow 填写已存在的标签重试失败的发布。保持标签不可变，不要把标签移动到另一份源码。

## 可选公开配置

Settings → Secrets and variables → Actions → Variables 中配置 `SOREN_RELEASE_RELAY_URL`、`SOREN_RELEASE_MOBILE_URL` 和 `SOREN_IOS_DOWNLOAD_URL`。全部留空可发布 Mac 本地版；填写 iOS 下载地址前必须上线连接服务。详情见 [iOS 分发说明](IOS-DISTRIBUTION.md)。

此工作流不需要用户的模型 API Key、Gmail/Notion 凭据或个人数据。Apple 签名及公证自动化尚未配置，当前不要向这个工作流注入 Apple 私钥。

## 本地构建与手动发布

```sh
npm --prefix frontend ci
npm run release:check
npm run build:mac
npm run release:package
```

生成 `release/Soren-版本号-macOS-arm64.dmg` 和 `release/SHA256SUMS-版本号.txt`。手动发布时同样先创建指向已验证源码的标签，上传到草稿并核对资源完整后再公开。不要从开发目录拷贝 `.env`、数据库、用户配置或证书到 Release。

GitHub Release 使用普通发布并设为 Latest，首页侧栏和 `/releases/latest` 均可找到当前下载版本。产品仍处于开发预览阶段，发布说明继续注明签名、公证及 iOS 限制；Latest 仅表示当前推荐下载版本。
