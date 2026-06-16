<script setup>
defineProps({
  tabs: {
    type: Array,
    required: true
  },
  modelValue: {
    type: String,
    required: true
  }
})

const emit = defineEmits(["update:modelValue"])
</script>

<template>
  <nav class="page-tabs">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      class="page-tab"
      :class="{ 'is-active': modelValue === tab.id, 'is-danger': tab.danger }"
      type="button"
      @click="emit('update:modelValue', tab.id)"
    >
      {{ tab.label }}
    </button>
  </nav>
</template>
