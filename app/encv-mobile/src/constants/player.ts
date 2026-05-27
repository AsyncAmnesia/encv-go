export const PLAY_MODE = {
  ARTPLAYER: 'artplayer',
  MPV_PLUGIN: 'mpv-plugin',
  EXTERNAL: 'external',
} as const

export type PlayMode = (typeof PLAY_MODE)[keyof typeof PLAY_MODE]

export const VIDEO_DEFAULT: PlayMode = PLAY_MODE.ARTPLAYER
export const AUDIO_DEFAULT: PlayMode = PLAY_MODE.MPV_PLUGIN
