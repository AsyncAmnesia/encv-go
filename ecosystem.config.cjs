/* eslint-disable */
// =============================================================================
// ecosystem.config.cjs
// -----------------------------------------------------------------------------
// pm2 配置：管理 4 个沙箱 dev 预览服务
//   ① encv-go              (Go backend, air 监视重载, :2025)
//   ② encv-mobile-vite     (Vite 主 app, :5173)
//   ③ openlist             (Go OpenList 真实 fork, :5244)
//   ④ plugin-openlist-vite (Vite plugin 管理 UI, :5174)
//
// 用法：
//   pm2 start ecosystem.config.cjs
//   pm2 stop  ecosystem.config.cjs
//   pm2 restart ecosystem.config.cjs
//   pm2 status
//   pm2 logs
//   pm2 monit
//
// 包装脚本（推荐）：scripts/previews.sh
//   bash scripts/previews.sh start|stop|restart|status|logs|monit
// =============================================================================

const path = require('path');

const REPO_ROOT    = '/workspace';
const MOBILE_DIR   = path.join(REPO_ROOT, 'app', 'encv-mobile');
const PLUGIN_DIR   = path.join(MOBILE_DIR, 'plugin-openlist', 'web');

// ⚠️ pm2 在 fork 模式下要求 `script` 指向一个可加载的文件。
//   - vite 的 shell 包装 (node_modules/.bin/vite) 不能用 — 它的第 2 行
//     basedir=$(dirname ...) 会被 pm2 当 JS 解析报 SyntaxError。
//   - 也不能写 script: 'node' + interpreter: 'node' — pm2 会试图把
//     /root/.nvm/.../bin/node (ELF 二进制) 当 .js 加载报 Invalid token。
//   - 正确做法：script 写 vite 的 JS 入口，interpreter 显式指定 node。
const VITE_BIN_RELPATH = 'node_modules/vite/bin/vite.js';
const VITE_BIN_MOBILE  = path.join(MOBILE_DIR,   VITE_BIN_RELPATH);
const VITE_BIN_PLUGIN  = path.join(PLUGIN_DIR,   VITE_BIN_RELPATH);

module.exports = {
  apps: [
    // ── ① encv-go backend (Go) ──────────────────────────────────────
    {
      name: 'encv-go',
      // air 包装（live reload），air 的 entrypoint 是 .air-run.sh
      script: '/workspace/.air-run.sh',
      cwd: REPO_ROOT,
      env: {
        ENCV_DEV_PREVIEW: '1',
        PATH: process.env.PATH,
      },
      max_memory_restart: '768M',
      listen_timeout: 30000,
      kill_timeout: 5000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-encv-go.log',
      error_file: '/tmp/pm2-encv-go.err.log',
    },

    // ── ② encv-mobile Vite (主 app) ─────────────────────────────────
    {
      name: 'encv-mobile-vite',
      script: VITE_BIN_MOBILE,
      args: '--host 0.0.0.0 --port 5173 --strictPort',
      interpreter: 'node',
      cwd: MOBILE_DIR,
      env: {
        PATH: process.env.PATH,
      },
      max_memory_restart: '512M',
      listen_timeout: 15000,
      kill_timeout: 3000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-encv-mobile-vite.log',
      error_file: '/tmp/pm2-encv-mobile-vite.err.log',
    },

    // ── ③ OpenList 真实 fork (Go) ───────────────────────────────────
    {
      name: 'openlist',
      // dev-openlist.sh 自动用本地 dist，fallback 到 release tarball
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

    // ── ④ plugin-openlist Vite (plugin 管理 UI) ─────────────────────
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

  // 部署配置（暂未用，但 pm2 推荐提供）
  deploy: {},
};
