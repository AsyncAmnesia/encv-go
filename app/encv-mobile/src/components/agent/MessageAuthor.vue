<!--
  MessageAuthor - 消息作者头
  参照 codex_web MessageAuthor{icon, label, meta}
  - icon: Ionicons 引用（必须先 import 后使用）
  - label: 作者名（"Codex" / "计划" / "审批" / "工具" / "出错"）
  - meta?:  状态文案（"正在思考" / "已完成" / ...）
-->
<template>
  <div class="messageAuthor">
    <div class="avatar" :class="`avatar_${variant}`">
      <ion-icon :icon="icon" class="avatarIcon"></ion-icon>
    </div>
    <div class="authorText">
      <div class="authorName">{{ label }}</div>
      <div v-if="meta" class="authorMeta">{{ meta }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IonIcon } from '@ionic/vue'
import type { Component } from 'vue'

const props = defineProps<{
  icon: Component | string
  label: string
  meta?: string
  /** 控制头像底色变体（可选） */
  variant?: 'default' | 'streaming' | 'error' | 'tool'
}>()

const variant = computed(() => props.variant || 'default')
</script>

<style scoped>
.messageAuthor {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
}

.avatar {
  width: 22px;
  height: 22px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: rgba(var(--ion-color-primary-rgb), 0.12);
  color: var(--ion-color-primary);
}

.avatar_streaming {
  background: rgba(var(--ion-color-primary-rgb), 0.18);
  animation: authorAvatarPulse 1.6s ease-in-out infinite;
}

.avatar_error {
  background: rgba(var(--ion-color-danger-rgb), 0.16);
  color: var(--ion-color-danger);
}

.avatar_tool {
  background: rgba(139, 92, 246, 0.16);
  color: #8b5cf6;
}

.avatarIcon {
  font-size: 13px;
}

.authorText {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
  min-width: 0;
}

.authorName {
  font-size: 13px;
  font-weight: 600;
  color: var(--ion-text-color);
}

.authorMeta {
  font-size: 11px;
  color: var(--encv-text-secondary);
  margin-top: 1px;
}

@keyframes authorAvatarPulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.55; }
}
</style>
