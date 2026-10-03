#!/usr/bin/env bash
# 敏感信息扫描：身份证号/手机号不得明文出现在代码、配置与文档中。
# 用法：bash deploy/scripts/scan-sensitive.sh
#
# 这是 PRD v2 §9.2 与 AGENTS.md 铁律 3 的自动化门禁，CI 与上线前检查都会跑。
set -euo pipefail

cd "$(dirname "$0")/../.."

# ---------------------------------------------------------------------------
# 排除规则（每条都说明为什么不会削弱门禁）
#
#   testdata/      —— 约定俗成的测试数据目录，不参与生产构建
#   *_test.go      —— 单元测试。要验证脱敏函数是否生效，**必须**喂一个格式合法的
#                     身份证号进去；若禁止，脱敏逻辑就无法被测试，反而是更大的风险。
#                     测试文件不编译进生产二进制，也不写入生产日志。
#   example/示例/样例 —— 文档与模板里的占位说明
#
# 配套约束（见 AGENTS.md）：测试中一律使用固定样例号，**不得使用真实号码**。
# ---------------------------------------------------------------------------
EXCLUDE='testdata|_test\.go|example|示例|样例'

echo "==> 扫描明文身份证号（18 位，含校验位写法）"
if grep -rnE '\b[1-9][0-9]{5}(19|20)[0-9]{2}(0[1-9]|1[0-2])(0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]\b' \
    --include='*.go' --include='*.js' --include='*.ts' --include='*.vue' \
    --include='*.md' --include='*.yml' --include='*.yaml' --include='*.json' \
    . 2>/dev/null | grep -v -iE "$EXCLUDE"; then
  echo "✗ 发现疑似明文身份证号，请改为脱敏/样例写法"
  exit 1
fi
echo "✓ 未发现明文身份证号"

echo "==> 扫描明文手机号（中国大陆 11 位）"
if grep -rnE '\b1[3-9][0-9]{9}\b' --include='*.go' --include='*.js' --include='*.vue' . 2>/dev/null \
   | grep -v -iE "$EXCLUDE"; then
  echo "✗ 发现疑似明文手机号"
  exit 1
fi
echo "✓ 未发现明文手机号"

echo "==> 扫描密钥文件是否误入库"
if git ls-files 2>/dev/null | grep -E '\.(pem|key)$|(^|/)\.env$'; then
  echo "✗ 密钥/环境文件被 Git 跟踪，请移出并加入 .gitignore"
  exit 1
fi
echo "✓ 未跟踪密钥/环境文件"

echo "==> 扫描后端是否绕过脱敏输出（裸打卡号字段）"
# 只做粗筛：把"直接用 fmt.Print/log 打印敏感字段名"挑出来人工复核。
# 真正的保证在 internal/redact 的单测（守门用例）里。
if grep -rnE '(Print|Println|Printf|log\.(Print|Printf|Println))\s*\(.*(id_card|idCard)' \
    --include='*.go' . 2>/dev/null | grep -v -iE "$EXCLUDE"; then
  echo "✗ 疑似直接打印身份证号字段，请改用 internal/redact 过滤"
  exit 1
fi
echo "✓ 未发现绕过脱敏的直出打印"

echo "全部通过。"
