<!--
  ErrorMessage - 错误消息块
  头部：MessageAuthor(alertCircle, "出错", status)
  主体：error 文本（pre 形式）
-->
<template>
  <div class="errorMessage">
    <MessageAuthor :icon="icon" :label="label" :variant="'error'" />
    <div class="errorMessageBody">
      <pre class="errorText">{{ text }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { alertCircleOutline } from 'ionicons/icons'
import MessageAuthor from './MessageAuthor.vue'

defineProps<{ text: string }>()
const icon = alertCircleOutline
const label = '出错'
</script>

<style scoped>
.errorMessage {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 0;
  max-width: 100%;
}

.errorMessageBody {
  padding-left: 30px;
}

.errorText {
  margin: 0;
  padding: 8px 12px;
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  border: 1px solid rgba(var(--ion-color-danger-rgb), 0.3);
  border-radius: 6px;
  color: var(--ion-color-danger-shade, #c1272d);
  font-size: 12.5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
