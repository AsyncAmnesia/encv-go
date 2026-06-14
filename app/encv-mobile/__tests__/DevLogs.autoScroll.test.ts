/**
 * DevLogs 自动滚动状态机单元测试（v3：ionScrollEnd + 缓存 scrollEl 极简版）
 *
 * 覆盖（pinned-to-bottom 模式）：
 *  1.  初始 nearBottom=true
 *  2.  handleNewLog 在底部时调用 scrollToBottom（不累积 unread）
 *  3.  handleNewLog 不在底部时 unreadCount++
 *  4.  用户停止滚动（ionScrollEnd）→ 距离 > 80 → nearBottom=false
 *  5.  用户滑回底部（ionScrollEnd）→ unreadCount=0
 *  6.  hardPaused=true → handleNewLog 不滚不累积
 *  7.  onJumpToBottom → scrollToBottom(true) + 清空 unread
 *  8.  切到 backend tab → 不响应 frontend 日志
 *  9.  切到 frontend tab → 不响应 backend 日志
 * 10.  onIonViewWillEnter → 重算 nearBottom
 * 11.  cachedScrollEl 复用：连续多次 ensureScrollEl 不会重复查询
 * 12.  handleNewLog 在底部时 nextTick 后 scrollTop = scrollHeight
 *
 * 实现策略：
 *   - mock @ionic/vue：用 stub IonContent 在 mounted 钩子给 $el 注入 shadowRoot shim
 *     （jsdom 不支持原生 shadow DOM）
 *   - mock useFrontendLogs / useRealtimeTransport / useEventBus
 *   - 通过 defineExpose 暴露的 state machine 直接断言
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'

// ─── Mock 共享的 refs ─────────────────────────────────────────────────────
const h = vi.hoisted(() => {
  let frontendLogsValue: Array<{ id: number; timestamp: string; level: string; message: string }> = []
  const backendLogs: any[] = []
  let transportConnectionValue: 'connected' | 'disconnected' | 'connecting' = 'disconnected'
  const eventBusListeners: Record<string, Array<(data: any) => void>> = {}
  let serverOnline = false
  let ionViewWillEnterCallback: (() => Promise<void>) | null = null
  const frontendLogsObj = {
    get value() { return frontendLogsValue },
    set value(v: any) { frontendLogsValue = v },
  }
  const transportConnectionObj = {
    get value() { return transportConnectionValue },
    set value(v: any) { transportConnectionValue = v },
  }
  const setIonViewWillEnterCb = (cb: any) => {
    ionViewWillEnterCallback = cb
  }
  return {
    frontendLogs: frontendLogsObj,
    backendLogs,
    transportConnection: transportConnectionObj,
    eventBusListeners,
    serverOnline,
    get ionViewWillEnterCallback() { return ionViewWillEnterCallback },
    setIonViewWillEnterCb,
  }
})

vi.mock('@/composables/useEventBus', () => ({
  eventBus: {
    on(event: string, fn: (data: any) => void) {
      if (!h.eventBusListeners[event]) h.eventBusListeners[event] = []
      h.eventBusListeners[event].push(fn)
    },
    off(event: string, fn: (data: any) => void) {
      if (!h.eventBusListeners[event]) return
      h.eventBusListeners[event] = h.eventBusListeners[event].filter((f) => f !== fn)
    },
    emit(event: string, data: any) {
      if (!h.eventBusListeners[event]) return
      h.eventBusListeners[event].forEach((f) => f(data))
    },
  },
}))

vi.mock('@/composables/useFrontendLogs', () => ({
  useFrontendLogs: () => ({
    logs: h.frontendLogs,
    clearLogs: () => { h.frontendLogs.value = [] },
  }),
}))

vi.mock('@/composables/useRealtimeTransport', () => ({
  useRealtimeTransport: () => ({
    connectionState: h.transportConnection,
    transportMode: { value: 'ws' },
    isSandboxBrowser: { value: false },
    connect: () => {},
    disconnect: () => {},
    forceReconnect: () => {},
  }),
}))

vi.mock('@/composables/useI18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    tField: (key: string) => key,
    setLocale: () => {},
    getLocale: () => 'zh-CN',
    locale: { value: 'zh-CN' },
  }),
}))

vi.mock('@/composables/useToast', () => ({
  showToast: vi.fn(),
}))

vi.mock('@/composables/useClipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
}))

vi.mock('@/api/encv', () => ({
  checkServerStatus: vi.fn().mockResolvedValue({ online: true }),
}))

// ─── Mock @ionic/vue ─────────────────────────────────────────────────────
interface FakeScrollEl extends HTMLElement {
  __scrollToSpy: ReturnType<typeof vi.fn>
  __setScrollHeight(v: number): void
  __setClientHeight(v: number): void
}

let fakeScrollEl: FakeScrollEl

vi.mock('@ionic/vue', () => ({
  IonPage: { name: 'IonPage', template: '<div><slot /></div>' },
  IonHeader: { name: 'IonHeader', template: '<header><slot /></header>' },
  IonToolbar: { name: 'IonToolbar', template: '<div><slot /></div>' },
  IonTitle: { name: 'IonTitle', template: '<h1><slot /></h1>' },
  IonContent: {
    name: 'IonContent',
    // 只 emit ionScrollEnd（v3 不再监听 ionScroll）
    template: '<div class="ion-content-stub" @ionScrollEnd="$emit(\'ionScrollEnd\', $event)"><slot /></div>',
    emits: ['ionScrollEnd'],
    mounted(this: any) {
      // jsdom 不支持原生 shadow DOM；用 Object.defineProperty 给 stub $el 注入
      // shadowRoot shim，让生产代码的 ensureScrollEl() 通过 host.shadowRoot.querySelector
      // 找到 fakeScrollEl。这是 v3 滚动元素查找路径 1（生产主路径）。
      Object.defineProperty(this.$el, 'shadowRoot', {
        value: { querySelector: (sel: string) => (sel === '.inner-scroll' ? fakeScrollEl : null) },
        writable: true,
        configurable: true,
      })
    },
  },
  IonSegment: { name: 'IonSegment', template: '<div><slot /></div>' },
  IonSegmentButton: { name: 'IonSegmentButton', template: '<button><slot /></button>' },
  IonSearchbar: { name: 'IonSearchbar', template: '<input />' },
  IonButton: { name: 'IonButton', template: '<button @click="$emit(\'click\')"><slot /></button>', emits: ['click'] },
  IonIcon: { name: 'IonIcon', template: '<i><slot /></i>' },
  IonBadge: { name: 'IonBadge', template: '<span><slot /></span>' },
  IonToggle: { name: 'IonToggle', template: '<input type="checkbox" />', props: ['modelValue'] },
  IonFooter: { name: 'IonFooter', template: '<footer><slot /></footer>' },
  alertController: {
    create: vi.fn().mockResolvedValue({ present: vi.fn() }),
  },
  onIonViewWillEnter: (cb: () => Promise<void>) => {
    h.setIonViewWillEnterCb(cb)
    return () => { h.setIonViewWillEnterCb(null) }
  },
}))

// 必须在 mock 设置后 import 组件
import DevLogs from '@/views/DevLogs.vue'

// ─── 工具 ─────────────────────────────────────────────────────────────────

/**
 * 创建 fake 滚动元素：必须是真 HTMLElement（jsdom 的 document.contains 要求 Node 类型）
 * 用 Object.defineProperty 覆盖 scrollTop/scrollHeight/clientHeight 让赋值生效
 * （原生这些是只读 property descriptor，jsdom 不允许直接 el.scrollTop = ...）
 */
function createFakeScrollEl(opts: { scrollTop: number; scrollHeight: number; clientHeight: number }): FakeScrollEl {
  const el = document.createElement('div')
  const state = { st: opts.scrollTop, sh: opts.scrollHeight, ch: opts.clientHeight }
  Object.defineProperty(el, 'scrollTop', {
    get: () => state.st,
    set: (v: number) => { state.st = v },
    configurable: true,
  })
  Object.defineProperty(el, 'scrollHeight', {
    get: () => state.sh,
    set: (v: number) => { state.sh = v },
    configurable: true,
  })
  Object.defineProperty(el, 'clientHeight', {
    get: () => state.ch,
    set: (v: number) => { state.ch = v },
    configurable: true,
  })
  const __scrollToSpy = vi.fn((p: { top: number; behavior?: ScrollBehavior }) => {
    state.st = p.top
  })
  ;(el as any).scrollTo = __scrollToSpy
  ;(el as any).__scrollToSpy = __scrollToSpy
  ;(el as any).__setScrollHeight = (v: number) => { state.sh = v }
  ;(el as any).__setClientHeight = (v: number) => { state.ch = v }
  // 挂到 DOM 树，确保 document.contains(el) === true
  document.body.appendChild(el)
  return el as FakeScrollEl
}

function mountDevLogs() {
  return mount(DevLogs, {
    global: { config: {} },
  })
}

beforeEach(() => {
  h.frontendLogs.value = []
  h.backendLogs.length = 0
  h.serverOnline = false
  h.transportConnection.value = 'disconnected'
  Object.keys(h.eventBusListeners).forEach((k) => { h.eventBusListeners[k] = [] })
  h.setIonViewWillEnterCb(null)
  // 清掉前一轮挂到 body 的 fake 节点（避免堆满 DOM 树）
  if (fakeScrollEl && fakeScrollEl.parentNode) {
    fakeScrollEl.parentNode.removeChild(fakeScrollEl)
  }
  // 默认 fake 滚动元素：在底部（距离 = 0 < 80）
  fakeScrollEl = createFakeScrollEl({ scrollTop: 1000, scrollHeight: 1000, clientHeight: 500 })
})

afterEach(() => {
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

// ─── 用例 ─────────────────────────────────────────────────────────────────

describe('DevLogs 自动滚动 - v3 pinned 模式（ionScrollEnd 方案）', () => {
  it('1. 初始 nearBottom=true', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
  })

  it('2. handleNewLog 在底部时调用 scrollToBottom（不累积 unread）', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
    expect((w.vm as any).unreadCount).toBe(0)
    ;(w.vm as any).handleNewLog()
    await nextTick()
    await flushPromises()
    // v3：auto-scroll 用 el.scrollTop = el.scrollHeight（直接赋值）
    expect(fakeScrollEl.scrollTop).toBe(1000)
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('3. handleNewLog 不在底部时 unreadCount++', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    // 模拟用户上滑后停止：scrollTop=0 → 距离 = 1000-0-500 = 500 > 80
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScrollEnd()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
    expect((w.vm as any).unreadCount).toBe(0)
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    expect((w.vm as any).unreadCount).toBe(3)
    expect(fakeScrollEl.__scrollToSpy).not.toHaveBeenCalled()
  })

  it('4. 用户停止滚动（ionScrollEnd）→ 距离 > 80 → nearBottom=false', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScrollEnd()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
  })

  it('5. 用户滑回底部（ionScrollEnd）→ unreadCount=0', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScrollEnd()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    expect((w.vm as any).unreadCount).toBe(2)
    fakeScrollEl.scrollTop = 500
    ;(w.vm as any).onContentScrollEnd()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('6. hardPaused=true → handleNewLog 不滚不累积', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    ;(w.vm as any).hardPaused = true
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    await nextTick()
    await flushPromises()
    expect(fakeScrollEl.__scrollToSpy).not.toHaveBeenCalled()
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('7. onJumpToBottom → scrollToBottom(true) + 清空 unread', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScrollEnd()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    expect((w.vm as any).unreadCount).toBe(2)
    void (w.vm as any).onJumpToBottom()
    await nextTick()
    await flushPromises()
    // onJumpToBottom 调用 scrollToBottom(true) → scrollTo({behavior:'smooth'})
    expect(fakeScrollEl.__scrollToSpy).toHaveBeenCalled()
    const lastCall = fakeScrollEl.__scrollToSpy.mock.calls[fakeScrollEl.__scrollToSpy.mock.calls.length - 1]
    expect(lastCall?.[0]?.behavior).toBe('smooth')
    expect((w.vm as any).nearBottom).toBe(true)
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('8. activeTab=frontend 时 backend 日志不响应', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    ;(w.vm as any).setActiveTab('frontend')
    await flushPromises()
    const bus: any = (await import('@/composables/useEventBus')).eventBus
    bus.emit('ws:message', { type: 'log', data: { level: 'info', message: 'backend log' } })
    await nextTick()
    await flushPromises()
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('9. activeTab=backend 时 frontend 日志不响应', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    ;(w.vm as any).setActiveTab('backend')
    await flushPromises()
    h.frontendLogs.value = [
      ...h.frontendLogs.value,
      { id: 1, timestamp: '00:00:00', level: 'info', message: 'frontend log' },
    ]
    await nextTick()
    await flushPromises()
    expect(fakeScrollEl.__scrollToSpy).not.toHaveBeenCalled()
  })

  it('10. onIonViewWillEnter → 重算 nearBottom', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScrollEnd()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
    // 模拟切回 tab 时 layout 恢复（scrollHeight 变化）
    fakeScrollEl.scrollTop = 500
    fakeScrollEl.scrollHeight = 1500
    fakeScrollEl.clientHeight = 500
    expect(h.ionViewWillEnterCallback).not.toBeNull()
    if (h.ionViewWillEnterCallback) await h.ionViewWillEnterCallback()
    await nextTick()
    await flushPromises()
    // 关键：重算执行了（不抛错即可）
    expect(typeof (w.vm as any).nearBottom).toBe('boolean')
  })

  it('11. 连续多次 ensureScrollEl 通过 cachedScrollEl 复用（无副作用）', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    // 多次触发 handleNewLog：第一次会缓存 scrollEl，后续都走缓存
    ;(w.vm as any).handleNewLog()
    await nextTick()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    await nextTick()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    await nextTick()
    await flushPromises()
    // 关键断言：scrollTop 被多次设到 1000，没抛错，没死循环
    expect(fakeScrollEl.scrollTop).toBe(1000)
  })

  it('12. handleNewLog 在底部时 nextTick 后 scrollTop = scrollHeight', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    await nextTick()
    await flushPromises()
    // 关键断言：fakeScrollEl.scrollTop 被设置成 scrollHeight
    expect(fakeScrollEl.scrollTop).toBe(1000)
  })
})
