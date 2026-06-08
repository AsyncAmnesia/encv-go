<!--
  StatusBadge - 三色调状态徽章
  参照 codex_web StatusBadge{label, tone}
  tone: ready=绿 (success), warn=橙 (warning), idle=灰 (medium)
  pulse: 是否显示脉冲（流式 / 进行中状态）
-->
<template>
  <span class="statusBadge" :class="[`statusBadge_${tone}`, { statusBadge_pulse: pulse }]">{{ label }}</span>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  tone: 'ready' | 'warn' | 'idle'
  pulse?: boolean
}>()
</script>

<style scoped>
.statusBadge {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 8px;
  border-radius: 9px;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0.02em;
  white-space: nowrap;
}

.statusBadge_ready {
  background: rgba(45, 211, 111, 0.14);
  color: var(--ion-color-success-shade, #28ba62);
}

.statusBadge_warn {
  background: rgba(255, 196, 9, 0.16);
  color: var(--ion-color-warning-shade, #e0ac08);
}

.statusBadge_idle {
  background: rgba(146, 148, 156, 0.16);
  color: var(--ion-color-medium-shade, #808289);
}

.statusBadge_pulse {
  animation: statusBadgePulse 1.4s ease-in-out infinite;
}

@keyframes statusBadgePulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

body.dark .statusBadge_ready {
  background: rgba(47, 223, 117, 0.18);
  color: #3de283;
}
body.dark .statusBadge_warn {
  background: rgba(255, 213, 72, 0.18);
  color: #ffda5a;
}
</style>
