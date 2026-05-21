<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button :default-href="'/player'" />
        </ion-buttons>
        <ion-title>播放器设置</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-item-group>
          <ion-item-divider>
            <ion-label>播放</ion-label>
          </ion-item-divider>

          <ion-item>
            <ion-label>自动播放</ion-label>
            <ion-toggle :checked="settings.autoPlay"
              @ion-change="settings.autoPlay = $event.detail.checked; save()" />
          </ion-item>

          <ion-item>
            <ion-label>画中画 (PiP)</ion-label>
            <ion-toggle :checked="settings.pipEnabled"
              @ion-change="settings.pipEnabled = $event.detail.checked; save()" />
          </ion-item>

          <ion-item>
            <ion-label>后台播放</ion-label>
            <ion-toggle :checked="settings.backgroundPlay"
              @ion-change="settings.backgroundPlay = $event.detail.checked; save()" />
          </ion-item>
        </ion-item-group>

        <ion-item-group>
          <ion-item-divider>
            <ion-label>高级</ion-label>
          </ion-item-divider>

          <ion-item>
            <ion-label>硬件解码</ion-label>
            <ion-toggle :checked="settings.hwDecode"
              @ion-change="settings.hwDecode = $event.detail.checked; save()" />
          </ion-item>

          <ion-item button @click="clearCache" detail>
            <ion-label>清除播放缓存</ion-label>
          </ion-item>
        </ion-item-group>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle,
  IonContent, IonList, IonItemGroup, IonItemDivider,
  IonItem, IonLabel, IonToggle, IonButtons, IonBackButton,
} from '@ionic/vue'

const PREFIX = 'player:'

interface PlayerSettings {
  autoPlay: boolean
  pipEnabled: boolean
  backgroundPlay: boolean
  hwDecode: boolean
}

const defaults: PlayerSettings = {
  autoPlay: true,
  pipEnabled: false,
  backgroundPlay: false,
  hwDecode: true,
}

const settings = ref<PlayerSettings>({ ...defaults })

function load() {
  const saved = localStorage.getItem(PREFIX + 'settings')
  if (saved) {
    try { Object.assign(settings.value, JSON.parse(saved)) } catch {}
  }
}

function save() {
  localStorage.setItem(PREFIX + 'settings', JSON.stringify(settings.value))
}

async function clearCache() {
  if ('caches' in window) {
    const names = await caches.keys()
    for (const name of names) {
      if (name.includes('player')) await caches.delete(name)
    }
  }
}

onMounted(load)
</script>
