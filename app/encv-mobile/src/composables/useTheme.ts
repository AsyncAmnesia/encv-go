import { ref } from 'vue'

const THEME_KEY = 'encv-theme-preference'
const COLOR_KEY = 'encv-theme-color'

const isDark = ref(false)
const currentColor = ref('#4f8cff')

export interface ThemePreset {
  name: string
  value: string
}

export const THEME_PRESETS: ThemePreset[] = [
  { name: 'Blue', value: '#4f8cff' },
  { name: 'Purple', value: '#8b5cf6' },
  { name: 'Green', value: '#22c55e' },
  { name: 'Orange', value: '#f97316' },
  { name: 'Red', value: '#ef4444' },
  { name: 'Pink', value: '#ec4899' },
  { name: 'Teal', value: '#14b8a6' },
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

function applyTheme(dark: boolean) {
  isDark.value = dark
  if (dark) {
    document.body.classList.add('dark')
  } else {
    document.body.classList.remove('dark')
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
}

function toggleDark() {
  const newDark = !isDark.value
  applyTheme(newDark)
  localStorage.setItem(THEME_KEY, newDark ? 'dark' : 'light')
}

function setThemeColor(color: string) {
  applyColor(color)
}

export function useTheme() {
  return {
    isDark,
    currentColor,
    initTheme,
    toggleDark,
    setThemeColor,
    THEME_PRESETS,
  }
}
