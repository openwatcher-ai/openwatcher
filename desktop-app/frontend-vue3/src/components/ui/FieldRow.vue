<script setup>
defineProps({
  label: {
    type: String,
    required: true
  }
})
</script>

<template>
  <label class="field-row">
    <span>{{ label }}</span>
    <slot />
  </label>
</template>
