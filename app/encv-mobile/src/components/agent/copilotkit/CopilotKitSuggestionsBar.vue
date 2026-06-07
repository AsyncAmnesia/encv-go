<!--
  CopilotKitSuggestionsBar.vue - CopilotKit 风格Suggestions Chip Bar

  模仿 CopilotKit React 版的 Suggestions API：
  - 水平滚动容器（overflow-x: auto）
  - 每个 preset 渲染为圆角 pill 形状 chip 按钮
  - 样式：透明背景 + primary 色边框，hover 时填充
  - 点击触发 pick 事件 → 发送预设文本

  与 MockPresetBar 的差异：
  - 无 header/scenario/phase 区域（纯 chip bar）
  - 更简洁的 pill 样式（CopilotKit 设计语言）
  - 放置在消息列表底部（非输入框上方）
-->
<template>
  <div class="ckSuggestionsBar" role="region" :aria-label="'Suggestions'">
    <div class="ckSuggestionsScroll" role="list">
      <button
        v-for="preset in presets"
        :key="preset.id"
        type="button"
        class="ckSuggestionChip"
        :class="{ 'ckSuggestionChip_disabled': disabled }"
        :disabled="disabled"
        :title="preset.tooltip || preset.label"
        :data-testid="`ck-suggestion-chip-${preset.id}`"
        :aria-label="preset.tooltip || preset.label"
        role="listitem"
        @click="onPick(preset)"
      >
        <span v-if="preset.icon" class="ckSuggestionChipIcon" aria-hidden="true">{{ preset.icon }}</span>
        <span class="ckSuggestionChipLabel">{{ preset.label }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { MockPreset } from '@/composables/useAgent'

defineProps<{
  /** 预设建议列表 */
  presets: MockPreset[]
  /** 流式进行中时禁用 chip */
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'pick', preset: MockPreset): void
}>()

function onPick(preset: MockPreset): void {
  emit('pick', preset)
}
</script>

<style scoped>
/* ── Suggestions Bar 容器 ── */
.ckSuggestionsBar {
  flex-shrink: 0;
  padding: 8px 16px 10px 16px;
  border-top: 1px solid rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.10);
  background: linear-gradient(
    180deg,
    rgba(255, 255, 255, 0) 0%,
    rgba(79, 140, 255, 0.03) 100%
  );
}

/* ── 水平滚动容器 ── */
.ckSuggestionsScroll {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 4px 0 6px 0;
  scrollbar-width: thin;
  -webkit-overflow-scrolling: touch;
  /* 隐藏 scrollbar 但保持功能（移动端更干净） */
  scrollbar-width: none;
}

.ckSuggestionsScroll::-webkit-scrollbar {
  display: none;
}

/* ── Suggestion Chip（pill 形状） ── */
.ckSuggestionChip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
  height: 32px;
  padding: 0 16px;
  border-radius: 16px;
  border: 1px solid rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.35);
  background: transparent;
  color: var(--ion-color-primary, #4f8cff);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
  transition:
    background 150ms ease,
    border-color 150ms ease,
    box-shadow 150ms ease,
    transform 80ms ease;
  user-select: none;
  -webkit-tap-highlight-color: transparent;
}

.ckSuggestionChip:hover {
  background: rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.08);
  border-color: rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.55);
}

.ckSuggestionChip:active {
  transform: scale(0.96);
  background: rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.14);
}

.ckSuggestionChip:focus-visible {
  outline: 2px solid var(--ion-color-primary, #4f8cff);
  outline-offset: 2px;
}

/* 禁用态 */
.ckSuggestionChip_disabled,
.ckSuggestionChip:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  pointer-events: none;
}

/* ── Chip 图标 ── */
.ckSuggestionChipIcon {
  font-size: 14px;
  line-height: 1;
}

/* ── Chip 文字 ── */
.ckSuggestionChipLabel {
  display: inline-block;
}

/* ══════════════════════════════════════
   暗黑模式适配
   ══════════════════════════════════════ */

:host-context(body.dark) .ckSuggestionsBar,
:global(body.dark) .ckSuggestionsBar {
  border-top-color: rgba(var(--ion-color-primary-rgb, 79, 140, 255), 0.18);
  background: linear-gradient(
    180deg,
    rgba(255, 255, 255, 0) 0%,
    rgba(79, 140, 255, 0.05) 100%
  );
}

:host-context(body.dark) .ckSuggestionChip,
:global(body.dark) .ckSuggestionChip {
  border-color: rgba(138, 177, 255, 0.35);
  color: var(--ion-color-primary-tint, #8ab1ff);
}

:host-context(body.dark) .ckSuggestionChip:hover,
:global(body.dark) .ckSuggestionChip:hover {
  background: rgba(138, 177, 255, 0.10);
  border-color: rgba(138, 177, 255, 0.55);
}

:host-context(body.dark) .ckSuggestionChip:active,
:global(body.dark) .ckSuggestionChip:active {
  background: rgba(138, 177, 255, 0.18);
}
</style>
