import { computed, onMounted, onUnmounted, ref } from 'vue'
import { collapse, getState, onState, refresh, toggle } from '../services/wails.js'
import { reconcileSelection, statusCopy } from './pure.mjs'

export {
  calendarCells,
  composition,
  dayTooltip,
  formatCompact,
  hourTooltip,
  hours24,
  week168,
  weekTooltip,
} from './pure.mjs'

export function useWidgetStore() {
  const state = ref({ status: 'loading', expanded: false, anchorCorner: 'bottom-right' })
  const busy = ref(false)
  const refreshFeedback = ref('')
  const selected = ref(null)
  let off = () => {}
  let feedbackTimer
  let refreshBaseline = null

  const isExpanded = computed(() => state.value.expanded === true)
  const hasPartial = computed(
    () => !state.value.quota || !state.value.heatmap24h || !state.value.heatmap7d || !state.value.trend30d,
  )
  const statusLabel = computed(() =>
    statusCopy(state.value.status, state.value.status === 'online' && hasPartial.value),
  )

  function update(next) {
    if (!next) return
    state.value = next
    selected.value = reconcileSelection(selected.value, next)
    if (refreshBaseline !== null && next.status === 'online' && next.observedAt && next.observedAt !== refreshBaseline) {
      refreshBaseline = null
      setRefreshFeedback('已刷新')
    }
  }

  function setRefreshFeedback(value) {
    refreshFeedback.value = value
    clearTimeout(feedbackTimer)
    if (value && value !== '正在刷新') {
      feedbackTimer = setTimeout(() => {
        refreshFeedback.value = ''
      }, 1800)
    }
  }

  async function load() {
    busy.value = true
    refreshBaseline = state.value.observedAt || ''
    setRefreshFeedback('正在刷新')
    try {
      const next = await refresh()
      update(next)
      if (next?.status === 'offline' || next?.status === 'invalid_credential') {
        refreshBaseline = null
        setRefreshFeedback('刷新失败')
      } else if (refreshBaseline !== null) {
        setRefreshFeedback('已请求刷新')
      }
    } catch {
      refreshBaseline = null
      setRefreshFeedback('刷新失败')
    } finally {
      busy.value = false
    }
  }

  async function doToggle() {
    update(await toggle())
  }

  async function doCollapse() {
    selected.value = null
    update(await collapse())
  }

  onMounted(async () => {
    update(await getState())
    off = onState(update)
  })
  onUnmounted(() => {
    clearTimeout(feedbackTimer)
    off()
  })

  return {
    state,
    busy,
    refreshFeedback,
    selected,
    isExpanded,
    statusLabel,
    load,
    update,
    doToggle,
    doCollapse,
  }
}
