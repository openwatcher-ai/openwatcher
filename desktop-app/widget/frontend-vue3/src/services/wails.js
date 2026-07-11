const previewNow = Math.floor(Date.now() / 1000)
const hourlyPattern = [3, 8, 13, 9, 4, 10, 4, 20, 11, 4, 3, 4, 5, 6, 10, 14, 18, 10, 9, 6, 10, 16, 12, 7]
const devFallback = {
  status: 'online',
  expanded: true,
  anchorCorner: 'bottom-right',
  observedAt: new Date().toISOString(),
  timezone: 'Asia/Shanghai',
  quota: {
    planType: '标准计划',
    fresh: true,
    status: 'ok',
    fiveHour: { remainingPercent: 80, resetAt: previewNow + 4 * 3600 + 45 * 60 },
    weekly: { remainingPercent: 53, resetAt: previewNow + 3 * 86400 + 3 * 3600 },
  },
  heatmap24h: {
    timezone: 'Asia/Shanghai',
    buckets: Array.from({ length: 24 }, (_, hour) => ({
      hourStart: `2026-07-11T${String(hour).padStart(2, '0')}:00:00+08:00`,
      inputTokens: hourlyPattern[hour] * 42000,
      cachedInputTokens: hourlyPattern[hour] * 39000,
      outputTokens: hourlyPattern[hour] * 2800,
      reasoningOutputTokens: hourlyPattern[hour] * 350,
      totalTokens: hourlyPattern[hour] * 45150,
      activeThreads: hour % 4,
    })),
  },
  heatmap7d: {
    timezone: 'Asia/Shanghai',
    peakTokens: 4000000,
    days: Array.from({ length: 7 }, (_, day) => ({
      date: `2026-07-${String(11 - day).padStart(2, '0')}`,
      hours: Array.from({ length: 24 }, (_, hour) => {
        const signal = (hour + day * 7) % 31
        if (signal === 0) return 4000000
        if (signal % 11 === 0) return 2600000
        return ((hour * 3 + day * 5) % 9) * 150000
      }),
    })),
  },
  today: {
    inputTokens: 14700000,
    cachedInputTokens: 13700000,
    outputTokens: 80800,
    reasoningOutputTokens: 31000,
    totalTokens: 14800000,
    valueLabel: '$14.03',
  },
  trend30d: {
    startDate: '2026-06-11',
    endDate: '2026-07-10',
    totalTokens: 13400000000,
    averageTokens: 446300000,
    peakTokens: 1700000000,
    valueLabel: '$6417.16',
    days: Array.from({ length: 30 }, (_, index) => ({
      date: new Date(Date.UTC(2026, 5, 11 + index)).toISOString().slice(0, 10),
      totalTokens: ((index * 5) % 12) * 145000000,
    })),
  },
}

let previewState = structuredClone(devFallback)
const wails = () => typeof window !== 'undefined' && window.go?.main?.App
const preview = () => structuredClone(previewState)

export async function getState() {
  if (wails()?.State) return wails().State()
  return import.meta.env.DEV ? preview() : { status: 'loading', expanded: false, anchorCorner: 'bottom-right' }
}

export async function refresh() {
  if (wails()?.Refresh) return wails().Refresh()
  if (import.meta.env.DEV) {
    previewState.observedAt = new Date().toISOString()
    return preview()
  }
  return getState()
}

export async function toggle() {
  if (wails()?.Toggle) return wails().Toggle()
  if (import.meta.env.DEV) {
    previewState.expanded = !previewState.expanded
    return preview()
  }
  return getState()
}

export async function collapse() {
  if (wails()?.Collapse) return wails().Collapse()
  if (import.meta.env.DEV) {
    previewState.expanded = false
    return preview()
  }
  return getState()
}

export function snapCurrentWindow() {
  return wails()?.SnapCurrentWindow?.()
}

export function openMainApp() {
  return wails()?.OpenMainApp?.()
}

export function onState(callback) {
  return window.runtime?.EventsOn ? window.runtime.EventsOn('widget:state', callback) : () => {}
}
