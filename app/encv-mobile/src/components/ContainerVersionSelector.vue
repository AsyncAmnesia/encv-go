<template>
  <div class="version-selector">
    <ion-radio-group :value="modelValue" @ionChange="handleChange">
      <ion-item
        v-for="ver in versions"
        :key="ver.version"
        :class="['version-item', `version-status-${ver.status}`, { 'item-disabled': ver.status === 'deprecated' }]"
        :disabled="ver.status === 'deprecated'"
        button
        :detail="false"
        lines="full"
        @click="ver.status !== 'deprecated' && selectVersion(ver.version)"
      >
        <ion-radio
          :value="ver.version"
          :disabled="ver.status === 'deprecated'"
          slot="start"
          :aria-label="ver.label"
        ></ion-radio>
        <ion-label>
          <span class="version-label">{{ ver.label }}</span>
        </ion-label>
        <ion-badge
          v-if="ver.status === 'recommended'"
          color="success"
          slot="end"
          class="status-badge"
        >{{ t('containerVersion.recommended') }}</ion-badge>
        <ion-badge
          v-else-if="ver.status === 'deprecated'"
          color="medium"
          slot="end"
          class="status-badge"
        >{{ t('containerVersion.deprecated') }}</ion-badge>
      </ion-item>
    </ion-radio-group>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  IonRadioGroup,
  IonRadio,
  IonItem,
  IonLabel,
  IonBadge,
} from '@ionic/vue'
import type { ContainerVersionInfo } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

const props = withDefaults(defineProps<{
  modelValue: number
  versions?: ContainerVersionInfo[]
}>(), {
  modelValue: 4,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: number): void
}>()

const { t } = useI18n()

// 容器格式版本：
//   - ECv3 = 旧版本（deprecated，但仍可创建/读取）
//   - ECv4 = 当前推荐版本
// 命名规则：ECv = ENCV Container，大写 EC，小写 v，避免与项目内 v2 架构命名混淆。
// 注：ECv2 已在 SupportedVersions 中移除，不再可选。
const defaultVersions: ContainerVersionInfo[] = [
  { version: 3, status: 'deprecated', label: 'ECv3' },
  { version: 4, status: 'recommended', label: 'ECv4' },
]

const versions = computed(() => props.versions || defaultVersions)

function handleChange(event: CustomEvent) {
  emit('update:modelValue', event.detail.value as number)
}

/**
 * 点击整行时主动同步选择（修复"只能点 radio 圆点才能切换"的 UX 问题）：
 *   - 在 Ionic 8 里，ion-radio-group 的 ionChange 事件只在 radio 本身被点击时触发；
 *     点击 ion-label / 空白区域不会冒泡到 radio。
 *   - 解决方案：ion-item 加 button 属性让整行可点击 + 显式 @click 触发 update:modelValue。
 *   - deprecated 状态直接禁用 item（不可点击），保留原视觉。
 */
function selectVersion(version: number) {
  if (version === props.modelValue) return
  emit('update:modelValue', version)
}
</script>

<style scoped>
.version-selector {
  width: 100%;
}

.version-item {
  --padding-start: 8px;
  --inner-padding-end: 12px;
  cursor: pointer;
}

.version-item.item-disabled {
  cursor: not-allowed;
  opacity: 0.55;
  pointer-events: auto;
}

.version-item.version-status-deprecated {
  --color: var(--ion-color-medium);
  opacity: 0.7;
}

.version-item.version-status-recommended {
  --background: rgba(var(--ion-color-success-rgb), 0.04);
}

.version-label {
  font-weight: 500;
}

.status-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 6px;
  --padding-top: 3px;
  --padding-bottom: 3px;
}
</style>
