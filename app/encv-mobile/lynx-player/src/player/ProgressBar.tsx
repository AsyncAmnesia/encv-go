import { useRef, useCallback } from '@lynx-js/react'

interface ProgressBarProps {
  progress: number
  currentTime: string
  duration: string
  onSeek: (progress: number) => void
}

export function ProgressBar(props: ProgressBarProps) {
  const { progress, currentTime, duration, onSeek } = props
  const trackRef = useRef<any>(null)

  const clampedProgress = Math.max(0, Math.min(1, progress))

  const handleTrackTap = useCallback(
    (e: any) => {
      const trackEl = trackRef.current
      if (!trackEl) return
      const trackX = e.detail?.clientX ?? e.clientX ?? 0
      const rect = trackEl.getBoundingClientRect?.()
      if (!rect) return
      const relativeX = trackX - rect.left
      const ratio = Math.max(0, Math.min(1, relativeX / rect.width))
      onSeek(ratio)
    },
    [onSeek]
  )

  return (
    <view className="ProgressRow">
      <text className="TimeLabel">{currentTime}</text>
      <view
        ref={trackRef}
        className="SliderTrackOuter"
        bindtap={handleTrackTap}
      >
        <view className="SliderTrackBg" />
        <view className="SliderFill" style={{ width: (clampedProgress * 100) + '%' }} />
        <view className="SliderThumbWrapper" style={{ left: 'calc(' + (clampedProgress * 100) + '% - 8px)' }}>
          <view className="SliderThumbDot" />
        </view>
      </view>
      <text className="TimeLabelEnd">{duration}</text>
    </view>
  )
}
