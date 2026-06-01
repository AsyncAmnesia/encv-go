#!/usr/bin/env bash
# ENCV Capacitor 预览一键启动
# 铁律：禁用任何符号链接；servingDir 直接指向项目内 mock-data 真实目录
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
MOCK_DIR="${REPO_ROOT}/app/encv-mobile/mock-data"
BACKEND_BIN="${ENCV_MOBILE_BIN:-/tmp/encv-mobile}"
MOBILE_PORT="${ENCV_MOBILE_PORT:-2025}"
FRONTEND_PORT="${ENCV_VITE_PORT:-5173}"

cd "${REPO_ROOT}"

step() { echo ""; echo "==> $*"; }

step "1/3 停止残留进程"
pkill -f "${BACKEND_BIN}\$" 2>/dev/null || true
pkill -f "vite" 2>/dev/null || true
sleep 1

step "2/3 生成 mock 数据到 ${MOCK_DIR}"
cd "${REPO_ROOT}/app/encv-mobile"
npx tsx scripts/generate-mock-files.ts
cd "${REPO_ROOT}"

# 验证 service guard 标记文件存在
if [[ ! -f "${MOCK_DIR}/01-plain-media" ]]; then
  echo "❌ 错误：mock 生成后仍缺少 01-plain-media 标记文件" >&2
  exit 1
fi

step "3/3 启动后端 (port=${MOBILE_PORT}) + Vite (port=${FRONTEND_PORT})"

# 启动后端（mobile overlay 自动启用）
ENCV_DEV_PREVIEW=1 nohup "${BACKEND_BIN}" > /tmp/encv-mobile.log 2>&1 &
BACKEND_PID=$!
echo "    backend pid=${BACKEND_PID}  log=/tmp/encv-mobile.log"

# 等待后端就绪
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -s "http://localhost:${MOBILE_PORT}/api/service-guard" >/dev/null 2>&1; then
    echo "    backend ready (port ${MOBILE_PORT})"
    break
  fi
  sleep 0.5
done

# 启动 Vite
cd "${REPO_ROOT}/app/encv-mobile"
nohup npx vite --host 0.0.0.0 --port "${FRONTEND_PORT}" > /tmp/encv-vite.log 2>&1 &
VITE_PID=$!
echo "    vite pid=${VITE_PID}    log=/tmp/encv-vite.log"

cat <<EOF

========================================
✅ ENCV 预览已启动

  前端:  http://localhost:${FRONTEND_PORT}/
  后端:  http://localhost:${MOBILE_PORT}/

  mock 根目录: ${MOCK_DIR}
  配置文件:    ${REPO_ROOT}/config.user.json

  停止:  pkill -f ${BACKEND_BIN}; pkill -f vite
  日志:  tail -f /tmp/encv-mobile.log /tmp/encv-vite.log
========================================
EOF
