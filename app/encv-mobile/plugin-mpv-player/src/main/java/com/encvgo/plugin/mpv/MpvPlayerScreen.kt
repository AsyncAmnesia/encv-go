package com.encvgo.plugin.mpv

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter

private const val SPEED_OPTIONS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)
private const val CONTROLS_HIDE_DELAY_MS = 3000L
private const val LOADING_TIMEOUT_MS = 15_000L
private const val POSITION_UPDATE_INTERVAL_MS = 1000L

@Composable
fun MpvPlayerScreen(
    filePath: String,
    fileName: String,
    mimeType: String,
    isExternal: Boolean,
    engine: MpvEngine,
    onBack: () -> Unit
) {
    var playerState by remember { mutableStateOf<PlayerState>(PlayerState.Idle) }
    var currentPosition by remember { mutableLongStateOf(0L) }
    var duration by remember { mutableLongStateOf(0L) }
    var showControls by remember { mutableStateOf(true) }
    var isLocked by remember { mutableStateOf(false) }
    var isFullscreen by remember { mutableStateOf(false) }
    var playbackSpeed by remember { mutableFloatStateOf(1f) }

    LaunchedEffect(filePath) {
        startPlayback(
            filePath = filePath,
            fileName = fileName,
            isExternal = isExternal,
            mimeType = mimeType,
            engine = engine,
            onStateChange = { playerState = it },
            onError = { msg -> playerState = PlayerState.Error(classifyError(msg), msg) }
        )
    }

    LaunchedEffect(playerState) {
        if (playerState == PlayerState.Playing || playerState == PlayerState.Paused || playerState == PlayerState.AudioOnly) {
            showControls = true
        }
    }

    LaunchedEffect(playerState, showControls) {
        snapshotFlow { playerState }
            .distinctUntilChanged()
            .filter { it == PlayerState.Playing || it == PlayerState.AudioOnly }
            .collect {
                while (true) {
                    delay(CONTROLS_HIDE_DELAY_MS)
                    if (playerState == PlayerState.Playing || playerState == PlayerState.AudioOnly) {
                        showControls = false
                    }
                }
            }
    }

    LaunchedEffect(Unit) {
        while (true) {
            delay(POSITION_UPDATE_INTERVAL_MS)
            if (playerState == PlayerState.Playing || playerState == PlayerState.Paused || playerState == PlayerState.AudioOnly) {
                try {
                    currentPosition = engine.getPosition()
                    duration = engine.getDuration()
                } catch (_: Exception) {}
            }
        }
    }

    DisposableEffect(Unit) {
        onDispose {
            try { engine.pause() } catch (_: Exception) {}
            try { engine.destroy() } catch (_: Exception) {}
        }
    }

    Surface(
        modifier = Modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .pointerInput(Unit) {
                    detectTapGestures(
                        onTap = {
                            if (isLocked) {
                                isLocked = false
                            } else {
                                showControls = !showControls
                            }
                        }
                    )
                }
        ) {
            MpvControls(
                state = playerState,
                fileName = fileName.ifEmpty { "Unknown" },
                currentPosition = currentPosition,
                duration = duration,
                isLocked = isLocked,
                isFullscreen = isFullscreen,
                playbackSpeed = playbackSpeed,
                showControls = showControls,
                onPlayPause = {
                    when (playerState) {
                        is PlayerState.Playing -> {
                            engine.pause()
                            playerState = PlayerState.Paused
                        }
                        is PlayerState.Paused -> {
                            engine.resume()
                            playerState = PlayerState.Playing
                        }
                        is PlayerState.AudioOnly -> {
                            if (engine.isPlaying()) {
                                engine.pause()
                            } else {
                                engine.resume()
                            }
                        }
                        else -> {
                            startPlayback(
                                filePath = filePath,
                                fileName = fileName,
                                isExternal = isExternal,
                                mimeType = mimeType,
                                engine = engine,
                                onStateChange = { playerState = it },
                                onError = { msg -> playerState = PlayerState.Error(classifyError(msg), msg) }
                            )
                        }
                    }
                    showControls = true
                },
                onSeek = { ms ->
                    engine.seek(ms)
                    currentPosition = ms
                    showControls = true
                },
                onSeekDelta = { deltaMs ->
                    val newPos = (currentPosition + deltaMs).coerceIn(0, duration)
                    engine.seek(newPos)
                    currentPosition = newPos
                    showControls = true
                },
                onToggleLock = {
                    isLocked = !isLocked
                    showControls = true
                },
                onChangeSpeed = {
                    val currentIdx = SPEED_OPTIONS.indexOf(playbackSpeed).coerceAtLeast(0)
                    playbackSpeed = SPEED_OPTIONS[(currentIdx + 1) % SPEED_OPTIONS.size]
                    engine.setProperty("speed", playbackSpeed.toString())
                    showControls = true
                },
                onToggleFullscreen = {
                    isFullscreen = !isFullscreen
                    TODO("Phase 3.1: Implement fullscreen via Activity orientation or WindowInsets controller")
                    showControls = true
                },
                onRetry = {
                    startPlayback(
                        filePath = filePath,
                        fileName = fileName,
                        isExternal = isExternal,
                        mimeType = mimeType,
                        engine = engine,
                        onStateChange = { playerState = it },
                        onError = { msg -> playerState = PlayerState.Error(classifyError(msg), msg) }
                    )
                },
                onBack = {
                    engine.pause()
                    onBack()
                }
            )
        }
    }
}

private suspend fun startPlayback(
    filePath: String,
    fileName: String,
    isExternal: Boolean,
    mimeType: String,
    engine: MpvEngine,
    onStateChange: (PlayerState) -> Unit,
    onError: (String) -> Unit
) {
    if (filePath.isEmpty()) {
        onError("文件路径为空 / File path is empty")
        return
    }

    onStateChange(PlayerState.Loading)

    var loadingTimedOut = false
    kotlinx.coroutines.async {
        delay(LOADING_TIMEOUT_MS)
        loadingTimedOut = true
        onError("播放超时，请检查网络或文件 / Playback timeout")
    }

    try {
        val streamUrl = resolveStreamUrl(filePath, isExternal)

        if (streamUrl.isEmpty() || !streamUrl.startsWith("http")) {
            onError("无法获取播放地址 / Unable to get stream URL: $streamUrl")
            return
        }

        engine.play(streamUrl)
        onStateChange(PlayerState.Loading)
    } catch (e: Exception) {
        val msg = e.message ?: e.toString()
        onError("播放异常 / Playback error: $msg")
    }
}

private suspend fun resolveStreamUrl(filePath: String, isExternal: Boolean): String {
    return ""
}

@Suppress("UNUSED_PARAMETER")
private fun WindowInsetsSafeTop(): Int = 0

@Suppress("UNUSED_PARAMETER")
private fun WindowInsetsSafeBottom(): Int = 0
