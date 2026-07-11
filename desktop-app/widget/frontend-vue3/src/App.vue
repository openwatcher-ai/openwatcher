<template>
  <main
    ref="root"
    data-ui-id="widget-root"
    class="widget-root"
    :class="[state.status, { expanded: isExpanded }]"
    tabindex="-1"
    @click="handleClick"
  >
    <FloatingOrb
      :quota="state.quota"
      :status="state.status"
      :expanded="isExpanded"
      :anchor-corner="state.anchorCorner"
      @toggle="doToggle"
      @drag-finished="snapCurrentWindow"
    />
    <OverviewPanel
      v-if="isExpanded"
      :state="state"
      :status-label="statusLabel"
      :busy="busy"
      :refresh-feedback="refreshFeedback"
      @refresh="load"
      @close="collapsePanel"
      @open-main="openMainApp"
    >
      <QuotaPanel :quota="state.quota" />
      <TodayPanel
        :today="state.today"
        :buckets="state.heatmap24h?.buckets"
        :selected="selected"
        :tooltip="tooltip"
        @select="select"
        @hover="hover"
      />
      <TodayAndTrendPanels
        :heatmap="state.heatmap7d"
        :trend="state.trend30d"
        :selected="selected"
        :tooltip="tooltip"
        @select="select"
        @hover="hover"
      />
    </OverviewPanel>
    <p v-if="state.errorText && isExpanded" class="state-banner">{{ state.errorText }}</p>
  </main>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { openMainApp, snapCurrentWindow } from './services/wails.js'
import { useWidgetStore } from './state/widget.js'
import FloatingOrb from './components/FloatingOrb.vue'
import OverviewPanel from './components/OverviewPanel.vue'
import QuotaPanel from './components/QuotaPanel.vue'
import TodayAndTrendPanels from './components/TodayAndTrendPanels.vue'
import TodayPanel from './components/TodayPanel.vue'

const {
  state,
  busy,
  refreshFeedback,
  selected,
  isExpanded,
  statusLabel,
  load,
  doToggle,
  doCollapse,
} = useWidgetStore()

const root = ref(null)
const hovered = ref(null)
const tooltip = computed(() => selected.value || hovered.value)

function select(item) {
  if (!item) return
  selected.value = selected.value?.kind === item.kind && selected.value?.key === item.key ? null : item
}

function hover(item) {
  hovered.value = item
}

function clearSelection() {
  selected.value = null
}

function handleClick(event) {
  if (!event.target.closest('button, [role="tooltip"], .quota-ring, .header-actions')) {
    clearSelection()
  }
}

async function collapsePanel() {
  hovered.value = null
  await doCollapse()
}

function onKeydown(event) {
  if (event.key === 'Escape' && isExpanded.value) collapsePanel()
}

watch(isExpanded, async (expanded) => {
  if (expanded) {
    await nextTick()
    root.value?.focus({ preventScroll: true })
  }
})

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>
