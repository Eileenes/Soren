# 本地构建产物

最新 macOS Apple Silicon 包：`Soren-0.4.1-macOS-arm64.dmg`，校验文件 `SHA256SUMS-0.4.1.txt`。

安装包不随 Git 推送。克隆仓库后按根目录 README 构建，产物在 `frontend/src-tauri/target/release/bundle/`。0.4.1 已完成本地签名和 DMG 完整性验证，尚未经过 Apple 公证。

iOS 历史模拟器包只用于开发验证，不能安装到真实 iPhone。正式手机分发仍需开发者签名和公共中继服务。
