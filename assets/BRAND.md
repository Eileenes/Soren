# Soren 品牌资源

Soren 采用黑、白、灰的视觉语言。头像保留圆润轮廓、小耳朵、短脚和眨眼笑脸；黑色轮廓与五官搭配灰白面部，使用轻微灰阶渐变表现体积。

- `logo.svg` / `icon.svg` / `logo-dark.svg`：透明底的完整角色，用于界面、头像与网页。
- `icon-mono.svg`：使用 `currentColor` 的单色轮廓，用于菜单栏等小尺寸场景。
- `icon-1024.png`：透明底的 1024 × 1024 角色。
- `app-icon.svg` / `app-icon-1024.png`：正式应用图标。一体式圆角轮廓内，顶部为左厚右薄的黑色弧面，与灰白脸面自然衔接；保留 Soren 的黑色圆眼、眨眼与微笑，通过中性灰阶光影表现体积。
- `app-icon-preview.png`：主图及 256 / 128 / 64 / 32 / 16px 多尺寸预览。
- `frontend/public/`：网页头像、favicon、PWA 与 Apple Touch 图标。
- `frontend/src-tauri/icons/`：桌面应用 PNG、ICNS、ICO。
- `mobile/src-tauri/icons/`：移动端与 Android 启动图标。iOS 图标随 Tauri 图标命令同步生成。

macOS ICNS 保留系统应用常见的透明边距：1024px 画布中的主体为 824px，四周各留 100px。其他尺寸按比例缩放。

## 色彩与使用

轮廓与五官为 `#111111`；面部为 `#FAFAFA`、`#EEEEEE`、`#DEDEDE`。应用图标底板为 `#FFFFFF`。界面以白色为默认背景，使用黑色主操作、灰色文字和浅灰选中态。深色主题采用中性灰阶。

头像展示完整角色，应用图标展示与外壳融为一体的眨眼笑脸；两者都不叠加彩色底板。其他 Bot 的角色颜色保留，用于区分身份。外部服务图标保留其品牌识别。

## 重新生成

SVG 是原始文件。应用图标用 SVG 渐变、蒙版和柔和投影表现立体感。统一通过 Chrome 渲染后再生成各端资源，保证不同平台使用相同的光影；不要用忽略 SVG 滤镜的渲染器重新导出应用图标。

需要前端 npm 依赖、Chrome，以及 Python Pillow：

```sh
node scripts/render-app-icon.mjs
python3 scripts/gen-icons.py assets/app-icon-1024.png frontend/src-tauri/icons
npm --prefix mobile run tauri -- icon ../assets/app-icon-1024.png -o src-tauri/icons --ios-color '#FFFFFF'
```

`render-app-icon.mjs` 同步网页 SVG、Apple Touch 与 PWA PNG，并更新多尺寸图标预览和品牌预览。透明底角色头像仍可用 CairoSVG 单独导出，不影响应用图标的立体表面。

同步 `assets/logo.svg` 到 `assets/icon.svg`、`assets/logo-dark.svg`、`frontend/public/logo.svg`、`frontend/public/soren-mark.svg`。调整头像面部渐变时，同时更新 `BotAvatar.tsx` 的眨眼遮罩渐变，防止动画时出现色块。
