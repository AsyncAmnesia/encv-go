import { ref } from 'vue'

const isPickerMode = ref(false)
const selectedPath = ref('')
const selectedName = ref('')
let resolvePicker: ((value: { path: string; name: string } | null) => void) | null = null

export function useFilePicker() {
  function startPicking(): Promise<{ path: string; name: string } | null> {
    return new Promise((resolve) => {
      isPickerMode.value = true
      selectedPath.value = ''
      selectedName.value = ''
      resolvePicker = resolve
    })
  }

  function confirmSelection(path: string, name: string) {
    selectedPath.value = path
    selectedName.value = name
    isPickerMode.value = false
    resolvePicker?.({ path, name })
    resolvePicker = null
  }

  function cancelPicking() {
    isPickerMode.value = false
    selectedPath.value = ''
    selectedName.value = ''
    resolvePicker?.(null)
    resolvePicker = null
  }

  return {
    isPickerMode,
    selectedPath,
    selectedName,
    startPicking,
    confirmSelection,
    cancelPicking,
  }
}
