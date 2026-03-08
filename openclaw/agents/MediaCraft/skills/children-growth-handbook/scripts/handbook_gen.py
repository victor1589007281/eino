#!/usr/bin/env python3
"""
儿童成长手册生成器
用法: python handbook_gen.py --name "小明" --birthday "2023-05-15" --photos ./photos/ --style forest_watercolor
"""

import argparse
import json
import os
import sys
from datetime import datetime
from pathlib import Path
from typing import List, Optional, Tuple

try:
    from PIL import Image, ImageDraw, ImageFont, ImageFilter
except ImportError:
    print("请先安装 Pillow: pip install Pillow")
    sys.exit(1)

try:
    from reportlab.lib.pagesizes import A4
    from reportlab.pdfgen import canvas as pdf_canvas
except ImportError:
    print("请先安装 reportlab: pip install reportlab")
    sys.exit(1)

try:
    from openai import OpenAI
except ImportError:
    print("请先安装 openai: pip install openai")
    sys.exit(1)


# ─── 常量 ──────────────────────────────────────────────

A4_W, A4_H = 2480, 3508  # 300 DPI
SCRIPT_DIR = Path(__file__).parent

STYLES = {
    "forest_watercolor": {
        "bg": "#FFF8F0", "title_color": "#5B4636", "text_color": "#6B5B4F",
        "tag_colors": ["#E8D5C4", "#D4E8D0", "#F5E6D3", "#D6E8E4", "#F0E0D0"],
        "personality": "森系水彩风格文案设计师。文案诗意温柔有画面感，使用自然元素装饰。",
    },
    "nordic_minimal": {
        "bg": "#FAFAFA", "title_color": "#2C2C2C", "text_color": "#555555",
        "tag_colors": ["#F0F0F0", "#E8E8E8", "#F5F5F5", "#EBEBEB", "#F2F2F2"],
        "personality": "北欧极简风格文案设计师。文案简洁有力，留白充足，几何元素点缀。",
    },
    "storybook": {
        "bg": "#FFF5E6", "title_color": "#8B5E3C", "text_color": "#7A6652",
        "tag_colors": ["#FFE4C4", "#E6F0FF", "#FFF0D4", "#E8FFE8", "#FFE8F0"],
        "personality": "绘本故事风格文案设计师。像讲故事一样，有童话感和想象力。",
    },
    "cute_cartoon": {
        "bg": "#FFF0F5", "title_color": "#FF6B8A", "text_color": "#666666",
        "tag_colors": ["#FFE0EB", "#E0F0FF", "#FFFACD", "#E8FFE8", "#F0E0FF"],
        "personality": "可爱卡通风格文案设计师。文案俏皮有趣充满活力，装饰圆润可爱。",
    },
    "vintage_journal": {
        "bg": "#F5F0E8", "title_color": "#6B5B4F", "text_color": "#7A6E62",
        "tag_colors": ["#E8E0D0", "#DDD8CC", "#E0D8C8", "#D8D0C0", "#E4DCD0"],
        "personality": "复古日记本风格文案设计师。文案文艺怀旧有仪式感，做旧纸张质感。",
    },
}

THEMES_DEFAULT = [
    "cover", "birth", "monthly", "monthly", "monthly",
    "milestone", "daily", "daily", "daily", "festival",
    "milestone", "daily", "daily", "travel", "daily",
    "birthday", "daily", "daily", "growth_data", "closing",
]


# ─── LLM 文案生成 ──────────────────────────────────────

def generate_caption(
    client: OpenAI,
    name: str, nickname: str, theme: str,
    style_key: str, page_num: int, total: int,
) -> dict:
    style = STYLES[style_key]
    prompt = f"""请为以下儿童成长手册页面生成内容：
- 孩子姓名：{name}，昵称：{nickname or name}
- 页面主题：{theme}（第 {page_num}/{total} 页）

以 JSON 输出：
{{"title":"页面标题（8字以内）","main_text":"主文案（50-100字，温馨有爱）","tags":["标签1","标签2","标签3"]}}"""

    try:
        resp = client.chat.completions.create(
            model="qwen3.5-plus",
            messages=[
                {"role": "system", "content": style["personality"]},
                {"role": "user", "content": prompt},
            ],
            temperature=0.8,
            max_tokens=300,
            response_format={"type": "json_object"},
        )
        return json.loads(resp.choices[0].message.content)
    except Exception as e:
        print(f"  ⚠️ LLM 调用失败: {e}，使用默认文案")
        return {
            "title": f"{name}的故事",
            "main_text": f"每一天都是新的冒险，{nickname or name}在慢慢长大。",
            "tags": ["成长", "快乐", "爱"],
        }


# ─── 图像合成 ──────────────────────────────────────────

def load_font(name: str, size: int) -> ImageFont.FreeTypeFont:
    candidates = [
        SCRIPT_DIR / "fonts" / name,
        Path("/usr/share/fonts/truetype/noto") / name,
        Path("/System/Library/Fonts") / name,
    ]
    for p in candidates:
        if p.exists():
            return ImageFont.truetype(str(p), size)
    return ImageFont.load_default()


def hex_to_rgb(h: str) -> Tuple[int, int, int]:
    h = h.lstrip("#")
    return tuple(int(h[i:i+2], 16) for i in (0, 2, 4))


def smart_crop(img: Image.Image, target: Tuple[int, int]) -> Image.Image:
    tw, th = target
    ratio_img = img.width / img.height
    ratio_tgt = tw / th
    if ratio_img > ratio_tgt:
        new_h = th
        new_w = int(new_h * ratio_img)
    else:
        new_w = tw
        new_h = int(new_w / ratio_img)
    img = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
    left = (new_w - tw) // 2
    top = (new_h - th) // 2
    return img.crop((left, top, left + tw, top + th))


def rounded_mask(size: Tuple[int, int], radius: int) -> Image.Image:
    mask = Image.new("L", size, 0)
    ImageDraw.Draw(mask).rounded_rectangle([(0, 0), size], radius=radius, fill=255)
    return mask


def wrap_text(text: str, font: ImageFont.FreeTypeFont, max_w: int) -> List[str]:
    lines, cur = [], ""
    for ch in text:
        test = cur + ch
        if font.getbbox(test)[2] > max_w:
            lines.append(cur)
            cur = ch
        else:
            cur = test
    if cur:
        lines.append(cur)
    return lines


def compose_page(
    caption: dict,
    photos: List[Path],
    style_key: str,
    output_path: Path,
) -> Path:
    style = STYLES[style_key]
    bg_color = hex_to_rgb(style["bg"])
    title_color = hex_to_rgb(style["title_color"])
    text_color = hex_to_rgb(style["text_color"])

    page = Image.new("RGB", (A4_W, A4_H), bg_color)
    draw = ImageDraw.Draw(page)

    font_title = load_font("NotoSansSC-Bold.ttf", 120)
    font_body = load_font("NotoSansSC-Regular.ttf", 48)
    font_tag = load_font("NotoSansSC-Medium.ttf", 36)

    # ── 标题 ──
    title = caption.get("title", "成长记录")
    bbox = draw.textbbox((0, 0), title, font=font_title)
    tx = (A4_W - (bbox[2] - bbox[0])) // 2
    draw.text((tx, 100), title, font=font_title, fill=title_color)

    # ── 照片区域 ──
    if photos:
        slots = _photo_slots(len(photos))
        for i, slot in enumerate(slots):
            if i >= len(photos):
                break
            try:
                photo = Image.open(photos[i]).convert("RGB")
                photo = smart_crop(photo, (slot["w"], slot["h"]))
                mask = rounded_mask((slot["w"], slot["h"]), 30)
                rgba = photo.convert("RGBA")
                rgba.putalpha(mask)
                page.paste(rgba, (slot["x"], slot["y"]), rgba)
            except Exception as e:
                print(f"  ⚠️ 照片加载失败 {photos[i]}: {e}")

    # ── 主文案 ──
    main_text = caption.get("main_text", "")
    lines = wrap_text(main_text, font_body, A4_W - 240)
    y = A4_H - 600
    for line in lines:
        draw.text((120, y), line, font=font_body, fill=text_color)
        y += 72

    # ── 标签 ──
    tags = caption.get("tags", [])
    tag_colors = style["tag_colors"]
    tx_pos = 120
    for i, tag in enumerate(tags[:5]):
        tc = hex_to_rgb(tag_colors[i % len(tag_colors)])
        bbox = draw.textbbox((0, 0), tag, font=font_tag)
        tw = bbox[2] - bbox[0] + 40
        draw.rounded_rectangle(
            [(tx_pos, A4_H - 200), (tx_pos + tw, A4_H - 140)],
            radius=15, fill=tc,
        )
        draw.text((tx_pos + 20, A4_H - 195), tag, font=font_tag, fill=text_color)
        tx_pos += tw + 20

    output_path.parent.mkdir(parents=True, exist_ok=True)
    page.save(str(output_path), "PNG", dpi=(300, 300))
    return output_path


def _photo_slots(count: int) -> List[dict]:
    center_x = A4_W // 2
    if count == 1:
        w, h = 1200, 900
        return [{"x": center_x - w // 2, "y": 350, "w": w, "h": h}]
    elif count == 2:
        w, h = 900, 700
        return [
            {"x": 100, "y": 350, "w": w, "h": h},
            {"x": A4_W - 100 - w, "y": 500, "w": w, "h": h},
        ]
    else:
        w, h = 700, 600
        return [
            {"x": 100, "y": 350, "w": w, "h": h},
            {"x": A4_W - 100 - w, "y": 350, "w": w, "h": h},
            {"x": center_x - w // 2, "y": 1000, "w": w, "h": h},
        ]


# ─── PDF 输出 ──────────────────────────────────────────

def pages_to_pdf(pages: List[Path], output: Path) -> Path:
    c = pdf_canvas.Canvas(str(output), pagesize=A4)
    w, h = A4
    for p in pages:
        c.drawImage(str(p), 0, 0, width=w, height=h, preserveAspectRatio=True)
        c.showPage()
    c.save()
    return output


# ─── 主流程 ────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="儿童成长手册生成器")
    parser.add_argument("--name", required=True, help="孩子姓名")
    parser.add_argument("--nickname", default="", help="昵称")
    parser.add_argument("--birthday", required=True, help="出生日期 YYYY-MM-DD")
    parser.add_argument("--photos", required=True, help="照片文件夹路径")
    parser.add_argument("--style", default="forest_watercolor",
                        choices=list(STYLES.keys()), help="风格")
    parser.add_argument("--pages", type=int, default=20, help="页数")
    parser.add_argument("--output", default="./output", help="输出目录")
    parser.add_argument("--api-key", default=None, help="DeepSeek API Key")
    args = parser.parse_args()

    api_key = args.api_key or os.environ.get("DASHSCOPE_API_KEY", "")
    if not api_key:
        print("⚠️  未设置 DASHSCOPE_API_KEY，将使用默认文案")
        llm_client = None
    else:
        llm_client = OpenAI(
            api_key=api_key,
            base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",
        )

    photo_dir = Path(args.photos).expanduser()
    photo_files = sorted(
        [f for f in photo_dir.iterdir()
         if f.suffix.lower() in (".jpg", ".jpeg", ".png", ".webp")]
    ) if photo_dir.is_dir() else []

    output_dir = Path(args.output).expanduser()
    pages_dir = output_dir / "pages"
    pages_dir.mkdir(parents=True, exist_ok=True)

    themes = THEMES_DEFAULT[:args.pages]
    if len(themes) < args.pages:
        themes += ["daily"] * (args.pages - len(themes))

    print(f"🎨 开始生成 {args.name} 的成长手册")
    print(f"   风格: {args.style}")
    print(f"   页数: {args.pages}")
    print(f"   照片: {len(photo_files)} 张")
    print()

    generated: List[Path] = []
    photos_per_page = max(1, len(photo_files) // args.pages) if photo_files else 0

    for i, theme in enumerate(themes):
        page_num = i + 1
        print(f"  📄 第 {page_num}/{args.pages} 页 [{theme}]...", end=" ", flush=True)

        # 文案
        if llm_client:
            caption = generate_caption(
                llm_client, args.name, args.nickname,
                theme, args.style, page_num, args.pages,
            )
        else:
            caption = {
                "title": f"{args.name}的{theme}",
                "main_text": f"记录{args.nickname or args.name}成长路上的每一个瞬间。",
                "tags": ["成长", "快乐", "爱"],
            }

        # 照片分配
        start = i * photos_per_page
        page_photos = photo_files[start:start + min(3, photos_per_page)] if photo_files else []

        # 合成
        page_path = pages_dir / f"page_{page_num:03d}.png"
        compose_page(caption, page_photos, args.style, page_path)
        generated.append(page_path)
        print("✅")

    # PDF
    pdf_path = output_dir / f"{args.name}_成长手册.pdf"
    pages_to_pdf(generated, pdf_path)

    print()
    print(f"✅ 生成完成！")
    print(f"   📁 PNG 文件: {pages_dir}")
    print(f"   📕 PDF 文件: {pdf_path}")


if __name__ == "__main__":
    main()
