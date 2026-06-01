#!/usr/bin/env bash
# ENCV Capacitor 预览一键启动
# 铁律：
#   1. 整合 mock 数据生成 + 后端 air 监视 + Vite 前端为一条命令
#   2. 后端必须用 air 监视重载（禁止 go build）
#   3. 不修改 config.user.json —— servingDir 永远为 /storage/emulated/0
#   4. 严禁任何符号链接 —— mock-data 真实目录在 /storage/emulated/0
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MOBILE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${MOBILE_DIR}/../.." && pwd)"

# 确保 air 在 PATH 中（mise 安装的 Go 自带 air，但不在标准 PATH）
export PATH="/root/.local/share/mise/installs/go/1.25.1/bin:${PATH}"

BACKEND_PORT="${ENCV_MOBILE_PORT:-2025}"
FRONTEND_PORT="${ENCV_VITE_PORT:-5173}"
MOCK_DIR="${ENCV_MOCK_ROOT:-/storage/emulated/0}"

cd "${REPO_ROOT}"

step() { echo ""; echo "==> $*"; }

# ---------- Step 0: 停止残留进程（按端口精确） ----------
step "0/4 停止残留 ENCV 进程"
BACKEND_PIDS="$(lsof -ti :"${BACKEND_PORT}" 2>/dev/null || true)"
FRONTEND_PIDS="$(lsof -ti :"${FRONTEND_PORT}" 2>/dev/null || true)"
AIR_PIDS="$(pgrep -x air 2>/dev/null || true)"
[[ -n "${BACKEND_PIDS}" ]] && kill ${BACKEND_PIDS} 2>/dev/null && echo "    killed backend: ${BACKEND_PIDS}"
[[ -n "${FRONTEND_PIDS}" ]] && kill ${FRONTEND_PIDS} 2>/dev/null && echo "    killed vite:    ${FRONTEND_PIDS}"
[[ -n "${AIR_PIDS}" ]] && kill ${AIR_PIDS} 2>/dev/null && echo "    killed air:     ${AIR_PIDS}"
sleep 1

# ---------- Step 1: 生成 mock 数据（脚本会自建 /storage/emulated/0） ----------
step "1/4 生成 mock 数据到 ${MOCK_DIR}"
cd "${MOBILE_DIR}"
npx tsx scripts/generate-mock-files.ts
cd "${REPO_ROOT}"

# 验证 service guard 标记文件存在
if [[ ! -d "${MOCK_DIR}/01-plain-media" ]]; then
  echo "❌ 错误：mock 生成后仍缺少 ${MOCK_DIR}/01-plain-media 标记目录" >&2
  exit 1
fi

# ---------- Step 2: air 启动后端 ----------
step "2/4 启动后端（air 监视重载，ENCV_DEV_PREVIEW=1）"
nohup env ENCV_DEV_PREVIEW=1 air > /tmp/encv-air.log 2>&1 &
AIR_PID=$!
echo "    air pid=${AIR_PID}  log=/tmp/encv-air.log"

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24; do
  if curl -s "http://localhost:${BACKEND_PORT}/api/service-guard" >/dev/null 2>&1; then
    echo "    backend ready (port ${BACKEND_PORT})"
    break
  fi
  sleep 0.5
done

# ---------- Step 3: Vite 前端 ----------
step "3/4 启动 Vite 前端（port ${FRONTEND_PORT}）"
cd "${MOBILE_DIR}"
nohup npx vite --host 0.0.0.0 --port "${FRONTEND_PORT}" > /tmp/encv-vite.log 2>&1 &
VITE_PID=$!
echo "    vite pid=${VITE_PID}  log=/tmp/encv-vite.log"

# ---------- Step 4: 状态报告 ----------
step "4/4 完成"
cat <<EOF

========================================
✅ ENCV 预览已启动

  前端:  http://localhost:${FRONTEND_PORT}/
  后端:  http://localhost:${BACKEND_PORT}/

  config:       ${REPO_ROOT}/config.user.json （未修改）
  servingDir:   ${MOCK_DIR}  （设计预期路径，脚本自建）

  air 热重载日志:  tail -f /tmp/encv-air.log
  vite 日志:       tail -f /tmp/encv-vite.log
  停止:            lsof -ti :${BACKEND_PORT} -ti :${FRONTEND_PORT} | xargs kill
                   pkill -x air

  后续上传测试文件（hyYGPCwJPQ3+xrdAvfnn2.bin）：
    - Files 页面 → Upload FAB → 选择文件
    - 或直接放到 ${MOCK_DIR}/ 目录
========================================
EOF
