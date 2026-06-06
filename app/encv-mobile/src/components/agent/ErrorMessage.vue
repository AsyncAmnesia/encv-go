<!--
  ErrorMessage - 每条消息独立的错误块
  紧跟在出错的 user/assistant 消息下方显示

  关键：错误码决定按钮形态——
    - errorCode === 'no_api_key' 时显示"前往 AI 设置"按钮（替代重试，因为重试无用）
    - 其他情况显示"重试"按钮
-->
<template>
  <div class="errorMessage">
    <MessageAuthor :icon="icon" :label="label" :variant="'error'" />
    <div class="errorMessageBody">
      <pre class="errorText">{{ displayText }}</pre>
      <div class="errorActions">
        <!-- no_api_key 错误：跳转到 AI 设置（让用户填 key） -->
        <button
          v-if="errorCode === 'no_api_key' && onGoToSettings"
          type="button"
          class="errorActionBtn errorActionBtnPrimary"
          @click="onGoToSettings"
        >
          <ion-icon :icon="settingsIcon" />
          <span>{{ t('agent.chatErrorGoToSettings') }}</span>
        </button>
        <!-- 其他错误：重试 -->
        <button v-else-if="onRetry" type="button" class="errorActionBtn" @click="onRetry">
          <ion-icon :icon="refreshIcon" />
          <span>{{ t('common.retry') || '重试' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { alertCircleOutline, refreshOutline, settingsOutline } from 'ionicons/icons'
import MessageAuthor from './MessageAuthor.vue'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{
  text: string
  errorCode?: string
  onRetry?: () => void
  onGoToSettings?: () => void
}>()

const { t } = useI18n()

const icon = alertCircleOutline
const refreshIcon = refreshOutline
const settingsIcon = settingsOutline
const label = t('common.error') || '出错'

// 错误信息也要按 code 分支——no_api_key 用专门文案，upstream 用后端 message 拼接
const displayText = computed(() => {
  if (props.errorCode === 'no_api_key') {
    return t('agent.chatErrorNoApiKey') || props.text
  }
  if (props.errorCode === 'upstream_error') {
    return t('agent.chatErrorUpstream', { message: stripHttpSuffix(props.text) }) || props.text
  }
  return props.text
})

// 把 "xxx（HTTP 503）" 后缀剥掉，只留核心文案
function stripHttpSuffix(s: string): string {
  return s.replace(/（HTTP\s*\d+）\s*$/, '').replace(/\(HTTP\s*\d+\)\s*$/, '').trim()
}
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
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.errorText {
  margin: 0;
  padding: 8px 12px;
  background: var(--error-bg, rgba(239, 68, 68, 0.08));
  border: 1px solid var(--error-border, rgba(239, 68, 68, 0.25));
  border-radius: 6px;
  color: var(--ion-color-danger-shade, #c1272d);
  font-size: 12.5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.errorActions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.errorActionBtn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 12px;
  border: 1px solid var(--error-border, rgba(239, 68, 68, 0.25));
  border-radius: 6px;
  background: transparent;
  color: var(--ion-color-danger, #ef4444);
  font-size: 11.5px;
  cursor: pointer;
  font-family: inherit;
}
.errorActionBtn:hover {
  background: var(--error-bg, rgba(239, 68, 68, 0.08));
}

/* 主要操作（去设置）用 primary 色，比重试按钮更醒目 */
.errorActionBtnPrimary {
  border-color: var(--ion-color-primary, #4f8cff);
  color: var(--ion-color-primary, #4f8cff);
}
.errorActionBtnPrimary:hover {
  background: rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.08);
}
</style>
