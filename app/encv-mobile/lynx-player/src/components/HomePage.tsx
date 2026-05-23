import { useCallback, useState, useEffect } from "@lynx-js/react";
import { Button } from "@lynx-js/lynx-ui";
import type { PlaybackItem } from "./AppComponent";

interface HomePageProps {
  onPlayItem: (item: PlaybackItem) => void;
  onPlayFromList: (items: PlaybackItem[], index: number) => void;
  onInitPlayback: () => void;
  hasInitData: boolean;
  onSettings: () => void;
}

interface RecentItem {
  fileName: string;
  filePath: string;
  mediaType: "video" | "audio";
  lastPlayed: string;
}

interface MediaCategory {
  label: string;
  icon: string;
  path: string;
  count: number;
}

export function HomePage({
  onPlayItem,
  onPlayFromList,
  onInitPlayback,
  hasInitData,
  onSettings,
}: HomePageProps) {
  const [recentItems, setRecentItems] = useState<RecentItem[]>([]);
  const [categories, setCategories] = useState<MediaCategory[]>([]);

  useEffect(() => {
    loadRecentItems();
    loadMediaCategories();
  }, []);

  const loadRecentItems = useCallback(() => {
    // TODO: 从后端 API 加载最近播放记录
    // GET /api/player/recent → RecentItem[]
    // 目前使用空数组作为占位
    setRecentItems([]);
  }, []);

  const loadMediaCategories = useCallback(() => {
    // TODO: 从后端 API 扫描媒体文件分类
    // GET /api/player/categories → MediaCategory[]
    // 目前使用占位分类
    setCategories([
      { label: "视频", icon: "\u{1F3AC}", path: "/videos", count: 0 },
      { label: "音频", icon: "\u{1F3B5}", path: "/audio", count: 0 },
      { label: "播放列表", icon: "\u{1F4DC}", path: "/playlists", count: 0 },
    ]);
  }, []);

  const handleCategoryTap = useCallback((category: MediaCategory) => {
    // TODO: 打开对应分类的文件浏览器，选择文件后播放
    // 需要后端 API: GET /api/player/browse?path=xxx
    // 选择文件后调用 onPlayItem 或 onPlayFromList
  }, [onPlayItem, onPlayFromList]);

  const handleRecentTap = useCallback((item: RecentItem) => {
    onPlayItem({
      filePath: item.filePath,
      fileName: item.fileName,
      mimeType: "",
      isExternal: false,
      mediaType: item.mediaType,
    });
  }, [onPlayItem]);

  return (
    <view className="HomePage">
      <view className="HomeHeader">
        <text className="HomeTitle">媒体中心</text>
        <Button onClick={onSettings} className="HeaderBtn">
          <text className="IconMd">&#x2699;</text>
        </Button>
      </view>

      {hasInitData && (
        <view className="QuickPlayBanner">
          <text className="QuickPlayText">有文件待播放</text>
          <Button onClick={onInitPlayback} className="QuickPlayBtn">
            <text className="QuickPlayBtnText">立即播放</text>
          </Button>
        </view>
      )}

      <view className="SectionHeader">
        <text className="SectionTitle">最近播放</text>
      </view>
      {recentItems.length === 0 ? (
        <view className="EmptySection">
          <text className="EmptyText">暂无播放记录</text>
        </view>
      ) : (
        <scroll-view className="RecentScroll" scroll-orientation="horizontal">
          {recentItems.map((item, idx) => (
            <view key={idx} className="RecentCard" bindtap={() => handleRecentTap(item)}>
              <view className="RecentCardIcon">
                <text className="RecentIconText">
                  {item.mediaType === "video" ? "\u{1F3AC}" : "\u{1F3B5}"}
                </text>
              </view>
              <text className="RecentCardName">{item.fileName}</text>
            </view>
          ))}
        </scroll-view>
      )}

      <view className="SectionHeader">
        <text className="SectionTitle">媒体分类</text>
      </view>
      <view className="CategoryGrid">
        {categories.map((cat, idx) => (
          <view key={idx} className="CategoryCard" bindtap={() => handleCategoryTap(cat)}>
            <text className="CategoryIcon">{cat.icon}</text>
            <text className="CategoryLabel">{cat.label}</text>
            {cat.count > 0 && (
              <text className="CategoryCount">{cat.count}</text>
            )}
          </view>
        ))}
      </view>

      <view className="SectionHeader">
        <text className="SectionTitle">播放列表</text>
      </view>
      <view className="EmptySection">
        <text className="EmptyText">
          {/* TODO: 播放列表功能 — 从后端加载用户保存的播放列表 */}
          暂无播放列表
        </text>
      </view>
    </view>
  );
}
