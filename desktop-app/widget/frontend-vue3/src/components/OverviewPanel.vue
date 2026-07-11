<template><section data-ui-id="overview-panel" class="overview-panel"><header data-ui-id="overview-header" class="overview-header"><h1 data-ui-id="overview-title">用量概览</h1><div class="header-actions"><span class="status-dot" :class="state.status">{{statusLabel}}</span><span class="updated">{{updated}}</span><button class="icon-button" :disabled="busy" @click="$emit('refresh')" title="刷新">↻</button><button class="icon-button" @click="$emit('close')" title="收起">×</button></div></header><div class="quadrants"><slot/></div></section></template>
<script setup>
defineProps({state:Object,statusLabel:String,busy:Boolean});defineEmits(['refresh','close']);const updated=new Date().toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'})
</script>
