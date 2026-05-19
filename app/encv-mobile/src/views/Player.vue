<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ fileName || 'Player' }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="!filePath" class="empty-state">
        <ion-icon :icon="playCircle" class="empty-icon"></ion-icon>
        <h3>No Media Selected</h3>
        <p>Select a video or audio file from the Files tab to play.</p>
        <ion-button router-link="/tabs/files">
          <ion-icon :icon="folder" slot="start"></ion-icon>
          Browse Files
        </ion-button>
      </div>

      <div v-else class="player-container">
        <video
          v-if="isVideo"
          ref="videoRef"
          :src="streamUrl"
          controls
          autoplay
          playsinline
          class="video-player"
          @error="handlePlayerError"
        ></video>

        <div v-if="isAudio" class="audio-player-wrapper">
          <div class="audio-visual">
            <ion-icon :icon="musicalNotes" class="audio-icon"></ion-icon>
          </div>
          <h3>{{ fileName }}</h3>
          <audio
            ref="audioRef"
            :src="streamUrl"
            controls
            autoplay
            class="audio-player"
            @error="handlePlayerError"
          ></audio>
        </div>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonContent,
  IonIcon,
  IonButton,
  toastController,
} from '@ionic/vue'
import { playCircle, folder, musicalNotes } from 'ionicons/icons'
import { getFileStreamUrl, getFileCategory } from '@/api/encv'

const route = useRoute()

const filePath = computed(() => (route.query.path as string) || '')
const fileName = computed(() => (route.query.name as string) || '')

const fileCategory = computed(() => {
  if (!fileName.value) return 'other'
  return getFileCategory(fileName.value)
})

const isVideo = computed(() => fileCategory.value === 'video' || fileCategory.value === 'encrypted')
const isAudio = computed(() => fileCategory.value === 'audio')

const streamUrl = computed(() => {
  if (!filePath.value) return ''
  return getFileStreamUrl(filePath.value)
})

const videoRef = ref<HTMLVideoElement | null>(null)
const audioRef = ref<HTMLAudioElement | null>(null)

async function handlePlayerError() {
  const toast = await toastController.create({
    message: `Failed to play "${fileName.value}". The file may be corrupted or the server is unavailable.`,
    duration: 3000,
    color: 'danger',
  })
  await toast.present()
}
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.empty-icon {
  font-size: 80px;
  margin-bottom: 16px;
  opacity: 0.4;
}

.player-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.video-player {
  width: 100%;
  max-height: 40vh;
  background: #000;
}

.audio-player-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  text-align: center;
}

.audio-visual {
  width: 120px;
  height: 120px;
  border-radius: 60px;
  background: var(--ion-color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
}

.audio-icon {
  font-size: 48px;
  color: white;
}

.audio-player {
  width: 100%;
  max-width: 400px;
  margin-top: 24px;
}
</style>
