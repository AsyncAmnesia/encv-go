<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const hardwareDecode = ref(true)
const autoPlayNext = ref(true)
const defaultSpeed = ref('1x')

const speedOptions = ['0.5x', '0.75x', '1x', '1.25x', '1.5x', '2x']

const toggleHardwareDecode = () => {
  const next = !hardwareDecode.value
  hardwareDecode.value = next
  // TODO: 持久化设置到后端
  // POST /api/player/settings { hardwareDecode: next }
  // 调用 NativeModules.MpvPlayerModule.setProperty("hwdec", next ? "auto" : "no")
}

const toggleAutoPlayNext = () => {
  const next = !autoPlayNext.value
  autoPlayNext.value = next
  // TODO: 持久化设置到后端
  // POST /api/player/settings { autoPlayNext: next }
}

const selectSpeed = (opt: string) => {
  defaultSpeed.value = opt
  // TODO: 持久化设置到后端
  // POST /api/player/settings { defaultSpeed: opt }
}
</script>

<template>
  <view class="SettingsPage">
    <view class="SettingsHeader">
      <view class="CtrlBtn" @tap="router.back()">
        <text class="IconMd">&#x2190;</text>
      </view>
      <text class="SettingsTitle">设置</text>
      <view class="HeaderSpacer" />
    </view>

    <scroll-view class="SettingsScroll" scroll-orientation="vertical">
      <view class="SettingsSection">
        <text class="SettingsSectionTitle">播放</text>

        <view class="SettingsItem">
          <view class="SettingsItemLeft">
            <text class="SettingsItemLabel">硬件解码</text>
            <text class="SettingsItemDesc">使用硬件加速视频解码</text>
          </view>
          <view
            :class="['ToggleSwitch', hardwareDecode ? 'ToggleActive' : '']"
            @tap="toggleHardwareDecode"
          >
            <view class="ToggleThumb" />
          </view>
        </view>

        <view class="SettingsItem">
          <view class="SettingsItemLeft">
            <text class="SettingsItemLabel">自动播放下一曲</text>
            <text class="SettingsItemDesc">播放列表中当前曲目结束后自动播放下一首</text>
          </view>
          <view
            :class="['ToggleSwitch', autoPlayNext ? 'ToggleActive' : '']"
            @tap="toggleAutoPlayNext"
          >
            <view class="ToggleThumb" />
          </view>
        </view>

        <view class="SettingsItem">
          <view class="SettingsItemLeft">
            <text class="SettingsItemLabel">默认播放速度</text>
            <text class="SettingsItemDesc">新视频开始播放时的速度</text>
          </view>
          <view class="SpeedSelector">
            <view
              v-for="opt in speedOptions"
              :key="opt"
              :class="['SpeedOption', defaultSpeed === opt ? 'SpeedOptionActive' : '']"
              @tap="selectSpeed(opt)"
            >
              <text class="SpeedOptionText">{{ opt }}</text>
            </view>
          </view>
        </view>
      </view>

      <view class="SettingsSection">
        <text class="SettingsSectionTitle">音频</text>

        <view class="SettingsItem">
          <view class="SettingsItemLeft">
            <text class="SettingsItemLabel">频谱动效</text>
            <text class="SettingsItemDesc">音频播放时显示频谱可视化效果</text>
          </view>
          <view class="ToggleSwitch ToggleDisabled">
            <view class="ToggleThumb" />
          </view>
        </view>

        <view class="SettingsItem">
          <view class="SettingsItemLeft">
            <text class="SettingsItemLabel">歌词显示</text>
            <text class="SettingsItemDesc">自动匹配并显示歌词</text>
          </view>
          <view class="ToggleSwitch ToggleDisabled">
            <view class="ToggleThumb" />
          </view>
        </view>
      </view>

      <view class="SettingsSection">
        <text class="SettingsSectionTitle">视频</text>

        <view class="SettingsItem">
          <view class="SettingsItemLeft">
            <text class="SettingsItemLabel">边下边播</text>
            <text class="SettingsItemDesc">加密视频边解密边播放（实验性）</text>
          </view>
          <view class="ToggleSwitch ToggleDisabled">
            <view class="ToggleThumb" />
          </view>
        </view>
      </view>

      <view class="SettingsFooter">
        <text class="SettingsVersion">EncvGo Player v1.0</text>
      </view>
    </scroll-view>
  </view>
</template>

<style scoped>
.SettingsPage {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background-color: #0a0a0f;
}

.SettingsHeader {
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

.SettingsTitle {
  color: #ffffff;
  font-size: 18px;
  font-weight: 600;
  flex: 1;
  margin-left: 8px;
}

.HeaderSpacer {
  width: 44px;
}

.SettingsScroll {
  display: flex;
  flex: 1;
  width: 100%;
}

.SettingsSection {
  display: flex;
  flex-direction: column;
  padding: 8px 16px;
  width: 100%;
  margin-bottom: 8px;
}

.SettingsSectionTitle {
  color: rgba(255, 255, 255, 0.5);
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}

.SettingsItem {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  width: 100%;
  border-bottom-width: 1px;
  border-bottom-style: solid;
  border-bottom-color: rgba(255, 255, 255, 0.06);
}

.SettingsItemLeft {
  display: flex;
  flex-direction: column;
  flex: 1;
  margin-right: 12px;
}

.SettingsItemLabel {
  color: rgba(255, 255, 255, 0.9);
  font-size: 15px;
}

.SettingsItemDesc {
  color: rgba(255, 255, 255, 0.35);
  font-size: 12px;
  margin-top: 2px;
}

.ToggleSwitch {
  display: flex;
  width: 48px;
  height: 28px;
  border-radius: 14px;
  background-color: rgba(255, 255, 255, 0.15);
  justify-content: flex-start;
  align-items: center;
  padding: 2px;
}

.ToggleActive {
  background-color: #4a90d9;
  justify-content: flex-end;
}

.ToggleDisabled {
  opacity: 0.4;
}

.ToggleThumb {
  width: 24px;
  height: 24px;
  border-radius: 12px;
  background-color: #ffffff;
}

.SpeedSelector {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  gap: 6px;
}

.SpeedOption {
  display: flex;
  padding: 4px 10px;
  border-radius: 8px;
  background-color: rgba(255, 255, 255, 0.08);
}

.SpeedOptionActive {
  background-color: rgba(74, 144, 217, 0.3);
}

.SpeedOptionText {
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
}

.SettingsFooter {
  display: flex;
  justify-content: center;
  padding: 24px 16px 40px;
}

.SettingsVersion {
  color: rgba(255, 255, 255, 0.2);
  font-size: 12px;
}
</style>
