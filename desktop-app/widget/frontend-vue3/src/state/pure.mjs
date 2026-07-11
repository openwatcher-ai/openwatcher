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
