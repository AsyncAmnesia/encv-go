/* eslint-disable */
// =============================================================================
// ecosystem.config.cjs
// -----------------------------------------------------------------------------
// pm2 配置：管理「沙箱辅助」dev 服务（主预览不在这里）
//
// ⚠️ 主预览（encv-go :2025 + encv-mobile-vite :5173）必须用
//    bash app/encv-mobile/scripts/start-preview.sh 前台运行，
//    目的是让 OpenPreview 工具的 command_id 能关联到前台 vite 进程。
//    详细铁律见 scripts/start-preview.sh 顶部注释。
//
// pm2 只管辅助服务：
//   ① openlist             (Go OpenList 真实 fork, :5244) — Capacitor plugin iframe
//   ② plugin-openlist-vite (Vite plugin 管理 UI, :5174)  — dev 模式 stub 后端
//
// 用法：
//   pm2 start ecosystem.config.cjs
//   pm2 stop  ecosystem.config.cjs
//   pm2 restart ecosystem.config.cjs
//   pm2 status
//   pm2 logs
//   pm2 monit
//
// 包装脚本：scripts/previews.sh
//   bash scripts/previews.sh start|stop|restart|status|logs|monit
// =============================================================================

const path = require('path');

const REPO_ROOT  = '/workspace';
const MOBILE_DIR = path.join(REPO_ROOT, 'app', 'encv-mobile');
const PLUGIN_DIR = path.join(MOBILE_DIR, 'plugin-openlist', 'web');

// ⚠️ pm2 fork 模式下 script 必须指向可加载文件（.js 路径 + interpreter）。
//   - vite 的 shell 包装 (node_modules/.bin/vite) 不可用 — 第 2 行
//     basedir=$(dirname ...) 会被 pm2 当 JS 解析报 SyntaxError。
//   - script: 'node' + interpreter: 'node' 也不可用 — pm2 会把
//     /root/.nvm/.../bin/node (ELF 二进制) 当 .js 加载报 Invalid token。
//   - 正确做法：script 写 vite 的 JS 入口，interpreter 显式指定 node。
const VITE_BIN_RELPATH = 'node_modules/vite/bin/vite.js';
const VITE_BIN_PLUGIN  = path.join(PLUGIN_DIR, VITE_BIN_RELPATH);

module.exports = {
  apps: [
    // ── ① OpenList 真实 fork (Go) ───────────────────────────────────
    {
      name: 'openlist',
      // dev-openlist.sh 会自动用本地 fork 的 dist，fallback 到 release tarball
      script: 'bash',
      args: `${MOBILE_DIR}/scripts/dev-openlist.sh --port 5244`,
      cwd: MOBILE_DIR,
      env: {
        PATH: process.env.PATH,
        OPENLIST_DATA: '/tmp/openlist-data',
      },
      // OpenList 内存占用较大（gorm + 加密 + sqlite）
      max_memory_restart: '1G',
      // go run 编译时间较长
      listen_timeout: 90000,
      kill_timeout: 5000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-openlist.log',
      error_file: '/tmp/pm2-openlist.err.log',
    },

    // ── ② plugin-openlist Vite (plugin 管理 UI) ─────────────────────
    {
      name: 'plugin-openlist-vite',
      script: VITE_BIN_PLUGIN,
      args: '--host 0.0.0.0 --port 5174 --strictPort',
      interpreter: 'node',
      cwd: PLUGIN_DIR,
      env: {
        PATH: process.env.PATH,
      },
      max_memory_restart: '512M',
      listen_timeout: 15000,
      kill_timeout: 3000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-plugin-openlist-vite.log',
      error_file: '/tmp/pm2-plugin-openlist-vite.err.log',
    },
  ],

  deploy: {},
};
