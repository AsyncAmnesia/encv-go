<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/home" />
        </ion-buttons>
        <ion-title>config.json 编辑器</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div class="editor-container">
        <textarea
          v-model="content"
          class="json-editor"
          :class="{ 'has-error': hasError }"
          spellcheck="false"
          @input="onInput"
        ></textarea>
        <div v-if="hasError" class="error-text">
          JSON 错误：{{ error }}
        </div>
        <div v-else class="success-text">JSON 有效</div>
      </div>
    </ion-content>

    <ion-footer>
      <ion-toolbar>
        <ion-buttons slot="end">
          <ion-button @click="discard" color="medium">取消</ion-button>
          <ion-button @click="showSaveOptions" :disabled="hasError || isSaving">
            <ion-spinner v-if="isSaving" name="crescent" />
            <span v-else>保存</span>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-footer>
  </ion-page>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonButtons,
  IonButton,
  IonBackButton,
  IonTitle,
  IonContent,
  IonFooter,
  IonSpinner,
  modalController,
} from '@ionic/vue'
import { OpenListNative, logBuffer } from '@/plugins/openlist-native'
import SaveOptionsDialog from '@/components/SaveOptionsDialog.vue'

const router = useRouter()

const content = ref('')
const hasError = ref(false)
const error = ref('')
const isSaving = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  content.value = OpenListNative.readConfig()
  validate()
})

function onInput() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(validate, 300)
}

function validate() {
  if (!content.value.trim()) {
    hasError.value = true
    error.value = '内容为空'
    return
  }
  try {
    JSON.parse(content.value)
    hasError.value = false
    error.value = ''
  } catch (e: any) {
    hasError.value = true
    error.value = e.message
  }
}

async function showSaveOptions() {
  const modal = await modalController.create({
    component: SaveOptionsDialog,
  })
  await modal.present()
  const { data } = await modal.onDidDismiss()
  if (data === 'saveOnly' || data === 'saveAndRestart') {
    await doSave(data === 'saveAndRestart')
  }
}

async function doSave(restart: boolean) {
  isSaving.value = true
  try {
    const ok = OpenListNative.writeConfig(content.value)
    logBuffer[ok ? 'info' : 'error'](ok ? 'config.json 已保存' : '保存失败')
    if (ok && restart) {
      logBuffer.info('重启 OpenList...')
      OpenListNative.stopOpenList()
      setTimeout(() => {
        OpenListNative.startOpenList()
        router.back()
      }, 1500)
    } else if (ok) {
      router.back()
    }
  } finally {
    isSaving.value = false
  }
}

function discard() {
  router.back()
}
</script>

<style scoped>
.editor-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 8px;
}
.json-editor {
  flex: 1;
  width: 100%;
  font-family: monospace;
  font-size: 12px;
  border: 1px solid var(--ion-color-medium);
  border-radius: 6px;
  padding: 8px;
  background: var(--ion-background-color, #1e1e1e);
  color: var(--ion-text-color);
  resize: none;
  outline: none;
}
.json-editor.has-error {
  border-color: var(--ion-color-danger);
}
.error-text {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin-top: 4px;
  font-family: monospace;
}
.success-text {
  color: var(--ion-color-success);
  font-size: 12px;
  margin-top: 4px;
}
</style>
