<script setup>
import AppIcon from "./Icon.vue"

defineProps({
  tone: {
    type: String,
    default: "secondary"
  },
  icon: {
    type: String,
    default: ""
  },
  block: {
    type: Boolean,
    default: false
  },
  small: {
    type: Boolean,
    default: false
  },
  iconOnly: {
    type: Boolean,
    default: false
  },
  loading: {
    type: Boolean,
    default: false
  },
  copied: {
    type: Boolean,
    default: false
  }
})
</script>

<template>
  <button
    class="ow-button"
    :class="[`tone-${tone}`, { block, small, 'icon-only': iconOnly, 'is-loading': loading, 'is-copied': copied }]"
    :aria-busy="loading ? 'true' : undefined"
    :data-copy-tip="copied ? '已复制' : null"
  >
    <AppIcon v-if="icon" :name="icon" :size="small ? 14 : 16" class="ow-button-icon" :class="{ 'is-spinning': loading }" />
    <slot />
  </button>
</template>
