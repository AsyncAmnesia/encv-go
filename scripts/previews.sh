#!/usr/bin/env bash
# =============================================================================
# previews.sh — 沙箱 dev 预览服务统一管理（基于 pm2）
# -----------------------------------------------------------------------------
# 4 个服务的总闸：encv-go (2025) / Vite encv-mobile (5173) /
#                OpenList (5244) / plugin Vite (5174)
#
# 用法：
#   bash scripts/previews.sh start         # 启全部（4 个）
#   bash scripts/previews.sh stop          # 停全部
#   bash scripts/previews.sh restart       # 重启全部
#   bash scripts/previews.sh status        # 状态 + 端口 + 内存
#   bash scripts/previews.sh logs          # 实时日志（4 个交错）
#   bash scripts/previews.sh logs encv-go  # 单服务日志
#   bash scripts/previews.sh monit         # 终端仪表盘
#   bash scripts/previews.sh kill          # 强杀全部（清空 pm2 进程列表）
#   bash scripts/previews.sh reload        # 重载 ecosystem（pickup 配置变更）
#
# 退出码:
#   0 = 成功
#   1 = pm2 未安装
#   2 = pm2 启动失败
# =============================================================================

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ECOSYSTEM="${REPO_ROOT}/ecosystem.config.cjs"

# ---- 颜色 ----
R='\033[1;31m'; G='\033[1;32m'; Y='\033[1;33m'; C='\033[1;36m'; B='\033[1;34m'; N='\033[0m'
log()  { printf "${C}[previews]${N} %s\n" "$*"; }
ok()   { printf "${G}[previews]${N} %s\n" "$*"; }
warn() { printf "${Y}[previews]${N} %s\n" "$*" >&2; }
err()  { printf "${R}[previews]${N} %s\n" "$*" >&2; }

usage() {
  cat <<EOF
用法: bash scripts/previews.sh <command> [app_name]

命令:
  start [app]   启全部 / 单个
  stop  [app]   停全部 / 单个
  restart [app] 重启全部 / 单个
  reload        0 秒停机重载（pickup ecosystem.config.cjs 变更）
  status        4 服务的 pm2 状态 + 端口 + 内存
  logs   [app]  实时日志（默认全部交错，指定 app 看单个）
  monit         终端仪表盘（CPU/内存/事件）
  kill          强杀全部（从 pm2 进程列表清空）

服务名:
  encv-go              (:2025)
  encv-mobile-vite     (:5173)
  openlist             (:5244)
  plugin-openlist-vite (:5174)
EOF
}

# ---- 检查 pm2 ----
if ! command -v pm2 >/dev/null 2>&1; then
  err "❌ pm2 未安装"
  log "  安装方法（任选一）："
  log "    npm i -g pm2"
  log "    pnpm add -g pm2"
  log "    npx pm2@latest ..."
  exit 1
fi

CMD="${1:-status}"
APP_NAME="${2:-}"

# ---- 辅助函数 ----
check_ports() {
  echo ""
  log "📡 端口状态："
  local ports=(
    "2025:encv-go"
    "5173:encv-mobile-vite"
    "5244:openlist"
    "5174:plugin-openlist-vite"
  )
  for entry in "${ports[@]}"; do
    local port="${entry%%:*}"
    local name="${entry##*:}"
    if lsof -i ":$port" >/dev/null 2>&1; then
      local pid
      pid=$(lsof -ti ":$port" 2>/dev/null | head -1)
      printf "   :%-5s  ${G}✅${N} %s (pid=%s)\n" "$port" "$name" "$pid"
    else
      printf "   :%-5s  ${R}❌${N} %s\n" "$port" "$name"
    fi
  done
}

# ---- 命令分发 ----
case "$CMD" in
  start)
    if [[ -n "$APP_NAME" ]]; then
      log "启 ${APP_NAME} ..."
      pm2 start "$ECOSYSTEM" --only "$APP_NAME" 2>&1 | tail -8
    else
      log "启全部 4 个服务 ..."
      pm2 start "$ECOSYSTEM" 2>&1 | tail -20
    fi
    sleep 2
    pm2 status
    check_ports
    ;;

  stop)
    if [[ -n "$APP_NAME" ]]; then
      log "停 ${APP_NAME} ..."
      pm2 stop "$APP_NAME" 2>&1 | tail -5
    else
      log "停全部 4 个服务 ..."
      pm2 stop "$ECOSYSTEM" 2>&1 | tail -5
    fi
    sleep 1
    check_ports
    ;;

  restart)
    if [[ -n "$APP_NAME" ]]; then
      log "重启 ${APP_NAME} ..."
      pm2 restart "$APP_NAME" 2>&1 | tail -5
    else
      log "重启全部 4 个服务 ..."
      pm2 restart "$ECOSYSTEM" 2>&1 | tail -5
    fi
    sleep 2
    check_ports
    ;;

  reload)
    log "重载 ecosystem（0 秒停机） ..."
    pm2 reload "$ECOSYSTEM" 2>&1 | tail -5
    pm2 status
    ;;

  status)
    pm2 status
    check_ports
    ;;

  logs)
    if [[ -n "$APP_NAME" ]]; then
      pm2 logs "$APP_NAME" --lines 100
    else
      pm2 logs --lines 50
    fi
    ;;

  monit)
    pm2 monit
    ;;

  kill)
    warn "⚠️  强杀所有 pm2 进程 ..."
    pm2 kill 2>&1 | tail -3
    sleep 1
    # 兜底清理端口残留
    for port in 2025 5173 5244 5174; do
      pids=$(lsof -ti ":$port" 2>/dev/null || true)
      if [[ -n "$pids" ]]; then
        log "清理 :$port 残留 pids: $pids"
        kill $pids 2>/dev/null || true
      fi
    done
    sleep 1
    check_ports
    ok "已清空"
    ;;

  -h|--help|help)
    usage
    exit 0
    ;;

  *)
    err "未知命令: $CMD"
    usage
    exit 2
    ;;
esac
