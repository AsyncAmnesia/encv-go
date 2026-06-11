/**
 * dev-start-guard.ts
 *
 * Vite plugin：dev 模式启动守卫，强制走 PM2 → preview-gateway 链路。
 *
 * ⚠️ 防御机制触发条件（必须**同时**满足才抛错）：
 *   ① env.command === 'serve'  （build 模式永远不抛 — 产线打包任何时候都应可执行）
 *   ② CI !== 'true' / '1'      （CI 环境跑 lint/build 不需要 PM2）
 *   ③ SPAWN_VITE !== '1'        （非 preview-gateway spawn）
 *   ④ !PM2_HOME                （非 PM2 进程树）
 *   ⑤ !PPA_SPAWNED              （非 PPA 子进程）
 *
 * 历史踩坑（2026-06-10）：守卫最初没看 env.command，CI 跑 `pnpm run build`
 * 也被拦截，导致产线打包失败。修复：build 模式直接 return。
 *
 * 历史踩坑（2026-06-10 之前）：沙箱里 PM2_HOME 是 agent-tool-host 设的，
 * 守卫会放行 — 但**正确**，因为 PM2 真的在管 vite（preview-gateway
 * spawn 的子进程继承 PM2_HOME env）。用户本地直接 `vite` 没 PM2 才会抛。
 */

import type { Plugin } from 'vite'

export interface DevStartGuardOptions {
  /** 自定义错误信息（测试可注入） */
  errorMessage?: string
}

export function devStartGuard(opts: DevStartGuardOptions = {}): Plugin {
  return {
    name: 'dev-start-guard',
    config(_config, env) {
      // ① build 模式直接跳过 — 产线打包任何时候都应可执行
      if (env?.command !== 'serve') return

      // ② CI 环境跳过 — GitHub Actions / GitLab CI / Jenkins 等
      if (process.env.CI === 'true' || process.env.CI === '1') return

      // ③ SPAWN_VITE=1 表示由 preview-gateway spawn，合法
      if (process.env.SPAWN_VITE === '1') return

      // ④ PM2 管理下也合法（PM2_HOME 由 agent-tool-host 或 pm2 daemon 设）
      const isPm2 = !!process.env.PM2_HOME

      // ⑤ PPA_SPAWNED 是 preview-gateway 老版本用的标记
      if (!isPm2 && !process.env.PPA_SPAWNED) {
        const msg = opts.errorMessage ?? DEFAULT_ERROR_MESSAGE
        throw new Error(msg)
      }
    },
  }
}

const DEFAULT_ERROR_MESSAGE = `
╔══════════════════════════════════════════════════════════╗
║  [dev-start-guard] 检测到非法启动方式！立即终止。        ║
╠══════════════════════════════════════════════════════════╣
║                                                          ║
║  ❌ 你正在直接运行 vite / npm run dev                    ║
║     这在本项目中是非法的。                               ║
║                                                          ║
║  原因：                                                   ║
║    ① preview-gateway(:16666) 是唯一对外入口               ║
║       内部管理子进程(vite:8100, air:2025 等)             ║
║    ② 直接 vite 不注入 ENCV_DEV_PREVIEW / ENCV_MOBILE env ║
║    ③ Vite 扫描 plugin-openlist/index.html → 文件找不到  ║
║    ④ HMR 缺 gateway dynamicHmrHostPlugin Host 头透传      ║
║                                                          ║
║  ✅ 正确启动方式：                                        ║
║    pm2 start /workspace/ecosystem.config.cjs              ║
║                                                          ║
║  或重启：                                                 ║
║    pm2 restart preview-gateway                           ║
║    pm2 logs preview-gateway --lines 20                   ║
║                                                          ║
║  预览地址：http://localhost:16666/                        ║
╚══════════════════════════════════════════════════════════╝
`.trim()


