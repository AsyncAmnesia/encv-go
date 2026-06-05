<!--
  AgentEntry - AI 助手浮动入口
  - 右下角绝对定位 + 高 z-index
  - 点击 → modalController.create({ component: AgentChat, componentProps: { apiBase: '/agent-api' } }).present()
  - 注意：只挂载在 HomePage（不在路由里）
-->
<template>
  <button
    type="button"
    class="agentEntry"
    :title="t('agent.fabLabel')"
    :aria-label="t('agent.fabLabel')"
    @click="handleOpen"
  >
    <ion-icon :icon="sparkleIcon" class="agentEntryIcon" />
  </button>
</template>

<script setup lang="ts">
import { IonIcon, modalController } from '@ionic/vue'
import { sparklesOutline } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import AgentChat from '@/views/AgentChat.vue'

const { t } = useI18n()
const sparkleIcon = sparklesOutline

async function handleOpen() {
  const modal = await modalController.create({
    component: AgentChat,
    componentProps: { apiBase: '/agent-api' },
    cssClass: 'agent-chat-modal',
    showBackdrop: true,
    breakpoints: undefined,
    initialBreakpoint: undefined,
  })
  await modal.present()
}
</script>

<style scoped>
.agentEntry {
  position: fixed;
  right: calc(16px + env(safe-area-inset-right, 0px));
  bottom: calc(72px + env(safe-area-inset-bottom, 0px));
  z-index: 999;
  width: 52px;
  height: 52px;
  border: 0;
  border-radius: 50%;
  background: var(--ion-color-primary);
  color: var(--ion-color-primary-contrast, #fff);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.25), 0 2px 4px rgba(var(--ion-color-primary-rgb), 0.4);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: transform 0.12s, box-shadow 0.12s;
}

.agentEntry:hover {
  transform: scale(1.05);
  box-shadow: 0 8px 22px rgba(0, 0, 0, 0.3), 0 3px 6px rgba(var(--ion-color-primary-rgb), 0.5);
}

.agentEntry:active {
  transform: scale(0.95);
}

.agentEntryIcon {
  font-size: 24px;
}

/* 暗黑模式额外 glow */
body.dark .agentEntry {
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(var(--ion-color-primary-rgb), 0.4);
}
</style>

<style>
/* 全局：AgentChat modal 全屏（不挡 header 但占满） */
ion-modal.agent-chat-modal {
  --width: 100%;
  --height: 100%;
  --max-width: 100%;
  --max-height: 100%;
  --border-radius: 0;
}
</style>
