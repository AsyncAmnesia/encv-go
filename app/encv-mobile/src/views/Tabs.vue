<template>
  <ion-page>
    <ion-tabs>
      <ion-router-outlet></ion-router-outlet>
      <ion-tab-bar slot="bottom">
        <ion-tab-button tab="files" href="/tabs/files">
          <ion-icon :icon="folder"></ion-icon>
          <ion-label>Files</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="player" href="/tabs/player">
          <ion-icon :icon="play"></ion-icon>
          <ion-label>Player</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="tasks" href="/tabs/tasks">
          <ion-icon :icon="list"></ion-icon>
          <ion-label>Tasks</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="webdav" href="/tabs/webdav">
          <ion-icon :icon="cloud"></ion-icon>
          <ion-label>WebDAV</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="settings" href="/tabs/settings">
          <ion-icon :icon="settings"></ion-icon>
          <ion-label>Settings</ion-label>
        </ion-tab-button>
      </ion-tab-bar>
    </ion-tabs>
  </ion-page>
</template>

<script setup>
import {
  IonIcon,
  IonLabel,
  IonPage,
  IonRouterOutlet,
  IonTabBar,
  IonTabButton,
  IonTabs,
} from '@ionic/vue'
import { folder, play, list, cloud, settings } from 'ionicons/icons'
</script>
