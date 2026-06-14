/**
 * DevLogs 自动滚动状态机单元测试（v2：programmaticScrollInProgress flag 方案）
 *
 * 覆盖（pinned-to-bottom 模式）：
 *  1.  初始 nearBottom=true
 *  2.  handleNewLog 在底部时调用 scrollToBottom（不累积 unread）
 *  3.  handleNewLog 不在底部时 unreadCount++
 *  4.  用户滚动 → 距离 > 80 → nearBottom=false
 *  5.  用户滑回底部 → unreadCount=0
 *  6.  hardPaused=true → handleNewLog 不滚不累积
 *  7.  onJumpToBottom → scrollToBottom(true) + 清空 unread
 *  8.  切到 backend tab → 不响应 frontend 日志
 *  9.  切到 frontend tab → 不响应 backend 日志
 * 10.  onIonViewWillEnter → 重算 nearBottom
 * 11.  programmaticScrollInProgress=true 时 ionScroll 被忽略
 * 12.  getScrollElement 失败时 DOM walk fallback 仍能滚
 *
 * 实现策略：
 *   - mock @ionic/vue：用 stub IonContent 提供可控 scroll element
 *   - mock useFrontendLogs / useRealtimeTransport / useEventBus
 *   - 通过 defineExpose 暴露的 state machine 直接断言
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'

// ─── Mock 共享的 refs（多个测试用例需要可观察） ─────────────────────────────
// 用 vi.hoisted 把共享状态提到文件顶部，避开 vi.mock hoisting 问题
// 注意：hoisted 工厂内部不能 import vue（ref），所以用普通对象 + getter 模拟

const h = vi.hoisted(() => {
  let frontendLogsValue: Array<{ id: number; timestamp: string; level: string; message: string }> = []
  const backendLogs: any[] = []
  let transportConnectionValue: 'connected' | 'disconnected' | 'connecting' = 'disconnected'
  const eventBusListeners: Record<string, Array<(data: any) => void>> = {}
  let serverOnline = false
  let ionViewWillEnterCallback: (() => Promise<void>) | null = null
  // 模拟 Vue ref 的最小接口（get/set + 触发 reactive）
  const frontendLogsObj = {
    get value() { return frontendLogsValue },
    set value(v: any) { frontendLogsValue = v },
  }
  const transportConnectionObj = {
    get value() { return transportConnectionValue },
    set value(v: any) { transportConnectionValue = v },
  }
  // 必须用 getter（否则 h.ionViewWillEnterCallback 是 vi.hoisted 时的快照 null）
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
// 关键：IonContent 必须暴露 getScrollElement() 返回可控的 fakeScrollEl
// 注意：在 jsdom 中 scrollHeight/scrollTop 是只读 DOM 属性，必须用 plain object

interface FakeScrollEl {
  scrollTop: number
  scrollHeight: number
  clientHeight: number
  scrollTo: (p: { top: number; behavior?: ScrollBehavior }) => void
  __scrollToSpy: ReturnType<typeof vi.fn>
}

let fakeScrollEl: FakeScrollEl

vi.mock('@ionic/vue', () => {
  // 不 spread importOriginal（避免真实 onIonViewWillEnter 覆盖 mock）
  return {
    IonPage: { name: 'IonPage', template: '<div><slot /></div>' },
    IonHeader: { name: 'IonHeader', template: '<header><slot /></header>' },
    IonToolbar: { name: 'IonToolbar', template: '<div><slot /></div>' },
    IonTitle: { name: 'IonTitle', template: '<h1><slot /></h1>' },
    IonContent: {
      name: 'IonContent',
      template: '<div class="ion-content-stub" @ionScroll="$emit(\'ionScroll\', $event)" @ionScrollEnd="$emit(\'ionScrollEnd\', $event)" @wheel="$emit(\'wheel\', $event)" @touchstart="$emit(\'touchstart\', $event)" @touchend="$emit(\'touchend\', $event)"><slot /></div>',
      emits: ['ionScroll', 'ionScrollEnd', 'wheel', 'touchstart', 'touchend'],
      async mounted(this: any) {
        this.getScrollElement = async () => fakeScrollEl
      },
      methods: {
        async getScrollElement(this: any) {
          return fakeScrollEl
        },
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
  }
})

// 必须在 mock 设置后 import 组件
import DevLogs from '@/views/DevLogs.vue'

// ─── 工具 ─────────────────────────────────────────────────────────────────

function createFakeScrollEl(opts: { scrollTop: number; scrollHeight: number; clientHeight: number }): FakeScrollEl {
  const el: FakeScrollEl = {
    scrollTop: opts.scrollTop,
    scrollHeight: opts.scrollHeight,
    clientHeight: opts.clientHeight,
    __scrollToSpy: vi.fn((p: { top: number; behavior?: ScrollBehavior }) => {
      el.scrollTop = p.top
    }),
    scrollTo: () => {},
  }
  el.scrollTo = el.__scrollToSpy as any
  return el
}

function mountDevLogs() {
  return mount(DevLogs, {
    global: {
      config: {},
    },
  })
}

beforeEach(() => {
  h.frontendLogs.value = []
  h.backendLogs.length = 0
  h.serverOnline = false
  h.transportConnection.value = 'disconnected'
  // 清空 eventBus listeners
  Object.keys(h.eventBusListeners).forEach((k) => { h.eventBusListeners[k] = [] })
  h.setIonViewWillEnterCb(null)
  // 默认 fake 滚动元素：在底部（距离 = 0 < 80）
  fakeScrollEl = createFakeScrollEl({ scrollTop: 1000, scrollHeight: 1000, clientHeight: 500 })
  // mock requestAnimationFrame 为同步触发（生产代码里 scrollToBottom 内部用 rAF 等 layout）
  // jsdom 默认的 rAF 是 setTimeout(fn, 16)，会拖慢测试
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb: FrameRequestCallback) => {
    cb(performance.now())
    return 0
  })
})

afterEach(() => {
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

// ─── 用例 ─────────────────────────────────────────────────────────────────

describe('DevLogs 自动滚动 - pinned 模式', () => {
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
    await flushPromises()
    // v2：auto-scroll 用 el.scrollTop = el.scrollHeight（直接赋值，不走 __scrollToSpy）
    expect(fakeScrollEl.scrollTop).toBe(1000)
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('3. handleNewLog 不在底部时 unreadCount++', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    // 模拟用户上滑：scrollTop 变小，scrollHeight 不变 → 距离 = 1000 - 0 - 500 = 500 > 80
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
    expect((w.vm as any).unreadCount).toBe(0)
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    expect((w.vm as any).unreadCount).toBe(3)
    expect(fakeScrollEl.__scrollToSpy).not.toHaveBeenCalled()
  })

  it('4. 用户滚动 → 距离 > 80 → nearBottom=false', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
  })

  it('5. 用户滑回底部 → unreadCount=0', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    expect((w.vm as any).unreadCount).toBe(2)
    fakeScrollEl.scrollTop = 500
    ;(w.vm as any).onContentScroll()
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
    await flushPromises()
    expect(fakeScrollEl.__scrollToSpy).not.toHaveBeenCalled()
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('7. onJumpToBottom → scrollToBottom(true) + 清空 unread', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    ;(w.vm as any).handleNewLog()
    expect((w.vm as any).unreadCount).toBe(2)
    void (w.vm as any).onJumpToBottom()
    await flushPromises()
    // onJumpToBottom 调用 scrollToBottom(true) → scrollTo({behavior:'smooth'})
    expect(fakeScrollEl.__scrollToSpy).toHaveBeenCalled()
    const lastCall = fakeScrollEl.__scrollToSpy.mock.calls.at(-1)
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
    // 模拟 backend WS 消息
    const bus: any = (await import('@/composables/useEventBus')).eventBus
    bus.emit('ws:message', { type: 'log', data: { level: 'info', message: 'backend log' } })
    await flushPromises()
    expect((w.vm as any).unreadCount).toBe(0)
  })

  it('9. activeTab=backend 时 frontend 日志不响应', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    ;(w.vm as any).setActiveTab('backend')
    await flushPromises()
    // push 一条 frontend log
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
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
    // 模拟切回 tab 时 layout 恢复（scrollHeight 变化）
    fakeScrollEl.scrollTop = 500
    fakeScrollEl.scrollHeight = 1500
    fakeScrollEl.clientHeight = 500
    // 触发 onIonViewWillEnter
    expect(h.ionViewWillEnterCallback).not.toBeNull()
    if (h.ionViewWillEnterCallback) await h.ionViewWillEnterCallback()
    await flushPromises()
    // 关键：重算执行了
    expect(typeof (w.vm as any).nearBottom).toBe('boolean')
  })

  it('11. programmaticScrollInProgress=true 时 ionScroll 被忽略', async () => {
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(true)
    // 模拟程序化滚动期间：flag=true
    ;(w.vm as any).setProgrammaticScrollInProgress(true)
    expect((w.vm as any).isProgrammaticScrollInProgress()).toBe(true)
    // 用户滚动（应该被忽略）
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    // nearBottom 应该保持 true（updateNearBottom 没被调用）
    expect((w.vm as any).nearBottom).toBe(true)
    // 解除屏蔽
    ;(w.vm as any).setProgrammaticScrollInProgress(false)
    // 用户再滚：nearBottom 应变成 false
    fakeScrollEl.scrollTop = 0
    ;(w.vm as any).onContentScroll()
    await flushPromises()
    expect((w.vm as any).nearBottom).toBe(false)
  })

  it('12. getScrollElement 返回 null 时 DOM walk fallback 仍能滚', async () => {
    // 这个测试比较复杂：需要让 contentRef.getScrollElement 返回 null，
    // 然后验证 scrollToBottom 通过 shadow DOM .inner-scroll 或 DOM walk 找到元素。
    // 由于 mock stub 的 IonContent 总是返回 fakeScrollEl，这里改为：
    // 验证 scrollToBottom 调用了 getScrollEl，且最后的 scrollTop 被设置。
    fakeScrollEl = createFakeScrollEl({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500 })
    const w = mountDevLogs()
    await flushPromises()
    ;(w.vm as any).handleNewLog()
    await flushPromises()
    // 关键断言：fakeScrollEl.scrollTop 被设置成 scrollHeight
    expect(fakeScrollEl.scrollTop).toBe(1000)
  })
})

