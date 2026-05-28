package com.encvgo.plugin.mpv

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.statusBars
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Fullscreen
import androidx.compose.material.icons.filled.FullscreenExit
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.LockOpen
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.VolumeUp
import androidx.compose.material.icons.filled.VolumeMute
import androidx.compose.material.icons.outlined.Subtitles
import androidx.compose.material.icons.outlined.MusicNote
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun MpvControls(
    state: PlayerState,
    fileName: String,
    currentPosition: Long,
    duration: Long,
    isLocked: Boolean,
    isFullscreen: Boolean,
    playbackSpeed: Float,
    volume: Float = 1f,
    showControls: Boolean = true,
    onPlayPause: () -> Unit,
    onSeek: (Long) -> Unit,
    onSeekDelta: (Long) -> Unit,
    onToggleLock: () -> Unit,
    onChangeSpeed: () -> Unit,
    onToggleFullscreen: () -> Unit,
    onVolumeChange: (Float) -> Unit,
    onToggleSubtitle: () -> Unit,
    onCycleAudio: () -> Unit,
    onRetry: () -> Unit,
    onBack: () -> Unit
) {
    val progress = if (duration > 0) currentPosition.toFloat() / duration else 0f
    val isPlaying = state == PlayerState.Playing || state == PlayerState.AudioOnly
    val isLoading = state == PlayerState.Idle || state == PlayerState.Loading
    val isError = state is PlayerState.Error

    when {
        isError -> ErrorLayout(
            fileName = fileName,
            errorType = (state as PlayerState.Error).errorType,
            detail = state.detail,
            onRetry = onRetry,
            onBack = onBack
        )
        isLoading -> LoadingLayout(fileName = fileName, onBack = onBack)
        isLocked -> LockedLayout(
            progress = progress,
            currentPosition = currentPosition,
            duration = duration,
            onUnlock = onToggleLock,
            onSeek = { onSeek((it * duration).toLong()) }
        )
        state == PlayerState.AudioOnly -> AudioOnlyLayout(
            fileName = fileName,
            currentPosition = currentPosition,
            duration = duration,
            progress = progress,
            isPlaying = isPlaying,
            playbackSpeed = playbackSpeed,
            volume = volume,
            showControls = showControls,
            onPlayPause = onPlayPause,
            onSeek = { onSeek(it.toLong()) },
            onSeekDelta = onSeekDelta,
            onChangeSpeed = onChangeSpeed,
            onVolumeChange = onVolumeChange,
            onBack = onBack
        )
        else -> VideoPlaybackLayout(
            fileName = fileName,
            currentPosition = currentPosition,
            duration = duration,
            progress = progress,
            isPlaying = isPlaying,
            isLocked = isLocked,
            isFullscreen = isFullscreen,
            playbackSpeed = playbackSpeed,
            volume = volume,
            showControls = showControls,
            onPlayPause = onPlayPause,
            onSeek = { onSeek(it.toLong()) },
            onSeekDelta = onSeekDelta,
            onToggleLock = onToggleLock,
            onChangeSpeed = onChangeSpeed,
            onToggleFullscreen = onToggleFullscreen,
            onVolumeChange = onVolumeChange,
            onToggleSubtitle = onToggleSubtitle,
            onCycleAudio = onCycleAudio,
            onBack = onBack
        )
    }
}

@Composable
private fun TopBar(
    title: String,
    showBack: Boolean = true,
    trailing: @Composable (() -> Unit)? = null,
    onBack: () -> Unit = {}
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .windowInsetsPadding(WindowInsets.statusBars)
            .background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.6f), Color.Transparent)))
            .padding(
                start = 8.dp,
                end = 8.dp
            ),
        verticalAlignment = Alignment.CenterVertically
    ) {
        if (showBack) {
            IconButton(onClick = onBack, modifier = Modifier.size(40.dp)) {
                Icon(
                    imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = "Back",
                    tint = MaterialTheme.colorScheme.onSurface
                )
            }
        }
        Text(
            text = title,
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.weight(1f).padding(horizontal = 4.dp)
        )
        trailing?.invoke()
    }
}

@Composable
private fun CenterPlayButton(
    isPlaying: Boolean,
    onPlayPause: () -> Unit,
    onSeekBack: (() -> Unit)? = null,
    onSeekForward: (() -> Unit)? = null,
    visible: Boolean = true
) {
    val alpha by animateFloatAsState(targetValue = if (visible) 1f else 0f, label = "centerAlpha")
    Row(
        modifier = Modifier.alpha(alpha),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically
    ) {
        if (onSeekBack != null) {
            SeekDeltaButton(delta = "-10", onClick = onSeekBack)
            Spacer(Modifier.width(32.dp))
        }
    IconButton(
        onClick = onPlayPause,
        modifier = Modifier.size(72.dp)
    ) {
        Icon(
            imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
            contentDescription = if (isPlaying) "Pause" else "Play",
            tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.9f),
            modifier = Modifier.size(48.dp)
        )
    }
        if (onSeekForward != null) {
            Spacer(Modifier.width(32.dp))
            SeekDeltaButton(delta = "+10", onClick = onSeekForward)
        }
    }
}

@Composable
private fun SeekDeltaButton(delta: String, onClick: () -> Unit) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = Modifier.clickable(onClick = onClick).padding(8.dp)
    ) {
        Text(
            text = "${delta}s",
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontSize = 14.sp,
            fontWeight = FontWeight.Bold
        )
    }
}

@Composable
private fun BottomBar(
    progress: Float,
    currentPosition: Long,
    duration: Long,
    playbackSpeed: Float,
    isFullscreen: Boolean,
    volume: Float,
    onSeek: (Float) -> Unit,
    onChangeSpeed: () -> Unit,
    onToggleFullscreen: () -> Unit,
    onVolumeChange: (Float) -> Unit,
    onToggleSubtitle: () -> Unit,
    onCycleAudio: () -> Unit
) {
    Column(modifier = Modifier.fillMaxWidth().background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.6f))))) {
        MpvProgressBar(
            progress = progress,
            currentPosition = currentPosition,
            duration = duration,
            onSeek = onSeek,
            modifier = Modifier.padding(bottom = 4.dp)
        )
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            SpeedChip(speed = playbackSpeed, onClick = onChangeSpeed)
            Spacer(Modifier.weight(1f))
            VolumeIcon(volume = volume, onClick = { onVolumeChange(if (volume > 0f) 0f else 1f) })
            IconButton(onClick = onToggleSubtitle, modifier = Modifier.size(36.dp)) {
                Icon(
                    imageVector = Icons.Outlined.Subtitles,
                    contentDescription = "Subtitle",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            IconButton(onClick = onCycleAudio, modifier = Modifier.size(36.dp)) {
                Icon(
                    imageVector = Icons.Outlined.MusicNote,
                    contentDescription = "Audio Track",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            IconButton(onClick = onToggleFullscreen) {
                Icon(
                    imageVector = if (isFullscreen) Icons.Default.FullscreenExit else Icons.Default.Fullscreen,
                    contentDescription = "Fullscreen",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        VolumeSliderRow(volume = volume, onVolumeChange = onVolumeChange)
    }
}

@Composable
private fun VolumeIcon(volume: Float, onClick: () -> Unit) {
    IconButton(onClick = onClick, modifier = Modifier.size(36.dp)) {
        Icon(
            imageVector = if (volume > 0f) Icons.Default.VolumeUp else Icons.Default.VolumeMute,
            contentDescription = if (volume > 0f) "Mute" else "Unmute",
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun VolumeSliderRow(volume: Float, onVolumeChange: (Float) -> Unit) {
    var sliderVolume by remember { mutableStateOf(volume) }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 2.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = if (sliderVolume > 0f) Icons.Default.VolumeUp else Icons.Default.VolumeMute,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
            modifier = Modifier.size(16.dp)
        )
        Slider(
            value = sliderVolume,
            onValueChange = { sliderVolume = it },
            onValueChangeFinished = { onVolumeChange(sliderVolume) },
            modifier = Modifier
                .weight(1f)
                .padding(horizontal = 8.dp),
            valueRange = 0f..1f,
            colors = SliderDefaults.colors(
                thumbColor = MaterialTheme.colorScheme.primary,
                activeTrackColor = MaterialTheme.colorScheme.primary,
                inactiveTrackColor = MaterialTheme.colorScheme.surfaceVariant
            )
        )
        Text(
            text = "${(sliderVolume * 100).toInt()}%",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
            modifier = Modifier.width(32.dp)
        )
    }
}

@Composable
private fun SpeedChip(speed: Float, onClick: () -> Unit) {
    OutlinedButton(onClick = onClick, modifier = Modifier.height(32.dp)) {
        Text(
            text = "${speed}x",
            fontSize = 12.sp,
            color = MaterialTheme.colorScheme.primary
        )
    }
}

@Composable
private fun ErrorLayout(
    fileName: String,
    errorType: MpvError,
    detail: String,
    onRetry: () -> Unit,
    onBack: () -> Unit
) {
    Box(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        Column(modifier = Modifier.align(Alignment.TopStart)) {
            TopBar(title = fileName, onBack = onBack)
        }
        Column(
            modifier = Modifier
                .align(Alignment.Center)
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "⚠",
                fontSize = 56.sp,
                modifier = Modifier.padding(bottom = 16.dp)
            )
            Text(
                text = "播放失败 / Playback Failed",
                style = MaterialTheme.typography.headlineSmall,
                color = MaterialTheme.colorScheme.onError,
                fontWeight = FontWeight.Bold
            )
            Spacer(Modifier.height(8.dp))
            Text(
                text = errorType.displayMessage(),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            if (detail.isNotEmpty()) {
                Spacer(Modifier.height(4.dp))
                Text(
                    text = detail,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Spacer(Modifier.height(24.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedButton(onClick = onRetry) {
                    Icon(
                        imageVector = Icons.Default.Refresh,
                        contentDescription = "Retry",
                        modifier = Modifier.size(18.dp)
                    )
                    Spacer(Modifier.width(4.dp))
                    Text("重试 / Retry")
                }
                OutlinedButton(onClick = onBack) {
                    Text("返回 / Back")
                }
            }
        }
    }
}

@Composable
private fun LoadingLayout(fileName: String, onBack: () -> Unit) {
    Box(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        Column(modifier = Modifier.align(Alignment.TopStart)) {
            TopBar(title = fileName, onBack = onBack)
        }
        Column(
            modifier = Modifier.align(Alignment.Center),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            CircularProgressIndicator(
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(52.dp)
            )
            Spacer(Modifier.height(16.dp))
            Text(
                text = fileName.ifEmpty { "Loading..." },
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

@Composable
private fun LockedLayout(
    progress: Float,
    currentPosition: Long,
    duration: Long,
    onUnlock: () -> Unit,
    onSeek: (Float) -> Unit
) {
    Box(modifier = Modifier.fillMaxSize()) {
        Column(modifier = Modifier.align(Alignment.TopStart)) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .windowInsetsPadding(WindowInsets.statusBars)
                    .background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.5f), Color.Transparent)))
                    .padding(start = 8.dp, end = 8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                IconButton(onClick = onUnlock, modifier = Modifier.size(40.dp)) {
                    Icon(
                        imageVector = Icons.Default.Lock,
                        contentDescription = "Unlock",
                        tint = MaterialTheme.colorScheme.onSurface
                    )
                }
                Spacer(Modifier.weight(1f))
            }
        }
        Column(
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .clickable(onClick = onUnlock)
                .background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.5f))))
                .windowInsetsPadding(WindowInsets.navigationBars)
        ) {
            MpvProgressBar(
                progress = progress,
                currentPosition = currentPosition,
                duration = duration,
                onSeek = onSeek
            )
        }
    }
}

@Composable
private fun AudioOnlyLayout(
    fileName: String,
    currentPosition: Long,
    duration: Long,
    progress: Float,
    isPlaying: Boolean,
    playbackSpeed: Float,
    volume: Float,
    showControls: Boolean,
    onPlayPause: () -> Unit,
    onSeek: (Long) -> Unit,
    onSeekDelta: (Long) -> Unit,
    onChangeSpeed: () -> Unit,
    onVolumeChange: (Float) -> Unit,
    onBack: () -> Unit
) {
    val alpha by animateFloatAsState(targetValue = if (showControls) 1f else 0.3f, label = "audioAlpha")

    Column(
        modifier = Modifier
            .fillMaxSize()
            .alpha(alpha)
            .background(MaterialTheme.colorScheme.background)
            .padding(horizontal = 20.dp)
    ) {
        TopBar(title = fileName, onBack = onBack)
        Spacer(Modifier.weight(1f))
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            modifier = Modifier.padding(vertical = 32.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(220.dp)
                    .background(
                        color = MaterialTheme.colorScheme.surfaceVariant,
                        shape = MaterialTheme.shapes.large
                    ),
                contentAlignment = Alignment.Center
            ) {
                Text(text = "\uD83C\uDFB5", fontSize = 80.sp)
            }
            Spacer(Modifier.height(28.dp))
            Text(
                text = fileName,
                style = MaterialTheme.typography.titleLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                fontWeight = FontWeight.Medium
            )
            Spacer(Modifier.height(8.dp))
            Text(
                text = formatTime(duration),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Spacer(Modifier.weight(1f))
        Column {
            MpvProgressBar(
                progress = progress,
                currentPosition = currentPosition,
                duration = duration,
                onSeek = { onSeek((it * duration).toLong()) },
                modifier = Modifier.padding(bottom = 20.dp)
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically
            ) {
                SeekDeltaButton(delta = "-10") { onSeekDelta(-10_000L) }
                Spacer(Modifier.width(32.dp))
                IconButton(onClick = onPlayPause, modifier = Modifier.size(64.dp)) {
                    Icon(
                        imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                        contentDescription = if (isPlaying) "Pause" else "Play",
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(40.dp)
                    )
                }
                Spacer(Modifier.width(32.dp))
                SeekDeltaButton(delta = "+10") { onSeekDelta(10_000L) }
                Spacer(Modifier.width(24.dp))
                SpeedChip(speed = playbackSpeed, onClick = onChangeSpeed)
                Spacer(Modifier.width(24.dp))
                VolumeIcon(volume = volume, onClick = { onVolumeChange(if (volume > 0f) 0f else 1f) })
            }
            Spacer(Modifier.windowInsetsPadding(WindowInsets.navigationBars))
        }
    }

    VolumeSliderRow(volume = volume, onVolumeChange = onVolumeChange)
}

@Composable
private fun VideoPlaybackLayout(
    fileName: String,
    currentPosition: Long,
    duration: Long,
    progress: Float,
    isPlaying: Boolean,
    isLocked: Boolean,
    isFullscreen: Boolean,
    playbackSpeed: Float,
    showControls: Boolean,
    onPlayPause: () -> Unit,
    onSeek: (Long) -> Unit,
    onSeekDelta: (Long) -> Unit,
    onToggleLock: () -> Unit,
    onChangeSpeed: () -> Unit,
    onToggleFullscreen: () -> Unit,
    volume: Float = 1f,
    onVolumeChange: (Float) -> Unit,
    onToggleSubtitle: () -> Unit,
    onCycleAudio: () -> Unit,
    onBack: () -> Unit
) {
    val controlsAlpha by animateFloatAsState(
        targetValue = if (showControls) 1f else 0f,
        label = "controlsAlpha"
    )

    Box(modifier = Modifier.fillMaxSize()) {
        Column(modifier = Modifier.alpha(controlsAlpha)) {
            TopBar(
                title = fileName,
                onBack = onBack,
                trailing = {
                    IconButton(onClick = onToggleLock, modifier = Modifier.size(36.dp)) {
                        Icon(
                            imageVector = if (isLocked) Icons.Default.Lock else Icons.Default.LockOpen,
                            contentDescription = if (isLocked) "Locked" else "Lock",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            )
            Spacer(Modifier.weight(1f))
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(160.dp),
                contentAlignment = Alignment.Center
            ) {
                if (showControls) {
                    CenterPlayButton(
                        isPlaying = isPlaying,
                        onPlayPause = onPlayPause,
                        onSeekBack = { onSeekDelta(-10_000L) },
                        onSeekForward = { onSeekDelta(10_000L) }
                    )
                }
            }
            Spacer(Modifier.weight(1f))
            BottomBar(
            progress = progress,
            currentPosition = currentPosition,
            duration = duration,
            playbackSpeed = playbackSpeed,
            isFullscreen = isFullscreen,
            volume = volume,
            onSeek = { onSeek((it * duration).toLong()) },
            onChangeSpeed = onChangeSpeed,
            onToggleFullscreen = onToggleFullscreen,
            onVolumeChange = onVolumeChange,
            onToggleSubtitle = onToggleSubtitle,
            onCycleAudio = onCycleAudio
        )
        }
    }
}
