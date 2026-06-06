<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/composables/useI18n'
import BlockHeader from './BlockHeader.vue'

export interface PlanTodo {
  id: string
  status: 'pending' | 'in_progress' | 'completed' | string
  content: string
}

const props = withDefaults(
  defineProps<{
    todos: PlanTodo[]
    streaming?: boolean
  }>(),
  {
    streaming: false,
  },
)

const { t } = useI18n()

// Split todos by status so completed items sit at the bottom
// of the list (they read as a log of what's been done) and
// the in_progress / pending items are at the top where the
// user's eye lands first. This is a presentational choice
// only — the underlying id+status+content is preserved so
// the LLM's notion of ordering can be reconstructed by id.
const orderedTodos = computed(() => {
  const inProgress = props.todos.filter((x) => x.status === 'in_progress')
  const pending = props.todos.filter((x) => x.status === 'pending')
  const completed = props.todos.filter((x) => x.status === 'completed')
  const unknown = props.todos.filter(
    (x) =>
      x.status !== 'in_progress' &&
      x.status !== 'pending' &&
      x.status !== 'completed',
  )
  return [...inProgress, ...pending, ...unknown, ...completed]
})

function statusLabel(status: string): string {
  if (status === 'in_progress') return t('agent.planStatusInProgress')
  if (status === 'completed') return t('agent.planStatusCompleted')
  if (status === 'pending') return t('agent.planStatusPending')
  return status
}
</script>

<template>
  <div class="plan-block" :class="{ 'is-streaming': props.streaming }">
    <BlockHeader
      icon="list-outline"
      :title="t('agent.plan')"
      :badge="props.streaming ? t('agent.streaming') : undefined"
      :expanded="true"
    />
    <ol v-if="orderedTodos.length > 0" class="plan-list" data-testid="plan-list">
      <li
        v-for="todo in orderedTodos"
        :key="todo.id"
        class="plan-item"
        :class="`plan-item--${todo.status}`"
        :data-testid="`plan-item-${todo.id}`"
      >
        <span class="plan-marker" aria-hidden="true">
          <template v-if="todo.status === 'completed'">✓</template>
          <template v-else-if="todo.status === 'in_progress'">●</template>
          <template v-else>○</template>
        </span>
        <span class="plan-content">{{ todo.content }}</span>
        <span class="plan-status">{{ statusLabel(todo.status) }}</span>
      </li>
    </ol>
    <p v-else class="plan-empty">{{ t('agent.planEmpty') }}</p>
  </div>
</template>

<style scoped>
.plan-block {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem 0.875rem;
  border: 1px solid var(--ion-color-step-200, #e4e4e7);
  border-radius: 0.5rem;
  background: var(--ion-color-step-50, #fafafa);
}

.plan-block.is-streaming {
  border-color: var(--ion-color-primary);
  box-shadow: 0 0 0 1px var(--ion-color-primary-tint, rgba(79, 140, 255, 0.3));
}

.plan-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.plan-item {
  display: flex;
  align-items: baseline;
  gap: 0.625rem;
  padding: 0.25rem 0;
  font-size: 0.875rem;
  line-height: 1.4;
}

.plan-item--completed .plan-content {
  text-decoration: line-through;
  color: var(--ion-color-step-600, #6b7280);
}

.plan-item--in_progress .plan-content {
  font-weight: 600;
  color: var(--ion-color-primary-shade, #1d4ed8);
}

.plan-item--pending .plan-content {
  color: var(--ion-color-step-700, #374151);
}

.plan-item--unknown .plan-content {
  color: var(--ion-color-step-700, #374151);
}

.plan-marker {
  flex: 0 0 auto;
  width: 1rem;
  text-align: center;
  font-size: 0.95rem;
}

.plan-item--completed .plan-marker {
  color: var(--ion-color-success, #16a34a);
}

.plan-item--in_progress .plan-marker {
  color: var(--ion-color-primary, #4f8cff);
  animation: plan-pulse 1.6s ease-in-out infinite;
}

.plan-item--pending .plan-marker {
  color: var(--ion-color-step-400, #9ca3af);
}

.plan-item--unknown .plan-marker {
  color: var(--ion-color-step-400, #9ca3af);
}

@keyframes plan-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}

.plan-content {
  flex: 1 1 auto;
  word-break: break-word;
}

.plan-status {
  flex: 0 0 auto;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--ion-color-step-500, #6b7280);
}

.plan-empty {
  margin: 0;
  font-size: 0.85rem;
  color: var(--ion-color-step-500, #6b7280);
  font-style: italic;
}
</style>
