<template>
  <ion-page>
    <ion-tabs>
      <ion-router-outlet></ion-router-outlet>
      <ion-tab-bar slot="bottom">
        <ion-tab-button tab="files" href="/tabs/files">
          <ion-icon :icon="folder"></ion-icon>
          <ion-label>{{ t('tabs.files') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="tasks" href="/tabs/tasks">
          <ion-icon :icon="list"></ion-icon>
          <ion-label>{{ t('tabs.tasks') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="webdav" href="/tabs/webdav">
          <ion-icon :icon="cloud"></ion-icon>
          <ion-label>{{ t('tabs.webdav') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="settings" href="/tabs/settings">
          <ion-icon :icon="settings"></ion-icon>
          <ion-label>{{ t('tabs.settings') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="devlogs" href="/tabs/devlogs">
          <ion-icon :icon="bug"></ion-icon>
          <ion-label>DevLogs</ion-label>
        </ion-tab-button>
      </ion-tab-bar>
    </ion-tabs>
  </ion-page>
</template>

<script setup lang="ts">
import {
  IonIcon,
  IonLabel,
  IonPage,
  IonRouterOutlet,
  IonTabBar,
  IonTabButton,
  IonTabs,
} from '@ionic/vue'
import { folder, list, cloud, settings, bug } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()
</script>
