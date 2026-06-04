#!/usr/bin/env bash
# ENCV Capacitor 预览一键启动
# 铁律：
#   1. 整合 mock 数据生成 + 后端 air 监视 + Vite 前端为一条命令
#   2. 后端必须用 air 监视重载（禁止 go build / 手动 go run）
#   3. 不修改 config.user.json —— servingDir 永远为 /storage/emulated/0
#   4. 严禁任何符号链接 —— mock-data 真实目录在 /storage/emulated/0
#   5. 自动检测端口占用：5173 被占时自动回退到 5174
#   6. 脚本必须保持前台运行（nohup 进程由脚本管理），便于 OpenPreview 激活
#   7. 脚本退出时优雅停止所有子进程
set -euo pipefail
shopt -s lastpipe

# ---- 模式开关 ----
#   ENCV_DETACH=0 (默认): 脚本前台 wait 阻塞，Ctrl+C 触发 cleanup
#                        适合本地开发终端
#   ENCV_DETACH=1:        air/vite 用 setsid+nohup 完全脱离脚本进程组
#                        脚本验证端口就绪后立即 exit 0
#                        适合沙箱/CI：tool call 不会被永久阻塞
#                        停止命令: pkill -x air ; pkill -f 'node.*vite'
DETACH="${ENCV_DETACH:-0}"

# ---- 信号陷阱：脚本退出时杀掉所有子进程 ----
SUBPIDS=()
cleanup() {
  echo ""
  echo "==> 收到退出信号，停止所有子进程..."
  for pid in "${SUBPIDS[@]}"; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" 2>/dev/null || true
    fi
  done
  sleep 1
  # 强制清理（air 还会启动 ./tmp/encv 子进程）
  pkill -P $$ 2>/dev/null || true
  pkill -x air 2>/dev/null || true
  pkill -f 'node.*vite' 2>/dev/null || true
  pkill -f 'Hi-Sillot-OpenList-Frontend.*vite' 2>/dev/null || true
  exit 0
}
# 分离模式：禁止 trap 让子进程在脚本退出后继续存活
if [[ "${DETACH}" == "1" ]]; then
  trap - INT TERM
else
  trap cleanup INT TERM
fi

# ---- 后台启动封装 ----
# detach=1: setsid+nohup 让子进程脱离 ctrl-c / 父进程组
# detach=0: 简单 & 子进程，与脚本同进程组
spawn_bg() {
  local logfile="$1"
  shift
  if [[ "${DETACH}" == "1" ]]; then
    setsid nohup "$@" </dev/null >>"${logfile}" 2>&1 &
  else
    "$@" &
  fi
  echo $!
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MOBILE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${MOBILE_DIR}/../.." && pwd)"

# 确保 air 在 PATH 中（mise 安装的 Go 自带 air，但不在标准 PATH）
export PATH="/root/.local/share/mise/installs/go/1.25.1/bin:${PATH}"

BACKEND_PORT="${ENCV_MOBILE_PORT:-2025}"
MOCK_DIR="${ENCV_MOCK_ROOT:-/storage/emulated/0}"

# 默认使用 5173（Vite 标准端口），被占用时自动回退到 5174
_VITE_PORT_DEFAULT="${ENCV_VITE_PORT:-5174}"
if lsof -i :5173 >/dev/null 2>&1; then
  echo "    :5173 已被占用，使用 :5174"
  VITE_PORT="${_VITE_PORT_DEFAULT}"
else
  VITE_PORT="5173"
fi

cd "${REPO_ROOT}"

step() { echo ""; echo "==> $*"; }

# ---------- Step 0: 停止残留 ENCV 进程 ----------
step "0/7 停止残留 ENCV 进程（端口 + 进程名双保险）"
# 进程名兜底
pkill -x air 2>/dev/null && echo "    killed air" || true
pkill -f 'tmp/encv' 2>/dev/null && echo "    killed tmp/encv" || true
pkill -f 'node.*vite' 2>/dev/null && echo "    killed vite" || true
pkill -f 'Hi-Sillot-OpenList-Frontend.*vite' 2>/dev/null && echo "    killed openlist vite" || true
# 端口兜底：按 BACKEND_PORT 强制杀掉残留（即使进程名匹配规则漏了，比如 /workspace/tmp/encv start）
BACKEND_PIDS="$(lsof -ti :"${BACKEND_PORT}" 2>/dev/null || true)"
if [[ -n "${BACKEND_PIDS}" ]]; then
  for pid in ${BACKEND_PIDS}; do
    kill "${pid}" 2>/dev/null && echo "    killed backend pid=${pid} (port ${BACKEND_PORT})"
  done
  sleep 1
  # 二次确认（部分进程是 setsid+nohup 起的，父进程死子进程未必死）
  BACKEND_PIDS="$(lsof -ti :"${BACKEND_PORT}" 2>/dev/null || true)"
  if [[ -n "${BACKEND_PIDS}" ]]; then
    for pid in ${BACKEND_PIDS}; do
      kill -9 "${pid}" 2>/dev/null && echo "    force-killed backend pid=${pid} (port ${BACKEND_PORT})"
    done
  fi
fi
sleep 1

# ---------- Step 1: 确保 node_modules 就绪 ----------
step "1/6 确保 ${MOBILE_DIR}/node_modules 就绪（走 MCP 代理）"
cd "${MOBILE_DIR}"
if [[ ! -d "node_modules/vite" ]]; then
  echo "    node_modules 缺失，npm install ..."
  npm install --no-audit --no-fund --prefer-offline
fi
cd "${REPO_ROOT}"

# ---------- Step 2: 生成 mock 数据 ----------
step "2/7 生成 mock 数据到 ${MOCK_DIR}"
cd "${MOBILE_DIR}"
npx tsx scripts/generate-mock-files.ts
cd "${REPO_ROOT}"

if [[ ! -d "${MOCK_DIR}/01-plain-media" ]]; then
  echo "❌ 错误：mock 生成后仍缺少 ${MOCK_DIR}/01-plain-media 标记目录" >&2
  exit 1
fi

# ---------- Step 3: air 启动后端（前台子进程） ----------
step "3/6 启动后端（air 监视重载，ENCV_DEV_PREVIEW=1）"
cd "${REPO_ROOT}"
# 把当前实际占用的 vite 端口告诉 dev_preview_proxy（$VITE_PORT 在上方已探测过）。
export ENCV_DEV_MOBILE_VITE_URL="http://127.0.0.1:${VITE_PORT}"
export ENCV_DEV_OPENLIST_VITE_URL="http://127.0.0.1:3000"
AIR_PID=$(spawn_bg /tmp/encv-air.log env ENCV_DEV_PREVIEW=1 ENCV_DEV_MOBILE_VITE_URL="${ENCV_DEV_MOBILE_VITE_URL}" ENCV_DEV_OPENLIST_VITE_URL="${ENCV_DEV_OPENLIST_VITE_URL}" air)
SUBPIDS+=("${AIR_PID}")
echo "    air pid=${AIR_PID}"

# 等待后端就绪
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24; do
  if curl -s "http://localhost:${BACKEND_PORT}/api/config" >/dev/null 2>&1; then
    echo "    backend ready (port ${BACKEND_PORT})"
    break
  fi
  sleep 0.5
done

# 验证 mobile overlay 生效：servingDir 必须包含 01-plain-media
# 这是 2026-06-04 修复的痛点：之前 tmp/encv 手工启动无 ENCV_DEV_PREVIEW=1，
# mobile overlay 未触发，server.dir 留在默认的 "/" → 解析为 /workspace → 看到 .md 等文件
SERVING_GUARD_OK=0
for i in 1 2 3 4 5 6 7 8 9 10; do
  GUARD_JSON=$(curl -s "http://localhost:${BACKEND_PORT}/api/service-guard" 2>/dev/null || true)
  if echo "${GUARD_JSON}" | grep -q '"ready":true'; then
    SERVING_DIR=$(echo "${GUARD_JSON}" | grep -oE '"servingDir":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "    ✅ service-guard OK: servingDir=${SERVING_DIR}"
    SERVING_GUARD_OK=1
    break
  fi
  sleep 0.5
done
if [[ "${SERVING_GUARD_OK}" != "1" ]]; then
  echo ""
  echo "❌ 错误：后端 service-guard 校验失败（10s 内未 ready）" >&2
  echo "   这通常意味着 mobile overlay (ENCV_DEV_PREVIEW=1) 没生效" >&2
  echo "   检查: ps -ef | grep -E 'air|tmp/encv' | grep -v grep" >&2
  echo "   检查: tail -20 /tmp/encv-air.log" >&2
  echo "   手工验证: curl -s http://localhost:${BACKEND_PORT}/api/service-guard | head -c 500" >&2
  echo ""
  curl -s "http://localhost:${BACKEND_PORT}/api/service-guard" | head -c 500
  echo ""
  exit 1
fi

# ---------- Step 4: Vite 前端（前台子进程） ----------
step "4/7 启动 Vite 前端（port ${VITE_PORT}）"
cd "${MOBILE_DIR}"
VITE_PID=$(spawn_bg /tmp/encv-vite.log ./node_modules/.bin/vite --host 0.0.0.0 --port "${VITE_PORT}" --strictPort)
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

# ---------- Step 5: OpenList 前端 dev server（沙箱预览 OpenList UI 入口） ----------
step "5/7 启动 OpenList 前端 vite dev server（port 3000，沙箱预览用）"
OPENLIST_DIR="${REPO_ROOT}/app/openlist/Hi-Sillot-OpenList-Frontend"
if [[ -d "${OPENLIST_DIR}" ]] && [[ -f "${OPENLIST_DIR}/vite.config.ts" ]]; then
  OPENLIST_PID=$(spawn_bg /tmp/encv-openlist.log env OPENLIST_PREVIEW_BASE="/openlist-ui/" OPENLIST_NO_HMR=1 pnpm --dir "${OPENLIST_DIR}" dev --host 127.0.0.1 --port 3000 --strictPort)
  SUBPIDS+=("${OPENLIST_PID}")
  echo "    openlist vite pid=${OPENLIST_PID}"
  # 等待 OpenList vite 就绪
  for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    if curl -s "http://127.0.0.1:3000/openlist-ui/" >/dev/null 2>&1; then
      echo "    openlist vite ready (port 3000, base=/openlist-ui/)"
      break
    fi
    sleep 0.5
  done
else
  echo "    ⚠️  ${OPENLIST_DIR} 不存在，跳过 OpenList dev 预览（沙箱预览 OpenList UI 入口将 302 到 503）"
fi

# ---------- Step 6: 状态报告 + OpenPreview 提示 ----------
step "6/7 ✅ 服务全部就绪"
cat <<EOF

========================================
✅ ENCV 预览已启动

  端口分配：
     :${VITE_PORT}  = Vite dev server（前端，用户直接访问）
     :${BACKEND_PORT} = Go Backend（air 监视重载）
     :3000     = OpenList 前端 vite dev server（沙箱预览 OpenList UI）

  用户访问地址（必须先 OpenPreview 激活）：
     http://localhost:${VITE_PORT}/
     http://localhost:${BACKEND_PORT}/    ← 现在也是合法的入口（dev_preview_proxy 兜底到 :${VITE_PORT}）

  沙箱预览 OpenList UI（dev 模式）：
     设置 → 开发者工具 → 沙箱预览 → 预览 OpenList 前端
     或直接:  http://localhost:${BACKEND_PORT}/api/preview/openlist-ui
     (后端 302 → /openlist-ui/ → dev_preview_proxy → :3000 openlist vite 独立)

  ⚠️ 重要：必须使用 OpenPreview 工具激活预览才能外部访问（注册 2025 端口，不再注册 5173）
     OpenPreview(command_id="<本脚本 command_id>", preview_url="http://localhost:${BACKEND_PORT}/")

  HMR 说明（沙箱预览下被刻意关掉）：
     16000 agent-tool-host 不支持 WebSocket 升级，@vite/client 连不上 WS 会刷错误。
     start-preview.sh 启的两个 vite 都设了 HMR=off 走 16000 沙箱。
     本地直连 5173 / 3000 时：杀掉 start-preview.sh 启的 vite，独立起就有 HMR。

  配置文件:    ${REPO_ROOT}/config.user.json （未修改）
  servingDir:  ${MOCK_DIR}  （设计预期路径，脚本自建）

  停止:  Ctrl+C  （脚本会自动清理所有子进程）

  后续上传测试文件（hyYGPCwJPQ3+xrdAvfnn2.bin）：
    - 浏览器访问 http://localhost:${VITE_PORT}/  （前提：OpenPreview 已激活）
    - Files 页面 → Upload FAB → 选择文件
========================================
EOF

# ---------- Step 7: 保持前台运行（等待子进程或信号） ----------
if [[ "${DETACH}" == "1" ]]; then
  step "7/7 ✅ detached 模式，脚本退出（子进程继续运行）"
  cat <<EOF

========================================
✅ ENCV 预览已启动（detach 模式）

  端口分配：
     :${VITE_PORT}  = Vite dev server（前端，用户直接访问）
     :${BACKEND_PORT} = Go Backend（air 监视重载）
     :3000     = OpenList 前端 vite dev server（沙箱预览 OpenList UI）

  PIDs:   air=${AIR_PID}  vite=${VITE_PID}  openlist=${OPENLIST_PID:-N/A}
  日志:   /tmp/encv-air.log  /tmp/encv-vite.log  /tmp/encv-openlist.log
  停止:   pkill -x air ; pkill -f 'node.*vite' ; pkill -f 'Hi-Sillot-OpenList-Frontend'

  ⚠️  OpenPreview 必须在脚本退出前激活（端口已就绪）
     OpenPreview(preview_url="http://localhost:${BACKEND_PORT}/")
========================================
EOF
  # 显式 exit 0，跳过原 wait -n 阻塞段
  exit 0
fi

step "7/7 保持前台运行（按 Ctrl+C 停止）"
echo "    air pid=${AIR_PID}  vite pid=${VITE_PID}  openlist pid=${OPENLIST_PID:-N/A}"
echo "    等待子进程..."

# 等待任何子进程退出
wait -n "${SUBPIDS[@]}" 2>/dev/null || true
echo ""
echo "==> 某个子进程退出，触发清理..."
cleanup
