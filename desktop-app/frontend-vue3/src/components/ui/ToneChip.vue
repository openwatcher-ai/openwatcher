<script setup>
defineProps({
  tone: {
    type: String,
    default: "soft"
  }
})
</script>

<template>
  <span class="tone-chip" :class="`tone-${tone}`">
    <slot />
  </span>
</template>
