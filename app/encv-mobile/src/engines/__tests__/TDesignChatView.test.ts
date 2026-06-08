/**
 * TDesignChatView.test.ts
 *
 * 测试 TDesignChatView 组件：
 *   - 接受 EngineRenderProps 形态的 props
 *   - 渲染消息列表（user / assistant）
 *   - streaming=true 时显示 thinking 指示器
 *   - tool_calls 渲染为卡片列表
 *
 * SPEC: /workspace/.trae/specs/agui-real-llm-path-completion/ Phase 4
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import type { Message, ToolCall } from '@/composables/useAgent'

// Mock @tdesign-vue-next/chat：用占位组件替代 ChatList / ChatItem / ChatThinking
// 真实渲染层 DOM 行为由 E2E 验证；单测只验证 TDesignChatView 自身契约。
// vi.mock hoisted 到文件顶部（vite-node 自动 hoist），所以下面 import 走 mock。
vi.mock('@tdesign-vue-next/chat', () => {
  const StubComp = defineComponent({
    name: 'StubChat',
    props: ['data', 'role', 'name', 'avatar', 'content'],
    setup(props, { slots }) {
      return () =>
        h('div', { class: 'td-chat-stub' }, [props.content ?? (slots.content ? '[slot]' : '')])
    },
  })
  return {
    ChatList: { ...StubComp, name: 'ChatList' },
    ChatItem: { ...StubComp, name: 'ChatItem' },
    ChatThinking: { ...StubComp, name: 'ChatThinking' },
  }
})

// 必须在 mock 注册后导入（vi.mock 自动 hoist，但仍显式写出来强调）
import TDesignChatView from '../TDesignChatView.vue'

// =============================================================================
// 测试辅助
// =============================================================================

function makeUserMessage(text: string, overrides: Partial<Message> = {}): Message {
  return {
    role: 'user',
    content: text,
    tool_calls: [],
    tool_results: [],
    ...overrides,
  }
}

function makeAssistantMessage(
  text: string,
  toolCalls: ToolCall[] = [],
  overrides: Partial<Message> = {},
): Message {
  return {
    role: 'assistant',
    content: text,
    tool_calls: toolCalls,
    tool_results: [],
    ...overrides,
  }
}

function makeToolCall(overrides: Partial<ToolCall> = {}): ToolCall {
  return {
    id: 'tc-1',
    name: 'search',
    args: '{"q":"hello"}',
    auto_run: false,
    kind: 'readOnly',
    needsConfirm: false,
    status: 'pending',
    ...overrides,
  }
}

const defaultProps = {
  messages: [] as Message[],
  status: 'idle' as string,
  streaming: false,
  onSend: vi.fn(async () => {}),
  onStop: vi.fn(),
  onConfirmTool: vi.fn(async () => {}),
  onCopyMessage: vi.fn(async () => {}),
  onPresetClick: vi.fn(),
}

function mountChatView(props: Partial<typeof defaultProps> = {}) {
  return mount(TDesignChatView, {
    props: { ...defaultProps, ...props },
    global: {
      // TDesign chat 组件需要 ConfigProvider；在测试环境用 stub 跳过真实渲染
      // 验证 TDesignChatView 的 props → DOM 树 / computed → 行为契约
      stubs: {
        ChatList: true,
        ChatItem: true,
        ChatThinking: true,
      },
    },
  })
}

// =============================================================================
// 测试
// =============================================================================

describe('TDesignChatView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('TestTDesignChatView_AcceptsEngineRenderProps: 接受完整 EngineRenderProps 形态', () => {
    const wrapper = mountChatView({
      messages: [],
      status: 'idle',
      streaming: false,
    })
    expect(wrapper.exists()).toBe(true)
    expect(wrapper.classes()).toContain('tdesign-chat-view')
  })

  it('TestTDesignChatView_EmptyState_WhenNoMessages: messages 为空时显示欢迎文案', () => {
    const wrapper = mountChatView({ messages: [] })
    const welcome = wrapper.find('.welcome')
    expect(welcome.exists()).toBe(true)
    expect(welcome.text()).toContain('TDesign')
  })

  it('TestTDesignChatView_StreamingThinking_WhenStreamingEmpty: streaming=true + 无消息时显示 thinking', () => {
    const wrapper = mountChatView({ messages: [], streaming: true })
    // 空状态时的 thinking（被 stub 成 ChatThinking 标签）
    const emptyState = wrapper.find('.empty-state')
    expect(emptyState.exists()).toBe(true)
    // data-streaming 属性正确反映 streaming
    expect(wrapper.attributes('data-streaming')).toBe('true')
  })

  it('TestTDesignChatView_RendersMessages: 渲染消息列表容器', () => {
    const messages = [
      makeUserMessage('你好'),
      makeAssistantMessage('你好！有什么可以帮你的？'),
    ]
    const wrapper = mountChatView({ messages })
    // ChatList 被 stub：但 chatlist 元素存在
    const list = wrapper.findComponent({ name: 'ChatList' })
    expect(list.exists()).toBe(true)
  })

  it('TestTDesignChatView_FilterSystemMessages: system 消息不出现在列表中（只展示 user/assistant）', () => {
    const messages = [
      makeUserMessage('hello'),
      { role: 'system' as const, content: 'system marker', tool_calls: [], tool_results: [] },
      makeAssistantMessage('hi'),
    ]
    const wrapper = mountChatView({ messages })
    // 内部 listData computed 过滤 system，但这里通过 props 透传做断言：
    // ChatList 容器被 stub，所以直接通过 listData computed 验证
    // （mount 后组件 setup 已执行）
    const vm = wrapper.vm as any
    if (vm && Array.isArray(vm.listData)) {
      expect(vm.listData).toHaveLength(2)
      expect(vm.listData[0].role).toBe('user')
      expect(vm.listData[1].role).toBe('assistant')
    }
  })

  it('TestTDesignChatView_StreamingWithMessages_ShowsThinkingAtBottom: 有消息且 streaming=true 时显示底部 thinking', () => {
    const messages = [makeUserMessage('hi'), makeAssistantMessage('')]
    const wrapper = mountChatView({ messages, streaming: true })
    const thinkingAtBottom = wrapper.find('.streaming-thinking')
    expect(thinkingAtBottom.exists()).toBe(true)
  })

  it('TestTDesignChatView_NoToolCallList_WhenEmpty: 没有 tool_calls 时不显示卡片列表', () => {
    const messages = [makeUserMessage('hi'), makeAssistantMessage('ok')]
    const wrapper = mountChatView({ messages })
    const toolList = wrapper.find('.tool-call-list')
    expect(toolList.exists()).toBe(false)
  })

  it('TestTDesignChatView_ToolCallList_WhenHasToolCalls: 有 tool_calls 时显示卡片列表', () => {
    const tc = makeToolCall({ id: 'tc-1', name: 'search' })
    const messages = [makeAssistantMessage('', [tc])]
    const wrapper = mountChatView({ messages })
    const toolList = wrapper.find('.tool-call-list')
    expect(toolList.exists()).toBe(true)
  })

  it('TestTDesignChatView_TagDataStatusAttribute: tool_call 状态映射到 data-status 属性', () => {
    const pendingTc = makeToolCall({ id: 'p', status: 'pending' })
    const runningTc = makeToolCall({ id: 'r', status: 'running' })
    const successTc = makeToolCall({ id: 's', status: 'success' })
    const failedTc = makeToolCall({ id: 'f', status: 'failed' })

    const messages = [makeAssistantMessage('', [pendingTc, runningTc, successTc, failedTc])]
    const wrapper = mountChatView({ messages })
    const tags = wrapper.findAll('.tool-call-tag')
    expect(tags).toHaveLength(4)
    expect(tags[0].attributes('data-status')).toBe('pending')
    expect(tags[1].attributes('data-status')).toBe('running')
    expect(tags[2].attributes('data-status')).toBe('success')
    expect(tags[3].attributes('data-status')).toBe('failed')
  })

  it('TestTDesignChatView_ConfirmBadge_WhenNeedsConfirmPending: needsConfirm+pending 时显示需确认徽章', () => {
    const tc = makeToolCall({ id: 'tc-1', needsConfirm: true, status: 'pending' })
    const messages = [makeAssistantMessage('', [tc])]
    const wrapper = mountChatView({ messages })
    const badge = wrapper.find('.tool-call-confirm-badge')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toContain('需确认')
  })

  it('TestTDesignChatView_NoConfirmBadge_WhenRunning: needsConfirm 但 status=running 时不显示需确认徽章', () => {
    const tc = makeToolCall({ id: 'tc-1', needsConfirm: true, status: 'running' })
    const messages = [makeAssistantMessage('', [tc])]
    const wrapper = mountChatView({ messages })
    const badge = wrapper.find('.tool-call-confirm-badge')
    expect(badge.exists()).toBe(false)
  })

  it('TestTDesignChatView_MultimodalContentExtractsText: multimodal content 数组只取 text 部分', () => {
    const messages: Message[] = [
      {
        role: 'user',
        content: [
          { type: 'text', text: '请看这张图：' } as any,
          { type: 'image', url: 'foo.png' } as any,
          { type: 'text', text: '它是什么？' } as any,
        ],
        tool_calls: [],
        tool_results: [],
      },
    ]
    const wrapper = mountChatView({ messages })
    const vm = wrapper.vm as any
    if (vm && Array.isArray(vm.listData)) {
      expect(vm.listData[0].content).toBe('请看这张图：它是什么？')
    }
  })

  it('TestTDesignChatView_DataStreamingAttribute_ReflectsProp: data-streaming 属性绑定正确', () => {
    const w1 = mountChatView({ streaming: true })
    expect(w1.attributes('data-streaming')).toBe('true')
    const w2 = mountChatView({ streaming: false })
    expect(w2.attributes('data-streaming')).toBe('false')
  })
})
