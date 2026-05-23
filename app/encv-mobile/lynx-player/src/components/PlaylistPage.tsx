import { useCallback } from "@lynx-js/react";
import { Button } from "@lynx-js/lynx-ui";
import type { PlaybackItem } from "./AppComponent";

interface PlaylistPageProps {
  playlist: PlaybackItem[];
  currentIndex: number;
  onSelect: (index: number) => void;
  onBack: () => void;
}

export function PlaylistPage({
  playlist,
  currentIndex,
  onSelect,
  onBack,
}: PlaylistPageProps) {
  return (
    <view className="PlaylistPage">
      <view className="PlaylistHeader">
        <Button onClick={onBack} className="CtrlBtn">
          <text className="IconMd">&#x2190;</text>
        </Button>
        <text className="PlaylistTitle">播放列表</text>
        <text className="PlaylistCount">{playlist.length} 首</text>
      </view>
      <scroll-view className="PlaylistScroll" scroll-orientation="vertical">
        {playlist.map((item, idx) => (
          <view
            key={idx}
            className={`PlaylistItem ${idx === currentIndex ? "PlaylistItemActive" : ""}`}
            bindtap={() => onSelect(idx)}
          >
            <view className="PlaylistItemIndex">
              {idx === currentIndex ? (
                <text className="PlayingIcon">&#x25B6;</text>
              ) : (
                <text className="ItemIndexText">{idx + 1}</text>
              )}
            </view>
            <view className="PlaylistItemInfo">
              <text className="PlaylistItemName" numberOfLines={1}>{item.fileName}</text>
              <text className="PlaylistItemType">
                {item.mediaType === "video" ? "视频" : "音频"}
              </text>
            </view>
          </view>
        ))}
      </scroll-view>
    </view>
  );
}
