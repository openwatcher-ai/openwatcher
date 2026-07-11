<template><div class="quota-ring" :class="{muted: !windowData}" tabindex="0" :aria-label="`${label} 剩余额度`"><svg viewBox="0 0 120 120" aria-hidden="true"><circle class="ring-track" cx="60" cy="60" r="48"/><circle class="ring-value" :style="{stroke: color, strokeDasharray: `${value*3.02} 302`}" cx="60" cy="60" r="48"/></svg><div class="ring-center"><strong>{{ windowData ? `${Math.round(value)}%` : '—' }}</strong><span>{{ label }}</span></div></div><div class="ring-meta"><span>{{ windowData ? resetLabel : '暂无数据' }}</span><small>{{ status }}</small></div></template>
<script setup>
import { computed } from 'vue'
const props=defineProps({label:String,windowData:Object,color:String,status:{type:String,default:'正常'}})
const value=computed(()=>Math.max(0,Math.min(100,props.windowData?.remainingPercent||0)))
const resetLabel=computed(()=>{if(!props.windowData?.resetAt)return '重置时间未知';const mins=Math.max(0,Math.round((props.windowData.resetAt-Date.now()/1000)/60));return mins<60?`${mins} 分钟后重置`:`${Math.floor(mins/60)} 小时后重置`})
</script>
