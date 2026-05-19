import { ref } from 'vue'

const THEME_KEY = 'encv-theme-preference'

const isDark = ref(false)

function applyTheme(dark: boolean) {
  isDark.value = dark
  if (dark) {
    document.body.classList.add('dark')
  } else {
    document.body.classList.remove('dark')
  }
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
}

function toggleDark() {
  const newDark = !isDark.value
  applyTheme(newDark)
  localStorage.setItem(THEME_KEY, newDark ? 'dark' : 'light')
}

export function useTheme() {
  return {
    isDark,
    initTheme,
    toggleDark,
  }
}
