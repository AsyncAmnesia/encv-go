#!/usr/bin/env bash
# ENCV Capacitor 预览一键启动
# 铁律：
#   1. 整合 mock 数据生成 + 后端 air 监视 + Vite 前端为一条命令
#   2. 后端必须用 air 监视重载（禁止 go build / 手动 go run）
#   3. 不修改 config.user.json —— servingDir 永远为 /storage/emulated/0
#   4. 严禁任何符号链接 —— mock-data 真实目录在 /storage/emulated/0
#   5. 严禁误杀 agent-tool-host —— 它在 :5173（沙箱基础设施，反向代理到 vite）
#   6. Vite 必须使用 --strictPort 失败时由 agent-tool-host 接管端口
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MOBILE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${MOBILE_DIR}/../.." && pwd)"

# 确保 air 在 PATH 中（mise 安装的 Go 自带 air，但不在标准 PATH）
export PATH="/root/.local/share/mise/installs/go/1.25.1/bin:${PATH}"

BACKEND_PORT="${ENCV_MOBILE_PORT:-2025}"
MOCK_DIR="${ENCV_MOCK_ROOT:-/storage/emulated/0}"

# 沙箱中 Vite 端口：5173 已被 agent-tool-host 占用，必须从 5174+ 开始
# agent-tool-host 会反向代理到 vite 实际监听的端口
VITE_PORT="${ENCV_VITE_PORT:-5174}"

cd "${REPO_ROOT}"

step() { echo ""; echo "==> $*"; }

# ---------- Step 0: 停止残留 ENCV 进程（精确到进程名，绝不碰 agent-tool-host） ----------
step "0/5 停止残留 ENCV 进程（不碰 agent-tool-host）"

# 仅杀 air 进程（go run 不会产生 air 进程）
pkill -x air 2>/dev/null && echo "    killed air" || true

# 杀后端二进制（air 启动的是 ./tmp/encv，进程名通常显示为 encv）
pkill -f '^./tmp/encv' 2>/dev/null && echo "    killed ./tmp/encv" || true
pkill -f '/tmp/encv start' 2>/dev/null && echo "    killed /tmp/encv start" || true

# 杀 Vite 进程（特征：vite 或 vite.config.mts）
# 注意：严禁用 lsof -i :5173 杀进程！那是 agent-tool-host！
pkill -f 'node.*vite' 2>/dev/null && echo "    killed vite" || true

# 杀端口 2025 上的进程（后端）
BACKEND_PIDS="$(lsof -ti :"${BACKEND_PORT}" 2>/dev/null || true)"
if [[ -n "${BACKEND_PIDS}" ]]; then
  # 排除 agent-tool-host（沙箱基础设施，CMD 通常含 agent-tool-host）
  for pid in ${BACKEND_PIDS}; do
    if ! grep -q 'agent-tool-host' "/proc/${pid}/cmdline" 2>/dev/null; then
      kill "${pid}" 2>/dev/null && echo "    killed backend pid=${pid}"
    fi
  done
fi

# ⚠️ 严禁：lsof -ti :5173 | xargs kill —— 那会杀 agent-tool-host！
# Vite 端口会漂移，不需要按端口杀，下次启动 vite 会复用之前端口

sleep 1

# ---------- Step 1: 确保 node_modules 就绪（vite 必须在 node_modules/vite） ----------
step "1/5 确保 ${MOBILE_DIR}/node_modules 就绪（走 MCP 代理）"
cd "${MOBILE_DIR}"
if [[ ! -d "node_modules/vite" ]]; then
  echo "    node_modules 缺失，npm install ..."
  # NODE_OPTIONS 自动注入 MCP 代理，npm 走 :18080 出网
  npm install --no-audit --no-fund --prefer-offline
fi
cd "${REPO_ROOT}"

# ---------- Step 2: 生成 mock 数据（脚本会自建 /storage/emulated/0） ----------
step "2/5 生成 mock 数据到 ${MOCK_DIR}"
cd "${MOBILE_DIR}"
npx tsx scripts/generate-mock-files.ts
cd "${REPO_ROOT}"

# 验证 service guard 标记目录存在
if [[ ! -d "${MOCK_DIR}/01-plain-media" ]]; then
  echo "❌ 错误：mock 生成后仍缺少 ${MOCK_DIR}/01-plain-media 标记目录" >&2
  exit 1
fi

# ---------- Step 3: air 启动后端 ----------
step "3/5 启动后端（air 监视重载，ENCV_DEV_PREVIEW=1）"
nohup env ENCV_DEV_PREVIEW=1 air > /tmp/encv-air.log 2>&1 &
AIR_PID=$!
echo "    air pid=${AIR_PID}  log=/tmp/encv-air.log"

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24; do
  if curl -s "http://localhost:${BACKEND_PORT}/api/health" >/dev/null 2>&1; then
    echo "    backend ready (port ${BACKEND_PORT})"
    break
  fi
  sleep 0.5
done

# ---------- Step 4: Vite 前端（从 5174+ 开始，agent-tool-host 已占 5173） ----------
step "4/5 启动 Vite 前端（port ${VITE_PORT}，agent-tool-host 反代）"
cd "${MOBILE_DIR}"

# 使用本地 vite（不是 npx 拉远程最新）
nohup ./node_modules/.bin/vite --host 0.0.0.0 --port "${VITE_PORT}" --strictPort > /tmp/encv-vite.log 2>&1 &
VITE_PID=$!
echo "    vite pid=${VITE_PID}  log=/tmp/encv-vite.log"

# 等待 Vite 就绪
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
  if curl -s "http://localhost:${VITE_PORT}/" >/dev/null 2>&1; then
    echo "    vite ready (port ${VITE_PORT})"
    break
  fi
  sleep 0.5
done

# ---------- Step 5: 状态报告 ----------
step "5/5 完成"
cat <<EOF

========================================
✅ ENCV 预览已启动

  ⚠️  沙箱端口身份：
     :5173     = agent-tool-host  （沙箱基础设施，用户访问入口）
     :${VITE_PORT}    = vite dev server  （agent-tool-host 反代到此）
     :${BACKEND_PORT}     = Go Backend (air 监视重载)

  用户访问地址（agent-tool-host 代理）：
     http://localhost:5173/  ← 永远访问这个，不要访问 vite 原始端口

  内部直连（仅调试用）：
     http://localhost:${VITE_PORT}/    ← vite 原始端口
     http://localhost:${BACKEND_PORT}/  ← 后端 API

  config:       ${REPO_ROOT}/config.user.json （未修改）
  servingDir:   ${MOCK_DIR}  （设计预期路径，脚本自建）

  air 热重载日志:  tail -f /tmp/encv-air.log
  vite 日志:       tail -f /tmp/encv-vite.log
  停止:            pkill -x air
                   pkill -f 'node.*vite'
                   pkill -f '/tmp/encv start'

  ⚠️  严禁：lsof -ti :5173 | xargs kill  （会杀 agent-tool-host！）

  后续上传测试文件（hyYGPCwJPQ3+xrdAvfnn2.bin）：
    - 浏览器访问 http://localhost:5173/
    - Files 页面 → Upload FAB → 选择文件
========================================
EOF
