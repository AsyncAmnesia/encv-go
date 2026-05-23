<script setup lang="ts">
import { ref, computed } from 'vue'

const props = withDefaults(defineProps<{
  progress: number
  currentTime: string
  duration: string
}>(), {
  progress: 0,
  currentTime: '0:00',
  duration: '0:00',
})

const emit = defineEmits<{
  seek: [progress: number]
}>()

const trackRef = ref<any>(null)

const clampedProgress = computed(() => Math.max(0, Math.min(1, props.progress)))

function handleTrackTap(e: any) {
  if (!trackRef.value) return
  // TODO: @panmove drag-to-seek gesture support
  const trackX = e.detail?.clientX ?? e.clientX ?? 0
  const rect = trackRef.value.getBoundingClientRect?.()
  if (!rect) return
  const relativeX = trackX - rect.left
  const ratio = Math.max(0, Math.min(1, relativeX / rect.width))
  emit('seek', ratio)
}
</script>

<template>
  <view class="ProgressRow">
    <text class="TimeLabel">{{ currentTime }}</text>
    <view
      ref="trackRef"
      class="SliderTrackOuter"
      @tap="handleTrackTap"
    >
      <view class="SliderTrackBg" />
      <view class="SliderFill" :style="{ width: (clampedProgress * 100) + '%' }" />
      <view class="SliderThumbWrapper" :style="{ left: 'calc(' + (clampedProgress * 100) + '% - 8px)' }">
        <view class="SliderThumbDot" />
      </view>
    </view>
    <text class="TimeLabelEnd">{{ duration }}</text>
  </view>
</template>

<style scoped>
.ProgressRow {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 4px 0;
  width: 100%;
}

.TimeLabel {
  color: rgba(255, 255, 255, 0.9);
  font-size: 13px;
  min-width: 48px;
}

.TimeLabelEnd {
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
  min-width: 48px;
  text-align: right;
}

.SliderTrackOuter {
  flex: 1;
  display: flex;
  position: relative;
  height: 20px;
  margin-left: 10px;
  margin-right: 10px;
  justify-content: center;
  align-items: center;
}

.SliderTrackBg {
  position: absolute;
  left: 0;
  right: 0;
  height: 4px;
  background-color: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
}

.SliderFill {
  position: absolute;
  left: 0;
  height: 4px;
  background-color: #4a90d9;
  border-radius: 2px;
}

.SliderThumbWrapper {
  position: absolute;
  display: flex;
  justify-content: center;
  align-items: center;
  top: 50%;
  transform: translateY(-50%);
}

.SliderThumbDot {
  width: 16px;
  height: 16px;
  border-radius: 8px;
  background-color: #4a90d9;
  border-width: 2px;
  border-style: solid;
  border-color: #ffffff;
}
</style>
