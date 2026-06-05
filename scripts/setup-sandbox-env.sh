#!/usr/bin/env bash
# =============================================================================
# setup-sandbox-env.sh
# -----------------------------------------------------------------------------
# 一键拉起 ENCV / OpenList / encv-mobile 沙箱 dev 环境。
#
# 包含：
#   ① 装 Go 工具：air (live reload) + 装 kotlinc
#   ② clone OpenList 后端 fork (Hi-Sillot-OpenList, dev 分支)
#   ③ clone OpenList 前端 fork (Hi-Sillot-OpenList-Frontend, main 分支)
#   ④ 构建前端 dist（dev-openlist.sh 会优先用本地 dist 而非下载 release）
#   ⑤ pnpm install encv-mobile
#   ⑥ 后台启 encv-go (air 监视) → :2025
#   ⑦ 后台启 Vite encv-mobile → :8100 (或 :5173/5174)
#   ⑧ 后台启 OpenList 真实 fork → :5244
#   ⑨ 后台启 plugin-openlist Vite → :5174/5175
#
# 退出码:
#   0  = 全部就绪
#   1  = 前置依赖缺失
#   2  = 网络/克隆失败
#   3  = pnpm install / fork build 失败
#   4  = 端口已被占用且未回退
# =============================================================================
set -uo pipefail   # 不开 -e，因为 pnpm install / go install 可能返回非零但仍能继续

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MOBILE_DIR="${REPO_ROOT}/app/encv-mobile"
OPENLIST_ROOT="${REPO_ROOT}/app/openlist"
FRONTEND_FORK_DIR="${OPENLIST_ROOT}/Hi-Sillot-OpenList-Frontend"
BACKEND_FORK_DIR="${OPENLIST_ROOT}/Hi-Sillot-OpenList"

# ---- 颜色 ----
R='\033[1;31m'; G='\033[1;32m'; Y='\033[1;33m'; C='\033[1;36m'; N='\033[0m'
log()  { printf "${C}[setup]${N} %s\n" "$*"; }
ok()   { printf "${G}[setup]${N} %s\n" "$*"; }
warn() { printf "${Y}[setup]${N} %s\n" "$*" >&2; }
err()  { printf "${R}[setup]${N} %s\n" "$*" >&2; }
step() { echo ""; printf "${C}==>${N} %s\n" "$*"; }

# ---- 端口 ----
BACKEND_PORT=2025
OPENLIST_PORT=5244
WEB_PORT=5174
ENCV_MOBILE_PORT="${ENCV_MOBILE_PORT:-${BACKEND_PORT}}"

FAILED=0

# ============================================================================
# 步骤 0: 前置检查
# ============================================================================
step "0/9 前置检查（go / node / pnpm / git / curl）"
for cmd in go node npm pnpm git curl; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    err "❌ 缺少命令: $cmd"
    exit 1
  fi
done
ok "基础工具链全部就绪"

# ============================================================================
# 步骤 1: 装 Go 工具 (air) + Kotlin 工具链
# ============================================================================
step "1/9 装 Go 工具链 (air) 和 Kotlin 工具链 (kotlinc)"

# 1a. air (Go live reload) — start-preview.sh 强依赖
if command -v air >/dev/null 2>&1; then
  ok "air 已安装: $(air -v 2>&1 | head -1)"
else
  log "安装 air (cosmtrek/air) ..."
  GOBIN="$(go env GOPATH)/bin"
  mkdir -p "$GOBIN"
  if go install github.com/cosmtrek/air@latest 2>&1 | tail -5; then
    if [[ -x "${GOBIN}/air" ]]; then
      ok "air 安装成功: ${GOBIN}/air"
      # 确保 PATH 包含 GOBIN
      export PATH="${GOBIN}:${PATH}"
    else
      warn "go install 返回 0 但 ${GOBIN}/air 不存在，跳过"
      FAILED=$((FAILED+1))
    fi
  else
    warn "air 安装失败，start-preview.sh 启动时会卡住"
    FAILED=$((FAILED+1))
  fi
fi

# 1b. kotlinc
if command -v kotlinc >/dev/null 2>&1; then
  ok "kotlinc 已安装: $(kotlinc -version 2>&1 | head -1)"
else
  if [[ -x "${REPO_ROOT}/.trae/scripts/setup-kotlinc.sh" ]]; then
    log "运行 setup-kotlinc.sh ..."
    if bash "${REPO_ROOT}/.trae/scripts/setup-kotlinc.sh" 2>&1 | tail -10; then
      ok "kotlinc 安装完成"
    else
      warn "kotlinc 安装失败"
      FAILED=$((FAILED+1))
    fi
  else
    warn "未找到 .trae/scripts/setup-kotlinc.sh，跳过"
  fi
fi

# ============================================================================
# 步骤 2: clone OpenList 后端 fork
# ============================================================================
step "2/9 clone OpenList 后端 fork (Hi-Sillot-OpenList, dev 分支)"

if [[ -d "${BACKEND_FORK_DIR}/.git" ]]; then
  ok "后端 fork 已存在: ${BACKEND_FORK_DIR}"
else
  log "git clone Hi-Sillot/OpenList (dev) ..."
  cd "${OPENLIST_ROOT}"
  if git clone --depth 1 --branch dev \
       https://github.com/Hi-Sillot/OpenList.git \
       Hi-Sillot-OpenList 2>&1 | tail -5; then
    ok "后端 fork clone 完成"
  else
    err "后端 fork clone 失败"
    FAILED=$((FAILED+1))
  fi
fi

# ============================================================================
# 步骤 3: clone OpenList 前端 fork
# ============================================================================
step "3/9 clone OpenList 前端 fork (Hi-Sillot-OpenList-Frontend, main 分支)"

if [[ -d "${FRONTEND_FORK_DIR}/.git" ]]; then
  ok "前端 fork 已存在: ${FRONTEND_FORK_DIR}"
else
  log "git clone Hi-Sillot/OpenList-Frontend (main) ..."
  cd "${OPENLIST_ROOT}"
  if git clone --depth 1 --branch main \
       https://github.com/Hi-Sillot/OpenList-Frontend.git \
       Hi-Sillot-OpenList-Frontend 2>&1 | tail -5; then
    ok "前端 fork clone 完成"
  else
    err "前端 fork clone 失败"
    FAILED=$((FAILED+1))
  fi
fi

# ============================================================================
# 步骤 4: 构建前端 dist
# ============================================================================
step "4/9 构建前端 fork 的 dist (Hi-Sillot-OpenList-Frontend/dist/)"

if [[ -f "${FRONTEND_FORK_DIR}/dist/index.html" ]]; then
  ok "前端 dist 已构建: ${FRONTEND_FORK_DIR}/dist/"
else
  if [[ ! -d "${FRONTEND_FORK_DIR}/node_modules" ]]; then
    log "pnpm install (前端 fork) ..."
    cd "${FRONTEND_FORK_DIR}"
    if pnpm install --prefer-offline 2>&1 | tail -10; then
      ok "前端 fork 依赖安装完成"
    else
      warn "前端 fork pnpm install 失败，将 fallback 到 release tarball"
      FAILED=$((FAILED+1))
    fi
  fi
  log "构建前端 dist ..."
  cd "${FRONTEND_FORK_DIR}"
  if pnpm build 2>&1 | tail -10; then
    if [[ -f "${FRONTEND_FORK_DIR}/dist/index.html" ]]; then
      ok "前端 dist 构建完成"
    else
      warn "构建脚本退出 0 但 dist/index.html 不存在"
      FAILED=$((FAILED+1))
    fi
  else
    warn "前端 dist 构建失败，将 fallback 到 release tarball"
    FAILED=$((FAILED+1))
  fi
fi

# ============================================================================
# 步骤 5: pnpm install encv-mobile
# ============================================================================
step "5/9 pnpm install encv-mobile + plugin-openlist/web"

if [[ -d "${MOBILE_DIR}/node_modules/vite" ]]; then
  ok "encv-mobile node_modules 已就绪"
else
  log "pnpm install encv-mobile ..."
  cd "${MOBILE_DIR}"
  if pnpm install --prefer-offline 2>&1 | tail -10; then
    ok "encv-mobile 依赖安装完成"
  else
    err "encv-mobile pnpm install 失败"
    FAILED=$((FAILED+1))
  fi
fi

if [[ -d "${MOBILE_DIR}/plugin-openlist/web/node_modules/vite" ]]; then
  ok "plugin-openlist/web node_modules 已就绪"
else
  log "pnpm install plugin-openlist/web ..."
  cd "${MOBILE_DIR}"
  if pnpm install --prefer-offline --filter '@encvgo/plugin-openlist-web...' 2>&1 | tail -10; then
    ok "plugin-openlist/web 依赖安装完成"
  else
    warn "plugin-openlist/web pnpm install 失败（5174 Vite 跑不起来）"
    FAILED=$((FAILED+1))
  fi
fi

# ============================================================================
# 步骤 6: 后台启 encv-go (2025) - 用 air 监视重载
# ============================================================================
step "6/9 后台启 encv-go (air 监视重载, :${BACKEND_PORT})"

# 杀掉已有进程
pkill -x air 2>/dev/null && warn "killed old air" || true
pkill -f '/tmp/encv start' 2>/dev/null && warn "killed old encv" || true
sleep 1

# 后台启 air
cd "${REPO_ROOT}"
if command -v air >/dev/null 2>&1; then
  ENCV_DEV_PREVIEW=1 nohup air >/tmp/setup-env-air.log 2>&1 &
  AIR_PID=$!
  log "air pid=${AIR_PID}, 日志: /tmp/setup-env-air.log"

  # 等后端就绪
  for i in $(seq 1 20); do
    if curl -s "http://localhost:${BACKEND_PORT}/api/config" >/dev/null 2>&1; then
      ok "encv-go 就绪 :${BACKEND_PORT} (等 ${i}×0.5s)"
      break
    fi
    sleep 0.5
  done
  if ! curl -s "http://localhost:${BACKEND_PORT}/api/config" >/dev/null 2>&1; then
    warn "encv-go 启动超时（日志最后 20 行：）"
    tail -20 /tmp/setup-env-air.log >&2
    FAILED=$((FAILED+1))
  fi
else
  warn "air 未安装，跳过 encv-go 启动"
  FAILED=$((FAILED+1))
fi

# ============================================================================
# 步骤 7: 后台启 Vite encv-mobile (8100)
# ============================================================================
step "7/9 后台启 Vite encv-mobile (默认 :${WEB_PORT}，被占回退 :5173)"

# 杀旧
pkill -f 'node.*vite' 2>/dev/null && warn "killed old vite" || true
sleep 1

# 端口选择
if lsof -i :5173 >/dev/null 2>&1; then
  ACTUAL_VITE_PORT=5174
else
  ACTUAL_VITE_PORT=5173
fi

cd "${MOBILE_DIR}"
if [[ -x "${MOBILE_DIR}/node_modules/.bin/vite" ]]; then
  nohup ./node_modules/.bin/vite --host 0.0.0.0 --port "${ACTUAL_VITE_PORT}" --strictPort \
    >/tmp/setup-env-vite.log 2>&1 &
  VITE_PID=$!
  log "vite pid=${VITE_PID} :${ACTUAL_VITE_PORT}，日志: /tmp/setup-env-vite.log"

  for i in $(seq 1 20); do
    if curl -s "http://localhost:${ACTUAL_VITE_PORT}/" >/dev/null 2>&1; then
      ok "Vite encv-mobile 就绪 :${ACTUAL_VITE_PORT} (等 ${i}×0.5s)"
      break
    fi
    sleep 0.5
  done
  if ! curl -s "http://localhost:${ACTUAL_VITE_PORT}/" >/dev/null 2>&1; then
    warn "Vite 启动超时（日志最后 20 行：）"
    tail -20 /tmp/setup-env-vite.log >&2
    FAILED=$((FAILED+1))
  fi
else
  warn "vite 未安装，跳过 encv-mobile Vite 启动"
  FAILED=$((FAILED+1))
fi

# ============================================================================
# 步骤 8: 后台启 OpenList 真实 fork (5244)
# ============================================================================
step "8/9 后台启 OpenList 真实 fork (:${OPENLIST_PORT})"

# 杀旧
pkill -f 'Hi-Sillot-OpenList' 2>/dev/null && warn "killed old OpenList" || true
pkill -f 'go run.*Hi-Sillot' 2>/dev/null && warn "killed old go run" || true
sleep 1

if [[ -d "${BACKEND_FORK_DIR}" ]]; then
  cd "${MOBILE_DIR}"
  if [[ -x "${MOBILE_DIR}/scripts/dev-openlist.sh" ]]; then
    nohup bash "${MOBILE_DIR}/scripts/dev-openlist.sh" --port "${OPENLIST_PORT}" \
      >/tmp/setup-env-openlist.log 2>&1 &
    OPENLIST_PID=$!
    log "OpenList 启动中 pid=${OPENLIST_PID}，日志: /tmp/setup-env-openlist.log"

    # 等 OpenList 就绪
    for i in $(seq 1 60); do
      if curl -s "http://localhost:${OPENLIST_PORT}/api/public/settings" >/dev/null 2>&1; then
        ok "OpenList 就绪 :${OPENLIST_PORT} (等 ${i}×0.5s)"
        break
      fi
      sleep 0.5
    done
    if ! curl -s "http://localhost:${OPENLIST_PORT}/api/public/settings" >/dev/null 2>&1; then
      warn "OpenList 启动超时（日志最后 30 行：）"
      tail -30 /tmp/setup-env-openlist.log >&2
      FAILED=$((FAILED+1))
    fi
  else
    warn "dev-openlist.sh 不存在"
    FAILED=$((FAILED+1))
  fi
else
  warn "后端 fork 不存在，跳过 OpenList 启动"
  FAILED=$((FAILED+1))
fi

# ============================================================================
# 步骤 9: 后台启 plugin-openlist Vite (5174)
# ============================================================================
step "9/9 后台启 plugin-openlist Vite (:${WEB_PORT})"

if [[ -d "${MOBILE_DIR}/plugin-openlist/web/node_modules/vite" ]]; then
  if [[ -x "${MOBILE_DIR}/plugin-openlist/web/node_modules/.bin/vite" ]]; then
    cd "${MOBILE_DIR}/plugin-openlist/web"
    nohup ./node_modules/.bin/vite --host 0.0.0.0 --port "${WEB_PORT}" --strictPort \
      >/tmp/setup-env-plugin-vite.log 2>&1 &
    PVITE_PID=$!
    log "plugin vite pid=${PVITE_PID} :${WEB_PORT}，日志: /tmp/setup-env-plugin-vite.log"

    for i in $(seq 1 20); do
      if curl -s "http://localhost:${WEB_PORT}/" >/dev/null 2>&1; then
        ok "plugin Vite 就绪 :${WEB_PORT} (等 ${i}×0.5s)"
        break
      fi
      sleep 0.5
    done
  else
    warn "plugin-openlist/web vite 不可执行"
    FAILED=$((FAILED+1))
  fi
else
  warn "plugin-openlist/web node_modules 不存在"
  FAILED=$((FAILED+1))
fi

# ============================================================================
# 状态报告
# ============================================================================
step "✅ 完成"
cat <<EOF

========================================
📊 沙箱环境状态

端口检查：
EOF
for port in 2025 5244 5173 5174; do
  if lsof -i :$port >/dev/null 2>&1; then
    proc=$(lsof -ti :$port 2>/dev/null | head -1)
    proc_cmd=$(ps -p "$proc" -o comm= 2>/dev/null | head -1)
    echo "   :${port}  ✅ ${proc_cmd} (pid=${proc})"
  else
    echo "   :${port}  ❌ 未占用"
  fi
done

cat <<EOF

进程：
EOF
for name in air encv go vite node kotlinc; do
  pids=$(pgrep -x "$name" 2>/dev/null | head -3 | tr '\n' ' ')
  if [[ -n "$pids" ]]; then
    echo "   ${name}    pids: ${pids}"
  fi
done

cat <<EOF

访问地址（OpenPreview 工具激活后）：
EOF
if lsof -i :5173 >/dev/null 2>&1; then
  echo "   http://localhost:5173/         (encv-mobile 主 app)"
elif lsof -i :5174 >/dev/null 2>&1; then
  echo "   http://localhost:5174/         (encv-mobile 主 app, 5173 被占)"
fi
if lsof -i :5174 >/dev/null 2>&1 && pgrep -f 'plugin-openlist/web' >/dev/null; then
  echo "   http://localhost:5174/webview  (plugin-openlist WebView, 走 iframe 127.0.0.1:5244)"
fi
if lsof -i :5244 >/dev/null 2>&1; then
  echo "   http://127.0.0.1:5244/         (OpenList 真实 fork)"
fi
if lsof -i :2025 >/dev/null 2>&1; then
  echo "   http://127.0.0.1:2025/api/config  (encv-go backend)"
fi

cat <<EOF

日志位置：
   /tmp/setup-env-air.log          (encv-go / air)
   /tmp/setup-env-vite.log         (encv-mobile Vite)
   /tmp/setup-env-openlist.log     (OpenList fork)
   /tmp/setup-env-plugin-vite.log  (plugin-openlist Vite)

停止所有服务:
   pkill -x air; pkill -f vite; pkill -f 'go run.*Hi-Sillot'; pkill -f /tmp/encv

========================================
EOF

if [[ $FAILED -gt 0 ]]; then
  warn "⚠️  共 ${FAILED} 步非致命失败，请查看上方 WARN 行"
  exit 0  # 不报错，因为非致命失败不应阻止其他服务
fi
ok "🎉 沙箱环境已全部就绪"
