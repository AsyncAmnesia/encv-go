<template>
  <ion-page>
    <ion-tabs @ionTabsWillChange="onTabsWillChange" @ionTabsDidChange="onTabsDidChange">
      <ion-router-outlet></ion-router-outlet>
      <ion-tab-bar slot="bottom">
        <ion-tab-button tab="home" href="/tabs/home">
          <ion-icon :icon="home"></ion-icon>
          <ion-label>{{ t('tabs.home') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="files" href="/tabs/files">
          <ion-icon :icon="folder"></ion-icon>
          <ion-label>{{ t('tabs.files') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="tasks" href="/tabs/tasks">
          <ion-icon :icon="list"></ion-icon>
          <ion-label>{{ t('tabs.tasks') }}</ion-label>
        </ion-tab-button>

        <ion-tab-button tab="remote" href="/tabs/remote">
          <ion-icon :icon="globe"></ion-icon>
          <ion-label>{{ t('tabs.remote') }}</ion-label>
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
import { home, folder, list, globe, settings, bug } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

function onTabsWillChange(event: CustomEvent) {
  const tab = event?.detail?.tab ?? '(unknown)'
  console.error('[SAT-DBG][Tabs] ionTabsWillChange →', tab, '| ts=', Date.now())
}

function onTabsDidChange(event: CustomEvent) {
  const tab = event?.detail?.tab ?? '(unknown)'
  console.error('[SAT-DBG][Tabs] ionTabsDidChange  →', tab, '| ts=', Date.now())
}
</script>
