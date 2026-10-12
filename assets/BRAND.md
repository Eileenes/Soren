# Soren 品牌资源

Soren 的角色是一只会眨眼的薄荷色小伙伴：圆润小耳朵、两只短脚、微微上扬的笑脸，以及额头的一点光。轮廓与五官均为原创 SVG 路径，可自由缩放。

- `logo.svg` / `icon.svg`：透明背景的彩色角色，用于界面、头像、网页。
- `logo-dark.svg`：适用于深色表面的较亮轮廓版本。
- `icon-mono.svg`：使用 `currentColor` 的单色轮廓，眼睛与笑脸镂空，用于菜单栏等场景。
- `icon-1024.png`：透明背景的 1024 × 1024 角色。
- `app-icon.svg` / `app-icon-1024.png`：薄荷渐变脸部铺满的应用图标，无灰色底板和额外外边距；沿用眨眼、笑脸与额头星光。
- `frontend/src-tauri/icons/`：由应用图标源文件生成的 PNG、ICNS、ICO 包装资源。

## 颜色

深青绿 `#164C45` 是品牌主色；薄荷 `#B8EFDC`、青绿 `#86DCC3` 与奶油微金 `#E7EAB9` 形成面部渐变。浅色背景使用 `#FBFDF2`，深色界面背景使用 `#14181D`。

界面头像保留完整身体轮廓，应用图标使用大脸特写；头像无需重复加圆形底。主图标最小建议尺寸为 24px；16px 场景使用单色版本可获得更清晰的五官。

## 重新生成应用图标

```sh
python3 scripts/gen-icons.py assets/app-icon-1024.png frontend/src-tauri/icons
```

此命令需要 Pillow。若安装了 CairoSVG，也可直接把第一个参数改为 `assets/app-icon.svg`。
