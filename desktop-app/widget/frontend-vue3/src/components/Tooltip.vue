<template><div v-if="text" class="tooltip" role="tooltip">{{ text }}</div></template><script setup>defineProps({text:{type:String,default:''}})</script>
