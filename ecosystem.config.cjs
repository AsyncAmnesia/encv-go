/* eslint-disable */
// =============================================================================
// ecosystem.config.cjs
// -----------------------------------------------------------------------------
// pm2 配置：统一管理 沙箱 dev 服务（统一预览网关 + 主预览 + 辅助服务）
//
//   ① preview-gateway     (统一预览网关, :16666 — 对外唯一端口)
//   ② start-preview       (主预览 — start-preview.sh, :2025 + :8100)
//   ③ openlist            (Go OpenList 真实 fork, :5244)
//   ④ plugin-openlist-vite(Vite plugin 管理 UI, :5174)
//   ⑤ encv-mobile-vite    (主 app Vite, :8100 — 独立 pm2 app)
//   ⑥ preview-helper      (OpenPreview 占位, :15002)
//   ⑦ openpreview-stub    (OpenPreview 注册源, :15003)
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
//
// 端口决策（spec/unify-sandbox-preview-port §D1-D9）:
//   :16666 = preview-gateway 唯一对外端口（用户决策 "好记"）
//            agent-tool-host 内部 preview-proxy 会在首次
//            agent-browser navigate :16666 时自动注册该端口
//   :8100  = encv-mobile Vite（纯净 SPA，不再做反向代理胶水）
//   :5174  = plugin-openlist-web Vite（被 :16666/openlist-ui 代理）
//   :2025  = encv-go（被 :16666/api + /openlist/ + /p/ + /play 代理）
//   :5244  = OpenList fork（被 encv-go :2025 内部代理）
//
//   ⚠️ 历史 :16000 = OpenPreview 工具用的外网入口（agent-tool-host），
//      仅用于把 :16666 转给外网用户；preview-gateway 自身不再监听 :16000。
// =============================================================================

const path = require('path');
const fs = require('fs');

const REPO_ROOT      = '/workspace';
const MOBILE_DIR     = path.join(REPO_ROOT, 'app', 'encv-mobile');
const PLUGIN_DIR     = path.join(MOBILE_DIR, 'plugin-openlist', 'web');
const GATEWAY_DIR    = path.join(REPO_ROOT, 'app', 'preview-gateway');

const PREVIEW_SCRIPT  = path.join(MOBILE_DIR, 'scripts', 'start-preview.sh');
const GATEWAY_SCRIPT  = path.join(GATEWAY_DIR, 'dist', 'server.js');

// ⚠️ pm2 fork 模式下 script 必须指向可加载文件（.js 路径 + interpreter）。
//   - vite 的 shell 包装 (node_modules/.bin/vite) 不可用 — 第 2 行
//     basedir=$(dirname ...) 会被 pm2 当 JS 解析报 SyntaxError。
//   - script: 'node' + interpreter: 'node' 也不可用 — pm2 会把
//     /root/.nvm/.../bin/node (ELF 二进制) 当 .js 加载报 Invalid token。
//   - 正确做法：script 写 vite 的 JS 入口，interpreter 显式指定 node。
const VITE_BIN_RELPATH = 'node_modules/vite/bin/vite.js';
const VITE_BIN_PLUGIN  = path.join(PLUGIN_DIR, VITE_BIN_RELPATH);
const VITE_BIN_MAIN    = path.join(MOBILE_DIR, VITE_BIN_RELPATH);

// 检查 start-preview.sh 存在性（缺失时给出明确报错）
if (!fs.existsSync(PREVIEW_SCRIPT)) {
  throw new Error(`start-preview.sh 不存在: ${PREVIEW_SCRIPT}`);
}

// 检查 preview-gateway 编译产物（缺失时报错，提示先 setup-sandbox-env.sh）
if (!fs.existsSync(GATEWAY_SCRIPT)) {
  throw new Error(
    `preview-gateway dist/server.js 不存在: ${GATEWAY_SCRIPT}\n` +
    `请先运行 setup-sandbox-env.sh（或 cd ${GATEWAY_DIR} && pnpm install && pnpm build）`,
  );
}

module.exports = {
  apps: [
    // ── ① preview-gateway (统一预览网关, :16666) ────────────────────
    //   唯一对外端口。浏览器、agent-browser、外网用户都走 :16666。
    //   网关内部分发到 :8100 / :5174 / :2025 / :5244 四个 upstream。
    //   health 端点：http://localhost:16666/__gateway/health
    //
    //   启动顺序：必须在 vite (:8100) 起来之后再 restart，否则
    //   第一次 health check 会短暂失败（不影响代理本身的可用性）
    {
      name: 'preview-gateway',
      script: GATEWAY_SCRIPT,
      interpreter: 'node',
      cwd: GATEWAY_DIR,
      env: {
        PATH: process.env.PATH,
        PORT: '16666',
        HOST: '0.0.0.0',
      },
      // 网关内存占用极小（纯转发）
      max_memory_restart: '256M',
      listen_timeout: 10000,
      kill_timeout: 3000,
      autorestart: true,
      // 启动后立即可用，预期常驻
      max_restarts: 10,
      min_uptime: '10s',
      out_file: '/tmp/pm2-preview-gateway.log',
      error_file: '/tmp/pm2-preview-gateway.err.log',
      merge_logs: true,
      time: true,
    },

    // ── ② start-preview (主预览) ────────────────────────────────────
    //   start-preview.sh 自身前台阻塞（wait -n 等待任一子进程退出），
    //   内部用 & 启 air 和 vite 子进程，由 trap INT/TERM 兜底清理。
    //   pm2 把它当作一个长跑进程管（fork 模式）：
    //     - max_memory_restart: 内存超限自动重启（杀 air+vite 再起）
    //     - kill_timeout: 8s（air 还要调 go build，1.6s 默认不够）
    //     - listen_timeout: 60s（mock 生成 + npm install + 第一次 go build 较慢）
    {
      name: 'start-preview',
      script: PREVIEW_SCRIPT,
      interpreter: 'bash',
      cwd: REPO_ROOT,
      // start-preview.sh 内部已经设了 ENCV_DEV_PREVIEW=1，这里不再设
      env: {
        PATH: process.env.PATH,
        // 让 mock 生成走沙箱 fallback（脚本默认 /storage/emulated/0，已存在）
        ENCV_MOCK_ROOT: '/storage/emulated/0',
      },
      // 内存超 1G 重启（air 跑久了 + go build 临时内存会涨）
      max_memory_restart: '1G',
      // 启动窗口 60s：mock 生成 + npm install + air 第一次 go build
      listen_timeout: 60000,
      // 8s 让 air 完成 go build 后再 SIGKILL
      kill_timeout: 8000,
      autorestart: true,
      // 主预览预期常驻；超过 5 次重启说明代码坏了，别再循环
      max_restarts: 5,
      min_uptime: '60s',
      out_file: '/tmp/pm2-start-preview.log',
      error_file: '/tmp/pm2-start-preview.err.log',
      // merge_logs: true 让多步输出不打乱
      merge_logs: true,
      time: true,
    },

    // ── ② encv-mobile Vite (主 app, :8100) ───────────────────────────
    //   历史上由 start-preview.sh 内部 & 启动，但 pm2 重启 start-preview 时
    //   会清理 8100 vite → 主 app 频繁短暂不可用。拆成独立 pm2 app 更稳。
    {
      name: 'encv-mobile-vite',
      script: VITE_BIN_MAIN,
      args: '--host 0.0.0.0 --port 8100 --strictPort',
      interpreter: 'node',
      cwd: MOBILE_DIR,
      env: { PATH: process.env.PATH },
      max_memory_restart: '512M',
      listen_timeout: 15000,
      kill_timeout: 3000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-encv-mobile-vite.log',
      error_file: '/tmp/pm2-encv-mobile-vite.err.log',
    },

    // ── ② OpenList 真实 fork (Go) ───────────────────────────────────
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

    // ── ④ plugin-openlist-vite (plugin 管理 UI, :5174) ────────────────
    //   实际是 4 号上游（preview-gateway → :16666/openlist-ui/），
    //   头部列表 ④ 与下方 apps 顺序保持一致。
    {
      name: 'plugin-openlist-vite',
      script: VITE_BIN_PLUGIN,
      args: '--host 0.0.0.0 --port 5174 --strictPort',
      interpreter: 'node',
      cwd: PLUGIN_DIR,
      env: {
        PATH: process.env.PATH,
        // ⚠️ 沙箱 dev 必须设 VITE_BASE=/openlist-ui/ —— 让 Vite 在 dev 模式下
        // 也用绝对 base 解析资源路径（HTML 内的 ./src/main.ts → /openlist-ui/src/main.ts）
        // 这是 plugin-openlist web 在 preview-gateway :16666/openlist-ui/ 下
        // 不再空白的核心修复（spec/unify-sandbox-preview-port §防御性 UI）
        VITE_BASE: '/openlist-ui/',
      },
      max_memory_restart: '512M',
      listen_timeout: 15000,
      kill_timeout: 3000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-plugin-openlist-vite.log',
      error_file: '/tmp/pm2-plugin-openlist-vite.err.log',
    },

    // ── ⑤ preview-helper (占位 HTTP server, :15001) ─────────────────
    //   唯一作用：让 OpenPreview 工具有一个 web_server 类型 command 可注册
    //   （它要求 command_id 来自 toolcall history 中的 web_server 命令）。
    //   真实预览走 :16666 preview-gateway，本服务纯返回 200 OK。
    //   不阻塞 sandbox 会话：pm2 fork 模式，daemon 化。
    //   持久化：pm2 save → /root/.pm2/dump.pm2；会话重置后 pm2 resurrect 自动恢复。
    {
      name: 'preview-helper',
      script: path.join(REPO_ROOT, '.preview-helper.js'),
      interpreter: 'node',
      cwd: REPO_ROOT,
      env: { PATH: process.env.PATH, PORT: '15002' },
      max_memory_restart: '64M',
      listen_timeout: 5000,
      kill_timeout: 2000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-preview-helper.log',
      error_file: '/tmp/pm2-preview-helper.err.log',
    },

    // ── ⑥ openpreview-stub (OpenPreview 注册源, :15003) ─────────────
    //   启动 /workspace/scripts/openpreview-stub.js (pm2 守护，daemon 化)。
    //   与 preview-helper :15002 共存，端口不冲突。
    //   真实预览仍走 :16666 preview-gateway。
    //   完整规则：.trae/rules/preview-management.md §三
    {
      name: 'openpreview-stub',
      script: path.join(REPO_ROOT, 'scripts', 'openpreview-stub.js'),
      interpreter: 'node',
      cwd: path.join(REPO_ROOT, 'scripts'),
      env: { PATH: process.env.PATH, PORT: '15003' },
      max_memory_restart: '64M',
      listen_timeout: 5000,
      kill_timeout: 2000,
      autorestart: true,
      max_restarts: 10,
      out_file: '/tmp/pm2-openpreview-stub.log',
      error_file: '/tmp/pm2-openpreview-stub.err.log',
    },
  ],

  deploy: {},
};
