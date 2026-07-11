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

export function calendarCells(days = []) {
  const cells = Array.from({ length: 35 }, () => null)
  const occurrence = Array(7).fill(0)
  const sorted = [...days].sort((a, b) => a.date.localeCompare(b.date)).slice(-30)

  for (const day of sorted) {
    const column = (new Date(`${day.date}T00:00:00Z`).getUTCDay() + 6) % 7
    const row = occurrence[column]++
    if (row < 5) cells[row * 7 + column] = day
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
  return `${hourRange(hour)} · 输入 ${bucket.inputTokens}，缓存输入 ${bucket.cachedInputTokens}，输出 ${bucket.outputTokens}，推理输出 ${bucket.reasoningOutputTokens}，总计 ${bucket.totalTokens} tokens，活跃任务 ${bucket.activeThreads}`
}

export function weekTooltip(day, hour, value) {
  const date = day?.date || '日期未知'
  return `${date} ${hourRange(hour)} · ${value === null ? '数据缺失' : `${value} tokens`}`
}

export function dayTooltip(day) {
  return `${day.date} · ${day.totalTokens} tokens`
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
  return new Intl.NumberFormat('en-US', {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(value)
}

function hourRange(hour) {
  const start = String(hour).padStart(2, '0')
  const end = String((hour + 1) % 24).padStart(2, '0')
  return `${start}:00–${end}:00`
}
