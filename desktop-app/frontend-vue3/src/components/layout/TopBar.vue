<script setup>
import { computed } from "vue"
import launcherIconUrl from "../../assets/openwatcher_launcher.png"
import { toggleWindowMaximise } from "../../services/wails.js"
import { useAppStore } from "../../state/useAppStore.js"
import AppIcon from "../ui/Icon.vue"

const store = useAppStore()
const productVersionLabel = computed(() => store.state.snapshot.productVersion || "dev")

async function onDoubleClick(event) {
  if (event.target.closest("button, input, select, a")) {
    return
  }
  await toggleWindowMaximise()
}
</script>

<template>
  <header class="topbar" @dblclick="onDoubleClick">
    <div class="topbar-left">
      <div class="brand">
        <span class="brand-mark">
          <img :src="launcherIconUrl" alt="OpenWatcher" />
        </span>
        <span>OpenWatcher</span>
      </div>
      <span class="version-badge">{{ productVersionLabel }}</span>
    </div>

    <div class="topbar-center">
      <div v-for="item in store.topbarItems.value" :key="item.id" class="top-status-chip" :class="{ 'is-ok': item.ok }">
        <AppIcon :name="item.icon" :size="15" />
        <span>{{ item.label }}</span>
      </div>
    </div>

    <div class="topbar-right">
      <button
        class="icon-chip"
        :class="{ 'is-loading': store.state.globalHealthRunning }"
        type="button"
        title="立即刷新健康检查"
        :disabled="store.state.globalHealthRunning"
        @click="store.actions.runGlobalHealthCheck({ manual: true })"
      >
        <AppIcon name="RefreshCw" :size="16" :class="{ 'is-spinning': store.state.globalHealthRunning }" />
      </button>
      <button class="icon-chip has-badge" type="button" title="通知" @click="store.actions.toggleNotificationPanel">
        <AppIcon name="Bell" :size="16" />
        <span v-if="store.unreadNotificationCount.value > 0" class="icon-badge">
          {{ store.unreadNotificationCount.value > 99 ? "99+" : store.unreadNotificationCount.value }}
        </span>
      </button>
      <button
        class="icon-chip active"
        type="button"
        :title="store.state.theme === 'dark' ? '切换浅色主题' : '切换深色主题'"
        :aria-label="store.state.theme === 'dark' ? '切换浅色主题' : '切换深色主题'"
        @click="store.actions.toggleTheme"
      >
        <AppIcon :name="store.state.theme === 'dark' ? 'Sun' : 'Moon'" :size="16" />
      </button>
    </div>
  </header>
</template>
