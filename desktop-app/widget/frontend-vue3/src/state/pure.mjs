export function composition(today) {
  if (!today) return []

  const uncached = Math.max(0, today.inputTokens - today.cachedInputTokens)
  const cached = Math.max(0, today.cachedInputTokens)
  const output = Math.max(0, today.outputTokens + today.reasoningOutputTokens)
  const total = uncached + cached + output

  return [
    {
      kind: '输入',
      value: today.inputTokens,
      segmentValue: uncached,
      fraction: total ? uncached / total : 0,
      color: 'blue',
    },
    {
      kind: '缓存输入',
      value: cached,
      segmentValue: cached,
      fraction: total ? cached / total : 0,
      color: 'purple',
    },
    {
      kind: '输出',
      value: output,
      segmentValue: output,
      fraction: total ? output / total : 0,
      color: 'green',
    },
  ]
}

export function buildHeatScale(values = [], levelCount = 5) {
  const sorted = values
    .filter((value) => Number.isFinite(value) && value > 0)
    .sort((a, b) => a - b)
  if (!sorted.length) return { flat: false, thresholds: [] }
  if (sorted[0] === sorted.at(-1)) return { flat: true, thresholds: [] }

  const quantile = (fraction) => {
    const position = (sorted.length - 1) * fraction
    const lower = Math.floor(position)
    const upper = Math.ceil(position)
    if (lower === upper) return sorted[lower]
    return sorted[lower] + (sorted[upper] - sorted[lower]) * (position - lower)
  }
  return {
    flat: false,
    thresholds: Array.from({ length: levelCount - 1 }, (_, index) => quantile((index + 1) / levelCount)),
  }
}

export function heatScaleLevel(value, scale, levelCount = 5) {
  if (!Number.isFinite(value) || value <= 0) return 0
  if (scale?.flat) return Math.ceil(levelCount / 2)
  const thresholds = scale?.thresholds || []
  return Math.min(levelCount, 1 + thresholds.filter((threshold) => value > threshold).length)
}

export function calendarCells(days = []) {
  const sorted = [...days].sort((a, b) => a.date.localeCompare(b.date)).slice(-30)
  if (!sorted.length) return Array.from({ length: 35 }, () => null)

  const start = Date.parse(`${sorted[0].date}T00:00:00Z`)
  if (!Number.isFinite(start)) return Array.from({ length: 35 }, () => null)
  const leading = (new Date(start).getUTCDay() + 6) % 7
  const last = Date.parse(`${sorted.at(-1).date}T00:00:00Z`)
  const span = Number.isFinite(last) ? Math.round((last - start) / 86400000) + 1 : sorted.length
  const rowCount = Math.max(5, Math.ceil((leading + span) / 7))
  const cells = Array.from({ length: rowCount * 7 }, () => null)
  for (const day of sorted) {
    const date = Date.parse(`${day.date}T00:00:00Z`)
    if (!Number.isFinite(date)) continue
    const index = leading + Math.round((date - start) / 86400000)
    if (index >= 0 && index < cells.length) cells[index] = day
  }
  return cells
}

export function hours24(buckets = []) {
  return Array.from(
    { length: 24 },
    (_, index) => buckets.find((bucket) => bucket?.hourStart?.slice(11, 13) === String(index).padStart(2, '0')) || null,
  )
}

export function week168(days = []) {
  return [...days]
    .sort((a, b) => b.date.localeCompare(a.date))
    .slice(0, 7)
    .concat(Array(7).fill(null))
    .slice(0, 7)
    .map((day) => ({
      day,
      hours: Array.from({ length: 24 }, (_, hour) => day?.hours?.[hour] ?? null),
    }))
}

export function hourTooltip(bucket, hour) {
  if (!bucket) return `${String(hour).padStart(2, '0')}:00 数据缺失`
  return `${hourRange(hour)} · 输入 ${formatCompact(bucket.inputTokens)}，缓存输入 ${formatCompact(bucket.cachedInputTokens)}，输出 ${formatCompact(bucket.outputTokens)}，推理输出 ${formatCompact(bucket.reasoningOutputTokens)}，总计 ${formatCompact(bucket.totalTokens)} tokens，活跃任务 ${bucket.activeThreads}`
}

export function weekTooltip(day, hour, value) {
  const date = day?.date || '日期未知'
  return `${date} ${hourRange(hour)} · ${value === null ? '数据缺失' : `${formatCompact(value)} tokens`}`
}

export function dayTooltip(day) {
  return `${day.date} · ${formatCompact(day.totalTokens)} tokens`
}

export function reconcileSelection(selection, state) {
  if (!selection) return null

  if (selection.kind === 'hour') {
    const bucket = state?.heatmap24h?.buckets?.find((item) => item.hourStart === selection.key)
    if (!bucket) return null
    const hour = Number(bucket.hourStart.slice(11, 13))
    return { ...selection, hour, text: hourTooltip(bucket, hour) }
  }

  if (selection.kind === 'week') {
    const day = state?.heatmap7d?.days?.find((item) => item.date === selection.date)
    if (!day || selection.hour < 0 || selection.hour > 23) return null
    const value = day.hours?.[selection.hour] ?? null
    return { ...selection, key: `${day.date}-${selection.hour}`, text: weekTooltip(day, selection.hour, value) }
  }

  if (selection.kind === 'day') {
    const day = state?.trend30d?.days?.find((item) => item.date === selection.key)
    return day ? { ...selection, text: dayTooltip(day) } : null
  }

  return null
}

export function statusCopy(status, hasPartial) {
  if (hasPartial) return '部分数据不可用'
  return ({
    loading: '正在连接',
    online: '接口在线',
    reconnecting: '正在重连',
    stale: '数据可能已过期',
    offline: '本机服务未运行',
    invalid_credential: '悬浮球凭据无效',
    partial: '部分数据不可用',
  })[status] || '状态未知'
}

export function formatCompact(value) {
  if (value == null) return '—'
  const useCompactUnit = Math.abs(Number(value)) >= 1000
  return new Intl.NumberFormat('en-US', {
    notation: useCompactUnit ? 'compact' : 'standard',
    minimumFractionDigits: useCompactUnit ? 1 : 0,
    maximumFractionDigits: useCompactUnit ? 1 : 0,
  }).format(value)
}

function hourRange(hour) {
  const start = String(hour).padStart(2, '0')
  const end = String((hour + 1) % 24).padStart(2, '0')
  return `${start}:00–${end}:00`
}
