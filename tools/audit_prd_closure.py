#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PRD 闭环性审计：把"画了菜单没写菜"从人工通读变成可重复运行的检查。

为什么要它：
    需求文档最常见的病**不是写得少，而是画了菜单没写菜** ——
    站点地图里加了 `/admin/showcase`，权限矩阵里却没有这一行；
    新增了模块 Q，验收清单里一条都没提。
    靠"通读一遍觉得挺全"查不出来：**缺的东西本来就不在纸上**（沿用 requirements-role-audit 的方法）。

检查项（只读，不改文档）：
    1. 章节引用有效性：`§X.Y` 指到的章节是否真的存在（**跨文档引用会被排除**，见 CROSS_DOC）
    2. §16 编号引用：`§16-N` 是否在"已结案 / 待确认"表里
    3. 文件引用有效性：`docs/xxx.md` · `tools/xxx.py` · `deploy/scripts/xxx.sh` 是否存在
    4. 模块闭环矩阵：每个模块（A~Q）在 **自查表 / §15.1 验收 / API 契约 / 数据字典** 四处是否有落点
    5. 枚举登记粗筛（**提示级，不判失败**）：`CHECK` 里的值是否出现在"枚举值集中表"

**为什么要有映射表而不是"关键词自动匹配"**：
    自动匹配会制造假警报，而**假警报多了门禁就会被绕过（比没有门禁更糟）**。
    所以下面两张映射表是**显式维护**的 —— 新增模块时**必须同时在这里登记**，
    这本身就是一次人工确认（与 PRD §16 的计数口径同一思路）。

用法：
    python tools/audit_prd_closure.py            # 报告 + 有缺口则退出码 1
    python tools/audit_prd_closure.py --quiet    # 只打印有问题的部分
"""

import argparse
import io
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PRD = os.path.join(ROOT, "docs", "PRD-第三课堂-v2.md")
API = os.path.join(ROOT, "docs", "API接口参考-v1.md")
DIC = os.path.join(ROOT, "docs", "数据字典-v1.md")

HEAD_RE = re.compile(r"(?m)^#{2,4}\s+(\d+(?:\.\d+)*)")
SECTION_REF_RE = re.compile(r"§(\d+(?:\.\d+)*)")
S16_REF_RE = re.compile(r"§16-(\d+)")
FILE_REF_RE = re.compile(
    r"`((?:docs|tools|deploy|packages|\.github)/[\w\u4e00-\u9fa5./-]+\.(?:md|py|sh|yml|yaml|go|vue|dart|conf))`"
)
MODULE_RE = re.compile(r"(?m)^###\s+(7\.\d+)\s+([^（(]+)（模块\s*([A-Z]+)")
ENUM_IN_RE = re.compile(r"CHECK\s*\([^)]*?in\s*\(([^)]+)\)")

# 跨文档引用前缀：这些 `§X.Y` 指的是**别份文档**的章节，不按本 PRD 的标题校验
CROSS_DOC = ("API", "数据字典", "技术选型", "M2 文档", "M4 文档", "对外展示内容", "技术方案")

# 引用的文件里，这几份是**待建**（文档里已标 ⏳）；上游脚本不在本仓库
FILE_ALLOW = {
    "docs/技术方案-v1.md": "待建（M0 末）",
    "docs/部署运维手册-v1.md": "待建（M6 前）",
    "docs/合规与隐私政策-v1.md": "待建（上线前）",
    "tools/_make_overview.py": "上游资产，不在本仓库",
}

# 模块 → 数据字典里的落点章节（`None` = 该模块本就不该有自己的表，如只读聚合接口）
DICT_MAP = {
    "7.1": ["## 2. 账号与实名"],
    "7.2": ["### 3.1", "### 3.2"],
    "7.3": ["## 4. 权益、订单与资金"],
    "7.4": ["### 3.4", "### 4.1"],
    "7.5": ["### 6.4"],
    "7.6": ["### 6.5"],
    "7.7": ["## 5. 测评与证书"],
    "7.8": ["### 5.4"],
    "7.9": ["### 7.5", "### 7.4"],
    "7.10": ["## 7. 通知、广播与审计"],
    "7.11": ["### 7.1"],
    "7.12": None,
    "7.13": ["### 4.1", "### 5.4", "### 7.3"],
    "7.14": ["### 6.6", "### 6.7"],
    "7.15": ["### 6.8"],
    "7.16": ["### 6.9"],
    "7.17": ["### 6.10"],
}

# 模块 → §15.1 验收里的关键词（用于判断"验收里到底管不管这个模块"）
ACCEPT_KW = {
    "7.1": "注册",
    "7.2": "课程广场",
    "7.3": "扫码",
    "7.4": "解锁",
    "7.5": "AI",
    "7.6": "笔记",
    "7.7": "测评",
    "7.8": "证书",
    "7.9": "教师",
    "7.10": "审计",
    "7.11": "广播",
    "7.12": "官网",
    "7.13": "我的",
    "7.14": "统计",
    "7.15": "讨论",
    "7.16": "草稿",
    "7.17": "落地页",
}


def read(p):
    return io.open(p, encoding="utf-8").read() if os.path.isfile(p) else ""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--quiet", action="store_true")
    a = ap.parse_args()

    prd, api, dic = read(PRD), read(API), read(DIC)
    problems, notes = [], []

    # ---------- 1. 章节引用 ----------
    heads = set(HEAD_RE.findall(prd)) | set(re.findall(r"(?m)^#{2,4}\s+(M\d+)\b", prd))
    bad_refs, skipped = {}, 0
    for m in SECTION_REF_RE.finditer(prd):
        ref = m.group(1)
        # ① 包在反引号里的 = 引用"历史上的写法"，不是活的引用（如变更摘要里写"旧写法 `§7.2.2`"）
        if prd[max(0, m.start() - 1):m.start()] == "`" and prd[m.end():m.end() + 1] == "`":
            skipped += 1
            continue
        # ② 同一行里带跨文档标记（`API §2.4` / `数据字典 §6.7` / `对外展示内容 §5.1`）→ 指的是别份文档
        line_start = prd.rfind("\n", 0, m.start()) + 1
        if any(p in prd[line_start:m.start()] for p in CROSS_DOC):
            skipped += 1
            continue
        if ref in heads or any(h.startswith(ref + ".") for h in heads):
            continue
        bad_refs[ref] = bad_refs.get(ref, 0) + 1
    if bad_refs:
        problems.append(("§ 章节引用指不到实际章节",
                         ["§%s（%d 处）" % (k, v) for k, v in sorted(bad_refs.items())]))
    if not a.quiet:
        print("（章节引用：跳过 %d 处跨文档 / 历史写法引用 —— 跳过是显式的，不是静默放行）\n" % skipped)

    # ---------- 2. §16 编号引用 ----------
    s16 = prd[prd.index("## 16. 待拍板事项"):] if "## 16. 待拍板事项" in prd else ""
    s16_nums = set(re.findall(r"(?m)^\|\s*\*{0,2}(\d+)", s16))
    bad16 = sorted({n for n in S16_REF_RE.findall(prd) if n not in s16_nums}, key=int)
    if bad16:
        problems.append(("§16-N 引用在 §16 两个表里找不到", ["§16-%s" % n for n in bad16]))

    # ---------- 3. 文件引用 ----------
    missing = []
    for f in sorted(set(FILE_REF_RE.findall(prd))):
        if f in FILE_ALLOW:
            continue
        if not os.path.exists(os.path.join(ROOT, f.replace("/", os.sep))):
            missing.append(f)
    if missing:
        problems.append(("PRD 引用的文件不存在", missing))

    # ---------- 4. 模块闭环矩阵 ----------
    self_sec = prd[prd.index("### 3.4"):prd.index("## 4. 核心用户旅程")] if "### 3.4" in prd else ""
    acc_sec = prd[prd.index("### 15.1"):prd.index("### 15.2")] if "### 15.1" in prd else ""
    matrix = []
    for sec, name, letter in MODULE_RE.findall(prd):
        has_sc = ("§" + sec) in self_sec or ("模块 %s" % letter) in self_sec
        kw = ACCEPT_KW.get(sec)
        has_ac = (("§" + sec) in acc_sec) or (kw is not None and kw in acc_sec)
        has_api = ("§" + sec) in api or ("（模块 %s）" % letter) in api or ("模块 %s" % letter) in api
        dm = DICT_MAP.get(sec, ["__unregistered__"])
        has_dc = dm is None or any(x in dic for x in dm)
        matrix.append((sec, letter, name.strip(), has_sc, has_ac, has_api, has_dc))
        if sec not in DICT_MAP or sec not in ACCEPT_KW:
            problems.append(("新模块未在审计脚本的映射表里登记（本脚本要求显式登记）",
                             ["§%s（模块 %s）：请在 DICT_MAP / ACCEPT_KW 里各加一行" % (sec, letter)]))
    bad_mods = [(s, l, n, sc, ac, ap, dc) for s, l, n, sc, ac, ap, dc in matrix
                if not all((sc, ac, ap, dc))]
    if bad_mods:
        lines = []
        for sec, letter, name, sc, ac, ap, dc in bad_mods:
            miss = []
            if not sc:
                miss.append("自查表")
            if not ac:
                miss.append("§15.1 验收")
            if not ap:
                miss.append("API 契约")
            if not dc:
                miss.append("数据字典")
            lines.append("§%s 模块 %s %s → 缺：%s" % (sec, letter, name, " · ".join(miss)))
        problems.append(("模块落点不完整（画了菜单没写菜）", lines))

    # ---------- 5. 枚举登记（提示级） ----------
    enum_sec = dic[dic.index("## 1. 枚举值集中表"):dic.index("## 2. ")] if "## 1. 枚举值集中表" in dic else ""
    uncovered = set()
    for grp in ENUM_IN_RE.findall(dic[dic.index("## 2. "):] if "## 2. " in dic else dic):
        for v in re.findall(r"'([a-z_]{2,30})'", grp):
            if v not in enum_sec:
                uncovered.add(v)
    if uncovered:
        notes.append(("枚举未登记在「枚举值集中表」（提示：核对是否需要补）",
                      ["`%s`" % v for v in sorted(uncovered)]))

    # ---------- 6. 版本一致性（版本号同步是项目约定，实测最常漂的就是它） ----------
    # ⚠️ 期望值**从契约文档自身的表头推导**，不在这里硬编码 ——
    #    硬编码的话，每次升版都要改脚本，而"要改脚本才能过"的门禁迟早被绕过。
    prd_ver = re.search(r"本 PRD 的版本\*\*：\*\*(v[\d.]+)\*\*", prd)
    # ⚠️ 版本行里的加粗块后面可能还有说明（如 `**v1.2（增量修订）**`），
    #    所以不能要求 `**` 紧跟数字，否则会"读不到版本号"→ 整个检查静默空转（已踩过）。
    api_ver = re.search(r"\| 版本 \| \*\*(v[\d.]+)", api)
    dic_ver = re.search(r"\| 版本 \| \*\*(v[\d.]+)", dic)
    tech = read(os.path.join(ROOT, "docs", "技术选型-v1.md"))
    tech_ver = re.search(r"\| 版本 \| \*\*(v[\d.]+)", tech)
    plan = read(os.path.join(ROOT, "docs", "技术方案-v1.md"))
    plan_ver = re.search(r"\| 版本 \| \*\*(v[\d.]+)", plan)
    peers = {"README.md": read(os.path.join(ROOT, "README.md")),
             "docs/README.md": read(os.path.join(ROOT, "docs", "README.md")),
             "SOUL.md": read(os.path.join(ROOT, "SOUL.md")),
             "docs/PRD-第三课堂-v2.md": prd,
             "AGENTS.md": read(os.path.join(ROOT, "AGENTS.md"))}
    drift = []
    for label, m in (("PRD", prd_ver), ("API", api_ver), ("数据字典", dic_ver)):
        if not m:
            drift.append("%s：读不到版本号（格式变了？）" % label)
            continue
        for fn, body in peers.items():
            if m.group(1) not in body:
                drift.append("%s 的 %s 未出现在 %s" % (label, m.group(1), fn))
    if drift:
        problems.append(("版本号不同步", drift))

    # ---------- 7. 陈旧版本号（"同步版本号"是项目约定，而它一直在漂） ----------
    # 统一去掉前缀 v 再比较（上面正则捕到的是 `v2.15` 这种带 v 的写法）
    cur = {"PRD": prd_ver.group(1).lstrip("v") if prd_ver else None,
           "API": api_ver.group(1).lstrip("v") if api_ver else None,
           "字典": dic_ver.group(1).lstrip("v") if dic_ver else None,
           "技术选型": tech_ver.group(1).lstrip("v") if tech_ver else None,
           "技术方案": plan_ver.group(1).lstrip("v") if plan_ver else None}
    if any(v is None for v in cur.values()):
        problems.append(("版本号读取失败（正则与文档格式不符，检查会空转）",
                         ["%s 读不到版本号" % k for k, v in cur.items() if v is None]))
    # 覆盖三种写法：`PRD v2.16` · `接口契约 v1.12` · `` `docs/数据字典-v1.md`（**v1.12**） ``
    pats = [("PRD", re.compile(r"PRD v(\d+\.\d+)")),
            ("API", re.compile(r"(?:接口契约|API 参考) v(\d+\.\d+)")),
            ("字典", re.compile(r"数据字典[^\n]{0,12}?v(\d+\.\d+)")),
            ("字典", re.compile(r"数据字典-v1\.md`\*{0,2}[（(]\*{0,2}v(\d+\.\d+)")),
            ("技术选型", re.compile(r"技术选型[^\n]{0,12}?v(\d+\.\d+)")),
            ("技术方案", re.compile(r"技术方案[^\n]{0,12}?v(\d+\.\d+)"))]
    stale = []
    for fn, body in peers.items():
        # PRD 的 §0 变更摘要与附录 C 版本历史里**本来就该有旧版本号**，跳过。
        skip_from = body.index("## 1. 第三课堂是什么") if fn.endswith("PRD-第三课堂-v2.md") else 0
        # ⚠️ 边界**不能**用 body.index("版本历史")：该词在 §0 变更摘要（约第 80 行）里
        #    就已经出现过，会把上界提前到文首 → 本检查对 PRD **整体空转**（已实测踩过）。
        #    改为锚定附录 C 的标题 —— 版本历史就在那一节里。
        hist_start = body.index("## 附录 C") if "## 附录 C" in body else len(body)
        for label, rx in pats:
            for m in rx.finditer(body):
                if m.start() < skip_from or m.start() > hist_start:
                    continue
                if cur[label] and m.group(1).lstrip("v") != cur[label]:
                    line = body[body.rfind("\n", 0, m.start()) + 1: body.find("\n", m.start())]
                    stale.append("%s 里写着 %s v%s（当前 %s）：%s" % (fn, label, m.group(1), cur[label], line.strip()[:60]))
    if stale:
        problems.append(("陈旧版本号残留", stale))

    # ---------- 8. 铁律条数自述一致（README 写"共 N 条"，AGENTS 实际几节） ----------
    # 铁律一直在加，而 README 里的"共 N 条"是手写的 —— 典型的会漂的地方。
    agents_body = peers.get("AGENTS.md", "")
    readme_body = peers.get("README.md", "")
    oath_sec = agents_body.split("## 铁律")[1] if "## 铁律" in agents_body else ""
    oath_nums = [int(x) for x in re.findall(r"(?m)^(\d+)\. \*\*", oath_sec)]
    actual_oath = max(oath_nums) if oath_nums else 0
    m_oath = re.search(r"铁律（共\s*(\d+)\s*条", readme_body)
    if m_oath and actual_oath and int(m_oath.group(1)) != actual_oath:
        problems.append(("铁律条数自述不一致",
                         ["README.md 写「共 %s 条」，AGENTS.md 实际 %d 条（铁律只会加，README 的手写数字要跟着改）"
                          % (m_oath.group(1), actual_oath)]))

    # ---------- 9. 自述数字 / 版本自洽（"数字要有出处"要能被机器验一次） ----------
    # 文档里凡"给了派生命令的数字"，都应当与那条命令的结果一致；否则就是编数或漂了。
    docs_readme = peers["docs/README.md"]
    n_api = len(re.findall(r"(?m)^\|\s*(?:GET|POST|PUT|PATCH|DELETE)(?:·[A-Z]+)?\s*\|", api))
    n_tbl = len(re.findall(r"(?m)^#### \d+\.\d+\.\d+ `|^### \d+\.\d+ `", dic))
    selfnum = []
    m = re.search(r"实测\s*(\d+)\s*个接口", docs_readme)
    if m and int(m.group(1)) != n_api:
        selfnum.append("docs/README 写「实测 %s 个接口」，按它给的那条 grep 命令实际是 %d" % (m.group(1), n_api))
    m = re.search(r"(\d+) 个表级小节", docs_readme)
    if m and int(m.group(1)) != n_tbl:
        selfnum.append("docs/README 写「%s 个表级小节」，实际 %d" % (m.group(1), n_tbl))
    # PRD 头部「版本」字段必须等于附录 C 的「本 PRD 的版本」
    hv = re.search(r"(?m)^\|\s*版本\s*\|\s*\*\*(v[\d.]+)", prd)
    cv = re.search(r"本 PRD 的版本\*\*：\*\*(v[\d.]+)", prd)
    if hv and cv and hv.group(1) != cv.group(1):
        selfnum.append("PRD 头部版本字段写 %s，附录 C 写 %s —— 两处必须一致" % (hv.group(1), cv.group(1)))
    # §16「待确认」计数：SOUL.md 里手写的那个数必须等于现场 count（口径：总行数 − 墓碑行数）
    s16 = prd[prd.index("## 16. 待拍板事项"):]
    todo16 = s16[s16.index("\n### 待确认\n"):]
    rows16 = [l for l in todo16.splitlines() if re.match(r"^\|\s*\*{0,2}\d+", l)]
    live16 = len([l for l in rows16 if "~~" not in l])
    m = re.search(r"待确认\s*\*{0,2}(\d+)\s*项", peers.get("SOUL.md", ""))
    if m and int(m.group(1)) != live16:
        selfnum.append("SOUL.md 写「待确认 %s 项」，实际 %d（§16 口径：总行数 − 墓碑行数）" % (m.group(1), live16))
    # §13 任务计划：自述任务数 vs 任务表行数；任务编号必须唯一（全项目唯一、废弃不复用）
    task_ids = re.findall(r"(?m)^\| (M[0-8]-\d{2}) \|", prd)
    m = re.search(r"共\s*\*{0,2}(\d+)\s*个任务", prd)
    if m and int(m.group(1)) != len(task_ids):
        selfnum.append("PRD 写「共 %s 个任务」，§13 任务表实际 %d 行" % (m.group(1), len(task_ids)))
    seen = {}
    for tid in task_ids:
        seen[tid] = seen.get(tid, 0) + 1
    dup = sorted([k for k, v in seen.items() if v > 1])
    if dup:
        selfnum.append("§13 任务编号重复：%s" % "、".join(dup))
    if selfnum:
        problems.append(("自述数字 / 版本自相矛盾", selfnum))

    # ---------- 输出 ----------
    if not a.quiet:
        print("== PRD 闭环性审计 ==")
        print("PRD %d 字符 / 章节 %d 个 · API %d 字符 · 数据字典 %d 字符" % (len(prd), len(heads), len(api), len(dic)))
        print("\n-- 模块闭环矩阵（%d 个模块）--\n" % len(matrix))
        print("| 模块 | 名称 | 自查表 | §15.1 | API | 字典 |")
        print("|---|---|---|---|---|---|")
        for sec, letter, name, sc, ac, ap, dc in matrix:
            m = lambda x: "✅" if x else "❌"  # noqa: E731
            print("| %s %s | %s | %s | %s | %s | %s |" % (sec, letter, name, m(sc), m(ac), m(ap), m(dc)))
        print()
    if problems or notes:
        for title, items in problems + notes:
            print("✗ %s：" % title)
            for it in items:
                print("   - %s" % it)
        print("\n致命问题 %d 类 · 提示 %d 类。" % (len(problems), len(notes)))
        return 1 if problems else 0
    print("✓ 未发现闭环缺口。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
