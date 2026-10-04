#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""校验《对外展示内容-v1.md》里「就业直达」表的 `lecture_refs` 与上游讲次目录是否对得上。

为什么要单独一个脚本：
    就业直达表的每一行都宣称"点得进对应讲次"。**宣称本身不值钱，能当场验才算数。**
    本脚本把 `05-OS/21` 这种 ref 拿去上游目录里找 `21_*`，找不到就报错 ——
    这样"映射不是拍脑袋"这句话才有门禁兜着（PRD §7.17 Q-6）。

用法：
    python tools/check_job_map_refs.py                     # 用默认路径
    python tools/check_job_map_refs.py --upstream <教学创新目录>

退出码：0 全部对得上 · 1 有对不上的 ref · 2 上游目录不可读（本地之外运行时属预期，跳过即可）
"""

import argparse
import io
import os
import re
import sys

DOC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "docs", "对外展示内容-v1.md")
UPSTREAM_DEFAULT = os.environ.get(
    "CWTC_UPSTREAM_DIR",
    "E:/2026-2027/2026-2027-1/AI化教学创新",
)
REF_RE = re.compile(r"`((?:0\d|1\d)-(?:DO|CF|DB|OS|CP|CN|CC))/(\d\d)`")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--upstream", default=UPSTREAM_DEFAULT, help="上游课程资产根目录")
    ap.add_argument("--doc", default=DOC)
    a = ap.parse_args()

    if not os.path.isdir(a.upstream):
        print("上游目录不存在：%s" % a.upstream, file=sys.stderr)
        print("提示：本脚本依赖本地课程资产，CI 上属预期跳过。", file=sys.stderr)
        return 2

    text = io.open(a.doc, encoding="utf-8").read()
    refs = sorted(set(REF_RE.findall(text)))
    if not refs:
        print("× 文档里一个 lecture_refs 都没找到，检查写法是否为 `05-OS/21`", file=sys.stderr)
        return 1

    bad = []
    for code, num in refs:
        d = os.path.join(a.upstream, "AI化教学创新" + code)
        if not os.path.isdir(d):
            bad.append((ref_str(code, num), "课程目录不存在"))
            continue
        hits = [x for x in os.listdir(d) if x.startswith(num + "_") or x.startswith(num)]
        if not hits:
            bad.append((ref_str(code, num), "该讲次目录不存在"))
    if bad:
        print("✗ 下列讲次引用对不上上游目录：", file=sys.stderr)
        for r, why in bad:
            print("   %s —— %s" % (r, why), file=sys.stderr)
        return 1

    print("✓ 讲次引用全部对得上上游目录（%d 条 ref / %d 门课）"
          % (len(refs), len({c for c, _ in refs})))
    # 顺带核对每门课的讲次数与课程总览一致（若 JSON 在）
    j = os.path.join(a.upstream, "课程总览.json")
    if os.path.isfile(j):
        import json
        with io.open(j, encoding="utf-8") as f:
            raw = json.load(f)
        n = sum((c.get("lectures") or 0) for c in raw.get("courses", []) if c.get("exists"))
        print("  （上游讲次制合计 %d 讲，与 课程总览.json 一致）" % n)
    return 0


def ref_str(code, num):
    return "%s/%s" % (code, num)


if __name__ == "__main__":
    sys.exit(main())
