<!--
  TDesignChatView.vue - TDesign Chat 渲染视图

  使用 SFC 模板语法渲染 Chatbot，确保 chatServiceConfig 对象
  能正确传递到 t-chatbot Web Component。
  omiVueify 包装的组件在 h() 渲染函数中传递复杂对象 prop 不可靠，
  但模板编译器能正确处理 v-bind / :prop 语法。
-->
<template>
  <div class="tdesign-chat-container" ref="containerRef">
    <Chatbot
      :chat-service-config="serviceConfig"
      layout="both"
      :auto-scroll="true"
      animation="gradient"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Chatbot } from '@tdesign-vue-next/chat'
import { getAgentApiBase } from '@/composables/useAgentApiBase'

const containerRef = ref<HTMLElement | null>(null)

// chatServiceConfig: AG-UI 协议配置
const apiBase = getAgentApiBase()
const serviceConfig = {
  endpoint: `${apiBase}/api/chat?protocol=agui`,
  protocol: 'agui' as const,
  stream: true,
}

onMounted(() => {
  // 注入 CSS 隐藏 Chatbot 自带的输入框
  const style = document.createElement('style')
  style.id = 'tdesign-chat-hide-sender'
  style.textContent = `
    .tdesign-chat-container t-chat-sender,
    .tdesign-chat-container .t-chat-sender,
    .tdesign-chat-container [class*="chat-sender"],
    .tdesign-chat-container [class*="chatSender"] {
      display: none !important;
    }
  `
  if (!document.getElementById('tdesign-chat-hide-sender')) {
    document.head.appendChild(style)
  }
})

onUnmounted(() => {
  const style = document.getElementById('tdesign-chat-hide-sender')
  if (style) style.remove()
})
</script>

<style scoped>
.tdesign-chat-container {
  height: 100%;
}
</style>
