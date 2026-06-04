<template>
  <ion-modal :is-open="isOpen" @did-dismiss="onDismiss">
    <ion-header>
      <ion-toolbar>
        <ion-title>设置管理员密码</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="onDismiss">取消</ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>
    <ion-content class="ion-padding">
      <ion-list>
        <ion-item>
          <ion-label position="stacked">新密码</ion-label>
          <ion-input
            v-model="password"
            type="password"
            placeholder="请输入新密码"
            :clear-input="true"
          ></ion-input>
        </ion-item>
        <ion-item>
          <ion-label position="stacked">确认密码</ion-label>
          <ion-input
            v-model="confirmPassword"
            type="password"
            placeholder="再次输入新密码"
            :clear-input="true"
          ></ion-input>
        </ion-item>
      </ion-list>
      <div v-if="error" class="error-text">{{ error }}</div>
      <ion-button
        expand="block"
        :disabled="!canConfirm"
        @click="onConfirm"
        class="ion-margin-top"
      >
        确认
      </ion-button>
    </ion-content>
  </ion-modal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  IonModal,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonButton,
  IonContent,
  IonList,
  IonItem,
  IonLabel,
  IonInput,
} from '@ionic/vue'

const props = defineProps<{
  isOpen?: boolean
  onConfirm: (password: string) => void | Promise<void>
}>()

const emit = defineEmits<{
  (e: 'update:isOpen', v: boolean): void
  (e: 'did-dismiss'): void
}>()

const password = ref('')
const confirmPassword = ref('')
const error = ref('')

const canConfirm = computed(() => {
  return password.value.length >= 4 && password.value === confirmPassword.value
})

async function onConfirm() {
  if (!canConfirm.value) {
    error.value = '密码不一致或太短（至少 4 位）'
    return
  }
  error.value = ''
  await props.onConfirm(password.value)
  emit('update:isOpen', false)
  emit('did-dismiss')
}

function onDismiss() {
  emit('update:isOpen', false)
  emit('did-dismiss')
}
</script>

<style scoped>
.error-text {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin: 8px 0;
}
</style>
