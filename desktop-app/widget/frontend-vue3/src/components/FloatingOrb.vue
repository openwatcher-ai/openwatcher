<template><button data-ui-id="floating-orb" class="floating-orb" :class="{expanded}" @click="$emit('toggle')" @mouseenter="paused=true" @mouseleave="paused=false" aria-label="展开或收起用量概览"><QuotaRing :label="active.label" :window-data="active.data" :color="active.color" :status="''"/></button></template>
<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'; import QuotaRing from './QuotaRing.vue'
const props=defineProps({quota:Object,expanded:Boolean}); defineEmits(['toggle']); const index=ref(0),paused=ref(false); let timer
const items=computed(()=>[{label:'5h',data:props.quota?.fiveHour,color:'#ffad32'},{label:'7d',data:props.quota?.weekly,color:'#a975ff'}]); const active=computed(()=>items.value[index.value])
onMounted(()=>timer=setInterval(()=>{if(!paused.value)index.value=(index.value+1)%2},4000));onUnmounted(()=>clearInterval(timer))
</script>
