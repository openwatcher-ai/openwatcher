import { computed, onMounted, onUnmounted, ref } from 'vue'
import { getState, refresh, toggle, collapse, onState } from '../services/wails.js'

export function useWidgetStore() {
  const state = ref({ status: 'loading' }); const busy = ref(false); const selected = ref(null); let off = () => {}
  const isExpanded = computed(() => state.value.expanded !== false)
  const statusLabel = computed(() => ({loading:'读取中',online:'已连接',reconnecting:'重连中',stale:'数据已过期',offline:'离线',invalid_credential:'凭据无效',partial:'部分可用'}[state.value.status] || '未知状态'))
  async function load() { busy.value=true; try { state.value=await refresh() } finally { busy.value=false } }
  function update(next) { const old=state.value; state.value=next; if(selected.value?.kind==='hour') { const found=next.heatmap24h?.buckets?.find(x=>x.hourStart===selected.value.key); if(!found) selected.value=null } if(selected.value?.kind==='day' && !next.trend30d?.days?.some(x=>x.date===selected.value.key)) selected.value=null; if(old.status!==next.status) selected.value=selected.value }
  onMounted(async()=>{ state.value=await getState(); off=onState(update) })
  onUnmounted(()=>off())
  return { state, busy, selected, isExpanded, statusLabel, load, update, toggle, collapse }
}

export function composition(today) {
  if (!today) return []
  const uncached=Math.max(0,today.inputTokens-today.cachedInputTokens), cached=Math.max(0,today.cachedInputTokens), output=Math.max(0,today.outputTokens+today.reasoningOutputTokens), total=uncached+cached+output
  return [{kind:'未缓存输入',value:uncached,fraction:total?uncached/total:0,color:'blue'},{kind:'缓存输入',value:cached,fraction:total?cached/total:0,color:'purple'},{kind:'输出',value:output,fraction:total?output/total:0,color:'green'}]
}

export function calendarCells(days=[]) {
  const cells=Array.from({length:35},()=>null); const counts=[0,0,0,0,0,0,0]
  for(const day of days.slice(0,30)) { const date=new Date(`${day.date}T00:00:00Z`); const col=(date.getUTCDay()+6)%7; const row=counts[col]++; if(row<5) cells[row*7+col]=day }
  return cells
}
