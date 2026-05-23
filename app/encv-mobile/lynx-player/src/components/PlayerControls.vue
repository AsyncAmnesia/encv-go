<script setup lang="ts">
import { computed } from 'vue'
import ProgressBar from './ProgressBar.vue'

const props = withDefaults(defineProps<{
  state: string
  isFullscreen: boolean
  fileName: string
  currentTime: number
  duration: number
  showControls: boolean
  error?: string | null
  mediaType: 'video' | 'audio'
  playbackRate: number
  locked: boolean
  hasNext: boolean
  hasPrev: boolean
}>(), {
  state: 'idle',
  isFullscreen: false,
  fileName: '',
  currentTime: 0,
  duration: 0,
  showControls: true,
  error: null,
  mediaType: 'video',
  playbackRate: 1,
  locked: false,
  hasNext: false,
  hasPrev: false,
})

const emit = defineEmits<{
  'play-pause': []
  'seek': [ms: number]
  'seek-relative': [ms: number]
  'toggle-fullscreen': []
  'cycle-speed': []
  'toggle-lock': []
  'back': []
  'next': []
  'prev': []
  'open-playlist': []
}>()

const isPlaying = computed(() => props.state === 'playing')

const progress = computed(() =>
  props.duration > 0 ? props.currentTime / props.duration : 0
)

function formatTime(ms: number): string {
  if (!isFinite(ms) || ms < 0) return '0:00'
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  return `${m}:${s.toString().padStart(2, '0')}`
}

const currentTimeStr = computed(() => formatTime(props.currentTime))
const durationStr = computed(() => formatTime(props.duration))

function handleSeek(progressVal: number) {
  emit('seek', progressVal * props.duration)
}

const isError = computed(() => props.error && props.state !== 'loading')
const isLoading = computed(() => props.state === 'loading')
</script>

<template>
  <view v-if="isError" class="ErrorContainer">
    <view class="PlayBtn" @tap="emit('play-pause')">
      <view class="PlayBtnInner">
        <text class="PlayIcon">&#x1F504;</text>
      </view>
    </view>
    <text class="ErrorTitle">&#x26A0; 播放失败</text>
    <text class="ErrorDetail">{{ error || '未知错误，点击重试' }}</text>
  </view>

  <view v-else-if="isLoading" class="CenterArea">
    <view class="LoadingSpinner" />
  </view>

  <view v-else-if="locked" class="LockedOverlay">
    <view class="TopGradient" />
    <view class="LockBar">
      <view class="CtrlBtn" @tap="emit('toggle-lock')">
        <text class="IconSm">&#x1F512;</text>
      </view>
      <view class="FlexSpacer" />
    </view>
    <view class="BottomGradient" />
    <view class="BottomBar">
      <ProgressBar
        :progress="progress"
        :current-time="currentTimeStr"
        :duration="durationStr"
        @seek="handleSeek"
      />
    </view>
  </view>

  <view v-else-if="mediaType === 'audio'" class="AudioOverlay">
    <view class="TopBar">
      <view class="CtrlBtn" @tap="emit('back')">
        <text class="IconMd">&#x2715;</text>
      </view>
      <text class="TitleText">{{ fileName }}</text>
      <view class="TopBarSpacer" />
    </view>
    <view class="AudioCoverContainer">
      <view class="AudioCover">
        <text class="AudioCoverIcon">&#x1F3B5;</text>
      </view>
      <text class="AudioTitle">{{ fileName }}</text>
    </view>
    <view class="AudioBottomSection">
      <ProgressBar
        :progress="progress"
        :current-time="currentTimeStr"
        :duration="durationStr"
        @seek="handleSeek"
      />
      <view class="AudioPlayRow">
        <view v-if="hasPrev" class="TrackBtn" @tap="emit('prev')">
          <text class="TrackIcon">&#x23EE;</text>
        </view>
        <view v-else class="SeekBtn" @tap="emit('seek-relative', -10000)">
          <view class="SeekBtnInner">
            <text class="SeekIcon">-10</text>
          </view>
        </view>

        <view class="PlayBtn" @tap="emit('play-pause')">
          <view class="PlayBtnInner">
            <text class="PlayIcon">{{ isPlaying ? '\u275A\u275A' : '\u25B6' }}</text>
          </view>
        </view>

        <view v-if="hasNext" class="TrackBtn" @tap="emit('next')">
          <text class="TrackIcon">&#x23ED;</text>
        </view>
        <view v-else class="SeekBtn" @tap="emit('seek-relative', 10000)">
          <view class="SeekBtnInner">
            <text class="SeekIcon">+10</text>
          </view>
        </view>

        <view class="SpeedChip" @tap="emit('cycle-speed')">
          <text class="SpeedText">{{ playbackRate }}x</text>
        </view>
        <view v-if="hasPrev || hasNext" class="CtrlBtn" @tap="emit('open-playlist')">
          <text class="IconMd">&#x2630;</text>
        </view>
      </view>
    </view>
  </view>

  <view v-else class="VideoOverlay">
    <view class="TopGradient" />
    <view class="TopBar">
      <view class="CtrlBtn" @tap="emit('back')">
        <text class="IconMd">&#x2715;</text>
      </view>
      <text class="TitleText">{{ fileName }}</text>
      <view class="CtrlBtn" @tap="emit('toggle-lock')">
        <text class="IconSm">&#x1F512;</text>
      </view>
    </view>
    <view class="CenterArea">
      <view v-if="showControls" class="CenterControls">
        <view v-if="hasPrev" class="TrackBtn" @tap="emit('prev')">
          <text class="TrackIcon">&#x23EE;</text>
        </view>
        <view v-else class="SeekBtn" @tap="emit('seek-relative', -10000)">
          <view class="SeekBtnInner">
            <text class="SeekIcon">-10</text>
          </view>
        </view>

        <view class="PlayBtn" @tap="emit('play-pause')">
          <view class="PlayBtnInner">
            <text class="PlayIcon">{{ isPlaying ? '\u275A\u275A' : '\u25B6' }}</text>
          </view>
        </view>

        <view v-if="hasNext" class="TrackBtn" @tap="emit('next')">
          <text class="TrackIcon">&#x23ED;</text>
        </view>
        <view v-else class="SeekBtn" @tap="emit('seek-relative', 10000)">
          <view class="SeekBtnInner">
            <text class="SeekIcon">+10</text>
          </view>
        </view>
      </view>
    </view>
    <view class="BottomGradient" />
    <view class="BottomBar">
      <ProgressBar
        :progress="progress"
        :current-time="currentTimeStr"
        :duration="durationStr"
        @seek="handleSeek"
      />
      <view class="BottomActions">
        <view class="SpeedChip" @tap="emit('cycle-speed')">
          <text class="SpeedText">{{ playbackRate }}x</text>
        </view>
        <view v-if="hasPrev || hasNext" class="CtrlBtn" @tap="emit('open-playlist')">
          <text class="IconMd">&#x2630;</text>
        </view>
        <view class="FlexSpacer" />
        <view class="CtrlBtn" @tap="emit('toggle-fullscreen')">
          <text class="IconMd">⛶</text>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.VideoOverlay {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  width: 100%;
  flex: 1;
}

.LockedOverlay {
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  width: 100%;
  flex: 1;
}

.AudioOverlay {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  width: 100%;
  flex: 1;
}

.FlexSpacer {
  flex: 1;
}

.TopGradient {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 120px;
  background-image: linear-gradient(to bottom, rgba(0, 0, 0, 0.6), rgba(0, 0, 0, 0));
  pointer-events: none;
}

.BottomGradient {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 160px;
  background-image: linear-gradient(to top, rgba(0, 0, 0, 0.7), rgba(0, 0, 0, 0));
  pointer-events: none;
}

.TopBar {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px 4px;
  width: 100%;
}

.TopBarSpacer {
  width: 44px;
}

.LockBar {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 4px 12px;
  width: 100%;
}

.CtrlBtn {
  display: flex;
  background-color: transparent;
  border: none;
  padding: 0;
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

.IconSm {
  color: rgba(255, 255, 255, 0.85);
  font-size: 18px;
  text-align: center;
}

.TitleText {
  color: #ffffff;
  font-size: 15px;
  font-weight: 500;
  flex: 1;
  margin-left: 8px;
  margin-right: 8px;
}

.CenterArea {
  display: flex;
  flex: 1;
  justify-content: center;
  align-items: center;
  width: 100%;
}

.CenterControls {
  display: flex;
  flex-direction: row;
  justify-content: center;
  align-items: center;
}

.SeekBtn {
  display: flex;
  background-color: transparent;
  border: none;
  padding: 0;
  margin-left: 24px;
  margin-right: 24px;
}

.SeekBtnInner {
  display: flex;
  width: 52px;
  height: 52px;
  border-radius: 26px;
  background-color: rgba(255, 255, 255, 0.15);
  justify-content: center;
  align-items: center;
}

.SeekIcon {
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
  text-align: center;
}

.TrackBtn {
  display: flex;
  background-color: transparent;
  border: none;
  padding: 0;
  margin-left: 16px;
  margin-right: 16px;
  min-width: 52px;
  min-height: 52px;
  justify-content: center;
  align-items: center;
}

.TrackIcon {
  color: #ffffff;
  font-size: 24px;
  text-align: center;
}

.PlayBtn {
  display: flex;
  background-color: transparent;
  border: none;
  padding: 0;
  margin-left: 24px;
  margin-right: 24px;
}

.PlayBtnInner {
  display: flex;
  width: 68px;
  height: 68px;
  border-radius: 34px;
  background-color: rgba(255, 255, 255, 0.25);
  justify-content: center;
  align-items: center;
}

.PlayIcon {
  color: #ffffff;
  font-size: 28px;
  text-align: center;
}

.LoadingSpinner {
  width: 40px;
  height: 40px;
  border-radius: 20px;
  border-width: 3px;
  border-style: solid;
  border-color: rgba(255, 255, 255, 0.2);
  border-top-color: #4a90d9;
}

.BottomBar {
  display: flex;
  flex-direction: column;
  padding: 0 12px 8px;
  width: 100%;
}

.BottomActions {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 2px 0 4px;
  width: 100%;
}

.SpeedChip {
  display: flex;
  background-color: rgba(255, 255, 255, 0.15);
  border: none;
  border-radius: 12px;
  padding: 0;
  min-width: 44px;
  min-height: 32px;
  justify-content: center;
  align-items: center;
}

.SpeedText {
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
  text-align: center;
}

.ErrorContainer {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 24px;
  width: 100%;
}

.ErrorTitle {
  color: #ff4444;
  font-size: 17px;
  text-align: center;
  margin-bottom: 8px;
}

.ErrorDetail {
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
  text-align: center;
}

.AudioCoverContainer {
  display: flex;
  flex: 1;
  justify-content: center;
  align-items: center;
  width: 100%;
}

.AudioCover {
  display: flex;
  width: 180px;
  height: 180px;
  border-radius: 16px;
  background-color: rgba(74, 144, 217, 0.12);
  justify-content: center;
  align-items: center;
}

.AudioCoverIcon {
  font-size: 56px;
  color: rgba(255, 255, 255, 0.5);
}

.AudioTitle {
  color: rgba(255, 255, 255, 0.85);
  font-size: 15px;
  margin-top: 16px;
  max-width: 280px;
  text-align: center;
}

.AudioBottomSection {
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 0 12px 16px;
}

.AudioPlayRow {
  display: flex;
  flex-direction: row;
  justify-content: center;
  align-items: center;
  padding: 8px 0;
}
</style>
