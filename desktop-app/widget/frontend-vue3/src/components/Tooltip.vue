<template><div v-if="visible" class="tooltip" role="tooltip"><slot/></div></template><script setup>defineProps({visible:Boolean})</script>
