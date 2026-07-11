const fallback = {
  status: 'online', expanded: true, observedAt: new Date().toISOString(), timezone: 'Asia/Shanghai',
  quota: { planType: '团队', fresh: true, status: 'ok', fiveHour: { remainingPercent: 72, resetAt: Date.now()/1000+7200 }, weekly: { remainingPercent: 48, resetAt: Date.now()/1000+86400*3 } },
  heatmap24h: { timezone: 'Asia/Shanghai', buckets: Array.from({length:24}, (_,i)=>({hourStart:`2026-07-11T${String(i).padStart(2,'0')}:00:00+08:00`,inputTokens:i*1000,cachedInputTokens:i*300,outputTokens:i*600,reasoningOutputTokens:i*100,totalTokens:i*1700,activeThreads:i%4})) },
  heatmap7d: { timezone:'Asia/Shanghai', peakTokens:18000, days:Array.from({length:7},(_,d)=>({date:`2026-07-${String(11-d).padStart(2,'0')}`,totalTokens:24000-d*1300,hours:Array.from({length:24},(_,h)=>((h+d)%8)*1000)})) },
  today: { inputTokens:27600,cachedInputTokens:8200,outputTokens:12000,reasoningOutputTokens:2100,totalTokens:41700,valueLabel:'$0.84' },
  trend30d: { timezone:'Asia/Shanghai', startDate:'2026-06-11',endDate:'2026-07-10',totalTokens:620000,averageTokens:20667,peakTokens:48200,valueLabel:'$12.40',days:Array.from({length:30},(_,i)=>({date:`2026-06-${String(11+i).padStart(2,'0')}`,totalTokens:(i%9)*4100})) }
}
const isWails = () => typeof window !== 'undefined' && (window.go || window.runtime)
export async function getState() {
  if (isWails() && window.go?.main?.App?.State) return window.go.main.App.State()
  return import.meta.env.DEV ? fallback : { status: 'offline', errorText: '等待桌面 helper 连接' }
}
export async function refresh() { if (isWails() && window.go?.main?.App?.Refresh) return window.go.main.App.Refresh(); return getState() }
export function toggle() { if (isWails() && window.go?.main?.App?.Toggle) window.go.main.App.Toggle() }
export function collapse() { if (isWails() && window.go?.main?.App?.Collapse) window.go.main.App.Collapse() }
export function onState(callback) { if (window.runtime?.EventsOn) return window.runtime.EventsOn('widget:state', callback); return () => {} }
