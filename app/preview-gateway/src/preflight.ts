/**
 * preflight.ts — gateway 启动前的预检
 * =====================================================
 *
 * 2026-06-10 改造：Node CLI mock 生成脚本已删，本模块简化为 noop 桩。
 *
 * 历史职责：在子进程启动前自动调 `npx tsx scripts/generate-mock-files.ts` 写 mock。
 * 当前职责：保留 `ensureMockData` 导出签名以兼容 gateway server.ts 调用方，直接 resolve。
 * mock 数据由用户主动调后端 /api/mock/generate（带 X-Confirm-Mock-Mutation header）生成。
 */

const LOG_PREFIX = '[preflight]'

function log(...args: unknown[]): void {
  console.log(LOG_PREFIX, ...args)
}

/**
 * 2026-06-10 noop 桩 —— 不再预生成 mock。
 * 保留导出是为了不破坏 gateway server.ts 的 `import { ensureMockData } from './preflight.js'` 引用。
 */
export async function ensureMockData(_mobileDir: string): Promise<void> {
  log('(noop) mock data generation moved to user-driven /api/mock/generate (2026-06-10)')
}
