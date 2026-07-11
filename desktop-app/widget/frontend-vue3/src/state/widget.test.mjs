import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildHeatScale,
  calendarCells,
  composition,
  dayTooltip,
  formatCompact,
  hourTooltip,
  hours24,
  heatScaleLevel,
  reconcileSelection,
  statusCopy,
  weekTooltip,
  week168,
} from './pure.mjs'

test('热力图保留零值档并将非零值分散到五个分位档', () => {
  const scale = buildHeatScale([0, 1, 2, 3, 4, 1000])
  assert.deepEqual([0, 1, 2, 3, 4, 1000].map((value) => heatScaleLevel(value, scale)), [0, 1, 2, 3, 4, 5])
  assert.equal(heatScaleLevel(10, buildHeatScale([10, 10, 10])), 3)
})

test('30 个日期按自然周逐行递增，必要时使用第六行', () => {
  for (let shift = 0; shift < 7; shift++) {
    const start = new Date(Date.UTC(2026, 5, 1 + shift))
    const days = Array.from({ length: 30 }, (_, index) => ({
      date: new Date(start.getTime() + index * 86400000).toISOString().slice(0, 10),
      totalTokens: index,
    }))
    const cells = calendarCells(days)
    const leading = (start.getUTCDay() + 6) % 7
    assert.equal(cells.length, Math.max(35, Math.ceil((leading + 30) / 7) * 7))
    assert.equal(cells.filter(Boolean).length, 30)
    assert.ok(cells.slice(0, leading).every((item) => item === null))
    assert.deepEqual(cells.slice(leading, leading + 30).map((item) => item?.date), days.map((item) => item.date))
  }
})

test('30 天日历为缺失日期保留原星期位置', () => {
  const cells = calendarCells([
    { date: '2026-06-11', totalTokens: 1 },
    { date: '2026-06-13', totalTokens: 3 },
  ])
  assert.equal(cells[3]?.date, '2026-06-11')
  assert.equal(cells[4], null)
  assert.equal(cells[5]?.date, '2026-06-13')
})

test('组成条不重复累计缓存输入，标签仍显示完整输入', () => {
  const parts = composition({
    inputTokens: 100,
    cachedInputTokens: 30,
    outputTokens: 20,
    reasoningOutputTokens: 10,
  })
  assert.deepEqual(parts.map((part) => part.segmentValue), [70, 30, 30])
  assert.deepEqual(parts.map((part) => part.value), [100, 30, 30])
})

test('小时柱恒为 24 个，按 hourStart 而不是数组位置', () => {
  const buckets = [
    { hourStart: '2026-07-11T23:00:00+08:00', totalTokens: 1 },
    { hourStart: '2026-07-11T00:00:00+08:00', totalTokens: 2 },
  ]
  const result = hours24(buckets)
  assert.equal(result.length, 24)
  assert.equal(result[0].totalTokens, 2)
  assert.equal(result[23].totalTokens, 1)
})

test('7 天热力图恒为最新日期在上的 7×24，缺失保留空位', () => {
  const rows = week168([
    { date: '2026-07-10', hours: [1] },
    { date: '2026-07-11', hours: [2] },
  ])
  assert.equal(rows.length, 7)
  assert.ok(rows.every((row) => row.hours.length === 24))
  assert.equal(rows[0].day.date, '2026-07-11')
  assert.equal(rows[0].hours[1], null)
  assert.equal(rows[6].hours[0], null)
})

test('固定提示按稳定键保留并刷新数值', () => {
  const state = {
    heatmap24h: {
      buckets: [{ hourStart: '2026-07-11T08:00:00+08:00', totalTokens: 9, inputTokens: 4, cachedInputTokens: 1, outputTokens: 3, reasoningOutputTokens: 2, activeThreads: 1 }],
    },
    heatmap7d: { days: [{ date: '2026-07-11', hours: [7] }] },
    trend30d: { days: [{ date: '2026-07-10', totalTokens: 11 }] },
  }
  assert.match(reconcileSelection({ kind: 'hour', key: '2026-07-11T08:00:00+08:00' }, state).text, /总计 9/)
  assert.match(reconcileSelection({ kind: 'week', key: 'old', date: '2026-07-11', hour: 0 }, state).text, /7 tokens/)
  assert.match(reconcileSelection({ kind: 'day', key: '2026-07-10' }, state).text, /11 tokens/)
  assert.equal(reconcileSelection({ kind: 'day', key: '2026-06-01' }, state), null)
})

test('状态中文文案覆盖连接与局部缺失', () => {
  assert.equal(statusCopy('invalid_credential', false), '悬浮球凭据无效')
  assert.equal(statusCopy('online', true), '部分数据不可用')
})

test('大数值沿用手表端紧凑单位', () => {
  assert.equal(formatCompact(14800000), '14.8M')
  assert.equal(formatCompact(13400000000), '13.4B')
  assert.equal(formatCompact(4000000), '4.0M')
})

test('热力方格提示统一使用紧凑 Token 单位', () => {
  assert.equal(hourTooltip({
    inputTokens: 14800000,
    cachedInputTokens: 13400000,
    outputTokens: 12500,
    reasoningOutputTokens: 4000,
    totalTokens: 14812500,
    activeThreads: 2,
  }, 3), '03:00–04:00 · 输入 14.8M，缓存输入 13.4M，输出 12.5K，推理输出 4.0K，总计 14.8M tokens，活跃任务 2')
  assert.equal(weekTooltip({ date: '2026-07-12' }, 3, 12500), '2026-07-12 03:00–04:00 · 12.5K tokens')
  assert.equal(dayTooltip({ date: '2026-07-12', totalTokens: 14800000 }), '2026-07-12 · 14.8M tokens')
  assert.equal(dayTooltip({ date: '2026-07-12', totalTokens: 13400000000 }), '2026-07-12 · 13.4B tokens')
})
