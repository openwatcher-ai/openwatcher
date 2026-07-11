<template>
  <div class="composition" aria-label="今日组成">
    <div class="composition-segments">
      <span
        v-for="part in parts"
        :key="part.kind"
        class="composition-segment"
        :class="part.color"
        :style="{ width: `${part.fraction * 100}%` }"
        :aria-label="`${part.kind} ${format(part.value)} tokens`"
      />
    </div>
    <div class="composition-copy" aria-hidden="true">
      <span v-for="part in parts" :key="part.kind">
        {{ part.kind }} {{ format(part.value) }}<small v-if="part.kind === '缓存输入'">（{{ chr }}）</small>
      </span>
    </div>
  </div>
</template>

<script setup>
import { formatCompact } from '../state/widget.js'

defineProps({
  parts: { type: Array, default: () => [] },
  chr: { type: String, default: '—' },
})

const format = formatCompact
</script>
