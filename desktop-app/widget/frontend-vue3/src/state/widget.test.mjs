import test from 'node:test'
import assert from 'node:assert/strict'
import {
  calendarCells,
  composition,
  formatCompact,
  hours24,
  reconcileSelection,
  statusCopy,
  week168,
} from './pure.mjs'

test('30 个日期按全部七种起始星期映射到 35 个槽', () => {
  for (let shift = 0; shift < 7; shift++) {
    const start = new Date(Date.UTC(2026, 5, 1 + shift))
    const days = Array.from({ length: 30 }, (_, index) => ({
      date: new Date(start.getTime() + index * 86400000).toISOString().slice(0, 10),
      totalTokens: index,
    }))
    const cells = calendarCells(days)
    assert.equal(cells.length, 35)
    assert.equal(cells.filter(Boolean).length, 30)
    assert.deepEqual(cells.filter(Boolean).map((item) => item.date).sort(), days.map((item) => item.date).sort())
  }
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
})
