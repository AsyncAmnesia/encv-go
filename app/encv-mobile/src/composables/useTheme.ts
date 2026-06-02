import { ref } from 'vue'

const THEME_KEY = 'encv-theme-preference'
const COLOR_KEY = 'encv-theme-color'
const BG_COLOR_KEY = 'encv-bg-color'
const BG_BLUR_KEY = 'encv-bg-blur'
const P3_KEY = 'encv-p3-mode'

const isDark = ref(false)
const currentColor = ref('#4f8cff')
const currentBgColor = ref<string | null>(null)
const bgBlur = ref(0)
const p3Mode = ref<'off' | 'on' | 'auto'>('auto')

export interface ThemePreset {
  name: string
  value: string
  colorRgb: string
}

export interface BgPreset {
  name: string
  value: string | null
  description: string
  category: 'light' | 'eye' | 'dark'
  textColor: string
}

export const THEME_PRESETS: ThemePreset[] = [
  { name: 'Blue', value: '#4f8cff', colorRgb: '79, 140, 255' },
  { name: 'Purple', value: '#8b5cf6', colorRgb: '139, 92, 246' },
  { name: 'Green', value: '#22c55e', colorRgb: '34, 197, 94' },
  { name: 'Orange', value: '#f97316', colorRgb: '249, 115, 22' },
  { name: 'Red', value: '#ef4444', colorRgb: '239, 68, 68' },
  { name: 'Pink', value: '#ec4899', colorRgb: '236, 72, 153' },
  { name: 'Teal', value: '#14b8a6', colorRgb: '20, 184, 166' },
]

export const BG_PRESETS: BgPreset[] = [
  { name: 'bg.white', value: '#ffffff', description: 'bg.whiteDesc', category: 'light', textColor: '#1a1a1a' },
  { name: 'bg.sepia', value: '#f4ecd8', description: 'bg.sepiaDesc', category: 'eye', textColor: '#5b4636' },
  { name: 'bg.sage', value: '#dce4d0', description: 'bg.sageDesc', category: 'eye', textColor: '#3a4a30' },
  { name: 'bg.lavender', value: '#e6e0f0', description: 'bg.lavenderDesc', category: 'eye', textColor: '#3a2f4a' },
  { name: 'bg.cream', value: '#f5efe0', description: 'bg.creamDesc', category: 'eye', textColor: '#4a3f2a' },
  { name: 'bg.lightBlack', value: '#1a1a1a', description: 'bg.lightBlackDesc', category: 'dark', textColor: '#e0e0e0' },
  { name: 'bg.darkBlack', value: '#000000', description: 'bg.darkBlackDesc', category: 'dark', textColor: '#ffffff' },
  { name: 'bg.midnight', value: '#0a0e1a', description: 'bg.midnightDesc', category: 'dark', textColor: '#d0d8e8' },
  { name: 'bg.charcoal', value: '#2a2a2e', description: 'bg.charcoalDesc', category: 'dark', textColor: '#d8d8d8' },
]

function hexToRgb(hex: string): string {
  const clean = hex.replace('#', '')
  if (clean.length !== 6) return '79, 140, 255'
  const r = parseInt(clean.substring(0, 2), 16)
  const g = parseInt(clean.substring(2, 4), 16)
  const b = parseInt(clean.substring(4, 6), 16)
  return `${r}, ${g}, ${b}`
}

function getContrastColor(hex: string): string {
  const rgb = hexToRgb(hex)
  const [r, g, b] = rgb.split(',').map(Number)
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance > 0.5 ? '#000000' : '#ffffff'
}

function isColorDark(hex: string): boolean {
  const rgb = hexToRgb(hex)
  const [r, g, b] = rgb.split(',').map(Number)
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance <= 0.5
}

function applyTheme(dark: boolean) {
  isDark.value = dark
  if (dark) {
    document.body.classList.add('dark')
  } else {
    document.body.classList.remove('dark')
  }
}

function applyBgColor(bgColor: string | null) {
  currentBgColor.value = bgColor
  const root = document.documentElement
  if (bgColor) {
    root.style.setProperty('--ion-background-color', bgColor)
    root.style.setProperty('--ion-background-color-rgb', hexToRgb(bgColor))
    root.style.setProperty('--encv-bg-text-color', getContrastColor(bgColor))
    document.body.style.backgroundColor = bgColor
    if (isColorDark(bgColor)) {
      document.body.classList.add('dark')
    } else {
      document.body.classList.remove('dark')
    }
  } else {
    root.style.removeProperty('--ion-background-color')
    root.style.removeProperty('--ion-background-color-rgb')
    root.style.removeProperty('--encv-bg-text-color')
    document.body.style.backgroundColor = ''
  }
}

function applyColor(color: string) {
  currentColor.value = color
  const root = document.documentElement
  const rgb = hexToRgb(color)
  const contrast = getContrastColor(color)

  const lighter = (hex: string, percent: number): string => {
    const clean = hex.replace('#', '')
    let r = parseInt(clean.substring(0, 2), 16)
    let g = parseInt(clean.substring(2, 4), 16)
    let b = parseInt(clean.substring(4, 6), 16)
    r = Math.min(255, Math.round(r + (255 - r) * percent / 100))
    g = Math.min(255, Math.round(g + (255 - g) * percent / 100))
    b = Math.min(255, Math.round(b + (255 - b) * percent / 100))
    return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
  }

  const darker = (hex: string, percent: number): string => {
    const clean = hex.replace('#', '')
    let r = parseInt(clean.substring(0, 2), 16)
    let g = parseInt(clean.substring(2, 4), 16)
    let b = parseInt(clean.substring(4, 6), 16)
    r = Math.max(0, Math.round(r * (1 - percent / 100)))
    g = Math.max(0, Math.round(g * (1 - percent / 100)))
    b = Math.max(0, Math.round(b * (1 - percent / 100)))
    return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
  }

  root.style.setProperty('--ion-color-primary', color)
  root.style.setProperty('--ion-color-primary-rgb', rgb)
  root.style.setProperty('--ion-color-primary-contrast', contrast)
  root.style.setProperty('--ion-color-primary-contrast-rgb', hexToRgb(contrast))
  root.style.setProperty('--ion-color-primary-shade', darker(color, 10))
  root.style.setProperty('--ion-color-primary-tint', lighter(color, 10))

  localStorage.setItem(COLOR_KEY, color)
}

function applyBgBlur(blur: number) {
  bgBlur.value = Math.max(0, Math.min(40, blur))
  const root = document.documentElement
  root.style.setProperty('--encv-bg-blur', `${bgBlur.value}px`)
  localStorage.setItem(BG_BLUR_KEY, String(bgBlur.value))
}

function applyP3Mode(mode: 'off' | 'on' | 'auto') {
  p3Mode.value = mode
  const root = document.documentElement
  if (mode === 'on') {
    root.style.setProperty('--encv-color-gamut', 'display-p3')
  } else if (mode === 'off') {
    root.style.setProperty('--encv-color-gamut', 'srgb')
  } else {
    root.style.removeProperty('--encv-color-gamut')
  }
  localStorage.setItem(P3_KEY, mode)
}

function supportsP3(): boolean {
  if (typeof window === 'undefined') return false
  return window.matchMedia('(color-gamut: p3)').matches
}

function initTheme() {
  const stored = localStorage.getItem(THEME_KEY)
  if (stored !== null) {
    applyTheme(stored === 'dark')
  } else {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')
    applyTheme(prefersDark.matches)
    prefersDark.addEventListener('change', (e) => {
      const current = localStorage.getItem(THEME_KEY)
      if (current === null) {
        applyTheme(e.matches)
      }
    })
  }

  const storedColor = localStorage.getItem(COLOR_KEY)
  if (storedColor && /^#[0-9a-fA-F]{6}$/.test(storedColor)) {
    applyColor(storedColor)
  }

  const storedBg = localStorage.getItem(BG_COLOR_KEY)
  if (storedBg && /^#[0-9a-fA-F]{6}$/.test(storedBg)) {
    applyBgColor(storedBg)
  }

  const storedBlur = localStorage.getItem(BG_BLUR_KEY)
  if (storedBlur !== null) {
    const v = parseInt(storedBlur, 10)
    if (!isNaN(v)) applyBgBlur(v)
  }

  const storedP3 = localStorage.getItem(P3_KEY) as 'off' | 'on' | 'auto' | null
  if (storedP3 && ['off', 'on', 'auto'].includes(storedP3)) {
    applyP3Mode(storedP3)
  }
}

function toggleDark() {
  const newDark = !isDark.value
  applyTheme(newDark)
  localStorage.setItem(THEME_KEY, newDark ? 'dark' : 'light')
}

function setThemeColor(color: string) {
  applyColor(color)
}

function setBgColor(color: string | null) {
  applyBgColor(color)
  if (color) {
    localStorage.setItem(BG_COLOR_KEY, color)
  } else {
    localStorage.removeItem(BG_COLOR_KEY)
  }
}

function setBgBlur(blur: number) {
  applyBgBlur(blur)
}

function setP3Mode(mode: 'off' | 'on' | 'auto') {
  applyP3Mode(mode)
}

const isP3Supported = ref(false)

function detectP3Support() {
  isP3Supported.value = supportsP3()
}

export function useTheme() {
  return {
    isDark,
    currentColor,
    currentBgColor,
    bgBlur,
    p3Mode,
    isP3Supported,
    initTheme,
    detectP3Support,
    toggleDark,
    setThemeColor,
    setBgColor,
    setBgBlur,
    setP3Mode,
    THEME_PRESETS,
    BG_PRESETS,
  }
}
