#!/usr/bin/env bash
# =============================================================================
# dev-openlist-web.sh
# -----------------------------------------------------------------------------
# 一键启动 plugin-openlist/web 的 Vite dev server（沙箱浏览器版）。
#
# ⚠️ 与 scripts/dev-openlist.sh 的关键区别：
#   - dev-openlist.sh      → 预览 OpenList 原生 SPA（Hi-Sillot-OpenList/public/dist）
#                             通过 Vite middleware 反代到 OpenList(5244) 后端
#   - dev-openlist-web.sh  → 预览 plugin-openlist/web 的 Capacitor 多例 UI
#                             (Vue3 + Ionic Vue 8 管理面板)
#                             不依赖 OpenList(5244)，不依赖 Android WebView
#                             由 `window.OpenListNative` 桥接 Android 端 OpenListBridge
#
# 沙箱浏览器模式下 `window.OpenListNative` 不存在 → 所有 JS-Native 调用走
# `safe(fallback, fn)` 安全 fallback → 显示「未安装/已停止」默认态
# → 这是预期的「UI 视觉预览」目标
#
# 用法：
#   bash scripts/dev-openlist-web.sh                  # 默认 5174
#   ENCV_OPENLIST_WEB_PORT=5180 bash scripts/dev-openlist-web.sh
#
# 配合主 app 预览（不冲突）：
#   Terminal 1: bash scripts/start-preview.sh          # 主 ENCV app (5173/5174)
#   Terminal 2: bash scripts/dev-openlist-web.sh       # plugin-openlist/web (5174/5175)
#   Browser A:  http://localhost:5173/                 # 主 app
#   Browser B:  http://localhost:5174/                 # plugin web
# =============================================================================

set -euo pipefail
shopt -s lastpipe

# ---- 信号陷阱：脚本退出时杀掉所有子进程 ----
SUBPIDS=()
cleanup() {
  echo ""
  echo "==> 收到退出信号，停止 plugin-openlist/web 预览..."
  for pid in "${SUBPIDS[@]}"; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" 2>/dev/null || true
    fi
  done
  sleep 1
  pkill -P $$ 2>/dev/null || true
  pkill -f 'vite.*plugin-openlist' 2>/dev/null || true
  pkill -f 'plugin-openlist/web' 2>/dev/null || true
  exit 0
}
trap cleanup INT TERM

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MOBILE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WEB_DIR="${MOBILE_DIR}/plugin-openlist/web"

# ---- 端口选择：默认 5174，plugin-openlist/web vite.config.ts 已设；
# ---- 被占时回退到 5175
VITE_PORT="${ENCV_OPENLIST_WEB_PORT:-5174}"
if lsof -i :5174 >/dev/null 2>&1; then
  echo "    :5174 已被占用，使用 :5175"
  VITE_PORT=5175
fi

cd "${MOBILE_DIR}"

step() { echo ""; echo "==> $*"; }

# ---- Step 0: 停止残留 vite 进程（plugin-openlist/web 范围）----
step "0/4 停止残留 plugin-openlist/web vite 进程"
pkill -f 'vite.*plugin-openlist' 2>/dev/null && echo "    killed vite" || true
pkill -f 'plugin-openlist/web' 2>/dev/null && echo "    killed vite" || true

WEB_VITE_PIDS="$(lsof -ti :${VITE_PORT} 2>/dev/null || true)"
if [[ -n "${WEB_VITE_PIDS}" ]]; then
  for pid in ${WEB_VITE_PIDS}; do
    kill "${pid}" 2>/dev/null && echo "    killed pid=${pid} on :${VITE_PORT}"
  done
fi
sleep 1

# ---- Step 1: 确保 plugin-openlist/web node_modules 就绪 ----
step "1/4 确保 ${WEB_DIR}/node_modules 就绪"
if [[ ! -d "${WEB_DIR}/node_modules/vite" ]]; then
  echo "    node_modules 缺失，pnpm install ..."
  cd "${MOBILE_DIR}"
  pnpm install --no-frozen-lockfile --filter '@encvgo/plugin-openlist-web...'
fi
cd "${WEB_DIR}"

# ---- Step 2: 校验 monorepo 链接（@encvgo/components 共享包）----
step "2/4 校验 @encvgo/components workspace 链接"
if [[ ! -d "${WEB_DIR}/node_modules/@encvgo/components" ]]; then
  echo "    ⚠️ @encvgo/components 未链接，pnpm install 重试"
  cd "${MOBILE_DIR}"
  pnpm install --no-frozen-lockfile
  cd "${WEB_DIR}"
fi
if [[ -L "${WEB_DIR}/node_modules/@encvgo/components" ]]; then
  echo "    ✅ @encvgo/components → $(readlink -f "${WEB_DIR}/node_modules/@encvgo/components")"
else
  echo "    ✅ @encvgo/components 已存在"
fi

# ---- Step 3: 启动 Vite dev server ----
step "3/4 启动 Vite dev server (port ${VITE_PORT})"
cd "${WEB_DIR}"
./node_modules/.bin/vite --host 0.0.0.0 --port "${VITE_PORT}" --strictPort &
VITE_PID=$!
SUBPIDS+=("${VITE_PID}")
echo "    vite pid=${VITE_PID}"

# 等待 Vite 就绪
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
  if curl -s "http://localhost:${VITE_PORT}/" >/dev/null 2>&1; then
    echo "    vite ready (port ${VITE_PORT})"
    break
  fi
  sleep 0.5
done

# ---- Step 4: 状态报告 + OpenPreview 提示 ----
step "4/4 ✅ plugin-openlist/web 预览已启动"
cat <<EOF

========================================
✅ plugin-openlist/web 预览已启动

  端口：    :${VITE_PORT}  = Vite dev server（plugin-openlist/web）

  路由：
     /            → 重定向到 /home
     /home        → OpenListHome（AppBar + 4 工具按钮 + StatusCard + LogList + FAB）
     /config      → OpenListConfigEditor（JSON 编辑器）
     /settings    → OpenListSettings（版本/数据目录占位）
     /webview     → OpenListWebView（需 Android WebView 容器提示）

  ⚠️ 沙箱浏览器限制：
     - window.OpenListNative 不存在（仅 Android WebView 注入）
     - 所有 Native 调用走 safe() fallback → 显示默认态「未安装/已停止」
     - 目标：UI 视觉预览 + HMR 实时迭代
     - 真机联调：需在主 app 内通过 plugin-openlist Content() 加载

  用户访问地址（必须先 OpenPreview 激活）：
     http://localhost:${VITE_PORT}/

  ⚠️ 重要：必须使用 OpenPreview 工具激活预览才能外部访问
     OpenPreview(command_id="<本脚本 command_id>", preview_url="http://localhost:${VITE_PORT}/")

  配套工具：
     - 与主 app 预览不冲突：bash scripts/start-preview.sh
     - 与 OpenList SPA 预览不冲突：bash scripts/dev-openlist.sh

  停止:  Ctrl+C  （脚本会自动清理所有子进程）

  修改 plugin-openlist/web/src/** 任意文件 → 浏览器自动 HMR
========================================
EOF

# ---- 保持前台运行 ----
echo "    vite pid=${VITE_PID}"
echo "    等待子进程..."

wait -n "${SUBPIDS[@]}" 2>/dev/null || true
echo ""
echo "==> 某个子进程退出，触发清理..."
cleanup
