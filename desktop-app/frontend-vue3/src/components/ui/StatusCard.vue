<script setup>
import AppIcon from "./Icon.vue"

defineProps({
  card: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <article class="status-card">
    <div class="status-card-icon" :class="`tone-${card.tone || 'blue'}`">
      <AppIcon :name="card.icon" :size="18" />
    </div>
    <div class="status-card-body">
      <span>{{ card.title }}</span>
      <strong :class="`tone-text-${card.tone || 'blue'}`">{{ card.value }}</strong>
      <em>{{ card.meta }}</em>
    </div>
  </article>
</template>
