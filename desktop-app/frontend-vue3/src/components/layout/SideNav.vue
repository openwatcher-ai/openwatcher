<script setup>
import { useAppStore } from "../../state/useAppStore.js"
import AppIcon from "../ui/Icon.vue"

const store = useAppStore()
</script>

<template>
  <aside class="sidebar">
    <nav class="nav-list">
      <button
        v-for="item in store.constants.navItems"
        :key="item.id"
        class="nav-item"
        :class="{ 'is-active': store.state.currentPage === item.id }"
        type="button"
        @click="store.actions.setPage(item.id)"
      >
        <AppIcon :name="item.icon" :size="17" />
        <span>{{ item.label }}</span>
      </button>
    </nav>
  </aside>
</template>
