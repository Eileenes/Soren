#!/usr/bin/env python3
"""
从 SVG 或正方形 PNG 源图生成 Tauri 打包所需的完整图标集。
生成 .icns (macOS) 和 .ico (Windows) 使用纯 Python 构造，
不依赖 Xcode 的 iconutil 或 ImageMagick 的 icotool。

用法：python3 scripts/gen-icons.py assets/app-icon-1024.png frontend/src-tauri/icons
SVG 输入额外需要 cairosvg；已提交的 1024px PNG 只需要 Pillow。
"""
import struct
import sys
from io import BytesIO
from pathlib import Path

from PIL import Image

if len(sys.argv) != 3:
    raise SystemExit("用法：gen-icons.py <source.svg|source.png> <output-directory>")

SRC = Path(sys.argv[1])
OUT = Path(sys.argv[2])
OUT.mkdir(parents=True, exist_ok=True)

if SRC.suffix.lower() == ".svg":
    try:
        import cairosvg
    except ImportError as exc:
        raise SystemExit("SVG 输入需要 cairosvg；也可以使用 assets/app-icon-1024.png。") from exc
    source_bytes = cairosvg.svg2png(url=str(SRC), output_width=1024, output_height=1024)
    src = Image.open(BytesIO(source_bytes)).convert("RGBA")
else:
    src = Image.open(SRC).convert("RGBA")

if src.width != src.height:
    raise SystemExit("图标源文件必须为正方形，避免缩放后变形。")


def png_bytes(size: int) -> bytes:
    """把源图缩放为指定尺寸并编码为 PNG 字节。"""
    buf = BytesIO()
    src.resize((size, size), Image.LANCZOS).save(buf, format="PNG")
    return buf.getvalue()


def write_icns(path: Path) -> None:
    """
    构造 macOS .icns 容器。
    格式：魔数 'icns' + 总长度(大端 u32)，随后是若干
    (4字节类型 + 大端 u32 块长度 + 数据) 的块，块长度包含自身 8 字节头。
    使用现代 PNG 类型而非旧式 raw 位图。
    """
    entries = [
        (b"ic07", 128),   # 128x128
        (b"ic08", 256),   # 256x256
        (b"ic09", 512),   # 512x512
        (b"ic10", 1024),  # 512x512@2x
        (b"ic11", 32),    # 16x16@2x
        (b"ic12", 64),    # 32x32@2x
        (b"ic13", 256),   # 128x128@2x
        (b"ic14", 512),   # 256x256@2x
    ]
    body = b""
    for tag, size in entries:
        payload = png_bytes(size)
        body += tag + struct.pack(">I", len(payload) + 8) + payload
    path.write_bytes(b"icns" + struct.pack(">I", len(body) + 8) + body)


def write_ico(path: Path) -> None:
    """多尺寸 Windows .ico。PIL 直接支持。"""
    src.save(
        path,
        format="ICO",
        sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )


for name, size in [
    ("32x32.png", 32),
    ("128x128.png", 128),
    ("128x128@2x.png", 256),
    ("icon.png", 512),
]:
    src.resize((size, size), Image.LANCZOS).save(OUT / name, format="PNG")
    print("  %s (%dx%d)" % (name, size, size))

for name, size in [
    ("Square30x30Logo.png", 30),
    ("Square44x44Logo.png", 44),
    ("Square71x71Logo.png", 71),
    ("Square89x89Logo.png", 89),
    ("Square107x107Logo.png", 107),
    ("Square142x142Logo.png", 142),
    ("Square150x150Logo.png", 150),
    ("Square284x284Logo.png", 284),
    ("Square310x310Logo.png", 310),
    ("StoreLogo.png", 50),
]:
    src.resize((size, size), Image.LANCZOS).save(OUT / name, format="PNG")

write_icns(OUT / "icon.icns")
print("  icon.icns")
write_ico(OUT / "icon.ico")
print("  icon.ico")
print("完成。")
