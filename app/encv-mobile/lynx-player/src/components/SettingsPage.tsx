import { useCallback, useState } from "@lynx-js/react";
import { Button } from "@lynx-js/lynx-ui";

interface SettingsPageProps {
  onBack: () => void;
}

export function SettingsPage({ onBack }: SettingsPageProps) {
  const [hardwareDecode, setHardwareDecode] = useState(true);
  const [autoPlayNext, setAutoPlayNext] = useState(true);
  const [defaultSpeed, setDefaultSpeed] = useState("1x");

  const speedOptions = ["0.5x", "0.75x", "1x", "1.25x", "1.5x", "2x"];

  return (
    <view className="SettingsPage">
      <view className="SettingsHeader">
        <Button onClick={onBack} className="CtrlBtn">
          <text className="IconMd">&#x2190;</text>
        </Button>
        <text className="SettingsTitle">设置</text>
        <view style={{ width: 44 }} />
      </view>

      <scroll-view className="SettingsScroll" scroll-orientation="vertical">
        <view className="SettingsSection">
          <text className="SettingsSectionTitle">播放</text>

          <view className="SettingsItem">
            <view className="SettingsItemLeft">
              <text className="SettingsItemLabel">硬件解码</text>
              <text className="SettingsItemDesc">使用硬件加速视频解码</text>
            </view>
            <view
              className={`ToggleSwitch ${hardwareDecode ? "ToggleActive" : ""}`}
              bindtap={() => {
                const next = !hardwareDecode;
                setHardwareDecode(next);
                // TODO: 持久化设置到后端
                // POST /api/player/settings { hardwareDecode: next }
                // 调用 NativeModules.MpvPlayerModule.setProperty("hwdec", next ? "auto" : "no")
              }}
            >
              <view className="ToggleThumb" />
            </view>
          </view>

          <view className="SettingsItem">
            <view className="SettingsItemLeft">
              <text className="SettingsItemLabel">自动播放下一曲</text>
              <text className="SettingsItemDesc">播放列表中当前曲目结束后自动播放下一首</text>
            </view>
            <view
              className={`ToggleSwitch ${autoPlayNext ? "ToggleActive" : ""}`}
              bindtap={() => {
                const next = !autoPlayNext;
                setAutoPlayNext(next);
                // TODO: 持久化设置到后端
                // POST /api/player/settings { autoPlayNext: next }
              }}
            >
              <view className="ToggleThumb" />
            </view>
          </view>

          <view className="SettingsItem">
            <view className="SettingsItemLeft">
              <text className="SettingsItemLabel">默认播放速度</text>
              <text className="SettingsItemDesc">新视频开始播放时的速度</text>
            </view>
            <view className="SpeedSelector">
              {speedOptions.map((opt) => (
                <view
                  key={opt}
                  className={`SpeedOption ${defaultSpeed === opt ? "SpeedOptionActive" : ""}`}
                  bindtap={() => {
                    setDefaultSpeed(opt);
                    // TODO: 持久化设置到后端
                    // POST /api/player/settings { defaultSpeed: opt }
                  }}
                >
                  <text className="SpeedOptionText">{opt}</text>
                </view>
              ))}
            </view>
          </view>
        </view>

        <view className="SettingsSection">
          <text className="SettingsSectionTitle">音频</text>

          <view className="SettingsItem">
            <view className="SettingsItemLeft">
              <text className="SettingsItemLabel">频谱动效</text>
              <text className="SettingsItemDesc">音频播放时显示频谱可视化效果</text>
            </view>
            <view
              className="ToggleSwitch ToggleDisabled"
              bindtap={() => {
                // TODO: 频谱动效功能
                // 需要 NativeModules.MpvPlayerModule 提供 audio-data 回调
                // 或使用 Web Audio API 分析
              }}
            >
              <view className="ToggleThumb" />
            </view>
          </view>

          <view className="SettingsItem">
            <view className="SettingsItemLeft">
              <text className="SettingsItemLabel">歌词显示</text>
              <text className="SettingsItemDesc">自动匹配并显示歌词</text>
            </view>
            <view
              className="ToggleSwitch ToggleDisabled"
              bindtap={() => {
                // TODO: 歌词显示功能
                // 需要: 1. LRC 文件解析 2. 歌词同步 3. 歌词 UI 组件
              }}
            >
              <view className="ToggleThumb" />
            </view>
          </view>
        </view>

        <view className="SettingsSection">
          <text className="SettingsSectionTitle">视频</text>

          <view className="SettingsItem">
            <view className="SettingsItemLeft">
              <text className="SettingsItemLabel">边下边播</text>
              <text className="SettingsItemDesc">加密视频边解密边播放（实验性）</text>
            </view>
            <view
              className="ToggleSwitch ToggleDisabled"
              bindtap={() => {
                // TODO: 边下边播功能
                // 需要: 1. 后端流式解密 API 2. HTTP Range 请求支持
                // 3. 缓冲管理 4. 断点续传
              }}
            >
              <view className="ToggleThumb" />
            </view>
          </view>
        </view>

        <view className="SettingsFooter">
          <text className="SettingsVersion">EncvGo Player v1.0</text>
        </view>
      </scroll-view>
    </view>
  );
}
