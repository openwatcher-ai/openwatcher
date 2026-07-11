<template>
  <div class="composition" aria-label="今日组成">
    <span
      v-for="part in parts"
      :key="part.kind"
      :class="part.color"
      :style="{ width: `${part.fraction * 100}%` }"
      :aria-label="`${part.kind} ${part.value} tokens`"
    />
  </div>
</template>

<script setup>
defineProps({ parts: { type: Array, default: () => [] } })
</script>
