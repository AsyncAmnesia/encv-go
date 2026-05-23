<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

interface RecentItem {
  fileName: string
  filePath: string
  mediaType: 'video' | 'audio'
  lastPlayed: string
}

interface MediaCategory {
  label: string
  icon: string
  path: string
  count: number
}

const props = defineProps<{ hasInitData: boolean }>()
const emit = defineEmits<{
  'play-init': []
  'open-settings': []
}>()

const router = useRouter()

const recentItems = ref<RecentItem[]>([])
const categories = ref<MediaCategory[]>([
  { label: '视频', icon: '🎬', path: '/videos', count: 0 },
  { label: '音频', icon: '🎵', path: '/audio', count: 0 },
  { label: '播放列表', icon: '📜', path: '/playlists', count: 0 },
])

// TODO: 从后端 API 加载最近播放记录
// GET /api/player/recent → RecentItem[]
// 目前使用空数组作为占位

// TODO: 从后端 API 扫描媒体文件分类
// GET /api/player/categories → MediaCategory[]
// 目前使用占位分类

function handleCategoryTap(category: MediaCategory) {
  // TODO: 打开对应分类的文件浏览器，选择文件后播放
  // 需要后端 API: GET /api/player/browse?path=xxx
  // 选择文件后导航到播放器或调用 play-init
  router.push({ name: 'player' })
}

function handleRecentTap(item: RecentItem) {
  router.push({ name: 'player' })
}
</script>

<template>
  <view class="home-page">
    <view class="home-header">
      <text class="home-title">媒体中心</text>
      <view class="settings-btn" @tap="emit('open-settings')">
        <text class="settings-icon">⚙</text>
      </view>
    </view>

    <scroll-view class="home-content" scroll-orientation="vertical">
      <view v-if="props.hasInitData" class="quick-play-banner">
        <text class="quick-play-text">有文件待播放</text>
        <view class="quick-play-btn" @tap="emit('play-init')">
          <text class="quick-play-btn-text">立即播放</text>
        </view>
      </view>

      <view class="section-header">
        <text class="section-title">最近播放</text>
      </view>
      <view v-if="recentItems.length === 0" class="empty-section">
        <text class="empty-text">暂无播放记录</text>
      </view>
      <scroll-view v-else class="recent-scroll" scroll-orientation="horizontal">
        <view
          v-for="(item, idx) in recentItems"
          :key="idx"
          class="recent-card"
          @tap="handleRecentTap(item)"
        >
          <view class="recent-card-icon">
            <text class="recent-icon-text">{{ item.mediaType === 'video' ? '🎬' : '🎵' }}</text>
          </view>
          <text class="recent-card-name">{{ item.fileName }}</text>
        </view>
      </scroll-view>

      <view class="section-header">
        <text class="section-title">媒体分类</text>
      </view>
      <view class="category-grid">
        <view
          v-for="(cat, idx) in categories"
          :key="idx"
          class="category-card"
          @tap="handleCategoryTap(cat)"
        >
          <text class="category-icon">{{ cat.icon }}</text>
          <text class="category-label">{{ cat.label }}</text>
          <text v-if="cat.count > 0" class="category-count">{{ cat.count }}</text>
        </view>
      </view>

      <view class="section-header">
        <text class="section-title">播放列表</text>
      </view>
      <view class="empty-section">
        <text class="empty-text">暂无播放列表</text>
      </view>
    </scroll-view>
  </view>
</template>

<style scoped>
.home-page {
  width: 100%;
  height: 100%;
  background-color: #0a0a0f;
}

.home-header {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  background-color: rgba(255, 255, 255, 0.03);
  border-bottom-width: 1px;
  border-bottom-color: rgba(255, 255, 255, 0.06);
}

.home-title {
  font-size: 22px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.9);
}

.settings-btn {
  width: 40px;
  height: 40px;
  align-items: center;
  justify-content: center;
  border-radius: 20px;
  background-color: rgba(255, 255, 255, 0.06);
}

.settings-icon {
  font-size: 20px;
  color: rgba(255, 255, 255, 0.7);
}

.home-content {
  flex: 1;
  padding: 0 20px;
}

.quick-play-banner {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
  padding: 16px 20px;
  background-color: rgba(74, 144, 217, 0.15);
  border-radius: 12px;
  border-width: 1px;
  border-color: rgba(74, 144, 217, 0.3);
}

.quick-play-text {
  font-size: 15px;
  color: rgba(255, 255, 255, 0.9);
}

.quick-play-btn {
  padding: 8px 20px;
  background-color: #4a90d9;
  border-radius: 8px;
}

.quick-play-btn-text {
  font-size: 14px;
  font-weight: 600;
  color: #ffffff;
}

.section-header {
  margin-top: 24px;
  margin-bottom: 12px;
}

.section-title {
  font-size: 17px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
}

.empty-section {
  padding: 24px 0;
  align-items: center;
}

.empty-text {
  font-size: 14px;
  color: rgba(255, 255, 255, 0.35);
}

.recent-scroll {
  flex-direction: row;
}

.recent-card {
  width: 120px;
  margin-right: 12px;
  padding: 12px;
  background-color: rgba(255, 255, 255, 0.04);
  border-radius: 10px;
  border-width: 1px;
  border-color: rgba(255, 255, 255, 0.06);
  align-items: center;
}

.recent-card-icon {
  width: 48px;
  height: 48px;
  align-items: center;
  justify-content: center;
  background-color: rgba(255, 255, 255, 0.06);
  border-radius: 24px;
  margin-bottom: 8px;
}

.recent-icon-text {
  font-size: 22px;
}

.recent-card-name {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.7);
  lines: 2;
  text-overflow: ellipsis;
}

.category-grid {
  flex-direction: row;
  flex-wrap: wrap;
  margin-left: -6px;
  margin-right: -6px;
}

.category-card {
  width: calc(33.33% - 12px);
  margin: 0 6px 12px;
  padding: 20px 12px;
  background-color: rgba(255, 255, 255, 0.04);
  border-radius: 12px;
  border-width: 1px;
  border-color: rgba(255, 255, 255, 0.06);
  align-items: center;
}

.category-icon {
  font-size: 32px;
  margin-bottom: 8px;
}

.category-label {
  font-size: 14px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.9);
}

.category-count {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.35);
  margin-top: 4px;
}
</style>
