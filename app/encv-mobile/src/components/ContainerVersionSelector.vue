<template>
  <div class="version-selector">
    <ion-radio-group :value="modelValue" @ionChange="handleChange">
      <RadioItem
        v-for="ver in versions"
        :key="ver.version"
        :value="ver.version"
        :selected="modelValue"
        :disabled="ver.status === 'deprecated'"
        :class="['version-item', `version-status-${ver.status}`]"
        @select="(v) => emit('update:modelValue', v as number)"
      >
        <span class="version-label">{{ ver.label }}</span>
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
      </RadioItem>
    </ion-radio-group>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  IonRadioGroup,
  IonBadge,
} from '@ionic/vue'
import type { ContainerVersionInfo } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import RadioItem from './RadioItem.vue'

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
