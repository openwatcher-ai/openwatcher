<script setup>
import { computed, onMounted, watchEffect } from "vue"
import TopBar from "./components/layout/TopBar.vue"
import SideNav from "./components/layout/SideNav.vue"
import NotificationPanel from "./components/layout/NotificationPanel.vue"
import InstallWizard from "./pages/InstallWizard.vue"
import WatchDevice from "./pages/WatchDevice.vue"
import LogsDiagnostics from "./pages/LogsDiagnostics.vue"
import SettingsPage from "./pages/SettingsPage.vue"
import DeveloperUsageDialog from "./pages/DeveloperUsageDialog.vue"
import { useAppStore } from "./state/useAppStore.js"

const store = useAppStore()

const pageComponent = computed(() => {
  if (store.state.currentPage === "watch") {
    return WatchDevice
  }
  if (store.state.currentPage === "logs") {
    return LogsDiagnostics
  }
  if (store.state.currentPage === "settings") {
    return SettingsPage
  }
  return InstallWizard
})

onMounted(() => {
  void store.actions.bootstrap()
})

watchEffect(() => {
  document.documentElement.dataset.theme = store.state.theme
  document.documentElement.dataset.platform = store.state.snapshot?.system?.platform || ""
})
</script>

<template>
  <div class="desktop-shell">
    <TopBar />
    <div class="desktop-main">
      <SideNav />
      <main class="content-area" :class="{ 'is-install-page': store.state.currentPage === 'install' }">
        <component :is="pageComponent" />
      </main>
    </div>
    <NotificationPanel />
    <DeveloperUsageDialog />
  </div>
</template>
