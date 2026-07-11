import test from 'node:test'
import assert from 'node:assert/strict'
import { calendarCells, composition } from './pure.mjs'
test('30日按 weekday occurrence 映射到35槽',()=>{const days=Array.from({length:30},(_,i)=>({date:`2026-06-${String(i+1).padStart(2,'0')}`,totalTokens:i}));const cells=calendarCells(days);assert.equal(cells.length,35);assert.equal(cells.filter(Boolean).length,30)})
test('组成条不重复累计缓存输入',()=>{const x=composition({inputTokens:100,cachedInputTokens:30,outputTokens:20,reasoningOutputTokens:10});assert.deepEqual(x.map(v=>v.value),[70,30,30])})
