<script setup>
defineProps({
  rows: {
    type: Array,
    required: true
  },
  compact: {
    type: Boolean,
    default: false
  }
})
</script>

<template>
  <div class="info-table" :class="{ compact }">
    <div v-for="row in rows" :key="row[0]" class="info-row">
      <span>{{ row[0] }}</span>
      <strong>{{ row[1] }}</strong>
    </div>
  </div>
</template>
