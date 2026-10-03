#!/usr/bin/env bash
# 敏感信息扫描：身份证号/手机号不得明文出现在代码、配置与文档中。
# 用法：bash deploy/scripts/scan-sensitive.sh
set -euo pipefail

cd "$(dirname "$0")/../.."

echo "==> 扫描明文身份证号（18 位，含校验位写法）"
if grep -rnE '\b[1-9][0-9]{5}(19|20)[0-9]{2}(0[1-9]|1[0-2])(0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]\b' \
    --include='*.go' --include='*.js' --include='*.ts' --include='*.vue' \
    --include='*.md' --include='*.yml' --include='*.yaml' --include='*.json' \
    . 2>/dev/null | grep -v -i 'testdata\|example\|示例\|样例'; then
  echo "✗ 发现疑似明文身份证号，请改为脱敏/样例写法"
  exit 1
fi
echo "✓ 未发现明文身份证号"

echo "==> 扫描明文手机号（中国大陆 11 位）"
if grep -rnE '\b1[3-9][0-9]{9}\b' --include='*.go' --include='*.js' --include='*.vue' . 2>/dev/null \
   | grep -v -i 'testdata\|example\|示例\|样例'; then
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

echo "全部通过。"
