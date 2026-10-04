#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""由上游 `课程总览.json` 现场刷新《对外展示内容-v1.md》里的 AUTO 区块。

为什么要这个脚本（而不是手写数字）：
    官网 / 落地页上的每一个数字都必须能追到上游只读 JSON（PRD 铁律二）。
    手抄的数字**会过期**，且没人知道它是什么时候抄的。
    本脚本让"数字"变成可重跑产物 —— 上游变，重跑即刷新，**页面无需改代码**（PRD §7.17 Q-7）。

用法：
    python tools/gen_public_content.py                      # 用默认上游路径，原地刷新
    python tools/gen_public_content.py --overview <路径>     # 指定课程总览.json
    python tools/gen_public_content.py --check              # 只校验是否为最新，不写文件（CI 用）

退出码：0 一致（或已刷新） · 1 `--check` 下有差异 · 2 上游文件不可读
"""

import argparse
import io
import json
import os
import re
import sys

DOC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "docs", "对外展示内容-v1.md")
UPSTREAM_DEFAULT = os.environ.get(
    "CWTC_UPSTREAM",
    "E:/2026-2027/2026-2027-1/AI化教学创新/课程总览.json",
)

BLOCK_FMT = r"(?s)(<!-- AUTO:%s:BEGIN -->\n)(.*?)(<!-- AUTO:%s:END -->)"


def load_courses(path):
    with io.open(path, encoding="utf-8") as f:
        raw = json.load(f)
    courses = [c for c in raw.get("courses", []) if c.get("exists")]
    return raw, courses


def totals(courses):
    def t(k):
        return sum((c.get("testbank") or {}).get(k, 0) or 0 for c in courses)

    def r(k):
        return sum((c.get("resources") or {}).get(k, 0) or 0 for c in courses)

    levels = {}
    for c in courses:
        for k, v in ((c.get("testbank") or {}).get("by_level") or {}).items():
            levels[k] = levels.get(k, 0) + v
    lectures = sum(c.get("lectures") or 0 for c in courses)
    lecture_courses = [c for c in courses if (c.get("lectures") or 0) > 0]
    return {
        "courses": len(courses),
        "lecture_courses": len(lecture_courses),
        "lectures": lectures,
        "questions": t("total"),
        "auto": t("auto_gradable"),
        "manual": t("manual"),
        "levels": levels,
        "res": {k: r(k) for k in ("课件", "HTML页", "Markdown", "视频MP4", "音频MP3", "代码文件", "冻结输出")},
    }


def render_stats(raw, courses):
    s = totals(courses)
    lv = s["levels"]
    src = "`课程总览.json`"
    lines = [
        "| 对外数字 | 值 | 出处（可逐项核对） |",
        "|---|---|---|",
        "| 课程门数 | **%d 门**（%d 门讲次制 + 1 门案例制） | %s `courses[].exists` |"
        % (s["courses"], s["lecture_courses"], src),
        "| 讲次总数 | **%d 讲** | %s `courses[].lectures` 求和（案例制课不计讲次） |" % (s["lectures"], src),
        "| 分级测评题量 | **%d 题**（L1 %d · L2 %d · L3 %d） | %s `testbank.total` / `by_level` |"
        % (s["questions"], lv.get("L1", 0), lv.get("L2", 0), lv.get("L3", 0), src),
        "| 客观题（机器判分） | **%d 题** | %s `testbank.auto_gradable` |" % (s["auto"], src),
        "| 主观题（人工采点） | **%d 题** | %s `testbank.manual` |" % (s["manual"], src),
        "| 证书档位 | **%d 档**（%d 门 × L1/L2/L3） | %s `testbank.levels` |"
        % (s["courses"] * 3, s["courses"], src),
        "| 三档合格线 | **60 / 70 / 80**（良好 75/82/90 · 优秀 90/92/96） | %s `bands` |" % src,
        "| 课件 | **%d 件**（含 HTML 页 %d 件） | %s `resources` |" % (s["res"]["课件"], s["res"]["HTML页"], src),
        "| 讲义（Markdown） | **%d 件** | %s `resources` |" % (s["res"]["Markdown"], src),
        "| 讲解视频（MP4） | **%d 个** | %s `resources` |" % (s["res"]["视频MP4"], src),
        "| 讲解音频（MP3） | **%d 个** | %s `resources` |" % (s["res"]["音频MP3"], src),
        "| 示例代码文件 | **%d 个** | %s `resources` |" % (s["res"]["代码文件"], src),
        "| 冻结输出（真机实测留档） | **%d 份** | %s `resources` |" % (s["res"]["冻结输出"], src),
        "| 证书签发方 | **%s** | %s `issuer` |" % (raw.get("issuer", "【未读到】"), src),
        "| 编号规则 | `CW-{课程码}-{级别位}{年份}{流水号6位}` | %s `_meta.cert_no_rule` |" % src,
        "",
        "> **以上数字由 `python tools/gen_public_content.py` 现场生成**，"
        "刷新时间随上游文件变动；**表格里的数不要手改** —— 手改的数下一页就过期。",
        "",
    ]
    return "\n".join(lines)


def render_matrix(courses):
    out = [
        "| 课程码 | 课程 | 类型 | 讲次 | 题量 | 三档证书 |",
        "|---|---|---|---|---|---|",
    ]
    for c in courses:
        tb = c.get("testbank") or {}
        names = tb.get("cert_names") or []
        short = []
        for n in names:
            m = re.search(r"·\s*(.+)$", n)
            short.append(m.group(1) if m else n)
        out.append(
            "| `%s` | %s | %s | %s | %d 题 | %s |"
            % (
                c["id"],
                (tb.get("title") or "").replace("分级测试题库", "").strip("《》") or c["id"],
                c.get("type") or "—",
                c.get("lectures") or "—",
                tb.get("total", 0),
                " · ".join(short) if short else "—",
            )
        )
    out += ["", "> 同上，由脚本现场生成；**证书名以 `/verify` 与证书台账为准**。", ""]
    return "\n".join(out)


def apply_block(text, name, body):
    pat = re.compile(BLOCK_FMT % (name, name))
    m = pat.search(text)
    if not m:
        raise SystemExit("文档里找不到 AUTO:%s 区块" % name)
    return pat.sub(lambda _: m.group(1) + body + m.group(3), text, count=1)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--overview", default=UPSTREAM_DEFAULT, help="上游 课程总览.json 路径")
    ap.add_argument("--doc", default=DOC, help="目标 markdown")
    ap.add_argument("--check", action="store_true", help="只校验，不写文件")
    a = ap.parse_args()

    if not os.path.exists(a.overview):
        print("上游文件不存在：%s" % a.overview, file=sys.stderr)
        print("提示：用 --overview 指定，或设环境变量 CWTC_UPSTREAM", file=sys.stderr)
        return 2

    raw, courses = load_courses(a.overview)
    text = io.open(a.doc, encoding="utf-8").read()
    new = apply_block(text, "STATS", render_stats(raw, courses))
    new = apply_block(new, "MATRIX", render_matrix(courses))

    if new == text:
        print("一致：AUTO 区块已是最新（%d 门课 / %d 题）" % (len(courses), totals(courses)["questions"]))
        return 0
    if a.check:
        print("不一致：`docs/对外展示内容-v1.md` 的 AUTO 区块落后于上游，请重跑本脚本", file=sys.stderr)
        return 1
    io.open(a.doc, "w", encoding="utf-8", newline="\n").write(new)
    print("已刷新：%s" % os.path.relpath(a.doc))
    return 0


if __name__ == "__main__":
    sys.exit(main())
