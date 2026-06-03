#!/usr/bin/env bash
# =============================================================================
# build-plugin-openlist-web.sh
# -----------------------------------------------------------------------------
# 一键构建 plugin-openlist/web 并把 dist 同步到 plugin-openlist/src/main/assets/openlist/
#
# 为什么需要这个脚本：
#   1. plugin-openlist 的 Android WebView 通过 file:///android_asset/openlist/ 加载 UI
#   2. Android assets 必须在 APK 打包前就位（编译期资源）
#   3. Vite 默认 base: '/' 在 file:// 协议下 404 → vite.config.ts 已设 base: './'
#   4. CI 不能依赖 dev server → 必须预构建
#
# 用法：
#   bash scripts/build-plugin-openlist-web.sh          # 默认 dev 构建（快）
#   bash scripts/build-plugin-openlist-web.sh --prod   # 生产构建（混淆、压缩）
#
# 配套：
#   - 沙箱开发：bash scripts/dev-openlist-web.sh         （Vite HMR）
#   - 真机构建：本脚本 + ./gradlew :plugin-openlist:assembleDebug
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MOBILE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WEB_DIR="${MOBILE_DIR}/plugin-openlist/web"
ASSETS_DIR="${MOBILE_DIR}/plugin-openlist/src/main/assets/openlist"

MODE_FLAG=""
if [[ "${1:-}" == "--prod" ]]; then
  MODE_FLAG="--prod"
  echo "==> 生产构建（混淆 + 压缩）"
else
  echo "==> 开发构建（未压缩，便于调试）"
fi

step() { echo ""; echo "==> $*"; }

# ---- Step 1: 确保 monorepo 安装 ----
step "1/5 pnpm install（确保 @encvgo/components 已链接）"
cd "${MOBILE_DIR}"
pnpm install --no-frozen-lockfile --silent

# ---- Step 2: 校验 monorepo 链接 ----
step "2/5 校验 @encvgo/components 链接"
if [[ ! -e "${WEB_DIR}/node_modules/@encvgo/components" ]]; then
  echo "    ❌ @encvgo/components 未链接"
  echo "    请检查 pnpm-workspace.yaml 是否包含 packages/*"
  exit 1
fi
echo "    ✅ @encvgo/components → $(readlink -f "${WEB_DIR}/node_modules/@encvgo/components" 2>/dev/null || echo "exists")"

# ---- Step 3: 构建 plugin web ----
step "3/5 pnpm build ${MODE_FLAG} （Vite 构建 plugin-openlist/web）"
cd "${WEB_DIR}"
pnpm exec vite build ${MODE_FLAG} --logLevel warn

if [[ ! -d "dist" ]]; then
  echo "    ❌ dist 目录未生成"
  exit 1
fi

# ---- Step 4: 校验产物结构 ----
step "4/5 校验构建产物"
if [[ ! -f "dist/index.html" ]]; then
  echo "    ❌ dist/index.html 缺失"
  exit 1
fi

# 校验 base: './' 是否生效（file:// 加载必须）
if grep -q 'href="/' dist/index.html; then
  echo "    ❌ dist/index.html 含绝对路径 /，file:// 加载会 404"
  echo "    请检查 vite.config.ts 的 base 配置"
  exit 1
fi
echo "    ✅ dist/index.html 资源路径全部相对化"

# 列出产物
echo "    产物文件："
find dist -type f | head -20 | sed 's/^/      /'
SIZE=$(du -sh dist | cut -f1)
echo "    总大小：${SIZE}"

# ---- Step 5: 同步到 plugin assets ----
step "5/5 同步到 ${ASSETS_DIR}"
rm -rf "${ASSETS_DIR}"
mkdir -p "${ASSETS_DIR}"
cp -r dist/. "${ASSETS_DIR}/"

# 验证
if [[ ! -f "${ASSETS_DIR}/index.html" ]]; then
  echo "    ❌ 同步后 index.html 缺失"
  exit 1
fi

echo ""
echo "========================================"
echo "✅ plugin-openlist/web 构建并同步完成"
echo ""
echo "  source:    ${WEB_DIR}/src"
echo "  build:     ${WEB_DIR}/dist"
echo "  assets:    ${ASSETS_DIR}"
echo "  apk load:  file:///android_asset/openlist/index.html"
echo ""
echo "  下一步："
echo "    cd ${MOBILE_DIR}/android"
echo "    ./gradlew :plugin-openlist:assembleDebug"
echo "========================================"
