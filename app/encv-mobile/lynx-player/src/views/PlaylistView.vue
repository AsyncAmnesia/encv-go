<script setup lang="ts">
import { useRouter } from 'vue-router'

interface PlaybackItem {
  filePath: string
  fileName: string
  mediaType: 'video' | 'audio'
}

const props = defineProps<{
  items: PlaybackItem[]
  currentIndex: number
}>()

const emit = defineEmits<{
  select: [index: number]
}>()

const router = useRouter()

const mediaTypeLabel = (type: string) => (type === 'video' ? '视频' : '音频')
</script>

<template>
  <view class="PlaylistPage">
    <view class="PlaylistHeader">
      <view class="CtrlBtn" @tap="router.back()">
        <text class="IconMd">&#x2190;</text>
      </view>
      <text class="PlaylistTitle">播放列表</text>
      <text class="PlaylistCount">{{ items.length }} 首</text>
    </view>

    <scroll-view class="PlaylistScroll" scroll-orientation="vertical">
      <view
        v-for="(item, idx) in items"
        :key="idx"
        :class="['PlaylistItem', idx === currentIndex ? 'PlaylistItemActive' : '']"
        @tap="emit('select', idx)"
      >
        <view class="PlaylistItemIndex">
          <text v-if="idx === currentIndex" class="PlayingIcon">&#x25B6;</text>
          <text v-else class="ItemIndexText">{{ idx + 1 }}</text>
        </view>
        <view class="PlaylistItemInfo">
          <text class="PlaylistItemName">{{ item.fileName }}</text>
          <text class="PlaylistItemType">{{ mediaTypeLabel(item.mediaType) }}</text>
        </view>
      </view>
    </scroll-view>
  </view>
</template>

<style scoped>
.PlaylistPage {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background-color: #0a0a0f;
}

.PlaylistHeader {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 12px 16px;
  width: 100%;
}

.CtrlBtn {
  display: flex;
  background-color: transparent;
  min-width: 44px;
  min-height: 44px;
  justify-content: center;
  align-items: center;
}

.IconMd {
  color: #ffffff;
  font-size: 22px;
  text-align: center;
}

.PlaylistTitle {
  color: #ffffff;
  font-size: 18px;
  font-weight: 600;
  flex: 1;
  margin-left: 8px;
}

.PlaylistCount {
  color: rgba(255, 255, 255, 0.4);
  font-size: 13px;
}

.PlaylistScroll {
  display: flex;
  flex: 1;
  width: 100%;
}

.PlaylistItem {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 12px 16px;
  width: 100%;
  border-bottom-width: 1px;
  border-bottom-style: solid;
  border-bottom-color: rgba(255, 255, 255, 0.06);
}

.PlaylistItemActive {
  background-color: rgba(74, 144, 217, 0.1);
}

.PlaylistItemIndex {
  display: flex;
  width: 36px;
  min-height: 36px;
  justify-content: center;
  align-items: center;
  margin-right: 12px;
}

.PlayingIcon {
  color: #4a90d9;
  font-size: 14px;
}

.ItemIndexText {
  color: rgba(255, 255, 255, 0.3);
  font-size: 14px;
}

.PlaylistItemInfo {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.PlaylistItemName {
  color: rgba(255, 255, 255, 0.9);
  font-size: 14px;
}

.PlaylistItemType {
  color: rgba(255, 255, 255, 0.35);
  font-size: 12px;
  margin-top: 2px;
}
</style>
