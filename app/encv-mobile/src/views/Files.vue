<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>Files</ion-title>
      </ion-toolbar>
    </ion-header>
    <ion-content class="ion-padding">
      <ion-card v-if="!serverOnline">
        <ion-card-header>
          <ion-card-title>Server Offline</ion-card-title>
        </ion-card-header>
        <ion-card-content>
          ENCV server is not running. Please start the server first.
        </ion-card-content>
      </ion-card>

      <ion-card v-else>
        <ion-card-header>
          <ion-card-title>Encrypted Files</ion-card-title>
        </ion-card-header>
        <ion-card-content>
          <ion-list>
            <ion-item v-for="file in files" :key="file.path" @click="playFile(file)">
              <ion-icon :icon="file.isDirectory ? folder : play" slot="start"></ion-icon>
              <ion-label>{{ file.name }}</ion-label>
            </ion-item>
          </ion-list>
        </ion-card-content>
      </ion-card>
    </ion-content>
  </ion-page>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  IonContent,
  IonHeader,
  IonPage,
  IonTitle,
  IonToolbar,
  IonCard,
  IonCardHeader,
  IonCardTitle,
  IonCardContent,
  IonList,
  IonItem,
  IonIcon,
  IonLabel,
} from '@ionic/vue'
import { folder, play } from 'ionicons/icons'
import { listFiles, checkServerStatus } from '../api/encv'

const serverOnline = ref(false)
const files = ref([])

onMounted(async () => {
  await loadFiles()
})

async function loadFiles() {
  serverOnline.value = await checkServerStatus()
  if (serverOnline.value) {
    try {
      files.value = await listFiles()
    } catch (error) {
      console.error('Failed to load files:', error)
    }
  }
}

function playFile(file) {
  console.log('Playing file:', file)
}
</script>
