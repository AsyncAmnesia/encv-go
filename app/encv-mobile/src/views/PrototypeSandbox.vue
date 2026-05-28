<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings/devtools"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ proto?.name || 'Prototype' }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="!proto" class="not-found">
        <p>Prototype not found</p>
      </div>

      <template v-else>
        <div class="sandbox-meta">
          <div class="meta-row">
            <span class="meta-label">Route</span>
            <code class="meta-value">{{ proto.route }}</code>
          </div>
          <div class="meta-row">
            <span class="meta-label">Compose</span>
            <code class="meta-value">{{ proto.composePath }}</code>
          </div>
        </div>

        <div class="tab-bar">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            class="tab-btn"
            :class="{ active: activeTab === tab.id }"
            @click="activeTab = tab.id"
          >
            <ion-icon :icon="tab.icon" class="tab-icon"></ion-icon>
            {{ tab.label }}
          </button>
        </div>

        <div class="tab-content">
          <div v-show="activeTab === 'preview'" class="preview-panel">
            <div class="preview-frame">
              <component :is="loadedComponent" v-if="loadedComponent" />
              <div v-else class="loading-state">
                <ion-spinner name="crescent"></ion-spinner>
              </div>
            </div>
          </div>

          <div v-show="activeTab === 'web'" class="code-panel">
            <div class="code-header">
              <span class="code-filename">Vue Component (Web)</span>
              <button class="copy-btn" @click="copySource(webSource)">
                <ion-icon :icon="copyOutline" size="small"></ion-icon>
                {{ t('devtools.copyCode') }}
              </button>
            </div>
            <pre class="code-block"><code>{{ webSource }}</code></pre>
          </div>

          <div v-show="activeTab === 'compose'" class="code-panel">
            <div class="code-header">
              <span class="code-filename">Kotlin Compose (Android)</span>
              <button class="copy-btn" @click="copySource(composeSource)">
                <ion-icon :icon="copyOutline" size="small"></ion-icon>
                {{ t('devtools.copyCode') }}
              </button>
            </div>
            <pre class="code-block"><code>{{ composeSource }}</code></pre>
          </div>
        </div>
      </template>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, watch, type Component as VueComponent } from 'vue'
import { useRoute } from 'vue-router'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonIcon, IonSpinner,
} from '@ionic/vue'
import { eyeOutline, logoVue, codeSlashOutline, copyOutline } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { getPrototype } from './prototypes/registry'

const { t } = useI18n()
const route = useRoute()

const protoId = computed(() => route.params.id as string || '')
const proto = computed(() => getPrototype(protoId.value))

const activeTab = ref<'preview' | 'web' | 'compose'>('preview')
const loadedComponent = ref<VueComponent | null>(null)
const webSource = ref('')
const composeSource = ref('')

const tabs = [
  { id: 'preview' as const, label: 'Preview', icon: eyeOutline },
  { id: 'web' as const, label: 'Web', icon: logoVue },
  { id: 'compose' as const, label: 'Compose', icon: codeSlashOutline },
]

watch(proto, async (p) => {
  if (!p) return
  loadedComponent.value = null
  webSource.value = ''
  composeSource.value = ''
  try {
    const mod = await p.component()
    loadedComponent.value = mod.default
  } catch (e) {
    console.error('Failed to load prototype component:', e)
  }
}, { immediate: true })

watch(activeTab, async (tab) => {
  if (!proto.value) return
  if (tab === 'web' && !webSource.value) {
    try {
      webSource.value = await proto.value.webSource()
    } catch (e) {
      webSource.value = '// Source not available'
    }
  }
  if (tab === 'compose' && !composeSource.value) {
    try {
      composeSource.value = await proto.value.composeSource()
    } catch (e) {
      composeSource.value = '// Source not available'
    }
  }
})

async function copySource(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    showToast({ message: t('devtools.copiedCode'), duration: 1500, color: 'success' })
  } catch {
    showToast({ message: t('devtools.copyFailed'), duration: 1500, color: 'danger' })
  }
}
</script>

<style scoped>
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--encv-text-secondary, #999);
}

.sandbox-meta {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb, 128,128,128), 0.12);
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.meta-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--ion-color-primary, #BB86FC);
  flex-shrink: 0;
  width: 64px;
}

.meta-value {
  font-size: 12px;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  color: var(--ion-text-color, #fff);
  background: rgba(var(--ion-color-medium-rgb, 128,128,128), 0.08);
  padding: 2px 8px;
  border-radius: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tab-bar {
  display: flex;
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb, 128,128,128), 0.12);
  padding: 0 8px;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 16px;
  border: none;
  background: none;
  color: var(--encv-text-secondary, #999);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}

.tab-btn.active {
  color: var(--ion-color-primary, #BB86FC);
  border-bottom-color: var(--ion-color-primary, #BB86FC);
}

.tab-icon {
  font-size: 16px;
}

.tab-content {
  min-height: 400px;
}

.preview-panel {
  padding: 16px;
}

.preview-frame {
  border-radius: 12px;
  overflow: hidden;
  background: #121212;
  box-shadow: 0 2px 12px rgba(0,0,0,0.3);
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
}

.code-panel {
  display: flex;
  flex-direction: column;
}

.code-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: rgba(var(--ion-color-medium-rgb, 128,128,128), 0.06);
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb, 128,128,128), 0.08);
}

.code-filename {
  font-size: 12px;
  font-weight: 600;
  color: var(--encv-text-secondary, #999);
}

.copy-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 1px solid rgba(var(--ion-color-medium-rgb, 128,128,128), 0.2);
  border-radius: 6px;
  background: transparent;
  color: var(--ion-text-color, #fff);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.copy-btn:hover {
  background: rgba(var(--ion-color-medium-rgb, 128,128,128), 0.1);
}

.code-block {
  margin: 0;
  padding: 16px;
  overflow-x: auto;
  font-size: 12px;
  line-height: 1.6;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  color: var(--ion-text-color, #e0e0e0);
  background: var(--ion-background-color, #121212);
  max-height: 60vh;
  white-space: pre;
  tab-size: 4;
}
</style>
